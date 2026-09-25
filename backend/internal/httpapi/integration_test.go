package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itsmangooo/Silicon/backend/db"
	"github.com/itsmangooo/Silicon/backend/internal/config"
	"github.com/itsmangooo/Silicon/backend/internal/deployments"
	"github.com/itsmangooo/Silicon/backend/internal/jobs"
	"github.com/jackc/pgx/v5"
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
	cloudflareServer, cloudflareState := newCloudflareServer(t)
	defer cloudflareServer.Close()
	cfg := config.Config{DatabaseURL: databaseURL, CookieName: "silicon_test_session", SessionTTL: time.Hour, FrontendOrigin: "http://localhost:5173", GitHubWebhookSecret: "webhook-test-secret", EncryptionKey: bytes.Repeat([]byte{5}, 32), CloudflareAPIURL: cloudflareServer.URL}
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

	// A verified GitHub push enters the same deployments/jobs pipeline with the
	// exact pushed revision. Delivery IDs are idempotency keys.
	var githubIntegrationID string
	if err := pool.QueryRow(ctx, `INSERT INTO github_integrations(organization_id,installation_id,account_login,connected_by) VALUES($1,123,'acme',NULL) RETURNING id`, orgA).Scan(&githubIntegrationID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO application_git_sources(application_id,organization_id,integration_id,repository_id,repository_full_name,branch,auto_deploy) VALUES($1,$2,$3,99,'acme/api','main',true)`, applicationID, orgA, githubIntegrationID); err != nil {
		t.Fatal(err)
	}
	sha := "1111111111111111111111111111111111111111"
	push := map[string]any{"ref": "refs/heads/main", "after": sha, "repository": map[string]any{"id": 99, "full_name": "acme/api"}, "installation": map[string]any{"id": 123}}
	if status := sendGitHubWebhook(t, server.URL, "webhook-test-secret", "delivery-1", push); status != http.StatusAccepted {
		t.Fatalf("valid webhook status=%d", status)
	}
	if status := sendGitHubWebhook(t, server.URL, "webhook-test-secret", "delivery-1", push); status != http.StatusOK {
		t.Fatalf("duplicate webhook status=%d", status)
	}
	if status := sendGitHubWebhook(t, server.URL, "wrong-secret", "delivery-invalid", push); status != http.StatusUnauthorized {
		t.Fatalf("invalid signature status=%d", status)
	}
	wrongBranch := map[string]any{"ref": "refs/heads/develop", "after": sha, "repository": map[string]any{"id": 99, "full_name": "acme/api"}, "installation": map[string]any{"id": 123}}
	if status := sendGitHubWebhook(t, server.URL, "webhook-test-secret", "delivery-wrong-branch", wrongBranch); status != http.StatusAccepted {
		t.Fatalf("wrong branch status=%d", status)
	}
	wrongRepo := map[string]any{"ref": "refs/heads/main", "after": sha, "repository": map[string]any{"id": 100, "full_name": "other/api"}, "installation": map[string]any{"id": 123}}
	if status := sendGitHubWebhook(t, server.URL, "webhook-test-secret", "delivery-wrong-repo", wrongRepo); status != http.StatusAccepted {
		t.Fatalf("wrong repo status=%d", status)
	}
	wrongInstallation := map[string]any{"ref": "refs/heads/main", "after": sha, "repository": map[string]any{"id": 99, "full_name": "acme/api"}, "installation": map[string]any{"id": 124}}
	if status := sendGitHubWebhook(t, server.URL, "webhook-test-secret", "delivery-wrong-installation", wrongInstallation); status != http.StatusAccepted {
		t.Fatalf("wrong installation status=%d", status)
	}
	if _, err := pool.Exec(ctx, `UPDATE application_git_sources SET auto_deploy=false WHERE application_id=$1`, applicationID); err != nil {
		t.Fatal(err)
	}
	if status := sendGitHubWebhook(t, server.URL, "webhook-test-secret", "delivery-disabled", push); status != http.StatusAccepted {
		t.Fatalf("disabled auto deploy status=%d", status)
	}
	if _, err := pool.Exec(ctx, `UPDATE application_git_sources SET auto_deploy=true WHERE application_id=$1`, applicationID); err != nil {
		t.Fatal(err)
	}
	githubView := owner.get("/organizations/"+orgA+"/integrations/github", http.StatusOK)
	encodedGitHub, _ := json.Marshal(githubView)
	if bytes.Contains(encodedGitHub, []byte("webhook-test-secret")) {
		t.Fatal("GitHub secret returned by API")
	}
	var githubCount int
	var storedSHA, trigger string
	if err := pool.QueryRow(ctx, `SELECT count(*),max(commit_sha),max(trigger_type) FROM deployments WHERE application_id=$1 AND trigger_type='github_push'`, applicationID).Scan(&githubCount, &storedSHA, &trigger); err != nil {
		t.Fatal(err)
	}
	if githubCount != 1 || storedSHA != sha || trigger != "github_push" {
		t.Fatalf("github deployments=%d sha=%q trigger=%q", githubCount, storedSHA, trigger)
	}
	var jobCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE job_type='deploy_application' AND payload->>'commitSha'=$1`, sha).Scan(&jobCount); err != nil || jobCount != 1 {
		t.Fatalf("pipeline jobs=%d err=%v", jobCount, err)
	}

	// Rapid deliveries remain one ordered deployment per delivery. The
	// application advisory lock makes numbering deterministic and collision-free.
	var wg sync.WaitGroup
	statuses := make(chan int, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rapid := map[string]any{"ref": "refs/heads/main", "after": strings.Repeat(fmt.Sprintf("%x", i+2), 40), "repository": map[string]any{"id": 99, "full_name": "acme/api"}, "installation": map[string]any{"id": 123}}
			statuses <- sendGitHubWebhook(t, server.URL, "webhook-test-secret", fmt.Sprintf("delivery-rapid-%d", i), rapid)
		}(i)
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != http.StatusAccepted {
			t.Fatalf("rapid webhook status=%d", status)
		}
	}
	var total, distinctNumbers int
	if err := pool.QueryRow(ctx, `SELECT count(*),count(DISTINCT number) FROM deployments WHERE application_id=$1`, applicationID).Scan(&total, &distinctNumbers); err != nil {
		t.Fatal(err)
	}
	if total != distinctNumbers {
		t.Fatalf("deployment numbering collided: total=%d distinct=%d", total, distinctNumbers)
	}
	var expectedSHA string
	if err := pool.QueryRow(ctx, `SELECT commit_sha FROM deployments WHERE application_id=$1 AND trigger_type='github_push' ORDER BY number DESC LIMIT 1`, applicationID).Scan(&expectedSHA); err != nil {
		t.Fatal(err)
	}
	executor := &captureExecutor{}
	runner := jobs.Runner{Pool: pool, Executor: executor, Logger: logger, WorkerID: "integration-test"}
	for {
		err := runner.RunOnce(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			break
		}
	}
	if executor.spec.CommitSHA != expectedSHA {
		t.Fatalf("executor commit=%q want exact newest %q", executor.spec.CommitSHA, expectedSHA)
	}
	var healthy, superseded int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE status='healthy'),count(*) FILTER(WHERE status='superseded') FROM deployments WHERE application_id=$1 AND trigger_type='github_push'`, applicationID).Scan(&healthy, &superseded); err != nil {
		t.Fatal(err)
	}
	if healthy != 1 || superseded != 6 {
		t.Fatalf("ordered results healthy=%d superseded=%d", healthy, superseded)
	}

	cloudflare := owner.post("/organizations/"+orgA+"/integrations/cloudflare", map[string]any{"accountId": "account-1", "apiToken": "scoped-secret-token"}, http.StatusCreated)
	if _, present := cloudflare["apiToken"]; present {
		t.Fatal("Cloudflare token returned by API")
	}
	var encryptedToken []byte
	if err := pool.QueryRow(ctx, `SELECT encrypted_api_token FROM cloudflare_integrations WHERE organization_id=$1`, orgA).Scan(&encryptedToken); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encryptedToken, []byte("scoped-secret-token")) {
		t.Fatal("Cloudflare token stored in plaintext")
	}
	zones := owner.get("/organizations/"+orgA+"/integrations/cloudflare/zones", http.StatusOK)
	if len(arrayField(t, zones, "zones")) != 1 {
		t.Fatalf("zones=%v", zones)
	}
	domain := owner.post("/organizations/"+orgA+"/applications/"+applicationID+"/domains", map[string]any{"hostname": "api.example.com", "targetPort": 3000, "recordType": "A", "content": "203.0.113.10", "proxied": true}, http.StatusCreated)
	if stringField(t, domain, "dnsState") != "active" {
		t.Fatalf("domain=%v", domain)
	}
	domainID := stringField(t, domain, "id")
	owner.post("/organizations/"+orgA+"/domains/"+domainID+"/sync", map[string]any{}, http.StatusOK)
	tunnel := owner.post("/organizations/"+orgA+"/integrations/cloudflare/tunnels", map[string]any{"name": "private-network"}, http.StatusCreated)
	tunnelID := stringField(t, tunnel, "id")
	owner.post("/organizations/"+orgA+"/integrations/cloudflare/tunnels/"+tunnelID+"/routes", map[string]any{"domainId": domainID, "serviceUrl": "http://api:3000", "proxied": true}, http.StatusCreated)
	if cloudflareState.routes.Load() != 1 {
		t.Fatalf("tunnel route calls=%d", cloudflareState.routes.Load())
	}
	external := owner.post("/organizations/"+orgA+"/integrations/cloudflare/tunnels", map[string]any{"name": "shared", "providerTunnelId": "shared-1", "ownership": "external"}, http.StatusCreated)
	owner.post("/organizations/"+orgA+"/integrations/cloudflare/tunnels/"+stringField(t, external, "id")+"/routes", map[string]any{"domainId": domainID, "serviceUrl": "http://api:3000", "proxied": true}, http.StatusConflict)
	owner.delete("/organizations/"+orgA+"/domains/"+domainID, http.StatusNoContent)
	if cloudflareState.deletes.Load() != 1 {
		t.Fatalf("managed DNS deletes=%d", cloudflareState.deletes.Load())
	}
	owner.post("/organizations/"+orgA+"/applications/"+applicationID+"/domains", map[string]any{"hostname": "conflict.example.com", "targetPort": 3000, "recordType": "A", "content": "203.0.113.20", "proxied": false}, http.StatusConflict)
	domainsResult := owner.get("/organizations/"+orgA+"/domains", http.StatusOK)
	var conflictID string
	for _, raw := range arrayField(t, domainsResult, "domains") {
		item := raw.(map[string]any)
		if item["hostname"] == "conflict.example.com" {
			conflictID = stringField(t, item, "id")
			if item["dnsState"] != "conflict" {
				t.Fatalf("conflict domain=%v", item)
			}
		}
	}
	if conflictID == "" {
		t.Fatal("conflict domain not persisted")
	}
	owner.delete("/organizations/"+orgA+"/domains/"+conflictID, http.StatusNoContent)
	if cloudflareState.deletes.Load() != 1 {
		t.Fatal("external DNS record was deleted")
	}
	viewer.get("/organizations/"+orgA+"/integrations/cloudflare", http.StatusNotFound)
	viewer.get("/organizations/"+orgA+"/integrations/github", http.StatusNotFound)

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

type captureExecutor struct{ spec jobs.DeploymentSpec }

func (e *captureExecutor) Execute(_ context.Context, spec jobs.DeploymentSpec, progress func(deployments.State, string) error) error {
	e.spec = spec
	for _, state := range []deployments.State{deployments.Building, deployments.Deploying, deployments.Starting, deployments.Healthy} {
		if err := progress(state, "integration executor"); err != nil {
			return err
		}
	}
	return nil
}

type cloudflareTestState struct {
	deletes atomic.Int32
	routes  atomic.Int32
	created atomic.Bool
}

func newCloudflareServer(t *testing.T) (*httptest.Server, *cloudflareTestState) {
	t.Helper()
	state := &cloudflareTestState{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer scoped-secret-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/user/tokens/verify":
			io.WriteString(w, `{"success":true,"result":{"status":"active"}}`)
		case r.URL.Path == "/zones":
			io.WriteString(w, `{"success":true,"result":[{"id":"zone-1","name":"example.com","status":"active"}],"result_info":{"page":1,"total_pages":1}}`)
		case r.URL.Path == "/zones/zone-1/dns_records" && r.Method == http.MethodGet:
			if r.URL.Query().Get("name") == "conflict.example.com" {
				io.WriteString(w, `{"success":true,"result":[{"id":"external-1","type":"A","name":"conflict.example.com","content":"198.51.100.2","proxied":false}]}`)
			} else if state.created.Load() {
				io.WriteString(w, `{"success":true,"result":[{"id":"record-1","type":"A","name":"api.example.com","content":"203.0.113.10","proxied":true}]}`)
			} else {
				io.WriteString(w, `{"success":true,"result":[]}`)
			}
		case r.URL.Path == "/zones/zone-1/dns_records" && r.Method == http.MethodPost:
			state.created.Store(true)
			var desired map[string]any
			_ = json.NewDecoder(r.Body).Decode(&desired)
			desired["id"] = "record-1"
			writeEnvelope(t, w, desired)
		case r.URL.Path == "/zones/zone-1/dns_records/record-1" && r.Method == http.MethodPut:
			var desired map[string]any
			_ = json.NewDecoder(r.Body).Decode(&desired)
			desired["id"] = "record-1"
			writeEnvelope(t, w, desired)
		case r.URL.Path == "/zones/zone-1/dns_records/record-1" && r.Method == http.MethodDelete:
			state.deletes.Add(1)
			state.created.Store(false)
			io.WriteString(w, `{"success":true,"result":{"id":"record-1"}}`)
		case r.URL.Path == "/accounts/account-1/cfd_tunnel" && r.Method == http.MethodPost:
			io.WriteString(w, `{"success":true,"result":{"id":"tunnel-1","name":"private-network","status":"inactive"}}`)
		case r.URL.Path == "/accounts/account-1/cfd_tunnel/tunnel-1/token":
			io.WriteString(w, `{"success":true,"result":"cloudflared-secret-token"}`)
		case r.URL.Path == "/accounts/account-1/cfd_tunnel" && r.Method == http.MethodGet:
			io.WriteString(w, `{"success":true,"result":[{"id":"shared-1","name":"shared","status":"healthy"}]}`)
		case r.URL.Path == "/accounts/account-1/cfd_tunnel/tunnel-1/configurations" && r.Method == http.MethodGet:
			io.WriteString(w, `{"success":true,"result":{"config":{"ingress":[{"hostname":"existing.example.com","service":"http://existing:8080"},{"service":"http_status:404"}]}}}`)
		case r.URL.Path == "/accounts/account-1/cfd_tunnel/tunnel-1/configurations" && r.Method == http.MethodPut:
			state.routes.Add(1)
			io.WriteString(w, `{"success":true,"result":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return server, state
}

func writeEnvelope(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result}); err != nil {
		t.Fatal(err)
	}
}

func sendGitHubWebhook(t *testing.T, base, secret, delivery string, payload map[string]any) int {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/webhooks/github", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", delivery)
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return response.StatusCode
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
