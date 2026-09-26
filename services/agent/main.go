package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/itsmangooo/Silicon/backend/internal/agent"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	dockerruntime "github.com/itsmangooo/Silicon/backend/internal/providers/runtime/docker"
	"github.com/itsmangooo/Silicon/backend/internal/sourcebuild"
)

var version = "0.1.0"

type configuration struct {
	ControlPlaneURL string `json:"controlPlaneUrl"`
	AgentID         string `json:"agentId"`
	ServerID        string `json:"serverId"`
	OrganizationID  string `json:"organizationId"`
	Credential      string `json:"credential"`
	AllowInsecure   bool   `json:"allowInsecure,omitempty"`
}

type client struct {
	config    configuration
	logger    *slog.Logger
	runtime   runtimeprovider.Provider
	writeMu   sync.Mutex
	archiveMu sync.Mutex
	archives  map[string]chan agent.ArchiveChunk
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "version" {
		fmt.Println(version)
		return
	}
	if len(args) > 0 && args[0] == "check" {
		flags := flag.NewFlagSet("check", flag.ExitOnError)
		configPath := flags.String("config", "/etc/silicon-agent/agent.json", "Agent configuration path")
		_ = flags.Parse(args[1:])
		config, err := loadConfiguration(*configPath)
		if err != nil || checkConnection(ctx, config) != nil {
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "enroll" {
		if err := enroll(ctx, logger, args[1:]); err != nil {
			logger.Error("agent enrollment failed", "error", safeError(err))
			os.Exit(1)
		}
		return
	}
	flags := flag.NewFlagSet("silicon-agent", flag.ExitOnError)
	configPath := flags.String("config", "/etc/silicon-agent/agent.json", "Agent configuration path")
	_ = flags.Parse(args)
	config, err := loadConfiguration(*configPath)
	if err != nil {
		logger.Error("load agent configuration", "error", safeError(err))
		os.Exit(1)
	}
	provider := dockerruntime.Provider{Binary: "docker", HealthTimeout: 90 * time.Second, OrganizationID: config.OrganizationID}
	instance := &client{config: config, logger: logger, runtime: provider, archives: make(map[string]chan agent.ArchiveChunk)}
	if err = instance.run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("agent stopped", "error", safeError(err))
		os.Exit(1)
	}
}

func enroll(ctx context.Context, logger *slog.Logger, args []string) error {
	flags := flag.NewFlagSet("enroll", flag.ContinueOnError)
	controlPlane := flags.String("url", "", "Silicon control-plane URL")
	serverID := flags.String("server", "", "Server enrollment ID")
	token := flags.String("token", "", "One-time enrollment token")
	configPath := flags.String("config", "/etc/silicon-agent/agent.json", "Agent configuration path")
	allowInsecure := flags.Bool("allow-insecure", false, "Allow plain HTTP for isolated development only")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *controlPlane == "" || *serverID == "" || *token == "" {
		return errors.New("--url, --server, and --token are required")
	}
	baseURL, err := validateControlPlaneURL(*controlPlane, *allowInsecure)
	if err != nil {
		return err
	}
	heartbeat := collectHeartbeat(context.Background(), nil)
	body, _ := json.Marshal(map[string]any{"serverId": *serverID, "token": *token, "protocolVersion": agent.ProtocolVersion, "agentVersion": version, "capabilities": heartbeat.Capabilities})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL.String(), "/")+"/api/v1/agent/enroll", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return fmt.Errorf("control plane rejected enrollment with status %d", response.StatusCode)
	}
	var output struct{ AgentID, ServerID, OrganizationID, Credential string }
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&output); err != nil {
		return err
	}
	if output.AgentID == "" || output.ServerID != *serverID || output.OrganizationID == "" || output.Credential == "" {
		return errors.New("control plane returned an invalid Agent identity")
	}
	config := configuration{ControlPlaneURL: strings.TrimRight(baseURL.String(), "/"), AgentID: output.AgentID, ServerID: output.ServerID, OrganizationID: output.OrganizationID, Credential: output.Credential, AllowInsecure: *allowInsecure}
	if err = saveConfiguration(*configPath, config); err != nil {
		return err
	}
	logger.Info("agent enrolled", "agent_id", output.AgentID, "server_id", output.ServerID, "config", *configPath)
	return nil
}

