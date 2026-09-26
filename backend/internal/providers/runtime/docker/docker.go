package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
)

const (
	managedLabel       = "silicon.managed"
	organizationLabel  = "silicon.organization_id"
	applicationLabel   = "silicon.application_id"
	deploymentLabel    = "silicon.deployment_id"
	maximumLogTail     = 1000
	maximumLogLineSize = 256 << 10
)

type Provider struct {
	Binary         string
	HealthTimeout  time.Duration
	OrganizationID string
	Executor       connection.CommandExecutor
}

func (p Provider) Deploy(ctx context.Context, spec runtimeprovider.DeploymentSpec) (runtimeprovider.InstanceStatus, error) {
	if err := validateSpec(spec); err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	if p.OrganizationID != "" && spec.OrganizationID != p.OrganizationID {
		return runtimeprovider.InstanceStatus{}, errors.New("deployment organization does not match this runtime target")
	}
	if spec.PullImage {
		if err := p.run(ctx, "pull", "--", spec.Image); err != nil {
			return runtimeprovider.InstanceStatus{}, fmt.Errorf("pull image: %w", err)
		}
	}
	name := "silicon-" + compact(spec.ApplicationID, 12) + "-" + compact(spec.DeploymentID, 12)
	args := []string{"create", "--name", name, "--restart", "unless-stopped",
		"--label", managedLabel + "=true",
		"--label", organizationLabel + "=" + spec.OrganizationID,
		"--label", applicationLabel + "=" + spec.ApplicationID,
		"--label", deploymentLabel + "=" + spec.DeploymentID,
	}
	envPath, cleanup, err := p.environmentFile(ctx, spec.Environment)
	if err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	defer cleanup(context.Background())
	if envPath != "" {
		args = append(args, "--env-file", envPath)
	}
	if spec.HostPort > 0 {
		args = append(args, "--publish", publishBinding(spec.HostAddress, spec.HostPort, spec.InternalPort))
	}
	args = append(args, "--", spec.Image)
	output, err := p.output(ctx, args...)
	if err != nil {
		return runtimeprovider.InstanceStatus{}, fmt.Errorf("create container: %w", err)
	}
	instanceID := strings.TrimSpace(output)
	if instanceID == "" {
		return runtimeprovider.InstanceStatus{}, errors.New("Docker returned an empty container ID")
	}
	status := runtimeprovider.InstanceStatus{InstanceID: instanceID, Image: spec.Image, State: "created", Health: "unknown"}
	if err = p.run(ctx, "start", "--", instanceID); err != nil {
		return status, fmt.Errorf("start container: %w", err)
	}
	return p.waitReady(ctx, instanceID)
}

func (p Provider) Start(ctx context.Context, id string) error {
	if _, err := p.managedInspect(ctx, id); err != nil {
		return err
	}
	err := p.run(ctx, "start", "--", id)
	return wrapCommand("start container", err)
}

func (p Provider) Stop(ctx context.Context, id string) error {
	if _, err := p.managedInspect(ctx, id); err != nil {
		return err
	}
	err := p.run(ctx, "stop", "--time", "10", "--", id)
	return wrapCommand("stop container", err)
}

func (p Provider) Restart(ctx context.Context, id string) error {
	if _, err := p.managedInspect(ctx, id); err != nil {
		return err
	}
	err := p.run(ctx, "restart", "--time", "10", "--", id)
	return wrapCommand("restart container", err)
}

func (p Provider) Remove(ctx context.Context, id string) error {
	if _, err := p.managedInspect(ctx, id); err != nil {
		return err
	}
	err := p.run(ctx, "rm", "--force", "--", id)
	return wrapCommand("remove container", err)
}

func (p Provider) Inspect(ctx context.Context, id string) (runtimeprovider.InstanceStatus, error) {
	item, err := p.managedInspect(ctx, id)
	if err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	return statusFromInspect(item), nil
}

func (p Provider) Status(ctx context.Context, id string) (runtimeprovider.InstanceStatus, error) {
	return p.Inspect(ctx, id)
}

