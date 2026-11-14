package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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
	privateSpec := runtimeprovider.DeploymentSpec{DeploymentID: "d", OrganizationID: "o", ApplicationID: "a", Image: "nginx", PortBindings: []runtimeprovider.PortBinding{{HostAddress: "10.44.0.2", HostPort: 3000, InternalPort: 3000}}, DNSServers: []string{"10.44.0.1"}, DNSSearch: []string{"internal"}}
	if err := validateSpec(privateSpec); err != nil {
		t.Fatalf("valid private network binding rejected: %v", err)
	}
	privateSpec.PortBindings[0].HostAddress = "0.0.0.0;unsafe"
	if err := validateSpec(privateSpec); err == nil {
		t.Fatal("invalid private network binding was accepted")
	}
	privateSpec.PortBindings[0].HostAddress = "0.0.0.0"
	if err := validateSpec(privateSpec); err == nil {
		t.Fatal("unspecified private network binding was accepted")
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

func TestRuntimeSettingsRejectUnsafeMounts(t *testing.T) {
	for _, mount := range []runtimeprovider.VolumeMount{
		{Name: "/etc", Target: "/data"},
		{Name: "data", Target: "/"},
		{Name: "data", Target: "/app/../data"},
		{Name: "data", Target: "/data,source=/etc"},
	} {
		settings := runtimeprovider.Settings{Volumes: []runtimeprovider.VolumeMount{mount}}
		if err := settings.Validate(); err == nil {
			t.Errorf("unsafe mount accepted: %#v", mount)
		}
	}
	settings := runtimeprovider.Settings{Volumes: []runtimeprovider.VolumeMount{
		{Name: "data", Target: "/data"}, {Name: "nested", Target: "/data/nested"},
	}}
	if err := settings.Validate(); err == nil {
		t.Fatal("overlapping mounts were accepted")
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
	t.Run("scoped storage and network survive replacement", func(t *testing.T) {
		org, environment, app := uuid.NewString(), uuid.NewString(), uuid.NewString()
		volume := "silicon-vol-" + org + "-" + app + "-data"
		network := "silicon-env-" + org + "-" + environment
		defer provider.run(context.Background(), "volume", "rm", "--", volume)
		defer provider.run(context.Background(), "network", "rm", "--", network)
		spec := runtimeprovider.DeploymentSpec{
			DeploymentID: uuid.NewString(), OrganizationID: org, ApplicationID: app, EnvironmentID: environment,
			Image: "alpine:3.22", PullImage: true,
			Settings: runtimeprovider.Settings{
				Volumes:            []runtimeprovider.VolumeMount{{Name: "data", Target: "/data"}},
				Arguments:          []string{"sh", "-c", "echo durable >/data/check; sleep 300"},
				EnvironmentNetwork: true, MemoryMiB: 64,
			},
		}
		first, err := provider.Deploy(ctx, spec)
		if first.InstanceID != "" {
			defer provider.Remove(context.Background(), first.InstanceID)
		}
		if err != nil || !first.Healthy {
			t.Fatalf("first instance: %#v, %v", first, err)
		}
		if err = provider.Remove(ctx, first.InstanceID); err != nil {
			t.Fatal(err)
		}
		spec.DeploymentID = uuid.NewString()
		spec.Settings.Arguments = []string{"sh", "-c", "test \"$(cat /data/check)\" = durable && sleep 300"}
		second, err := provider.Deploy(ctx, spec)
		if second.InstanceID != "" {
			defer provider.Remove(context.Background(), second.InstanceID)
		}
		if err != nil || !second.Healthy {
			t.Fatalf("replacement instance: %#v, %v", second, err)
		}
		if got, err := provider.output(ctx, "exec", second.InstanceID, "cat", "/data/check"); err != nil || strings.TrimSpace(got) != "durable" {
			t.Fatalf("persistent volume was not attached: %q, %v", got, err)
		}
		format := fmt.Sprintf("{{.HostConfig.Memory}} {{json (index .NetworkSettings.Networks %q).Aliases}}", network)
		got, err := provider.output(ctx, "inspect", "--format", format, "--", second.InstanceID)
		if err != nil || !strings.HasPrefix(got, "67108864 ") || !strings.Contains(got, "app-"+app) {
			t.Fatalf("memory and network settings missing: %q, %v", got, err)
		}
		if err := provider.ensureVolume(ctx, volume, uuid.NewString(), app); err == nil {
			t.Fatal("volume with different organization ownership was accepted")
		}
		if err := provider.ensureEnvironmentNetwork(ctx, network, org, uuid.NewString()); err == nil {
			t.Fatal("network with different environment ownership was accepted")
		}
	})
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
