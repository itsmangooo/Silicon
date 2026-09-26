package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
)

func TestValidateSpecAndEnvironmentFile(t *testing.T) {
	spec := runtimeprovider.DeploymentSpec{DeploymentID: "deployment", OrganizationID: "organization", ApplicationID: "application", Image: "nginx:alpine", InternalPort: 80, HostAddress: "127.0.0.1", HostPort: 32781, Environment: map[string]string{"APP_MODE": "production", "lower_case": "allowed"}}
	if err := validateSpec(spec); err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := (Provider{}).environmentFile(context.Background(), spec.Environment)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(context.Background())
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "APP_MODE=production\nlower_case=allowed\n" {
		t.Fatalf("environment file=%q", data)
	}
	if err := validateSpec(runtimeprovider.DeploymentSpec{DeploymentID: "d", OrganizationID: "o", ApplicationID: "a", Image: "nginx", HostAddress: "0.0.0.0"}); err == nil {
		t.Fatal("host binding without explicit ports was accepted")
	}
	if _, _, err := (Provider{}).environmentFile(context.Background(), map[string]string{"TOKEN": "secret\nleak"}); err == nil {
		t.Fatal("multiline environment value was accepted")
	}
	if got := publishBinding("::1", 32781, 80); got != "[::1]:32781:80" {
		t.Fatalf("IPv6 binding=%q", got)
	}
}

func TestStatusFromDockerInspectUsesRealHealth(t *testing.T) {
	created := time.Now().UTC().Truncate(time.Second)
	item := inspectResult{ID: "container", Created: created}
	item.Config.Image = "nginx:alpine"
	item.State.Status = "running"
	item.State.Running = true
	item.State.StartedAt = created.Format(time.RFC3339Nano)
	item.State.Health = &struct {
		Status string `json:"Status"`
	}{Status: "starting"}
	status := statusFromInspect(item)
	if status.Healthy || status.Health != "starting" || status.State != "running" || status.StartedAt == nil {
		t.Fatalf("status=%#v", status)
	}
	item.State.Health.Status = "healthy"
	if status := statusFromInspect(item); !status.Healthy || status.Health != "healthy" {
		t.Fatalf("healthy status=%#v", status)
	}
}

func TestDockerRuntimeProviderIntegration(t *testing.T) {
	if os.Getenv("SILICON_DOCKER_INTEGRATION") != "1" {
		t.Skip("set SILICON_DOCKER_INTEGRATION=1 to run destructive, namespaced Docker tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	provider := Provider{Binary: os.Getenv("SILICON_DOCKER_BINARY"), HealthTimeout: 20 * time.Second}
	if _, err := provider.output(ctx, "version"); err != nil {
		t.Fatalf("Docker is unavailable: %v", err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	spec := runtimeprovider.DeploymentSpec{DeploymentID: "deployment-" + suffix, OrganizationID: "organization-" + suffix, ApplicationID: "application-" + suffix, Application: "nginx", Image: "nginx:alpine", PullImage: true, Environment: map[string]string{"SILICON_TEST_VALUE": "injected", "SILICON_TEST_SECRET": "encrypted-before-injection"}}
	status, err := provider.Deploy(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Remove(context.Background(), status.InstanceID)
	if status.InstanceID == "" || !status.Healthy || status.State != "running" {
		t.Fatalf("deployed status=%#v", status)
	}
	environment, err := provider.output(ctx, "inspect", "--format", "{{range .Config.Env}}{{println .}}{{end}}", status.InstanceID)
	if err != nil || !strings.Contains(environment, "SILICON_TEST_VALUE=injected") || !strings.Contains(environment, "SILICON_TEST_SECRET=encrypted-before-injection") {
		t.Fatalf("container environment missing: %q err=%v", environment, err)
	}
	logs, err := provider.Logs(ctx, status.InstanceID, runtimeprovider.LogRequest{Tail: 50})
	if err != nil {
		t.Fatal(err)
	}
	for range logs {
	}
	if err = provider.Stop(ctx, status.InstanceID); err != nil {
		t.Fatal(err)
	}
	if stopped, err := provider.Status(ctx, status.InstanceID); err != nil || stopped.State == "running" {
		t.Fatalf("stopped status=%#v err=%v", stopped, err)
	}
	if err = provider.Start(ctx, status.InstanceID); err != nil {
		t.Fatal(err)
	}
	if err = provider.Restart(ctx, status.InstanceID); err != nil {
		t.Fatal(err)
	}

	unmanaged, err := provider.output(ctx, "create", "--name", "silicon-unmanaged-test-"+suffix, "nginx:alpine")
	if err != nil {
		t.Fatal(err)
	}
	unmanaged = strings.TrimSpace(unmanaged)
	defer provider.output(context.Background(), "rm", "--force", unmanaged)
	if err = provider.Remove(ctx, unmanaged); err == nil {
		t.Fatal("unmanaged container removal was allowed")
	}
	if _, err = provider.output(ctx, "inspect", unmanaged); err != nil {
		t.Fatal("unmanaged container was modified")
	}

	if failed, err := provider.Deploy(ctx, runtimeprovider.DeploymentSpec{DeploymentID: "missing-" + suffix, OrganizationID: spec.OrganizationID, ApplicationID: spec.ApplicationID, Image: "invalid.example.invalid/silicon/missing:" + suffix, PullImage: true}); err == nil || failed.InstanceID != "" {
		t.Fatalf("missing image result=%#v err=%v", failed, err)
	}

	healthDir := t.TempDir()
	dockerfile := "FROM alpine:3.20\nHEALTHCHECK --interval=1s --timeout=1s --retries=1 CMD false\nCMD [\"sleep\",\"300\"]\n"
	if err = os.WriteFile(filepath.Join(healthDir, "Dockerfile"), []byte(dockerfile), 0600); err != nil {
		t.Fatal(err)
	}
	healthImage := "silicon/health-failure-test:" + suffix
	if _, err = provider.output(ctx, "build", "-t", healthImage, healthDir); err != nil {
		t.Fatal(err)
	}
	defer provider.output(context.Background(), "image", "rm", "--force", healthImage)
	failed, err := provider.Deploy(ctx, runtimeprovider.DeploymentSpec{DeploymentID: "health-" + suffix, OrganizationID: spec.OrganizationID, ApplicationID: spec.ApplicationID, Image: healthImage})
	if err == nil || failed.InstanceID == "" || failed.Health != "unhealthy" {
		t.Fatalf("unhealthy deployment=%#v err=%v", failed, err)
	}
	if removeErr := provider.Remove(ctx, failed.InstanceID); removeErr != nil {
		t.Fatal(removeErr)
	}
}
