package mailservice

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

func TestSMTPPresetsRemainConfigurationHelpers(t *testing.T) {
	for _, preset := range []string{"billionmail", "stalwart", "mailcow", "postal", "generic"} {
		settings := ApplySMTPPreset(Settings{SMTPPreset: preset})
		if settings.SMTPPreset != preset || settings.SMTPPort != 587 || settings.SMTPEncryption != "starttls" {
			t.Fatalf("unexpected %s preset: %#v", preset, settings)
		}
	}
	settings := ApplySMTPPreset(Settings{SMTPPreset: "billionmail", SMTPEncryption: "tls"})
	if settings.SMTPPort != 465 {
		t.Fatalf("implicit TLS should default to port 465: %#v", settings)
	}
}

func TestProviderValidationIsSpecific(t *testing.T) {
	base := Configuration{FromName: "Silicon", FromAddress: "silicon@example.com", Credentials: Credentials{Secret: "secret"}}
	cases := []struct {
		name   string
		config Configuration
	}{
		{"resend", withProvider(base, "resend", Settings{})},
		{"postmark", withProvider(base, "postmark", Settings{})},
		{"mailgun", withProvider(base, "mailgun", Settings{MailgunDomain: "mg.example.com"})},
		{"ses", withCredentials(withProvider(base, "ses", Settings{SESRegion: "eu-central-1"}), Credentials{AccessKeyID: "access", Secret: "secret"})},
		{"smtp", withProvider(base, "smtp", Settings{SMTPHost: "mail.example.com", SMTPPort: 587, SMTPEncryption: "starttls"})},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := Validate(test.config); err != nil {
				t.Fatal(err)
			}
		})
	}
	bad := withProvider(base, "smtp", Settings{SMTPHost: "mail.example.com", SMTPPort: 70000, SMTPEncryption: "starttls"})
	if err := Validate(bad); err == nil || err.Error() != "SMTP host and port are invalid" {
		t.Fatalf("unexpected validation error: %v", err)
	}
	unauthenticatedSMTP := withCredentials(withProvider(base, "smtp", Settings{SMTPHost: "relay.internal", SMTPPort: 25, SMTPEncryption: "none"}), Credentials{})
	if err := Validate(unauthenticatedSMTP); err != nil {
		t.Fatalf("trusted unauthenticated SMTP relay should be valid: %v", err)
	}
	authenticatedWithoutPassword := withCredentials(withProvider(base, "smtp", Settings{SMTPHost: "mail.example.com", SMTPPort: 587, SMTPUsername: "silicon", SMTPEncryption: "starttls"}), Credentials{})
	if err := Validate(authenticatedWithoutPassword); err == nil || err.Error() != "SMTP password is required when a username is configured" {
		t.Fatalf("unexpected authenticated SMTP validation: %v", err)
	}
}

func withCredentials(configuration Configuration, credentials Credentials) Configuration {
	configuration.Credentials = credentials
	return configuration
}

func withProvider(configuration Configuration, provider string, settings Settings) Configuration {
	configuration.Provider = provider
	configuration.Settings = settings
	return configuration
}

type fakeDeliveryRepository struct {
	delivery  store.MailDelivery
	claimed   bool
	completed bool
	retried   bool
	healthy   bool
	terminal  bool
	safeError string
}

func (r *fakeDeliveryRepository) ClaimMailDelivery(context.Context, string) (store.MailDelivery, error) {
	if r.claimed {
		return store.MailDelivery{}, store.ErrNotFound
	}
	r.claimed = true
	return r.delivery, nil
}
func (r *fakeDeliveryRepository) CompleteMailDelivery(context.Context, uuid.UUID) error {
	r.completed = true
	return nil
}
func (r *fakeDeliveryRepository) RetryMailDelivery(_ context.Context, _ uuid.UUID, terminal bool, _ time.Time, safeError string) error {
	r.retried, r.terminal, r.safeError = true, terminal, safeError
	return nil
}
func (r *fakeDeliveryRepository) SetSystemMailHealth(_ context.Context, _ uuid.UUID, healthy bool, _ string) error {
	r.healthy = healthy
	return nil
}

type fakeMailProvider struct {
	message mailprovider.Message
	err     error
}

func (p *fakeMailProvider) Send(_ context.Context, message mailprovider.Message) error {
	p.message = message
	return p.err
}
func (*fakeMailProvider) Test(context.Context) error              { return nil }
func (*fakeMailProvider) Capabilities() mailprovider.Capabilities { return mailprovider.Capabilities{} }

