package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
)

const remotePrefix = "agent:"

type RemoteRuntimeProvider struct{ Hub *Hub }

func (p RemoteRuntimeProvider) Build(ctx context.Context, spec runtimeprovider.BuildSpec, archive io.Reader) (string, error) {
	serverID, err := uuid.Parse(spec.ServerID)
	if err != nil {
		return "", errors.New("a valid target server is required")
	}
	commandID := uuid.NewString()
	responses, err := p.Hub.Call(ctx, serverID, CapabilityDocker, Command{ID: commandID, Operation: OperationBuild, Build: &spec})
	if err != nil {
		return "", err
	}
	if err = p.Hub.SendArchive(ctx, serverID, commandID, archive); err != nil {
		return "", err
	}
	result, err := finalResult(ctx, responses)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Image) == "" {
		return "", errors.New("agent returned no built image")
	}
	return result.Image, nil
}

func (p RemoteRuntimeProvider) Deploy(ctx context.Context, spec runtimeprovider.DeploymentSpec) (runtimeprovider.InstanceStatus, error) {
	serverID, err := uuid.Parse(spec.ServerID)
	if err != nil {
		return runtimeprovider.InstanceStatus{}, errors.New("a valid target server is required")
	}
	responses, err := p.Hub.Call(ctx, serverID, CapabilityDocker, Command{Operation: OperationDeploy, Deployment: &spec})
	if err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	result, err := finalResult(ctx, responses)
	if err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	if result.Status == nil {
		return runtimeprovider.InstanceStatus{}, errors.New("agent returned no runtime status")
	}
	status := *result.Status
	status.InstanceID = encodeRemote(serverID, status.InstanceID)
	status.ServerID = serverID.String()
	return status, nil
}

func (p RemoteRuntimeProvider) Start(ctx context.Context, id string) error {
	return p.lifecycle(ctx, OperationStart, id)
}
func (p RemoteRuntimeProvider) Stop(ctx context.Context, id string) error {
	return p.lifecycle(ctx, OperationStop, id)
}
func (p RemoteRuntimeProvider) Restart(ctx context.Context, id string) error {
	return p.lifecycle(ctx, OperationRestart, id)
}
func (p RemoteRuntimeProvider) Remove(ctx context.Context, id string) error {
	return p.lifecycle(ctx, OperationRemove, id)
}
func (p RemoteRuntimeProvider) Inspect(ctx context.Context, id string) (runtimeprovider.InstanceStatus, error) {
	return p.status(ctx, OperationInspect, id)
}
func (p RemoteRuntimeProvider) Status(ctx context.Context, id string) (runtimeprovider.InstanceStatus, error) {
	return p.status(ctx, OperationStatus, id)
}

func (p RemoteRuntimeProvider) lifecycle(ctx context.Context, operation Operation, encoded string) error {
	serverID, instanceID, err := decodeRemote(encoded)
	if err != nil {
		return err
	}
	responses, err := p.Hub.Call(ctx, serverID, CapabilityDocker, Command{Operation: operation, InstanceID: instanceID})
	if err != nil {
		return err
	}
	_, err = finalResult(ctx, responses)
	return err
}

func (p RemoteRuntimeProvider) status(ctx context.Context, operation Operation, encoded string) (runtimeprovider.InstanceStatus, error) {
	serverID, instanceID, err := decodeRemote(encoded)
	if err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	responses, err := p.Hub.Call(ctx, serverID, CapabilityDocker, Command{Operation: operation, InstanceID: instanceID})
	if err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	result, err := finalResult(ctx, responses)
	if err != nil {
		return runtimeprovider.InstanceStatus{}, err
	}
	if result.Status == nil {
		return runtimeprovider.InstanceStatus{}, errors.New("agent returned no runtime status")
	}
	status := *result.Status
	status.InstanceID = encoded
	status.ServerID = serverID.String()
	return status, nil
}

func (p RemoteRuntimeProvider) Logs(ctx context.Context, encoded string, request runtimeprovider.LogRequest) (<-chan runtimeprovider.LogLine, error) {
	serverID, instanceID, err := decodeRemote(encoded)
	if err != nil {
		return nil, err
	}
	responses, err := p.Hub.Call(ctx, serverID, CapabilityLogs, Command{Operation: OperationLogs, InstanceID: instanceID, Logs: &request})
	if err != nil {
		return nil, err
	}
	lines := make(chan runtimeprovider.LogLine, 64)
	go func() {
		defer close(lines)
		for {
			select {
			case <-ctx.Done():
				return
			case result := <-responses:
				if result.Log != nil {
					select {
					case lines <- *result.Log:
					case <-ctx.Done():
						return
					}
				}
				if result.Final {
					return
				}
			}
		}
	}()
	return lines, nil
}

func finalResult(ctx context.Context, responses <-chan Result) (Result, error) {
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case result := <-responses:
			if !result.Final {
				continue
			}
			if result.Error != "" {
				return result, errors.New(result.Error)
			}
			return result, nil
		}
	}
}

func encodeRemote(serverID uuid.UUID, instanceID string) string {
	return remotePrefix + serverID.String() + ":" + instanceID
}
func decodeRemote(value string) (uuid.UUID, string, error) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) != 3 || parts[0] != strings.TrimSuffix(remotePrefix, ":") || strings.TrimSpace(parts[2]) == "" {
		return uuid.Nil, "", fmt.Errorf("invalid remote runtime instance ID")
	}
	serverID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, "", errors.New("invalid remote server identity")
	}
	return serverID, parts[2], nil
}

var _ runtimeprovider.Provider = RemoteRuntimeProvider{}
var _ runtimeprovider.ImageBuilder = RemoteRuntimeProvider{}
