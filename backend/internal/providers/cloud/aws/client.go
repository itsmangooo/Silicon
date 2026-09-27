package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

type SDKFactory struct{}

type Client struct {
	config  sdk.Config
	ec2     *ec2.Client
	ssm     *ssm.Client
	sts     *sts.Client
	cost    *costexplorer.Client
	pricing *pricing.Client
	region  string
}

func (SDKFactory) Open(ctx context.Context, input AccountConfig) (Provider, error) {
	region := strings.TrimSpace(input.Region)
	if region == "" {
		region = "us-east-1"
	}
	options := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if input.AccessKeyID != "" || input.SecretAccessKey != "" {
		if input.AccessKeyID == "" || input.SecretAccessKey == "" {
			return nil, errors.New("both AWS access key fields are required")
		}
		options = append(options, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(input.AccessKeyID, input.SecretAccessKey, "")))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, safeError("load AWS configuration", err)
	}
	if input.RoleARN != "" {
		baseSTS := sts.NewFromConfig(cfg)
		provider := stscreds.NewAssumeRoleProvider(baseSTS, input.RoleARN, func(options *stscreds.AssumeRoleOptions) {
			options.RoleSessionName = "silicon-control-plane"
			if input.ExternalID != "" {
				options.ExternalID = sdk.String(input.ExternalID)
			}
		})
		cfg.Credentials = sdk.NewCredentialsCache(provider)
	}
	client := &Client{config: cfg, ec2: ec2.NewFromConfig(cfg), ssm: ssm.NewFromConfig(cfg), sts: sts.NewFromConfig(cfg), cost: costexplorer.NewFromConfig(cfg), pricing: pricing.NewFromConfig(cfg, func(o *pricing.Options) { o.Region = "us-east-1" }), region: region}
	return client, nil
}

func (c *Client) Identity(ctx context.Context) (Identity, error) {
	out, err := c.sts.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return Identity{}, safeError("verify AWS identity", err)
	}
	identity := Identity{AccountID: sdk.ToString(out.Account), ARN: sdk.ToString(out.Arn), UserID: sdk.ToString(out.UserId)}
	if identity.AccountID == "" {
		return Identity{}, errors.New("AWS returned an empty account identity")
	}
	return identity, nil
}

func safeError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		code := api.ErrorCode()
		switch code {
		case "AccessDenied", "AccessDeniedException", "UnauthorizedOperation":
			return fmt.Errorf("%s: AWS denied the required permission", operation)
		case "Throttling", "ThrottlingException", "RequestLimitExceeded":
			return fmt.Errorf("%s: AWS throttled the request; retry later", operation)
		case "InstanceLimitExceeded", "VcpuLimitExceeded", "VolumeLimitExceeded":
			return fmt.Errorf("%s: AWS account quota exceeded", operation)
		case "InsufficientInstanceCapacity":
			return fmt.Errorf("%s: AWS has insufficient capacity in the selected location", operation)
		case "InvalidAMIID.NotFound":
			return fmt.Errorf("%s: selected AMI is not available in this region", operation)
		case "InvalidSubnetID.NotFound":
			return fmt.Errorf("%s: selected subnet is not available in this region", operation)
		case "DependencyViolation":
			return fmt.Errorf("%s: AWS resource still has dependent resources", operation)
		}
		return fmt.Errorf("%s: AWS error %s", operation, code)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%s: provider request failed", operation)
}
