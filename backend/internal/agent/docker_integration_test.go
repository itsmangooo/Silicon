package agent

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/itsmangooo/Silicon/backend/internal/execution"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	dockerruntime "github.com/itsmangooo/Silicon/backend/internal/providers/runtime/docker"
)

func TestControlPlaneAgentDockerIntegration(t *testing.T) {
	if os.Getenv("SILICON_AGENT_DOCKER_INTEGRATION") != "1" {
		t.Skip("SILICON_AGENT_DOCKER_INTEGRATION is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	serverID, agentID, organizationID := uuid.New(), uuid.New(), uuid.NewString()
	hub := NewHub(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = hub.Serve(w, r, agentID, serverID, []string{CapabilityDocker, CapabilityLogs, CapabilityMetrics})
	}))
	defer server.Close()
	socket, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	provider := dockerruntime.Provider{HealthTimeout: 30 * time.Second, OrganizationID: organizationID}
	var writeMu sync.Mutex
	write := func(value Envelope) {
		writeMu.Lock()
		defer writeMu.Unlock()
		if err := socket.WriteJSON(value); err != nil && ctx.Err() == nil {
			t.Error(err)
		}
	}
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			writeMu.Lock()
			err := socket.WriteJSON(Envelope{Type: "heartbeat", Heartbeat: &Heartbeat{ProtocolVersion: ProtocolVersion, AgentVersion: "integration-test", Capabilities: []string{CapabilityDocker, CapabilityLogs, CapabilityMetrics}, DockerAvailable: true}})
			writeMu.Unlock()
			if err != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	go func() {
		for {
			var envelope Envelope
			if err := socket.ReadJSON(&envelope); err != nil {
				return
			}
			if envelope.Command == nil {
				continue
			}
			command := *envelope.Command
			finish := func(status *runtimeprovider.InstanceStatus, commandErr error) {
				result := Result{CommandID: command.ID, Final: true, Status: status}
				if commandErr != nil {
					result.Error = commandErr.Error()
				}
				write(Envelope{Type: "result", Result: &result})
			}
			switch command.Operation {
			case OperationBuild:
				file, fileErr := os.CreateTemp("", "silicon-agent-test-*.tar.gz")
				if fileErr != nil {
					finish(nil, fileErr)
					continue
				}
				for fileErr == nil {
					var chunkEnvelope Envelope
					if fileErr = socket.ReadJSON(&chunkEnvelope); fileErr != nil {
						break
					}
					if chunkEnvelope.Archive == nil || chunkEnvelope.Archive.CommandID != command.ID {
						continue
					}
					if chunkEnvelope.Archive.Final {
						break
					}
					_, fileErr = file.Write(chunkEnvelope.Archive.Data)
				}
				if fileErr == nil {
					_, fileErr = file.Seek(0, io.SeekStart)
				}
				if fileErr == nil {
					fileErr = execution.BuildDockerArchive(ctx, "docker", command.Build.Image, command.Build.OrganizationID, command.Build.ApplicationID, command.Build.CommitSHA, file)
				}
				file.Close()
				os.Remove(file.Name())
				result := Result{CommandID: command.ID, Final: true}
				if fileErr != nil {
					result.Error = fileErr.Error()
				} else {
					result.Image = command.Build.Image
				}
				write(Envelope{Type: "result", Result: &result})
			case OperationDeploy:
				status, commandErr := provider.Deploy(ctx, *command.Deployment)
				finish(&status, commandErr)
			case OperationStatus:
				status, commandErr := provider.Status(ctx, command.InstanceID)
				finish(&status, commandErr)
			case OperationStart:
				finish(nil, provider.Start(ctx, command.InstanceID))
			case OperationStop:
				finish(nil, provider.Stop(ctx, command.InstanceID))
			case OperationRestart:
				finish(nil, provider.Restart(ctx, command.InstanceID))
			case OperationRemove:
				finish(nil, provider.Remove(ctx, command.InstanceID))
			case OperationLogs:
				lines, commandErr := provider.Logs(ctx, command.InstanceID, *command.Logs)
				if commandErr != nil {
					finish(nil, commandErr)
					continue
				}
				for line := range lines {
					value := line
					write(Envelope{Type: "result", Result: &Result{CommandID: command.ID, Log: &value}})
				}
				finish(nil, nil)
			}
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !hub.Connected(serverID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !hub.Connected(serverID) {
		t.Fatal("agent transport did not connect")
	}
	remote := RemoteRuntimeProvider{Hub: hub}
	applicationID, deploymentID := uuid.NewString(), uuid.NewString()
	image := "silicon/agent-integration:" + strings.ReplaceAll(deploymentID, "-", "")[:12]
	built, err := remote.Build(ctx, runtimeprovider.BuildSpec{DeploymentID: deploymentID, OrganizationID: organizationID, ApplicationID: applicationID, ServerID: serverID.String(), CommitSHA: strings.Repeat("a", 40), Image: image}, testDockerArchive(t))
	if err != nil || built != image {
		t.Fatalf("remote build image=%q err=%v", built, err)
	}
	status, err := remote.Deploy(ctx, runtimeprovider.DeploymentSpec{DeploymentID: deploymentID, OrganizationID: organizationID, ApplicationID: applicationID, Application: "remote-nginx", Image: built, PullImage: false, ServerID: serverID.String(), Environment: map[string]string{"SILICON_REMOTE_TEST": "present"}})
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Remove(context.Background(), status.InstanceID)
	if !status.Healthy || status.ServerID != serverID.String() || !strings.HasPrefix(status.InstanceID, "agent:") {
		t.Fatalf("unexpected deployment status: %#v", status)
	}
	if _, err = remote.Status(ctx, status.InstanceID); err != nil {
		t.Fatal(err)
	}
	if err = remote.Stop(ctx, status.InstanceID); err != nil {
		t.Fatal(err)
	}
	if err = remote.Start(ctx, status.InstanceID); err != nil {
		t.Fatal(err)
	}
	if err = remote.Restart(ctx, status.InstanceID); err != nil {
		t.Fatal(err)
	}
	logs, err := remote.Logs(ctx, status.InstanceID, runtimeprovider.LogRequest{Tail: 20})
	if err != nil {
		t.Fatal(err)
	}
	for range logs {
	}
	if err = remote.Remove(ctx, status.InstanceID); err != nil {
		t.Fatal(err)
	}
}

func testDockerArchive(t *testing.T) io.Reader {
	t.Helper()
	var output bytes.Buffer
	gz := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gz)
	body := []byte("FROM nginx:alpine\n")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "repository/Dockerfile", Mode: 0600, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(output.Bytes())
}
