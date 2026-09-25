package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/itsmangooo/Silicon/backend/db"
	"github.com/itsmangooo/Silicon/backend/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testClient struct {
	t      *testing.T
	client *http.Client
	base   string
	csrf   string
}

func TestMilestoneOneFlowAndOrganizationIsolation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if poolConfig.ConnConfig.Database != "silicon_test" {
		t.Fatalf("integration tests refuse to reset database %q; expected silicon_test", poolConfig.ConnConfig.Database)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(ctx, pool); err != nil {
		t.Fatalf("fresh migration: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{DatabaseURL: databaseURL, CookieName: "silicon_test_session", SessionTTL: time.Hour, FrontendOrigin: "http://localhost:5173"}
	server := httptest.NewServer(New(cfg, pool, logger).Handler())
	defer server.Close()
	owner := newTestClient(t, server.URL)
	viewer := newTestClient(t, server.URL)
	owner.register("owner@example.com", "Owner User", "correct horse battery staple")
	viewer.register("viewer@example.com", "Viewer User", "another correct horse battery")

	organizationA := owner.post("/organizations", map[string]any{"name": "Organization A", "slug": "organization-a"}, http.StatusCreated)
	organizationB := viewer.post("/organizations", map[string]any{"name": "Organization B", "slug": "organization-b"}, http.StatusCreated)
	orgA := stringField(t, organizationA, "id")
	orgB := stringField(t, organizationB, "id")

	project := owner.post("/organizations/"+orgA+"/projects", map[string]any{"name": "Backend", "slug": "backend", "description": "API workloads"}, http.StatusCreated)
	projectID := stringField(t, project, "id")
	updatedProject := owner.put("/organizations/"+orgA+"/projects/"+projectID, map[string]any{"name": "Backend", "slug": "backend", "description": "Updated API workloads"}, http.StatusOK)
	if got := stringField(t, updatedProject, "description"); got != "Updated API workloads" {
		t.Fatalf("updated description=%q", got)
	}
	temporary := owner.post("/organizations/"+orgA+"/projects", map[string]any{"name": "Temporary", "slug": "temporary", "description": "Deletion coverage"}, http.StatusCreated)
	owner.delete("/organizations/"+orgA+"/projects/"+stringField(t, temporary, "id"), http.StatusNoContent)
	environment := owner.post("/organizations/"+orgA+"/projects/"+projectID+"/environments", map[string]any{"name": "production", "slug": "production"}, http.StatusCreated)
	environmentID := stringField(t, environment, "id")
	application := owner.post("/organizations/"+orgA+"/environments/"+environmentID+"/applications", map[string]any{"name": "api", "sourceType": "docker_image", "image": "example/api:1", "internalPort": 3000}, http.StatusCreated)
	applicationID := stringField(t, application, "id")
	if _, err := pool.Exec(ctx, `INSERT INTO domains(organization_id,environment_id,application_id,hostname,target_port) VALUES($1,$2,$3,'api.example.test',3000)`, orgA, environmentID, applicationID); err != nil {
		t.Fatalf("insert scoped domain: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO domains(organization_id,environment_id,application_id,hostname,target_port) VALUES($1,$2,$3,'cross.example.test',3000)`, orgB, environmentID, applicationID); err == nil {
		t.Fatal("cross-organization domain relation was accepted")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO secrets(organization_id,environment_id,application_id,name,encrypted_value) VALUES($1,$2,$3,'DATABASE_PASSWORD',$4)`, orgA, environmentID, applicationID, []byte("ciphertext-test-fixture")); err != nil {
		t.Fatalf("insert scoped secret: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO secrets(organization_id,environment_id,application_id,name,encrypted_value) VALUES($1,$2,$3,'CROSS_TENANT',$4)`, orgB, environmentID, applicationID, []byte("ciphertext-test-fixture")); err == nil {
		t.Fatal("cross-organization secret relation was accepted")
	}
	deployment := owner.post("/organizations/"+orgA+"/applications/"+applicationID+"/deployments", map[string]any{"source": "registry", "sourceRevision": "sha256:abc", "image": "example/api:1"}, http.StatusCreated)
	if got := stringField(t, deployment, "status"); got != "queued" {
		t.Fatalf("deployment status=%q want queued", got)
	}
	deploymentID := stringField(t, deployment, "id")
	owner.post("/organizations/"+orgA+"/deployments/"+deploymentID+"/transitions", map[string]any{"status": "preparing", "message": "integration test transition"}, http.StatusOK)
	owner.post("/organizations/"+orgA+"/deployments/"+deploymentID+"/transitions", map[string]any{"status": "healthy", "message": "invalid skip"}, http.StatusConflict)

	// An authenticated user outside Organization A receives a not-found response,
	// avoiding both IDOR access and resource enumeration.
	viewer.get("/organizations/"+orgA+"/projects", http.StatusNotFound)
	owner.get("/organizations/"+orgB+"/servers", http.StatusNotFound)
	viewer.get("/organizations/"+orgA+"/applications", http.StatusNotFound)
	viewer.get("/organizations/"+orgA+"/deployments", http.StatusNotFound)
	viewer.get("/organizations/"+orgA+"/audit-events", http.StatusNotFound)

	owner.post("/organizations/"+orgA+"/members", map[string]any{"email": "viewer@example.com", "role": "viewer"}, http.StatusCreated)
	viewer.post("/organizations/"+orgA+"/projects", map[string]any{"name": "Forbidden", "slug": "forbidden"}, http.StatusForbidden)
	viewer.get("/organizations/"+orgA+"/projects", http.StatusOK)

	audit := owner.get("/organizations/"+orgA+"/audit-events", http.StatusOK)
	if len(arrayField(t, audit, "auditEvents")) < 5 {
		t.Fatalf("expected audit events, got %v", audit)
	}

	owner.post("/auth/logout", map[string]any{}, http.StatusNoContent)
	owner.get("/auth/session", http.StatusUnauthorized)

	// Validation rejects malformed identifiers and unknown JSON fields.
	viewer.post("/organizations/"+orgB+"/projects", map[string]any{"name": "x", "slug": "bad slug", "unexpected": true}, http.StatusBadRequest)
}

func newTestClient(t *testing.T, base string) *testClient {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &testClient{t: t, client: &http.Client{Jar: jar}, base: base}
}

func (c *testClient) register(email, name, password string) {
	payload := c.post("/auth/register", map[string]any{"email": email, "displayName": name, "password": password}, http.StatusCreated)
	c.csrf = stringField(c.t, payload, "csrfToken")
}

func (c *testClient) get(path string, status int) map[string]any {
	return c.request(http.MethodGet, path, nil, status)
}
func (c *testClient) post(path string, body map[string]any, status int) map[string]any {
	return c.request(http.MethodPost, path, body, status)
}
func (c *testClient) put(path string, body map[string]any, status int) map[string]any {
	return c.request(http.MethodPut, path, body, status)
}
func (c *testClient) delete(path string, status int) map[string]any {
	return c.request(http.MethodDelete, path, nil, status)
}

func (c *testClient) request(method, path string, body map[string]any, status int) map[string]any {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, c.base+"/api/v1"+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if c.csrf != "" {
		request.Header.Set("X-CSRF-Token", c.csrf)
	}
	response, err := c.client.Do(request)
	if err != nil {
		c.t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != status {
		raw, _ := io.ReadAll(response.Body)
		c.t.Fatalf("%s %s status=%d want=%d body=%s", method, path, response.StatusCode, status, raw)
	}
	if status == http.StatusNoContent {
		return nil
	}
	var result map[string]any
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		c.t.Fatal(err)
	}
	return result
}

func stringField(t *testing.T, value map[string]any, key string) string {
	t.Helper()
	result, ok := value[key].(string)
	if !ok {
		t.Fatalf("field %q is not a string in %#v", key, value)
	}
	return result
}
func arrayField(t *testing.T, value map[string]any, key string) []any {
	t.Helper()
	result, ok := value[key].([]any)
	if !ok {
		t.Fatalf("field %q is not an array in %#v", key, value)
	}
	return result
}
