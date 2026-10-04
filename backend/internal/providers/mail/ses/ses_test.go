package ses

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
)

type fakeSES struct {
	input *sesv2.SendEmailInput
	err   error
}

func (f *fakeSES) SendEmail(_ context.Context, input *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.input = input
	return &sesv2.SendEmailOutput{}, f.err
}
func (f *fakeSES) GetAccount(context.Context, *sesv2.GetAccountInput, ...func(*sesv2.Options)) (*sesv2.GetAccountOutput, error) {
	return &sesv2.GetAccountOutput{}, f.err
}

func TestSendUsesSESv2AndDoesNotExposeProviderErrors(t *testing.T) {
	client := &fakeSES{}
	provider := Provider{Client: client}
	message := mailprovider.Message{FromName: "Silicon", From: "silicon@example.com", To: "user@example.com", ReplyTo: "support@example.com", Subject: "Subject", Text: "plain", HTML: "<p>html</p>"}
	if err := provider.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if client.input == nil || len(client.input.Destination.ToAddresses) != 1 || client.input.Destination.ToAddresses[0] != message.To || len(client.input.ReplyToAddresses) != 1 {
		t.Fatalf("unexpected SES request: %#v", client.input)
	}
	client.err = errors.New("credential=must-not-leak")
	if err := provider.Send(context.Background(), message); err == nil || err.Error() != "Amazon SES rejected the message" {
		t.Fatalf("unsafe or missing SES error: %v", err)
	}
}
