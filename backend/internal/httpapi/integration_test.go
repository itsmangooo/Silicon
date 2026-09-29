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

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/db"
	"github.com/itsmangooo/Silicon/backend/internal/config"
	"github.com/itsmangooo/Silicon/backend/internal/deployments"
	"github.com/itsmangooo/Silicon/backend/internal/execution"
	"github.com/itsmangooo/Silicon/backend/internal/jobs"
	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	"github.com/itsmangooo/Silicon/backend/internal/updates"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testClient struct {
	t      *testing.T
	client *http.Client
	base   string
	csrf   string
}

type fakeRuntime struct {
	mu        sync.Mutex
	spec      runtimeprovider.DeploymentSpec
	instances map[string]runtimeprovider.InstanceStatus
	actions   []string
	next      int
}

type fakeConnectionProvider struct{ installations atomic.Int32 }

type fakeAWSFactory struct{ provider *fakeAWSProvider }

type fakeReleaseSource struct{}

func (fakeReleaseSource) LatestStable(context.Context) (updates.Release, error) {
	return updates.Release{TagName: "v0.4.2", Name: "Silicon v0.4.2", Notes: "A safe update."}, nil
}

func (fakeReleaseSource) Release(_ context.Context, tag string) (updates.Release, error) {
	if tag != "v0.4.2" {
		return updates.Release{}, updates.ErrNoRelease
	}
	return updates.Release{TagName: tag, Name: "Silicon v0.4.2", Notes: "A safe update."}, nil
}

func (f fakeAWSFactory) Open(_ context.Context, input cloudaws.AccountConfig) (cloudaws.Provider, error) {
	if strings.Contains(input.RoleARN, "Denied") {
		return nil, errors.New("assume role: AWS denied the required permission")
	}
	return f.provider, nil
}

type fakeAWSProvider struct {
	cloudaws.Provider
	mu      sync.Mutex
	actions []string
}

func (*fakeAWSProvider) Identity(context.Context) (cloudaws.Identity, error) {
	return cloudaws.Identity{AccountID: "123456789012", ARN: "arn:aws:sts::123456789012:assumed-role/Silicon/test", UserID: "test"}, nil
}
func (*fakeAWSProvider) Regions(context.Context) ([]string, error) {
	return []string{"eu-central-1", "eu-west-1"}, nil
}
func (*fakeAWSProvider) Instance(_ context.Context, id string) (cloudaws.Instance, error) {
	return cloudaws.Instance{ID: id, Name: "existing-ec2", State: "running", InstanceType: "t3.small", Architecture: "x86_64", Region: "eu-central-1", AvailabilityZone: "eu-central-1a", ImageID: "ami-123", PrivateIP: "10.20.1.10", PublicIP: "203.0.113.30", Ownership: cloudaws.OwnershipExternal}, nil
}
func (f *fakeAWSProvider) StartInstance(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.actions = append(f.actions, "start:"+id)
	return nil
}

func (*fakeConnectionProvider) Check(context.Context, connection.Config) (connection.Status, error) {
	return connection.Status{Reachable: true, DockerAvailable: true, DockerVersion: "28.0.1", OperatingSystem: "linux", Architecture: "amd64", CheckedAt: time.Now().UTC()}, nil
}
func (*fakeConnectionProvider) Executor(context.Context, connection.Config) (connection.CommandExecutor, error) {
	return nil, errors.New("executor is not used by this integration test")
}
func (f *fakeConnectionProvider) InstallTunnel(_ context.Context, _ connection.Config, installation connection.TunnelInstallation) error {
	if installation.Name == "" || len(installation.Token) == 0 {
		return errors.New("missing tunnel installation data")
	}
	f.installations.Add(1)
	return nil
}