func TestWorkerDecryptsAndDeliversPersistedMessage(t *testing.T) {
	key := make([]byte, 32)
	box, err := cryptoenvelope.New(key)
	if err != nil {
		t.Fatal(err)
	}
	configurationID, deliveryID := uuid.New(), uuid.New()
	credentials, _ := json.Marshal(Credentials{Secret: "must-not-leak"})
	message, _ := json.Marshal(mailprovider.Message{FromName: "Silicon", From: "silicon@example.com", To: "user@example.com", Subject: "Test", Text: "body"})
	encryptedCredentials, _ := box.Seal(credentials, "system-mail-config:"+configurationID.String())
	encryptedMessage, _ := box.Seal(message, "mail-delivery:"+deliveryID.String())
	repository := &fakeDeliveryRepository{delivery: store.MailDelivery{ID: deliveryID, ConfigurationID: configurationID, Provider: "resend", Purpose: "test", Recipient: "user@example.com", EncryptedCredentials: encryptedCredentials, EncryptedMessage: encryptedMessage, Settings: json.RawMessage(`{}`), FromName: "Silicon", FromAddress: "silicon@example.com", Attempts: 1}}
	provider := &fakeMailProvider{}
	worker := Worker{Repository: repository, Box: &box, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Factory: func(Configuration) (mailprovider.Provider, error) { return provider, nil }}
	if !worker.RunOnce(context.Background()) || !repository.completed || !repository.healthy {
		t.Fatalf("delivery did not complete: %#v", repository)
	}
	if provider.message.To != "user@example.com" || provider.message.Subject != "Test" {
		t.Fatalf("wrong message delivered: %#v", provider.message)
	}
}

func TestWorkerRetriesTransientAndStopsRejectedDelivery(t *testing.T) {
	for _, test := range []struct {
		name     string
		failure  error
		attempts int
		terminal bool
	}{
		{"transient", errors.New("network unavailable"), 1, false},
		{"rejected", mailprovider.ErrRejected, 1, true},
		{"max attempts", errors.New("network unavailable"), 5, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			key := make([]byte, 32)
			box, _ := cryptoenvelope.New(key)
			configurationID, deliveryID := uuid.New(), uuid.New()
			credentials, _ := json.Marshal(Credentials{Secret: "secret"})
			message, _ := json.Marshal(mailprovider.Message{From: "from@example.com", To: "to@example.com", Subject: "subject", Text: "text"})
			encryptedCredentials, _ := box.Seal(credentials, "system-mail-config:"+configurationID.String())
			encryptedMessage, _ := box.Seal(message, "mail-delivery:"+deliveryID.String())
			repository := &fakeDeliveryRepository{delivery: store.MailDelivery{ID: deliveryID, ConfigurationID: configurationID, Provider: "resend", Attempts: test.attempts, EncryptedCredentials: encryptedCredentials, EncryptedMessage: encryptedMessage, Settings: json.RawMessage(`{}`), FromAddress: "from@example.com"}}
			worker := Worker{Repository: repository, Box: &box, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Factory: func(Configuration) (mailprovider.Provider, error) { return &fakeMailProvider{err: test.failure}, nil }}
			worker.RunOnce(context.Background())
			if !repository.retried || repository.terminal != test.terminal || repository.healthy {
				t.Fatalf("unexpected retry state: %#v", repository)
			}
		})
	}
}

func TestWorkerRedactsCredentialFromProviderFailure(t *testing.T) {
	key := make([]byte, 32)
	box, _ := cryptoenvelope.New(key)
	configurationID, deliveryID := uuid.New(), uuid.New()
	credentials, _ := json.Marshal(Credentials{AccessKeyID: "access-id-secret", Secret: "provider-secret", SessionToken: "session-secret"})
	message, _ := json.Marshal(mailprovider.Message{From: "from@example.com", To: "to@example.com", Subject: "subject", Text: "text"})
	encryptedCredentials, _ := box.Seal(credentials, "system-mail-config:"+configurationID.String())
	encryptedMessage, _ := box.Seal(message, "mail-delivery:"+deliveryID.String())
	repository := &fakeDeliveryRepository{delivery: store.MailDelivery{ID: deliveryID, ConfigurationID: configurationID, Provider: "resend", Attempts: 1, EncryptedCredentials: encryptedCredentials, EncryptedMessage: encryptedMessage, Settings: json.RawMessage(`{}`), FromAddress: "from@example.com"}}
	worker := Worker{Repository: repository, Box: &box, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Factory: func(Configuration) (mailprovider.Provider, error) {
		return &fakeMailProvider{err: errors.New("provider-secret access-id-secret session-secret")}, nil
	}}
	worker.RunOnce(context.Background())
	if strings.Contains(repository.safeError, "provider-secret") || strings.Contains(repository.safeError, "access-id-secret") || strings.Contains(repository.safeError, "session-secret") || strings.Count(repository.safeError, "[redacted]") != 3 {
		t.Fatalf("provider credential reached persisted/loggable error: %q", repository.safeError)
	}
}
