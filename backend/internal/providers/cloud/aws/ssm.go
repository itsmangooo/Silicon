package aws

import (
	"context"
	"errors"
	"time"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func (c *Client) RunSSMCommand(ctx context.Context, instanceID string, commands []string, timeout time.Duration) (CommandResult, error) {
	if instanceID == "" || len(commands) == 0 {
		return CommandResult{}, errors.New("SSM instance and commands are required")
	}
	seconds := int32(timeout.Seconds())
	if seconds < 30 {
		seconds = 30
	}
	if seconds > 3600 {
		seconds = 3600
	}
	out, err := c.ssm.SendCommand(ctx, &ssm.SendCommandInput{InstanceIds: []string{instanceID}, DocumentName: sdk.String("AWS-RunShellScript"), Parameters: map[string][]string{"commands": commands}, TimeoutSeconds: sdk.Int32(seconds), CloudWatchOutputConfig: &ssmtypes.CloudWatchOutputConfig{CloudWatchOutputEnabled: false}})
	if err != nil {
		return CommandResult{}, safeError("run typed SSM operation", err)
	}
	commandID := ""
	if out.Command != nil {
		commandID = sdk.ToString(out.Command.CommandId)
	}
	if commandID == "" {
		return CommandResult{}, errors.New("AWS SSM returned no command identifier")
	}
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return CommandResult{}, errors.New("AWS SSM command timed out")
		}
		select {
		case <-ctx.Done():
			return CommandResult{}, ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
		invocation, getErr := c.ssm.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{CommandId: sdk.String(commandID), InstanceId: sdk.String(instanceID)})
		if getErr != nil {
			continue
		}
		status := string(invocation.Status)
		switch invocation.Status {
		case ssmtypes.CommandInvocationStatusSuccess:
			return CommandResult{Stdout: sdk.ToString(invocation.StandardOutputContent), Stderr: sdk.ToString(invocation.StandardErrorContent), Status: status}, nil
		case ssmtypes.CommandInvocationStatusPending, ssmtypes.CommandInvocationStatusInProgress, ssmtypes.CommandInvocationStatusDelayed:
			continue
		default:
			return CommandResult{Stdout: sdk.ToString(invocation.StandardOutputContent), Stderr: sdk.ToString(invocation.StandardErrorContent), Status: status}, errors.New("AWS SSM command failed with status " + status)
		}
	}
}

var _ SSMRunner = (*Client)(nil)