func (f *fakeRuntime) Deploy(_ context.Context, spec runtimeprovider.DeploymentSpec) (runtimeprovider.InstanceStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	copiedEnvironment := make(map[string]string, len(spec.Environment))
	for name, value := range spec.Environment {
		copiedEnvironment[name] = value
	}
	spec.Environment = copiedEnvironment
	f.spec = spec
	f.next++
	now := time.Now().UTC()
	status := runtimeprovider.InstanceStatus{InstanceID: fmt.Sprintf("managed-container-%d", f.next), Image: spec.Image, State: "running", Health: "running", Healthy: true, HostAddress: spec.HostAddress, HostPort: spec.HostPort, CreatedAt: &now, StartedAt: &now}
	if f.instances == nil {
		f.instances = map[string]runtimeprovider.InstanceStatus{}
	}
	f.instances[status.InstanceID] = status
	return status, nil
}
func (f *fakeRuntime) Start(_ context.Context, id string) error {
	f.record("start", id, "running", "running")
	return nil
}
func (f *fakeRuntime) Stop(_ context.Context, id string) error {
	f.record("stop", id, "exited", "exited")
	return nil
}
func (f *fakeRuntime) Restart(_ context.Context, id string) error {
	f.record("restart", id, "running", "running")
	return nil
}
func (f *fakeRuntime) Remove(_ context.Context, id string) error {
	f.record("remove", id, "removed", "unknown")
	return nil
}
func (f *fakeRuntime) Inspect(_ context.Context, id string) (runtimeprovider.InstanceStatus, error) {
	return f.current(id)
}
func (f *fakeRuntime) Status(_ context.Context, id string) (runtimeprovider.InstanceStatus, error) {
	return f.current(id)
}
func (f *fakeRuntime) Logs(context.Context, string, runtimeprovider.LogRequest) (<-chan runtimeprovider.LogLine, error) {
	result := make(chan runtimeprovider.LogLine, 2)
	result <- runtimeprovider.LogLine{Timestamp: time.Now().UTC(), Stream: "stdout", Message: "runtime output <script>is text</script>"}
	result <- runtimeprovider.LogLine{Timestamp: time.Now().UTC(), Stream: "stderr", Message: "diagnostic"}
	close(result)
	return result, nil
}
func (f *fakeRuntime) record(action, id, state, health string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.actions = append(f.actions, action+":"+id)
	status := f.instances[id]
	status.State, status.Health, status.Healthy = state, health, state == "running"
	f.instances[id] = status
}
func (f *fakeRuntime) current(id string) (runtimeprovider.InstanceStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, ok := f.instances[id]
	if !ok {
		return runtimeprovider.InstanceStatus{}, errors.New("not found")
	}
	return status, nil
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
	cfg := config.Config{DatabaseURL: databaseURL, CookieName: "silicon_test_session", SessionTTL: time.Hour, FrontendOrigin: "http://localhost:5173", PublicURL: "http://silicon.test", GitHubWebhookSecret: "webhook-test-secret", EncryptionKey: bytes.Repeat([]byte{5}, 32), CloudflareAPIURL: cloudflareServer.URL, RuntimeLogFollowTimeout: time.Minute, LocalDockerEnabled: true}
	dockerRuntime := &fakeRuntime{}
	apiServer := NewWithProviders(cfg, pool, logger, dockerRuntime, nil)
	apiServer.SetUpdateChecker(&updates.Checker{Source: fakeReleaseSource{}, CurrentVersion: "v0.4.1", CommitSHA: "test-commit", CacheTTL: time.Hour})
	fakeAWS := &fakeAWSProvider{}
	apiServer.SetAWSFactory(fakeAWSFactory{provider: fakeAWS})
	connections := &fakeConnectionProvider{}
	apiServer.connections.Local = connections
	server := httptest.NewServer(apiServer.Handler())
	defer server.Close()
	owner := newTestClient(t, server.URL)
	viewer := newTestClient(t, server.URL)
	owner.register("owner@example.com", "Owner User", "correct horse battery staple")
	viewer.register("viewer@example.com", "Viewer User", "another correct horse battery")
	ownerSession := mapField(t, owner.get("/auth/session", http.StatusOK), "user")
	viewerSession := mapField(t, viewer.get("/auth/session", http.StatusOK), "user")
	if ownerSession["isSystemAdmin"] != true || viewerSession["isSystemAdmin"] != false {
		t.Fatalf("unexpected bootstrap system administration: owner=%#v viewer=%#v", ownerSession, viewerSession)
	}
	viewer.post("/system/updates/check", map[string]any{}, http.StatusForbidden)
	update := owner.post("/system/updates", map[string]any{"targetVersion": "v0.4.2"}, http.StatusAccepted)
	if update["targetVersion"] != "v0.4.2" || update["status"] != "queued" {
		t.Fatalf("unexpected persisted update operation: %#v", update)
	}
	status := owner.get("/system/updates", http.StatusOK)
	if mapField(t, status, "operation")["id"] != update["id"] {
		t.Fatalf("latest persisted update was not returned: %#v", status)
	}

	organizationA := owner.post("/organizations", map[string]any{"name": "Organization A", "slug": "organization-a"}, http.StatusCreated)
	organizationB := viewer.post("/organizations", map[string]any{"name": "Organization B", "slug": "organization-b"}, http.StatusCreated)
	orgA := stringField(t, organizationA, "id")
	orgB := stringField(t, organizationB, "id")

	// AWS credentials are verified through the provider, encrypted at rest, and
	// scoped to the owning organization. Discovered instances remain read-only
	// until the user explicitly imports them.
	owner.post("/organizations/"+orgA+"/aws/accounts", map[string]any{"displayName": "Denied account", "accountId": "123456789012", "roleArn": "arn:aws:iam::123456789012:role/Denied", "externalId": "must-not-leak", "defaultRegion": "eu-central-1", "enabledRegions": []string{"eu-central-1"}}, http.StatusBadGateway)
	awsAccount := owner.post("/organizations/"+orgA+"/aws/accounts", map[string]any{"displayName": "Production AWS", "accountId": "123456789012", "roleArn": "arn:aws:iam::123456789012:role/Silicon", "externalId": "external-id-secret", "accessKeyId": "AKIA_TEST_SECRET", "secretAccessKey": "bootstrap-secret", "defaultRegion": "eu-central-1", "enabledRegions": []string{"eu-central-1", "eu-west-1"}}, http.StatusCreated)
	awsAccountID := stringField(t, awsAccount, "id")
	if awsAccount["externalIdConfigured"] != true || awsAccount["staticKeysConfigured"] != true || awsAccount["externalId"] != nil || awsAccount["accessKeyId"] != nil {
		t.Fatalf("AWS account response exposed or omitted credential state: %#v", awsAccount)
	}
	var encryptedExternalID, encryptedAccessKeyID, encryptedAWSSecret []byte
	if err = pool.QueryRow(ctx, `SELECT encrypted_external_id, encrypted_access_key_id, encrypted_secret_access_key FROM aws_accounts WHERE id=$1`, awsAccountID).Scan(&encryptedExternalID, &encryptedAccessKeyID, &encryptedAWSSecret); err != nil {
		t.Fatal(err)
	}
	for _, ciphertext := range [][]byte{encryptedExternalID, encryptedAccessKeyID, encryptedAWSSecret} {
		if len(ciphertext) == 0 || bytes.Contains(ciphertext, []byte("secret")) || bytes.Contains(ciphertext, []byte("AKIA")) {
			t.Fatalf("AWS credential was not envelope encrypted: %q", ciphertext)
		}
	}
	viewer.get("/organizations/"+orgB+"/aws/accounts/"+awsAccountID+"/regions", http.StatusNotFound)
	instanceID := "i-0123456789abcdef0"
	owner.post("/organizations/"+orgA+"/aws/accounts/"+awsAccountID+"/instances/"+instanceID+"/actions/start?region=eu-central-1", map[string]any{}, http.StatusConflict)
	imported := owner.post("/organizations/"+orgA+"/aws/accounts/"+awsAccountID+"/instances/"+instanceID+"/actions/import?region=eu-central-1", map[string]any{"connectionMethod": "aws_ssm"}, http.StatusCreated)
	if imported["ownership"] != "imported" || imported["connectionMethod"] != "aws_ssm" {
		t.Fatalf("unexpected imported AWS instance: %#v", imported)
	}
	owner.post("/organizations/"+orgA+"/aws/accounts/"+awsAccountID+"/instances/"+instanceID+"/actions/start?region=eu-central-1", map[string]any{}, http.StatusNoContent)
	fakeAWS.mu.Lock()
	if len(fakeAWS.actions) != 1 || fakeAWS.actions[0] != "start:"+instanceID {
		t.Fatalf("unexpected AWS lifecycle calls: %#v", fakeAWS.actions)
	}
	fakeAWS.mu.Unlock()

	serverItem := owner.post("/organizations/"+orgA+"/servers", map[string]any{"name": "edge-a", "connectionType": "local", "connectivityType": "private", "publicAddress": "203.0.113.10"}, http.StatusCreated)
	serverID := stringField(t, serverItem, "id")
	checkedServer := owner.post("/organizations/"+orgA+"/servers/"+serverID+"/check", map[string]any{}, http.StatusOK)
	checkedServer = mapField(t, checkedServer, "server")
	if checkedServer["connectionStatus"] != "connected" || checkedServer["dockerVersion"] != "28.0.1" {
		t.Fatalf("active connection check not reflected: %#v", checkedServer)
	}
	serverB := viewer.post("/organizations/"+orgB+"/servers", map[string]any{"name": "edge-b", "connectionType": "local", "connectivityType": "public", "publicAddress": "198.51.100.10"}, http.StatusCreated)
	updatedServer := owner.put("/organizations/"+orgA+"/servers/"+serverID+"/connection", map[string]any{"connectionType": "local", "host": "localhost", "publicAddress": "203.0.113.11"}, http.StatusOK)
	if updatedServer["publicAddress"] != "203.0.113.11" || updatedServer["credentialConfigured"] != false {
		t.Fatalf("server connection update not reflected: %#v", updatedServer)
	}
	viewer.put("/organizations/"+orgB+"/servers/"+serverID+"/connection", map[string]any{"connectionType": "local", "host": "localhost"}, http.StatusNotFound)

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
	owner.put("/organizations/"+orgA+"/applications/"+applicationID+"/server", map[string]any{"serverId": serverID}, http.StatusOK)
	viewer.put("/organizations/"+orgB+"/applications/"+applicationID+"/server", map[string]any{"serverId": stringField(t, serverB, "id")}, http.StatusNotFound)
	if _, err := pool.Exec(ctx, `INSERT INTO domains(organization_id,environment_id,application_id,hostname,target_port) VALUES($1,$2,$3,'api.example.test',3000)`, orgA, environmentID, applicationID); err != nil {
		t.Fatalf("insert scoped domain: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO domains(organization_id,environment_id,application_id,hostname,target_port) VALUES($1,$2,$3,'cross.example.test',3000)`, orgB, environmentID, applicationID); err == nil {
		t.Fatal("cross-organization domain relation was accepted")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO secrets(organization_id,project_id,environment_id,application_id,name,encrypted_value) VALUES($1,$2,$3,$4,'DATABASE_PASSWORD',$5)`, orgA, projectID, environmentID, applicationID, []byte("ciphertext-test-fixture")); err != nil {
		t.Fatalf("insert scoped secret: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO secrets(organization_id,project_id,environment_id,application_id,name,encrypted_value) VALUES($1,$2,$3,$4,'CROSS_TENANT',$5)`, orgB, projectID, environmentID, applicationID, []byte("ciphertext-test-fixture")); err == nil {
		t.Fatal("cross-organization secret relation was accepted")
	}
	deployment := owner.post("/organizations/"+orgA+"/applications/"+applicationID+"/deployments", map[string]any{"source": "registry", "sourceRevision": "sha256:abc", "image": "example/api:1"}, http.StatusCreated)
	if got := stringField(t, deployment, "status"); got != "queued" {
		t.Fatalf("deployment status=%q want queued", got)
	}
	deploymentID := stringField(t, deployment, "id")
	owner.post("/organizations/"+orgA+"/deployments/"+deploymentID+"/transitions", map[string]any{"status": "preparing", "message": "integration test transition"}, http.StatusOK)
	owner.post("/organizations/"+orgA+"/deployments/"+deploymentID+"/transitions", map[string]any{"status": "healthy", "message": "invalid skip"}, http.StatusConflict)
	if detail := owner.get("/organizations/"+orgA+"/deployments/"+deploymentID, http.StatusOK); len(arrayField(t, detail, "events")) != 2 {
		t.Fatalf("deployment detail events missing: %#v", detail)
	}
	owner.get("/organizations/"+orgA+"/environments/"+environmentID, http.StatusOK)
	owner.get("/organizations/"+orgA+"/applications/"+applicationID, http.StatusOK)
	owner.put("/organizations/"+orgA+"/environments/"+environmentID, map[string]any{"name": "Production", "slug": "production"}, http.StatusOK)
	owner.put("/organizations/"+orgA+"/applications/"+applicationID, map[string]any{"name": "api", "sourceType": "docker_image", "image": "example/api:1", "internalPort": 3000, "serverId": serverID}, http.StatusOK)
	temporaryEnvironment := owner.post("/organizations/"+orgA+"/projects/"+projectID+"/environments", map[string]any{"name": "Temporary", "slug": "temporary"}, http.StatusCreated)
	temporaryEnvironmentID := stringField(t, temporaryEnvironment, "id")
	temporaryApplication := owner.post("/organizations/"+orgA+"/environments/"+temporaryEnvironmentID+"/applications", map[string]any{"name": "temporary", "sourceType": "docker_image", "image": "busybox:latest"}, http.StatusCreated)
	owner.delete("/organizations/"+orgA+"/environments/"+temporaryEnvironmentID, http.StatusConflict)
	owner.delete("/organizations/"+orgA+"/applications/"+stringField(t, temporaryApplication, "id"), http.StatusNoContent)
	owner.delete("/organizations/"+orgA+"/environments/"+temporaryEnvironmentID, http.StatusNoContent)

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
	var deliveryOrganizationID string
	if err := pool.QueryRow(ctx, `SELECT organization_id FROM github_webhook_deliveries WHERE delivery_id='delivery-1'`).Scan(&deliveryOrganizationID); err != nil || deliveryOrganizationID != orgA {
		t.Fatalf("GitHub delivery tenant=%q want %q err=%v", deliveryOrganizationID, orgA, err)
	}
	var unknownInstallationDeliveries int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM github_webhook_deliveries WHERE delivery_id='delivery-wrong-installation'`).Scan(&unknownInstallationDeliveries); err != nil || unknownInstallationDeliveries != 0 {
		t.Fatalf("unknown GitHub installation journal rows=%d err=%v", unknownInstallationDeliveries, err)
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

	// Docker image deployments use the same job runner and RuntimeProvider as
	// GitHub builds. Configuration and encrypted secrets are resolved only for
	// the scoped application at execution time.
	runtimeApplication := owner.post("/organizations/"+orgA+"/environments/"+environmentID+"/applications", map[string]any{"name": "web", "sourceType": "docker_image", "image": "nginx:alpine", "internalPort": 80, "hostAddress": "127.0.0.1", "publishedPort": 32781}, http.StatusCreated)
	runtimeApplicationID := stringField(t, runtimeApplication, "id")
	owner.put("/organizations/"+orgA+"/projects/"+projectID+"/environment-variables", map[string]any{"variables": []map[string]string{{"name": "APP_MODE", "value": "project"}, {"name": "PROJECT_ONLY", "value": "project-value"}, {"name": "PROJECT_DELETE_ME", "value": "temporary"}, {"name": "SHARED", "value": "project"}}}, http.StatusOK)
	owner.put("/organizations/"+orgA+"/environments/"+environmentID+"/environment-variables", map[string]any{"variables": []map[string]string{{"name": "APP_MODE", "value": "environment"}, {"name": "ENVIRONMENT_ONLY", "value": "environment-value"}, {"name": "ENVIRONMENT_DELETE_ME", "value": "temporary"}, {"name": "SHARED", "value": "environment"}}}, http.StatusOK)
	owner.put("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/environment-variables", map[string]any{"variables": []map[string]string{{"name": "APP_MODE", "value": "production"}, {"name": "SHARED", "value": "application"}}}, http.StatusOK)
	preview := owner.post("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/environment-variables/parse", map[string]any{"text": "PUBLIC_URL=https://example.test\nDATABASE_PASSWORD=do-not-return"}, http.StatusOK)
	parsed := arrayField(t, preview, "variables")
	if len(parsed) != 2 || parsed[0].(map[string]any)["secretSuggested"] != true && parsed[1].(map[string]any)["secretSuggested"] != true {
		t.Fatalf("dotenv secret suggestion missing: %#v", preview)
	}
	owner.put("/organizations/"+orgA+"/projects/"+projectID+"/secrets/PROJECT_SECRET", map[string]any{"value": "project-secret-value"}, http.StatusOK)
	owner.put("/organizations/"+orgA+"/environments/"+environmentID+"/secrets/ENVIRONMENT_SECRET", map[string]any{"value": "environment-secret-value"}, http.StatusOK)
	owner.put("/organizations/"+orgA+"/projects/"+projectID+"/secrets/API_TOKEN", map[string]any{"value": "project-api-token"}, http.StatusOK)
	owner.put("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/secrets/API_TOKEN", map[string]any{"value": "runtime-secret-value"}, http.StatusOK)
	effective := owner.get("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/configuration", http.StatusOK)
	effectiveVariables := arrayField(t, effective, "variables")
	resolvedVariables := map[string]map[string]any{}
	for _, raw := range effectiveVariables {
		item := raw.(map[string]any)
		resolvedVariables[stringField(t, item, "name")] = item
	}
	if resolvedVariables["APP_MODE"]["value"] != "production" || resolvedVariables["APP_MODE"]["scope"] != "application" || resolvedVariables["SHARED"]["value"] != "application" || resolvedVariables["PROJECT_ONLY"]["scope"] != "project" || resolvedVariables["ENVIRONMENT_ONLY"]["scope"] != "environment" {
		t.Fatalf("effective variable precedence incorrect: %#v", effectiveVariables)
	}
	effectiveSecrets := arrayField(t, effective, "secrets")
	encodedEffectiveSecrets, _ := json.Marshal(effectiveSecrets)
	if bytes.Contains(encodedEffectiveSecrets, []byte("secret-value")) || len(effectiveSecrets) != 3 {
		t.Fatalf("effective secret metadata leaked plaintext or omitted inherited names: %s", encodedEffectiveSecrets)
	}
	secretView := owner.get("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/secrets", http.StatusOK)
	encodedSecrets, _ := json.Marshal(secretView)
	if bytes.Contains(encodedSecrets, []byte("runtime-secret-value")) || len(arrayField(t, secretView, "secrets")) != 1 {
		t.Fatalf("secret API exposed plaintext or metadata is missing: %s", encodedSecrets)
	}
	var encryptedSecret []byte
	if err := pool.QueryRow(ctx, `SELECT encrypted_value FROM secrets WHERE organization_id=$1 AND application_id=$2 AND name='API_TOKEN'`, orgA, runtimeApplicationID).Scan(&encryptedSecret); err != nil || bytes.Contains(encryptedSecret, []byte("runtime-secret-value")) {
		t.Fatalf("secret encryption failed: err=%v", err)
	}
	var secretAudit string
	if err := pool.QueryRow(ctx, `SELECT metadata::text FROM audit_events WHERE organization_id=$1 AND action='secret.changed' ORDER BY created_at DESC LIMIT 1`, orgA).Scan(&secretAudit); err != nil || strings.Contains(secretAudit, "runtime-secret-value") {
		t.Fatalf("secret leaked into audit metadata: %q err=%v", secretAudit, err)
	}
	viewer.put("/organizations/"+orgB+"/applications/"+runtimeApplicationID+"/secrets/CROSS_TENANT", map[string]any{"value": "forbidden"}, http.StatusNotFound)

	firstRuntimeDeployment := owner.post("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/deployments", map[string]any{}, http.StatusCreated)
	dockerRunner := jobs.Runner{Pool: pool, Executor: execution.DockerDeploymentExecutor{Pool: pool, Runtime: dockerRuntime, Secrets: apiServer.secrets, DockerBinary: "docker"}, Logger: logger, WorkerID: "docker-integration-test"}
	if err := dockerRunner.RunOnce(ctx); err != nil {
		t.Fatalf("Docker image deployment: %v", err)
	}
	dockerRuntime.mu.Lock()
	capturedSpec := dockerRuntime.spec
	dockerRuntime.mu.Unlock()
	if capturedSpec.Image != "nginx:alpine" || !capturedSpec.PullImage || capturedSpec.Environment["APP_MODE"] != "production" || capturedSpec.Environment["SHARED"] != "application" || capturedSpec.Environment["PROJECT_ONLY"] != "project-value" || capturedSpec.Environment["ENVIRONMENT_ONLY"] != "environment-value" || capturedSpec.Environment["API_TOKEN"] != "runtime-secret-value" || capturedSpec.Environment["PROJECT_SECRET"] != "project-secret-value" || capturedSpec.Environment["ENVIRONMENT_SECRET"] != "environment-secret-value" || capturedSpec.HostAddress != "127.0.0.1" || capturedSpec.HostPort != 32781 {
		t.Fatalf("runtime deployment spec is incomplete: image=%q pull=%t mode=%q shared=%q secretPresent=%t host=%q port=%d", capturedSpec.Image, capturedSpec.PullImage, capturedSpec.Environment["APP_MODE"], capturedSpec.Environment["SHARED"], capturedSpec.Environment["API_TOKEN"] != "", capturedSpec.HostAddress, capturedSpec.HostPort)
	}
	owner.put("/organizations/"+orgA+"/projects/"+projectID+"/environment-variables", map[string]any{"variables": []map[string]string{{"name": "APP_MODE", "value": "project"}, {"name": "PROJECT_ONLY", "value": "project-value"}, {"name": "SHARED", "value": "project"}}}, http.StatusOK)
	owner.put("/organizations/"+orgA+"/environments/"+environmentID+"/environment-variables", map[string]any{"variables": []map[string]string{{"name": "APP_MODE", "value": "environment"}, {"name": "ENVIRONMENT_ONLY", "value": "environment-value"}, {"name": "SHARED", "value": "environment"}}}, http.StatusOK)
	projectVariables, _ := json.Marshal(owner.get("/organizations/"+orgA+"/projects/"+projectID+"/environment-variables", http.StatusOK))
	environmentVariables, _ := json.Marshal(owner.get("/organizations/"+orgA+"/environments/"+environmentID+"/environment-variables", http.StatusOK))
	if bytes.Contains(projectVariables, []byte("PROJECT_DELETE_ME")) || bytes.Contains(environmentVariables, []byte("ENVIRONMENT_DELETE_ME")) {
		t.Fatalf("scoped variable replacement did not delete omitted records: project=%s environment=%s", projectVariables, environmentVariables)
	}
	owner.put("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/environment-variables", map[string]any{"variables": []map[string]string{{"name": "APP_MODE", "value": "production"}}}, http.StatusOK)
	applicationVariables, _ := json.Marshal(owner.get("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/environment-variables", http.StatusOK))
	if bytes.Contains(applicationVariables, []byte("SHARED")) {
		t.Fatalf("application variable replacement did not delete omitted override: %s", applicationVariables)
	}
	var runtimeID, runtimeState, deploymentState string
	if err := pool.QueryRow(ctx, `SELECT external_id,state FROM runtime_instances WHERE deployment_id=$1`, stringField(t, firstRuntimeDeployment, "id")).Scan(&runtimeID, &runtimeState); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM deployments WHERE id=$1`, stringField(t, firstRuntimeDeployment, "id")).Scan(&deploymentState); err != nil || runtimeID != "managed-container-1" || runtimeState != "running" || deploymentState != "healthy" {
		t.Fatalf("persisted runtime id=%q state=%q deployment=%q err=%v", runtimeID, runtimeState, deploymentState, err)
	}

	// A newer healthy instance replaces only the prior database-owned instance.
	secondRuntimeDeployment := owner.post("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/deployments", map[string]any{"image": "nginx:stable-alpine"}, http.StatusCreated)
	if err := dockerRunner.RunOnce(ctx); err != nil {
		t.Fatalf("replacement Docker deployment: %v", err)
	}
	var removedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT removed_at FROM runtime_instances WHERE deployment_id=$1`, stringField(t, firstRuntimeDeployment, "id")).Scan(&removedAt); err != nil || removedAt == nil {
		t.Fatalf("previous runtime was not cleaned up: removedAt=%v err=%v", removedAt, err)
	}
	currentRuntime := owner.get("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/runtime", http.StatusOK)
	currentInstance := currentRuntime["instance"].(map[string]any)
	if currentInstance["instanceId"] != "managed-container-2" || currentInstance["deploymentId"] != stringField(t, secondRuntimeDeployment, "id") {
		t.Fatalf("current runtime=%v", currentRuntime)
	}
	logView := owner.get("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/runtime/logs?tail=20", http.StatusOK)
	runtimeLogs := arrayField(t, logView, "logs")
	firstLog, ok := runtimeLogs[0].(map[string]any)
	if !ok || firstLog["message"] != "runtime output <script>is text</script>" || len(runtimeLogs) != 2 {
		t.Fatalf("runtime logs missing decoded untrusted text: %#v", logView)
	}
	owner.post("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/runtime/stop", map[string]any{}, http.StatusOK)
	owner.post("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/runtime/start", map[string]any{}, http.StatusOK)
	owner.post("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/runtime/restart", map[string]any{}, http.StatusOK)
	owner.post("/organizations/"+orgA+"/applications/"+runtimeApplicationID+"/runtime/remove", map[string]any{}, http.StatusOK)
	dockerRuntime.mu.Lock()
	actions := strings.Join(dockerRuntime.actions, ",")
	dockerRuntime.mu.Unlock()
	if !strings.Contains(actions, "remove:managed-container-1") || !strings.Contains(actions, "stop:managed-container-2") || !strings.Contains(actions, "start:managed-container-2") || !strings.Contains(actions, "restart:managed-container-2") || !strings.Contains(actions, "remove:managed-container-2") {
		t.Fatalf("runtime actions were not scoped to persisted managed instances: %s", actions)
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
	cloudflareZoneAID := stringField(t, arrayField(t, zones, "zones")[0].(map[string]any), "id")
	domain := owner.post("/organizations/"+orgA+"/applications/"+applicationID+"/domains", map[string]any{"hostname": "api.example.com", "targetPort": 3000, "protocol": "http", "routingMode": "cloudflare_proxied"}, http.StatusCreated)
	if stringField(t, domain, "dnsState") != "active" {
		t.Fatalf("domain=%v", domain)
	}
	domainID := stringField(t, domain, "id")
	owner.post("/organizations/"+orgA+"/domains/"+domainID+"/sync", map[string]any{}, http.StatusOK)
	owner.put("/organizations/"+orgA+"/domains/"+domainID, map[string]any{"targetPort": 3000, "protocol": "http", "routingMode": "cloudflare_tunnel"}, http.StatusOK)
	tunnel := owner.post("/organizations/"+orgA+"/integrations/cloudflare/tunnels", map[string]any{"name": "private-network", "serverId": serverID}, http.StatusCreated)
	tunnelID := stringField(t, tunnel, "id")
	owner.post("/organizations/"+orgA+"/integrations/cloudflare/tunnels/"+tunnelID+"/routes", map[string]any{"domainId": domainID}, http.StatusCreated)
	if connections.installations.Load() != 1 {
		t.Fatalf("tunnel installations=%d", connections.installations.Load())
	}
	if cloudflareState.routes.Load() != 1 {
		t.Fatalf("tunnel route calls=%d", cloudflareState.routes.Load())
	}
	external := owner.post("/organizations/"+orgA+"/integrations/cloudflare/tunnels", map[string]any{"name": "shared", "providerTunnelId": "shared-1", "ownership": "external"}, http.StatusCreated)
	owner.post("/organizations/"+orgA+"/integrations/cloudflare/tunnels/"+stringField(t, external, "id")+"/routes", map[string]any{"domainId": domainID}, http.StatusConflict)
	owner.delete("/organizations/"+orgA+"/domains/"+domainID, http.StatusNoContent)
	if cloudflareState.deletes.Load() != 1 {
		t.Fatalf("managed DNS deletes=%d", cloudflareState.deletes.Load())
	}
	owner.post("/organizations/"+orgA+"/applications/"+applicationID+"/domains", map[string]any{"hostname": "conflict.example.com", "targetPort": 3000, "protocol": "http", "routingMode": "dns_only"}, http.StatusConflict)
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
	viewer.put("/organizations/"+orgA+"/projects/"+projectID+"/environment-variables", map[string]any{"variables": []map[string]string{{"name": "FORBIDDEN", "value": "write"}}}, http.StatusForbidden)
	viewer.put("/organizations/"+orgA+"/environments/"+environmentID+"/secrets/FORBIDDEN", map[string]any{"value": "write"}, http.StatusForbidden)

	// Membership in both tenants must not make resource identifiers portable
	// between them. The same user is a viewer in A and owner in B for these
	// representative nested-resource and provider checks.
	projectB := viewer.post("/organizations/"+orgB+"/projects", map[string]any{"name": "Tenant B", "slug": "tenant-b"}, http.StatusCreated)
	projectBID := stringField(t, projectB, "id")
	environmentB := viewer.post("/organizations/"+orgB+"/projects/"+projectBID+"/environments", map[string]any{"name": "production", "slug": "production"}, http.StatusCreated)
	environmentBID := stringField(t, environmentB, "id")
	applicationB := viewer.post("/organizations/"+orgB+"/environments/"+environmentBID+"/applications", map[string]any{"name": "api-b", "sourceType": "docker_image", "image": "example/api-b:1", "internalPort": 8080}, http.StatusCreated)
	applicationBID := stringField(t, applicationB, "id")

	searchA := owner.get("/organizations/"+orgA+"/search?q=api", http.StatusOK)
	for _, raw := range arrayField(t, searchA, "results") {
		item := raw.(map[string]any)
		if stringField(t, item, "id") == applicationBID || strings.Contains(strings.ToLower(stringField(t, item, "title")), "tenant b") {
			t.Fatalf("organization A search leaked organization B result: %#v", item)
		}
	}
	searchB := viewer.get("/organizations/"+orgB+"/search?q=tenant-b", http.StatusOK)
	if results := arrayField(t, searchB, "results"); len(results) != 1 || stringField(t, results[0].(map[string]any), "id") != projectBID {
		t.Fatalf("organization B search results=%#v, want project %s only", results, projectBID)
	}
	viewer.get("/organizations/"+orgB+"/search?q=a", http.StatusUnprocessableEntity)

	viewer.get("/organizations/"+orgB+"/projects/"+projectID+"/environments", http.StatusNotFound)
	viewer.post("/organizations/"+orgB+"/projects/"+projectID+"/environments", map[string]any{"name": "cross", "slug": "cross"}, http.StatusNotFound)
	viewer.get("/organizations/"+orgB+"/environments/"+environmentID+"/applications", http.StatusNotFound)
	viewer.post("/organizations/"+orgB+"/environments/"+environmentID+"/applications", map[string]any{"name": "cross", "sourceType": "docker_image", "image": "example/cross:1"}, http.StatusNotFound)
	viewer.put("/organizations/"+orgB+"/applications/"+applicationBID+"/server", map[string]any{"serverId": serverID}, http.StatusNotFound)
	viewer.get("/organizations/"+orgB+"/applications/"+runtimeApplicationID+"/secrets", http.StatusNotFound)
	viewer.put("/organizations/"+orgB+"/applications/"+runtimeApplicationID+"/secrets/CROSS_TENANT", map[string]any{"value": "forbidden"}, http.StatusNotFound)
	viewer.get("/organizations/"+orgB+"/applications/"+runtimeApplicationID+"/configuration", http.StatusNotFound)
	viewer.get("/organizations/"+orgB+"/projects/"+projectID+"/environment-variables", http.StatusNotFound)
	viewer.put("/organizations/"+orgB+"/projects/"+projectID+"/environment-variables", map[string]any{"variables": []map[string]string{{"name": "CROSS", "value": "tenant"}}}, http.StatusNotFound)
	viewer.get("/organizations/"+orgB+"/environments/"+environmentID+"/environment-variables", http.StatusNotFound)
	viewer.put("/organizations/"+orgB+"/environments/"+environmentID+"/secrets/CROSS_TENANT", map[string]any{"value": "forbidden"}, http.StatusNotFound)
	viewer.post("/organizations/"+orgB+"/applications/"+applicationID+"/domains", map[string]any{"hostname": "cross.example.com", "targetPort": 3000, "protocol": "http", "routingMode": "dns_only"}, http.StatusNotFound)
	viewer.get("/organizations/"+orgB+"/applications/"+applicationID+"/git-source", http.StatusNotFound)
	viewer.post("/organizations/"+orgB+"/applications/"+applicationID+"/deployments", map[string]any{}, http.StatusNotFound)
	viewer.get("/organizations/"+orgB+"/applications/"+applicationID+"/deployments", http.StatusNotFound)
	viewer.get("/organizations/"+orgB+"/applications/"+runtimeApplicationID+"/runtime", http.StatusNotFound)
	viewer.get("/organizations/"+orgB+"/aws/accounts/"+awsAccountID+"/regions", http.StatusNotFound)
	viewer.post("/organizations/"+orgB+"/budgets", map[string]any{"projectId": projectID, "name": "Cross tenant", "monthlyAmount": 100, "currency": "USD", "thresholds": []float64{80}}, http.StatusNotFound)

	if _, err := pool.Exec(ctx, `INSERT INTO environments(organization_id,project_id,name,slug) VALUES($1,$2,'cross-tenant','cross-tenant')`, orgB, projectID); err == nil {
		t.Fatal("cross-organization project/environment relation was accepted")
	}
	var githubIntegrationBID string
	if err := pool.QueryRow(ctx, `INSERT INTO github_integrations(organization_id,installation_id,account_login,connected_by) VALUES($1,456,'tenant-b',NULL) RETURNING id`, orgB).Scan(&githubIntegrationBID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO application_git_sources(application_id,organization_id,integration_id,repository_id,repository_full_name,branch,auto_deploy) VALUES($1,$2,$3,199,'tenant-b/cross','main',true)`, runtimeApplicationID, orgB, githubIntegrationBID); err == nil {
		t.Fatal("cross-organization GitHub application relation was accepted")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO aws_resource_ownership(organization_id,account_id,region,resource_type,provider_resource_id,ownership) VALUES($1,$2,'eu-central-1','instance','i-cross-tenant','managed')`, orgB, awsAccountID); err == nil {
		t.Fatal("cross-organization AWS account/resource relation was accepted")
	}

	viewer.post("/organizations/"+orgB+"/integrations/cloudflare", map[string]any{"accountId": "account-1", "apiToken": "scoped-secret-token"}, http.StatusCreated)
	viewer.get("/organizations/"+orgB+"/integrations/cloudflare/zones", http.StatusOK)
	viewer.put("/organizations/"+orgB+"/integrations/cloudflare/zones/"+cloudflareZoneAID, map[string]any{"selected": false}, http.StatusNotFound)

	var crossTenantOperationID, tenantBJobID string
	if err := pool.QueryRow(ctx, `INSERT INTO aws_operations(organization_id,account_id,operation_type) VALUES($1,$2,'prepare_machine') RETURNING id`, orgA, awsAccountID).Scan(&crossTenantOperationID); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"operationId": crossTenantOperationID})
	if err := pool.QueryRow(ctx, `INSERT INTO jobs(organization_id,job_type,payload) VALUES($1,'prepare_aws_machine',$2) RETURNING id`, orgB, payload).Scan(&tenantBJobID); err != nil {
		t.Fatal(err)
	}
	awsRunner := jobs.AWSRunner{Repository: apiServer.repo, Logger: logger, WorkerID: "tenant-isolation-test"}
	if err := awsRunner.RunOnce(ctx); err == nil {
		t.Fatal("cross-organization AWS job payload unexpectedly reached its operation")
	}
	var operationStatus, jobStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM aws_operations WHERE id=$1`, crossTenantOperationID).Scan(&operationStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id=$1`, tenantBJobID).Scan(&jobStatus); err != nil {
		t.Fatal(err)
	}
	if operationStatus != "queued" || jobStatus != "failed" {
		t.Fatalf("cross-tenant AWS job mutated wrong records: operation=%q job=%q", operationStatus, jobStatus)
	}
	if _, err := pool.Exec(ctx, `UPDATE aws_operations SET job_id=$1 WHERE id=$2`, tenantBJobID, crossTenantOperationID); err == nil {
		t.Fatal("cross-organization AWS operation/job relation was accepted")
	}

	_, err = apiServer.repo.SaveRuntimeInstance(ctx, uuid.MustParse(orgB), uuid.MustParse(applicationBID), uuid.MustParse(stringField(t, firstRuntimeDeployment, "id")), runtimeprovider.InstanceStatus{InstanceID: "cross-tenant-runtime", Image: "example/cross:1", State: "running", Health: "running"}, 8080)
	if err == nil {
		t.Fatal("cross-organization runtime upsert mutated an existing deployment instance")
	}
	var preservedRuntimeID string
	if err = pool.QueryRow(ctx, `SELECT external_id FROM runtime_instances WHERE deployment_id=$1`, stringField(t, firstRuntimeDeployment, "id")).Scan(&preservedRuntimeID); err != nil || preservedRuntimeID != "managed-container-1" {
		t.Fatalf("cross-tenant runtime upsert changed existing instance: id=%q err=%v", preservedRuntimeID, err)
	}

	var crossDeploymentJobID, deploymentStatusBefore string
	if err = pool.QueryRow(ctx, `SELECT status FROM deployments WHERE id=$1`, deploymentID).Scan(&deploymentStatusBefore); err != nil {
		t.Fatal(err)
	}
	deploymentPayload, _ := json.Marshal(map[string]string{"deploymentId": deploymentID})
	if err = pool.QueryRow(ctx, `INSERT INTO jobs(organization_id,job_type,payload) VALUES($1,'deploy_application',$2) RETURNING id`, orgB, deploymentPayload).Scan(&crossDeploymentJobID); err != nil {
		t.Fatal(err)
	}
	if err = runner.RunOnce(ctx); err == nil {
		t.Fatal("cross-organization deployment job payload unexpectedly reached its deployment")
	}
	var deploymentStatusAfter, deploymentJobStatus string
	if err = pool.QueryRow(ctx, `SELECT status FROM deployments WHERE id=$1`, deploymentID).Scan(&deploymentStatusAfter); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id=$1`, crossDeploymentJobID).Scan(&deploymentJobStatus); err != nil {
		t.Fatal(err)
	}
	if deploymentStatusAfter != deploymentStatusBefore || deploymentJobStatus != "failed" {
		t.Fatalf("cross-tenant deployment job mutated wrong records: before=%q after=%q job=%q", deploymentStatusBefore, deploymentStatusAfter, deploymentJobStatus)
	}

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

func mapField(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()
	result, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("field %q is not an object in %#v", key, value)
	}
	return result
}
