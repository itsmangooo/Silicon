package serverruntime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
)

type fakeConnections struct {
	executor        connection.CommandExecutor
	orgID, serverID uuid.UUID
}

func (f fakeConnections) Executor(_ context.Context, orgID, serverID uuid.UUID) (connection.CommandExecutor, connection.Config, error) {
	if orgID != f.orgID || serverID != f.serverID {
		return nil, connection.Config{}, errors.New("scope mismatch")
	}
	return f.executor, connection.Config{Type: "ssh"}, nil
}

type fakeRemoteDocker struct {
	mu       sync.Mutex
	commands []string
	files    [][]byte
}

func (f *fakeRemoteDocker) Output(_ context.Context, input io.Reader, name string, args ...string) (string, error) {
	f.mu.Lock()
	f.commands = append(f.commands, strings.Join(append([]string{name}, args...), " "))
	f.mu.Unlock()
	if input != nil {
		_, _ = io.Copy(io.Discard, input)
	}
	if len(args) > 0 && args[0] == "create" {
		return "remote-container\n", nil
	}
	if len(args) > 0 && args[0] == "inspect" {
		return `{"Id":"remote-container","Created":"2026-01-01T00:00:00Z","Config":{"Image":"nginx:alpine","Labels":{"silicon.managed":"true","silicon.organization_id":"` + f.organization() + `","silicon.application_id":"application","silicon.deployment_id":"deployment"}},"State":{"Status":"running","Running":true,"StartedAt":"2026-01-01T00:00:01Z","FinishedAt":"","Health":null},"NetworkSettings":{"Ports":{}}}`, nil
	}
	return "", nil
}
func (f *fakeRemoteDocker) Run(ctx context.Context, input io.Reader, name string, args ...string) error {
	_, err := f.Output(ctx, input, name, args...)
	return err
}
func (*fakeRemoteDocker) Start(context.Context, string, ...string) (connection.Stream, error) {
	return connection.Stream{Stdout: strings.NewReader("2026-01-01T00:00:00Z remote stdout\n"), Stderr: strings.NewReader(""), Wait: func() error { return nil }}, nil
}
func (f *fakeRemoteDocker) WriteFile(_ context.Context, _ string, value []byte, _ uint32) (string, func(context.Context) error, error) {
	f.files = append(f.files, append([]byte(nil), value...))
	return "/tmp/silicon-env", func(context.Context) error { return nil }, nil
}
func (f *fakeRemoteDocker) organization() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, command := range f.commands {
		for _, part := range strings.Fields(command) {
			if strings.HasPrefix(part, "silicon.organization_id=") {
				return strings.TrimPrefix(part, "silicon.organization_id=")
			}
		}
	}
	return ""
}

func TestRemoteDockerLifecycleLogsAndEnvironment(t *testing.T) {
	organizationID, serverID := uuid.New(), uuid.New()
	executor := &fakeRemoteDocker{}
	provider := Provider{Connections: fakeConnections{executor: executor, orgID: organizationID, serverID: serverID}, HealthTimeout: time.Second}
	spec := runtimeprovider.DeploymentSpec{DeploymentID: "deployment", OrganizationID: organizationID.String(), ApplicationID: "application", ServerID: serverID.String(), Image: "nginx:alpine", Environment: map[string]string{"API_TOKEN": "runtime-secret"}}
	status, err := provider.Deploy(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if status.InstanceID != encode(organizationID, serverID, "remote-container") || status.ServerID != serverID.String() || !status.Healthy {
		t.Fatalf("status=%#v", status)
	}
	if len(executor.files) != 1 || !bytes.Contains(executor.files[0], []byte("API_TOKEN=runtime-secret")) {
		t.Fatal("remote environment was not transferred through a protected file")
	}
	for _, action := range []func(context.Context, string) error{provider.Stop, provider.Start, provider.Restart} {
		if err := action(context.Background(), status.InstanceID); err != nil {
			t.Fatal(err)
		}
	}
	logs, err := provider.Logs(context.Background(), status.InstanceID, runtimeprovider.LogRequest{Tail: 10})
	if err != nil {
		t.Fatal(err)
	}
	line := <-logs
	if line.Message != "remote stdout" || line.Stream != "stdout" {
		t.Fatalf("log=%#v", line)
	}
	if err := provider.Remove(context.Background(), status.InstanceID); err != nil {
		t.Fatal(err)
	}
	executor.mu.Lock()
	commands := strings.Join(executor.commands, "\n")
	executor.mu.Unlock()
	for _, operation := range []string{"docker create", "docker stop", "docker start", "docker restart", "docker rm"} {
		if !strings.Contains(commands, operation) {
			t.Fatalf("missing %q in commands:\n%s", operation, commands)
		}
	}
}

func TestRemoteBuildPinsExactRevision(t *testing.T) {
	organizationID, serverID := uuid.New(), uuid.New()
	executor := &fakeRemoteDocker{}
	provider := Provider{Connections: fakeConnections{executor: executor, orgID: organizationID, serverID: serverID}}
	sha := strings.Repeat("a", 40)
	image, err := provider.Build(context.Background(), runtimeprovider.BuildSpec{OrganizationID: organizationID.String(), ApplicationID: "application", ServerID: serverID.String(), CommitSHA: sha, Image: "silicon/app:" + sha[:12]}, strings.NewReader("archive"))
	if err != nil || image == "" {
		t.Fatalf("image=%q err=%v", image, err)
	}
	executor.mu.Lock()
	commands := strings.Join(executor.commands, "\n")
	executor.mu.Unlock()
	if !strings.Contains(commands, "silicon.commit_sha="+sha) || !strings.Contains(commands, "docker build") {
		t.Fatalf("exact revision build command missing:\n%s", commands)
	}
}
