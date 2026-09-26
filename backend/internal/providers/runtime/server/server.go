package serverruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	dockerruntime "github.com/itsmangooo/Silicon/backend/internal/providers/runtime/docker"
)

const instancePrefix = "server:"

type Connections interface {
	Executor(context.Context, uuid.UUID, uuid.UUID) (connection.CommandExecutor, connection.Config, error)
}

type Provider struct {
	Connections   Connections
	HealthTimeout time.Duration
}

func (p Provider) Build(ctx context.Context, spec runtimeprovider.BuildSpec, archive io.Reader) (string, error) {
	organizationID, serverID, err := identities(spec.OrganizationID, spec.ServerID)
	if err != nil {
		return "", err
	}
	if len(spec.CommitSHA) != 40 || invalidImage(spec.Image) {
		return "", errors.New("a valid exact revision and image are required")
	}
	executor, _, err := p.Connections.Executor(ctx, organizationID, serverID)
	if err != nil {
		return "", err
	}
	_, err = executor.Output(ctx, archive, "docker", "build", "--pull",
		"--label", "silicon.managed=true",
		"--label", "silicon.organization_id="+spec.OrganizationID,
		"--label", "silicon.application_id="+spec.ApplicationID,
		"--label", "silicon.commit_sha="+spec.CommitSHA,
		"-t", spec.Image, "-")
	if err != nil {
		return "", fmt.Errorf("remote Docker build failed: %w", err)
	}
	return spec.Image, nil
}

func (p Provider) Deploy(ctx context.Context, spec runtimeprovider.DeploymentSpec) (runtimeprovider.InstanceStatus, error) {
	organizationID, serverID, err := identities(spec.OrganizationID, spec.ServerID)
	if err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	executor, _, err := p.Connections.Executor(ctx, organizationID, serverID)
	if err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	provider := p.docker(executor, organizationID)
	status, err := provider.Deploy(ctx, spec)
	if status.InstanceID != "" {
		status.InstanceID = encode(organizationID, serverID, status.InstanceID)
		status.ServerID = serverID.String()
	}
	return status, err
}

func (p Provider) Start(ctx context.Context, id string) error { return p.lifecycle(ctx, id, "start") }
func (p Provider) Stop(ctx context.Context, id string) error  { return p.lifecycle(ctx, id, "stop") }
func (p Provider) Restart(ctx context.Context, id string) error {
	return p.lifecycle(ctx, id, "restart")
}
func (p Provider) Remove(ctx context.Context, id string) error { return p.lifecycle(ctx, id, "remove") }

func (p Provider) Inspect(ctx context.Context, id string) (runtimeprovider.InstanceStatus, error) {
	return p.status(ctx, id, true)
}

func (p Provider) Status(ctx context.Context, id string) (runtimeprovider.InstanceStatus, error) {
	return p.status(ctx, id, false)
}

func (p Provider) Logs(ctx context.Context, id string, request runtimeprovider.LogRequest) (<-chan runtimeprovider.LogLine, error) {
	organizationID, serverID, instanceID, err := decode(id)
	if err != nil {
		return nil, err
	}
	executor, _, err := p.Connections.Executor(ctx, organizationID, serverID)
	if err != nil {
		return nil, err
	}
	return p.docker(executor, organizationID).Logs(ctx, instanceID, request)
}

func (p Provider) lifecycle(ctx context.Context, id, operation string) error {
	organizationID, serverID, instanceID, err := decode(id)
	if err != nil {
		return err
	}
	executor, _, err := p.Connections.Executor(ctx, organizationID, serverID)
	if err != nil {
		return err
	}
	provider := p.docker(executor, organizationID)
	switch operation {
	case "start":
		return provider.Start(ctx, instanceID)
	case "stop":
		return provider.Stop(ctx, instanceID)
	case "restart":
		return provider.Restart(ctx, instanceID)
	case "remove":
		return provider.Remove(ctx, instanceID)
	default:
		return errors.New("unsupported runtime operation")
	}
}

func (p Provider) status(ctx context.Context, id string, inspect bool) (runtimeprovider.InstanceStatus, error) {
	organizationID, serverID, instanceID, err := decode(id)
	if err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	executor, _, err := p.Connections.Executor(ctx, organizationID, serverID)
	if err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	provider := p.docker(executor, organizationID)
	var status runtimeprovider.InstanceStatus
	if inspect {
		status, err = provider.Inspect(ctx, instanceID)
	} else {
		status, err = provider.Status(ctx, instanceID)
	}
	if err == nil {
		status.InstanceID = id
		status.ServerID = serverID.String()
	}
	return status, err
}

func (p Provider) docker(executor connection.CommandExecutor, organizationID uuid.UUID) dockerruntime.Provider {
	return dockerruntime.Provider{Executor: executor, HealthTimeout: p.HealthTimeout, OrganizationID: organizationID.String()}
}

func identities(organization, server string) (uuid.UUID, uuid.UUID, error) {
	organizationID, err := uuid.Parse(organization)
	if err != nil {
		return uuid.Nil, uuid.Nil, errors.New("a valid organization is required")
	}
	serverID, err := uuid.Parse(server)
	if err != nil {
		return uuid.Nil, uuid.Nil, errors.New("a valid target server is required")
	}
	return organizationID, serverID, nil
}

func encode(organizationID, serverID uuid.UUID, instanceID string) string {
	return instancePrefix + organizationID.String() + ":" + serverID.String() + ":" + instanceID
}

func decode(value string) (uuid.UUID, uuid.UUID, string, error) {
	parts := strings.SplitN(value, ":", 4)
	if len(parts) != 4 || parts[0] != strings.TrimSuffix(instancePrefix, ":") || strings.TrimSpace(parts[3]) == "" {
		return uuid.Nil, uuid.Nil, "", errors.New("invalid server runtime instance ID")
	}
	organizationID, serverID, err := identities(parts[1], parts[2])
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}
	return organizationID, serverID, parts[3], nil
}

func invalidImage(value string) bool {
	return strings.TrimSpace(value) == "" || strings.HasPrefix(value, "-") || strings.ContainsAny(value, " \t\r\n\x00")
}

var _ runtimeprovider.Provider = Provider{}
var _ runtimeprovider.ImageBuilder = Provider{}