func (c *client) run(ctx context.Context) error {
	delay := time.Second
	for ctx.Err() == nil {
		err := c.connect(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.logger.Warn("agent disconnected; retrying", "error", safeError(err), "retry_in", delay)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if delay < 30*time.Second {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}
	return ctx.Err()
}

func (c *client) connect(ctx context.Context) error {
	endpoint, err := websocketURL(c.config.ControlPlaneURL, c.config.AllowInsecure)
	if err != nil {
		return err
	}
	header := http.Header{"Authorization": []string{"Bearer " + c.config.Credential}, "X-Silicon-Agent-ID": []string{c.config.AgentID}}
	socket, response, err := websocket.DefaultDialer.DialContext(ctx, endpoint, header)
	if response != nil && response.Body != nil {
		response.Body.Close()
	}
	if err != nil {
		return err
	}
	defer socket.Close()
	socket.SetReadLimit(2 << 20)
	c.logger.Info("agent connected", "server_id", c.config.ServerID, "version", version)
	heartbeatCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go c.heartbeatLoop(heartbeatCtx, socket)
	for {
		var envelope agent.Envelope
		if err = socket.ReadJSON(&envelope); err != nil {
			return err
		}
		if envelope.Type == "archive" && envelope.Archive != nil {
			c.archiveMu.Lock()
			output := c.archives[envelope.Archive.CommandID]
			c.archiveMu.Unlock()
			if output != nil {
				select {
				case output <- *envelope.Archive:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			continue
		}
		if envelope.Type != "command" || envelope.Command == nil {
			continue
		}
		if envelope.Command.Operation == agent.OperationBuild {
			c.archiveMu.Lock()
			c.archives[envelope.Command.ID] = make(chan agent.ArchiveChunk, 8)
			c.archiveMu.Unlock()
		}
		go c.execute(ctx, socket, *envelope.Command)
	}
}

func (c *client) heartbeatLoop(ctx context.Context, socket *websocket.Conn) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	var sample *cpuSample
	for {
		heartbeat := collectHeartbeat(ctx, sample)
		sample = heartbeatSample()
		if err := c.write(socket, agent.Envelope{Type: "heartbeat", Heartbeat: &heartbeat}); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *client) execute(ctx context.Context, socket *websocket.Conn, command agent.Command) {
	finish := func(status *runtimeprovider.InstanceStatus, err error) {
		result := agent.Result{CommandID: command.ID, Final: true, Status: status}
		if err != nil {
			result.Error = safeError(err).Error()
		}
		_ = c.write(socket, agent.Envelope{Type: "result", Result: &result})
	}
	switch command.Operation {
	case agent.OperationDeploy:
		if command.Deployment == nil || command.Deployment.ServerID != c.config.ServerID {
			finish(nil, errors.New("deployment target does not match this Agent"))
			return
		}
		status, err := c.runtime.Deploy(ctx, *command.Deployment)
		finish(&status, err)
	case agent.OperationBuild:
		image, err := c.buildArchive(ctx, command)
		result := agent.Result{CommandID: command.ID, Final: true, Image: image}
		if err != nil {
			result.Error = safeError(err).Error()
		}
		_ = c.write(socket, agent.Envelope{Type: "result", Result: &result})
	case agent.OperationInspect, agent.OperationStatus:
		status, err := c.runtime.Status(ctx, command.InstanceID)
		finish(&status, err)
	case agent.OperationStart:
		finish(nil, c.runtime.Start(ctx, command.InstanceID))
	case agent.OperationStop:
		finish(nil, c.runtime.Stop(ctx, command.InstanceID))
	case agent.OperationRestart:
		finish(nil, c.runtime.Restart(ctx, command.InstanceID))
	case agent.OperationRemove:
		finish(nil, c.runtime.Remove(ctx, command.InstanceID))
	case agent.OperationLogs:
		if command.Logs == nil {
			finish(nil, errors.New("log request is required"))
			return
		}
		lines, err := c.runtime.Logs(ctx, command.InstanceID, *command.Logs)
		if err != nil {
			finish(nil, err)
			return
		}
		for line := range lines {
			value := line
			if err = c.write(socket, agent.Envelope{Type: "result", Result: &agent.Result{CommandID: command.ID, Log: &value}}); err != nil {
				return
			}
		}
		finish(nil, nil)
	default:
		finish(nil, errors.New("unsupported typed Agent operation"))
	}
}

func (c *client) buildArchive(ctx context.Context, command agent.Command) (string, error) {
	c.archiveMu.Lock()
	chunks := c.archives[command.ID]
	c.archiveMu.Unlock()
	defer func() { c.archiveMu.Lock(); delete(c.archives, command.ID); c.archiveMu.Unlock() }()
	if command.Build == nil || chunks == nil {
		return "", errors.New("build specification and archive stream are required")
	}
	if command.Build.ServerID != c.config.ServerID || command.Build.OrganizationID != c.config.OrganizationID {
		return "", errors.New("build target does not match this Agent identity")
	}
	file, err := os.CreateTemp("", "silicon-agent-source-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer func() { file.Close(); os.Remove(file.Name()) }()
	if err = file.Chmod(0600); err != nil {
		return "", err
	}
	var written int64
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case chunk := <-chunks:
			if chunk.Final {
				if _, err = file.Seek(0, io.SeekStart); err != nil {
					return "", err
				}
				if err = sourcebuild.BuildDockerArchive(ctx, "docker", command.Build.Image, command.Build.OrganizationID, command.Build.ApplicationID, command.Build.CommitSHA, file); err != nil {
					return "", err
				}
				return command.Build.Image, nil
			}
			written += int64(len(chunk.Data))
			if written > 1<<30 {
				return "", errors.New("source archive exceeds one GiB")
			}
			if _, err = file.Write(chunk.Data); err != nil {
				return "", err
			}
		}
	}
}

func (c *client) write(socket *websocket.Conn, value agent.Envelope) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return socket.WriteJSON(value)
}

func loadConfiguration(path string) (configuration, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return configuration{}, err
	}
	var config configuration
	if err = json.Unmarshal(body, &config); err != nil {
		return configuration{}, err
	}
	if config.AgentID == "" || config.ServerID == "" || config.OrganizationID == "" || config.Credential == "" {
		return configuration{}, errors.New("Agent configuration is incomplete")
	}
	if _, err = validateControlPlaneURL(config.ControlPlaneURL, config.AllowInsecure); err != nil {
		return configuration{}, err
	}
	return config, nil
}

func saveConfiguration(path string, config configuration) error {
	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err = os.WriteFile(temporary, body, 0600); err != nil {
		return err
	}
	if err = os.Chmod(temporary, 0600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func validateControlPlaneURL(value string, allowInsecure bool) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("control-plane URL is invalid")
	}
	if parsed.Scheme != "https" && !(allowInsecure && parsed.Scheme == "http") {
		return nil, errors.New("control-plane URL must use HTTPS")
	}
	return parsed, nil
}

func websocketURL(value string, allowInsecure bool) (string, error) {
	parsed, err := validateControlPlaneURL(value, allowInsecure)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "https" {
		parsed.Scheme = "wss"
	} else {
		parsed.Scheme = "ws"
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/v1/agent/connect"
	return parsed.String(), nil
}

func checkConnection(ctx context.Context, config configuration) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(config.ControlPlaneURL, "/")+"/api/v1/agent/check", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+config.Credential)
	request.Header.Set("X-Silicon-Agent-ID", config.AgentID)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("agent connection is not ready")
	}
	return nil
}

func safeError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ReplaceAll(strings.ReplaceAll(err.Error(), "\r", " "), "\n", " ")
	if len(message) > 500 {
		message = message[:500]
	}
	return errors.New(message)
}

func agentCapabilities(dockerAvailable bool) []string {
	values := []string{agent.CapabilityMetrics}
	if dockerAvailable {
		values = append(values, agent.CapabilityDocker, agent.CapabilityLogs)
	}
	return values
}