func (p Provider) Logs(ctx context.Context, id string, request runtimeprovider.LogRequest) (<-chan runtimeprovider.LogLine, error) {
	if _, err := p.managedInspect(ctx, id); err != nil {
		return nil, err
	}
	if request.Tail < 0 || request.Tail > maximumLogTail {
		return nil, fmt.Errorf("log tail must be between 0 and %d", maximumLogTail)
	}
	args := []string{"logs", "--timestamps", "--tail", strconv.Itoa(request.Tail)}
	if request.Follow {
		args = append(args, "--follow")
	}
	if !request.Since.IsZero() {
		args = append(args, "--since", request.Since.UTC().Format(time.RFC3339Nano))
	}
	args = append(args, "--", id)
	stream, err := p.executor().Start(ctx, p.binary(), args...)
	if err != nil {
		return nil, redactError(err)
	}
	lines := make(chan runtimeprovider.LogLine, 64)
	go func() {
		defer close(lines)
		done := make(chan struct{}, 2)
		go scanLogs(ctx, stream.Stdout, "stdout", lines, done)
		go scanLogs(ctx, stream.Stderr, "stderr", lines, done)
		<-done
		<-done
		_ = stream.Wait()
	}()
	return lines, nil
}

func (p Provider) waitReady(ctx context.Context, id string) (runtimeprovider.InstanceStatus, error) {
	timeout := p.HealthTimeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		status, err := p.Inspect(ctx, id)
		if err != nil {
			return runtimeprovider.InstanceStatus{InstanceID: id}, err
		}
		if !status.Healthy && status.State != "running" {
			return status, fmt.Errorf("container exited with state %s", status.State)
		}
		switch status.Health {
		case "healthy", "running":
			return status, nil
		case "unhealthy":
			return status, errors.New("container health check reported unhealthy")
		}
		select {
		case <-ctx.Done():
			return status, ctx.Err()
		case <-deadline.C:
			return status, fmt.Errorf("container did not become healthy within %s", timeout)
		case <-ticker.C:
		}
	}
}

type inspectResult struct {
	ID      string    `json:"Id"`
	ImageID string    `json:"Image"`
	Created time.Time `json:"Created"`
	Config  struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
		Health     *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
}

func (p Provider) managedInspect(ctx context.Context, id string) (inspectResult, error) {
	if strings.TrimSpace(id) == "" {
		return inspectResult{}, errors.New("runtime instance ID is required")
	}
	const inspectFormat = `{"Id":{{json .Id}},"Created":{{json .Created}},"Config":{"Image":{{json .Config.Image}},"Labels":{{json .Config.Labels}}},"State":{"Status":{{json .State.Status}},"Running":{{json .State.Running}},"StartedAt":{{json .State.StartedAt}},"FinishedAt":{{json .State.FinishedAt}},"Health":{{with (index .State "Health")}}{"Status":{{json .Status}}}{{else}}null{{end}}},"NetworkSettings":{"Ports":{{json .NetworkSettings.Ports}}}}`
	output, err := p.output(ctx, "inspect", "--format", inspectFormat, "--", id)
	if err != nil {
		return inspectResult{}, fmt.Errorf("inspect container: %w", err)
	}
	var value inspectResult
	if err := json.Unmarshal([]byte(output), &value); err != nil {
		return inspectResult{}, errors.New("Docker returned an invalid inspect response")
	}
	if value.Config.Labels[managedLabel] != "true" || value.Config.Labels[deploymentLabel] == "" || value.Config.Labels[applicationLabel] == "" || value.Config.Labels[organizationLabel] == "" {
		return inspectResult{}, errors.New("refusing to operate on a container not managed by Silicon")
	}
	if p.OrganizationID != "" && value.Config.Labels[organizationLabel] != p.OrganizationID {
		return inspectResult{}, errors.New("refusing to operate on a container owned by another organization")
	}
	return value, nil
}

func statusFromInspect(item inspectResult) runtimeprovider.InstanceStatus {
	health := item.State.Status
	if item.State.Health != nil {
		health = item.State.Health.Status
	} else if item.State.Running {
		health = "running"
	}
	status := runtimeprovider.InstanceStatus{
		InstanceID: item.ID,
		Image:      item.Config.Image,
		State:      item.State.Status,
		Health:     health,
		Healthy:    item.State.Running && (health == "running" || health == "healthy"),
	}
	if !item.Created.IsZero() {
		created := item.Created.UTC()
		status.CreatedAt = &created
	}
	status.StartedAt = parseDockerTime(item.State.StartedAt)
	status.FinishedAt = parseDockerTime(item.State.FinishedAt)
	keys := make([]string, 0, len(item.NetworkSettings.Ports))
	for key := range item.NetworkSettings.Ports {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		bindings := item.NetworkSettings.Ports[key]
		if len(bindings) == 0 {
			continue
		}
		port, err := strconv.Atoi(bindings[0].HostPort)
		if err == nil {
			status.HostAddress = bindings[0].HostIP
			status.HostPort = port
			break
		}
	}
	return status
}

