package ses

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
)

type API interface {
	SendEmail(context.Context, *sesv2.SendEmailInput, ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
	GetAccount(context.Context, *sesv2.GetAccountInput, ...func(*sesv2.Options)) (*sesv2.GetAccountOutput, error)
}

type Provider struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey []byte
	SessionToken    []byte
	Client          API
	Timeout         time.Duration
}

func (p Provider) Send(ctx context.Context, message mailprovider.Message) error {
	if err := mailprovider.ValidateMessage(message); err != nil {
		return err
	}
	client, err := p.client(ctx)
	if err != nil {
		return err
	}
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(mailprovider.FromAddress(message.FromName, message.From)),
		Destination:      &types.Destination{ToAddresses: []string{message.To}},
		Content: &types.EmailContent{Simple: &types.Message{
			Subject: &types.Content{Data: aws.String(message.Subject), Charset: aws.String("UTF-8")},
			Body: &types.Body{
				Text: &types.Content{Data: aws.String(message.Text), Charset: aws.String("UTF-8")},
				Html: &types.Content{Data: aws.String(message.HTML), Charset: aws.String("UTF-8")},
			},
		}},
	}
	if message.ReplyTo != "" {
		input.ReplyToAddresses = []string{message.ReplyTo}
	}
	requestCtx, cancel := context.WithTimeout(ctx, p.timeout())
	defer cancel()
	if _, err = client.SendEmail(requestCtx, input); err != nil {
		return errors.New("Amazon SES rejected the message")
	}
	return nil
}

func (p Provider) Test(ctx context.Context) error {
	client, err := p.client(ctx)
	if err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(ctx, p.timeout())
	defer cancel()
	if _, err = client.GetAccount(requestCtx, &sesv2.GetAccountInput{}); err != nil {
		return errors.New("Amazon SES account check failed")
	}
	return nil
}

func (p Provider) Capabilities() mailprovider.Capabilities {
	return mailprovider.Capabilities{HTML: true, ReplyTo: true, SecureTLS: true, APIProvider: true}
}

func (p Provider) client(ctx context.Context) (API, error) {
	if p.Client != nil {
		return p.Client, nil
	}
	if p.Region == "" || p.AccessKeyID == "" || len(p.SecretAccessKey) == 0 {
		return nil, errors.New("Amazon SES region and credentials are required")
	}
	configuration, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(p.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(p.AccessKeyID, string(p.SecretAccessKey), string(p.SessionToken))),
	)
	if err != nil {
		return nil, errors.New("Amazon SES configuration failed")
	}
	return sesv2.NewFromConfig(configuration), nil
}

func (p Provider) timeout() time.Duration {
	if p.Timeout <= 0 {
		return 15 * time.Second
	}
	return p.Timeout
}

var _ mailprovider.Provider = Provider{}
