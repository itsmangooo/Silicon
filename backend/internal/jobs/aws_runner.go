package jobs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
	"github.com/itsmangooo/Silicon/backend/internal/serverconnections"
	"github.com/itsmangooo/Silicon/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

type AWSProviderResolver interface {
	OpenAWS(context.Context, uuid.UUID, uuid.UUID, string) (cloudaws.Provider, error)
}

type AWSRunner struct {
	Repository        store.Repository
	Resolver          AWSProviderResolver
	Box               *cryptoenvelope.Box
	Logger            *slog.Logger
	WorkerID          string
	ConnectionTimeout time.Duration
	PollInterval      time.Duration
}

func (r AWSRunner) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.RunOnce(ctx); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				r.Logger.Error("AWS infrastructure job failed", "error", safeError(err))
			}
		}
	}
}

func (r AWSRunner) RunOnce(ctx context.Context) error {
	jobID, organizationID, payload, jobType, err := r.Repository.ClaimAWSJob(ctx, r.WorkerID)
	if err != nil {
		return err
	}
	var reference struct {
		OperationID uuid.UUID `json:"operationId"`
	}
	if json.Unmarshal(payload, &reference) != nil || reference.OperationID == uuid.Nil {
		return r.fail(ctx, jobID, uuid.Nil, errors.New("invalid AWS operation payload"))
	}
	operation, err := r.Repository.AWSOperationForUpdate(ctx, reference.OperationID)
	if err != nil {
		return r.fail(ctx, jobID, reference.OperationID, err)
	}
	if operation.OrganizationID != organizationID {
		return r.fail(ctx, jobID, operation.ID, errors.New("AWS operation organization mismatch"))
	}
	_ = r.Repository.UpdateAWSOperation(ctx, operation.ID, "running", "", "", "")
	switch jobType {
	case "provision_aws_machine":
		err = r.provision(ctx, operation)
	case "terminate_aws_machine":
		err = r.terminate(ctx, operation)
	case "create_aws_snapshot":
		err = r.snapshot(ctx, operation)
	case "prepare_aws_machine":
		err = errors.New("prepare operation requires an explicit server connection check")
	default:
		err = errors.New("unsupported AWS operation")
	}
	if err != nil {
		return r.fail(ctx, jobID, operation.ID, err)
	}
	if updateErr := r.Repository.UpdateAWSOperation(ctx, operation.ID, "succeeded", operation.ResourceType, operation.ProviderResourceID, ""); updateErr != nil {
		return r.fail(ctx, jobID, operation.ID, updateErr)
	}
	return r.Repository.CompleteAWSJob(ctx, jobID, "succeeded", "")
}

func (r AWSRunner) provision(ctx context.Context, operation store.AWSOperation) error {
	var request struct {
		Machine             cloudaws.CreateMachineInput `json:"machine"`
		SSHUsername         string                      `json:"sshUsername"`
		EncryptedPrivateKey string                      `json:"encryptedPrivateKey"`
	}
	if json.Unmarshal(operation.Request, &request) != nil {
		return errors.New("invalid AWS machine request")
	}
	provider, err := r.Resolver.OpenAWS(ctx, operation.OrganizationID, operation.AccountID, request.Machine.Region)
	if err != nil {
		return err
	}
	_ = r.Repository.UpdateAWSOperation(ctx, operation.ID, "waiting_for_aws", "instance", "", "")
	instance, err := provider.CreateMachine(ctx, request.Machine)
	if err != nil {
		return err
	}
	if request.Machine.ElasticIP {
		address, allocationErr := provider.AllocateElasticIP(ctx, operation.OrganizationID.String())
		if allocationErr != nil {
			return allocationErr
		}
		if _, claimErr := r.Repository.ClaimAWSResource(ctx, operation.OrganizationID, operation.AccountID, actor(operation), instance.Region, "elastic_ip", address.AllocationID, "managed", uuidPointer(request.Machine.ProjectID), uuidPointer(request.Machine.EnvironmentID), uuid.New(), nil); claimErr != nil {
			return claimErr
		}
		if associationErr := provider.AssociateElasticIP(ctx, address.AllocationID, instance.ID); associationErr != nil {
			return associationErr
		}
		if refreshed, refreshErr := provider.Instance(ctx, instance.ID); refreshErr == nil {
			instance = refreshed
		}
	}
	serverID := uuid.New()
	var encryptedKey []byte
	if request.EncryptedPrivateKey != "" {
		if r.Box == nil {
			return errors.New("AWS machine credential encryption is unavailable")
		}
		ciphertext, decodeErr := base64.StdEncoding.DecodeString(request.EncryptedPrivateKey)
		if decodeErr != nil {
			return errors.New("invalid encrypted AWS machine credential")
		}
		plaintext, openErr := r.Box.Open(ciphertext, cloudaws.MachineKeyContext(operation.OrganizationID, operation.AccountID))
		if openErr != nil {
			return errors.New("AWS machine credential could not be decrypted")
		}
		encryptedKey, err = r.Box.Seal(plaintext, serverconnections.CredentialContext(operation.OrganizationID, serverID))
		zeroSecret(plaintext)
		if err != nil {
			return err
		}
	}
	_, err = r.Repository.SaveAWSInstance(ctx, operation.OrganizationID, operation.AccountID, actor(operation), serverID, instance, "managed", request.Machine.ConnectionMethod, encryptedKey, request.SSHUsername, uuid.New(), nil)
	if err != nil {
		return err
	}
	if request.Machine.ConnectionMethod == "aws_ssm" {
		_ = r.Repository.UpdateAWSOperation(ctx, operation.ID, "waiting_for_connection", "instance", instance.ID, "")
		runner, ok := provider.(cloudaws.SSMRunner)
		if !ok {
			return errors.New("AWS provider does not support SSM readiness checks")
		}
		version, checkErr := r.waitForSSMDocker(ctx, runner, instance.ID)
		if checkErr != nil {
			_ = r.Repository.UpdateServerCheck(ctx, operation.OrganizationID, serverID, "unreachable", "unhealthy", "", "", "", "SSM connected machine did not become Docker-ready before the timeout.", false)
			return checkErr
		}
		if err = r.Repository.UpdateServerCheck(ctx, operation.OrganizationID, serverID, "connected", "healthy", "linux", request.Machine.Architecture, version, "", true); err != nil {
			return err
		}
	}
	operation.ResourceType = "instance"
	operation.ProviderResourceID = instance.ID
	return r.Repository.UpdateAWSOperation(ctx, operation.ID, "running", "instance", instance.ID, "")
}