func validateSpec(spec runtimeprovider.DeploymentSpec) error {
	if strings.TrimSpace(spec.DeploymentID) == "" || strings.TrimSpace(spec.OrganizationID) == "" || strings.TrimSpace(spec.ApplicationID) == "" {
		return errors.New("deployment, organization, and application IDs are required")
	}
	if strings.TrimSpace(spec.Image) == "" || strings.HasPrefix(spec.Image, "-") || strings.ContainsAny(spec.Image, " \t\r\n\x00") {
		return errors.New("a valid image reference is required")
	}
	if spec.InternalPort < 0 || spec.InternalPort > 65535 || spec.HostPort < 0 || spec.HostPort > 65535 {
		return errors.New("ports must be between 1 and 65535")
	}
	if spec.HostPort > 0 && spec.InternalPort == 0 {
		return errors.New("a container port is required when publishing a host port")
	}
	if spec.HostPort > 0 && spec.HostAddress == "" {
		return errors.New("an explicit host address is required when publishing a port")
	}
	if spec.HostAddress != "" && net.ParseIP(spec.HostAddress) == nil {
		return errors.New("host address must be an IPv4 or IPv6 address")
	}
	if spec.HostPort == 0 && spec.HostAddress != "" {
		return errors.New("host address requires a published host port")
	}
	for name, value := range spec.Environment {
		if !validEnvironmentName(name) || strings.ContainsAny(value, "\x00\r\n") {
			return fmt.Errorf("invalid environment variable %q", name)
		}
	}
	return nil
}

func (p Provider) environmentFile(ctx context.Context, values map[string]string) (string, func(context.Context) error, error) {
	if len(values) == 0 {
		return "", func(context.Context) error { return nil }, nil
	}
	var body bytes.Buffer
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !validEnvironmentName(name) || strings.ContainsAny(values[name], "\x00\r\n") {
			return "", func(context.Context) error { return nil }, fmt.Errorf("invalid environment variable %q", name)
		}
		if _, err := fmt.Fprintf(&body, "%s=%s\n", name, values[name]); err != nil {
			return "", func(context.Context) error { return nil }, err
		}
	}
	content := append([]byte(nil), body.Bytes()...)
	path, cleanup, err := p.executor().WriteFile(ctx, "silicon-env", content, 0600)
	for index := range content {
		content[index] = 0
	}
	if err != nil {
		return "", func(context.Context) error { return nil }, err
	}
	return path, cleanup, nil
}

func validEnvironmentName(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if !(char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || index > 0 && char >= '0' && char <= '9') {
			return false
		}
	}
	return true
}

func publishBinding(address string, hostPort, containerPort int) string {
	if strings.Contains(address, ":") {
		address = "[" + address + "]"
	}
	return fmt.Sprintf("%s:%d:%d", address, hostPort, containerPort)
}

func scanLogs(ctx context.Context, reader io.Reader, stream string, output chan<- runtimeprovider.LogLine, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), maximumLogLineSize)
	for scanner.Scan() {
		line := scanner.Text()
		entry := runtimeprovider.LogLine{Timestamp: time.Now().UTC(), Stream: stream, Message: line}
		if index := strings.IndexByte(line, ' '); index > 0 {
			if timestamp, err := time.Parse(time.RFC3339Nano, line[:index]); err == nil {
				entry.Timestamp = timestamp.UTC()
				entry.Message = line[index+1:]
			}
		}
		select {
		case output <- entry:
		case <-ctx.Done():
			return
		}
	}
}

func (p Provider) output(ctx context.Context, args ...string) (string, error) {
	output, err := p.executor().Output(ctx, nil, p.binary(), args...)
	if err != nil {
		return "", redactError(err)
	}
	return output, nil
}

func (p Provider) run(ctx context.Context, args ...string) error {
	return redactError(p.executor().Run(ctx, nil, p.binary(), args...))
}

func (p Provider) executor() connection.CommandExecutor {
	if p.Executor != nil {
		return p.Executor
	}
	return connection.LocalExecutor{}
}

func (p Provider) binary() string {
	if strings.TrimSpace(p.Binary) == "" {
		return "docker"
	}
	return p.Binary
}

func wrapCommand(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", action, err)
}

func redactError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ReplaceAll(err.Error(), "\r", " ")
	message = strings.ReplaceAll(message, "\n", " ")
	if len(message) > 500 {
		message = message[:500]
	}
	return errors.New(message)
}

func parseDockerTime(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.Year() <= 1 {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}

func compact(value string, length int) string {
	value = strings.ReplaceAll(value, "-", "")
	if len(value) > length {
		return value[:length]
	}
	return value
}

var _ runtimeprovider.Provider = Provider{}