func (r AWSRunner) waitForSSMDocker(ctx context.Context, runner cloudaws.SSMRunner, instanceID string) (string, error) {
	timeout := r.ConnectionTimeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	interval := r.PollInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	waitContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	deadline, _ := waitContext.Deadline()
	for {
		commandTimeout := 45 * time.Second
		if remaining := time.Until(deadline); remaining < commandTimeout {
			commandTimeout = remaining
		}
		if commandTimeout > 0 {
			result, err := runner.RunSSMCommand(waitContext, instanceID, []string{"docker version --format '{{.Server.Version}}'"}, commandTimeout)
			if err == nil && result.Status == "Success" && strings.TrimSpace(result.Stdout) != "" {
				return strings.TrimSpace(result.Stdout), nil
			}
		}
		if time.Now().After(deadline) {
			return "", errors.New("AWS SSM machine did not become Docker-ready before the timeout")
		}
		select {
		case <-waitContext.Done():
			if errors.Is(waitContext.Err(), context.DeadlineExceeded) {
				return "", errors.New("AWS SSM machine did not become Docker-ready before the timeout")
			}
			return "", waitContext.Err()
		case <-time.After(interval):
		}
	}
}

func (r AWSRunner) terminate(ctx context.Context, operation store.AWSOperation) error {
	var request struct {
		InstanceID string `json:"instanceId"`
		Region     string `json:"region"`
	}
	if json.Unmarshal(operation.Request, &request) != nil || request.InstanceID == "" {
		return errors.New("invalid terminate request")
	}
	provider, err := r.Resolver.OpenAWS(ctx, operation.OrganizationID, operation.AccountID, request.Region)
	if err != nil {
		return err
	}
	if err = provider.TerminateInstance(ctx, request.InstanceID); err != nil {
		return err
	}
	if err = r.Repository.MarkAWSInstanceTerminated(ctx, operation.OrganizationID, operation.AccountID, request.Region, request.InstanceID); err != nil {
		return err
	}
	operation.ResourceType = "instance"
	operation.ProviderResourceID = request.InstanceID
	return r.Repository.RecordOrganizationAudit(ctx, operation.OrganizationID, actor(operation), "aws.instance_terminated", "aws_instance", nil, uuid.New(), map[string]any{"instanceId": request.InstanceID, "region": request.Region}, nil)
}
func (r AWSRunner) snapshot(ctx context.Context, operation store.AWSOperation) error {
	var request struct{ VolumeID, Description, Region string }
	if json.Unmarshal(operation.Request, &request) != nil || request.VolumeID == "" {
		return errors.New("invalid snapshot request")
	}
	provider, err := r.Resolver.OpenAWS(ctx, operation.OrganizationID, operation.AccountID, request.Region)
	if err != nil {
		return err
	}
	item, err := provider.CreateSnapshot(ctx, request.VolumeID, request.Description, operation.OrganizationID.String())
	if err != nil {
		return err
	}
	_, err = r.Repository.ClaimAWSResource(ctx, operation.OrganizationID, operation.AccountID, actor(operation), request.Region, "snapshot", item.ID, "managed", nil, nil, uuid.New(), nil)
	if err != nil {
		return err
	}
	operation.ResourceType = "snapshot"
	operation.ProviderResourceID = item.ID
	return r.Repository.UpdateAWSOperation(ctx, operation.ID, "running", "snapshot", item.ID, "")
}
func (r AWSRunner) fail(ctx context.Context, jobID, operationID uuid.UUID, cause error) error {
	message := safeError(cause)
	if operationID != uuid.Nil {
		_ = r.Repository.UpdateAWSOperation(ctx, operationID, "failed", "", "", message)
	}
	_ = r.Repository.CompleteAWSJob(ctx, jobID, "failed", message)
	return cause
}
func actor(operation store.AWSOperation) uuid.UUID {
	if operation.CreatedBy != nil {
		return *operation.CreatedBy
	}
	return uuid.Nil
}
func uuidPointer(value string) *uuid.UUID {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil
	}
	return &parsed
}
func zeroSecret(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
