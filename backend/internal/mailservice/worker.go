package mailservice

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

type DeliveryRepository interface {
	ClaimMailDelivery(context.Context, string) (store.MailDelivery, error)
	CompleteMailDelivery(context.Context, uuid.UUID) error
	RetryMailDelivery(context.Context, uuid.UUID, bool, time.Time, string) error
	SetSystemMailHealth(context.Context, uuid.UUID, bool, string) error
}

type ProviderFactory func(Configuration) (mailprovider.Provider, error)

type Worker struct {
	Repository DeliveryRepository
	Box        *cryptoenvelope.Box
	Logger     *slog.Logger
	WorkerID   string
	Factory    ProviderFactory
	Interval   time.Duration
}

func (w Worker) Run(ctx context.Context) {
	interval := w.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		for w.RunOnce(ctx) {
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w Worker) RunOnce(ctx context.Context) bool {
	if w.Repository == nil || w.Box == nil {
		return false
	}
	delivery, err := w.Repository.ClaimMailDelivery(ctx, w.workerID())
	if errors.Is(err, store.ErrNotFound) {
		return false
	}
	if err != nil {
		w.log().Error("mail delivery claim failed", "error", safeWorkerError(err))
		return false
	}
	credentialsJSON, err := w.Box.Open(delivery.EncryptedCredentials, "system-mail-config:"+delivery.ConfigurationID.String())
	if err != nil {
		w.fail(ctx, delivery, errors.New("mail credentials could not be decrypted"), true)
		return true
	}
	defer zero(credentialsJSON)
	messageJSON, err := w.Box.Open(delivery.EncryptedMessage, "mail-delivery:"+delivery.ID.String())
	if err != nil {
		w.fail(ctx, delivery, errors.New("mail message could not be decrypted"), true)
		return true
	}
	defer zero(messageJSON)
	settings, err := ParseSettings(delivery.Settings)
	if err != nil {
		w.fail(ctx, delivery, errors.New("mail settings are invalid"), true)
		return true
	}
	credentials, err := ParseCredentials(credentialsJSON)
	if err != nil {
		w.fail(ctx, delivery, errors.New("mail credentials are invalid"), true)
		return true
	}
	var message mailprovider.Message
	if err = json.Unmarshal(messageJSON, &message); err != nil {
		w.fail(ctx, delivery, errors.New("mail message is invalid"), true)
		return true
	}
	factory := w.Factory
	if factory == nil {
		factory = Provider
	}
	provider, err := factory(Configuration{Provider: delivery.Provider, FromName: delivery.FromName, FromAddress: delivery.FromAddress, ReplyTo: delivery.ReplyTo, Settings: settings, Credentials: credentials})
	if err == nil {
		err = provider.Send(ctx, message)
	}
	if err != nil {
		terminal := delivery.Attempts >= 5 || errors.Is(err, mailprovider.ErrRejected)
		w.fail(ctx, delivery, err, terminal, credentials.AccessKeyID, credentials.Secret, credentials.SessionToken)
		return true
	}
	if err = w.Repository.CompleteMailDelivery(ctx, delivery.ID); err != nil {
		w.log().Error("mail delivery completion failed", "delivery_id", delivery.ID, "error", safeWorkerError(err))
		return true
	}
	_ = w.Repository.SetSystemMailHealth(ctx, delivery.ConfigurationID, true, "")
	w.log().Info("mail delivery completed", "delivery_id", delivery.ID, "provider", delivery.Provider, "purpose", delivery.Purpose)
	return true
}

func (w Worker) fail(ctx context.Context, delivery store.MailDelivery, failure error, terminal bool, sensitive ...string) {
	safe := safeWorkerError(failure, sensitive...)
	if err := w.Repository.RetryMailDelivery(ctx, delivery.ID, terminal, time.Now().Add(retryDelay(delivery.Attempts)), safe); err != nil {
		w.log().Error("mail delivery state update failed", "delivery_id", delivery.ID, "error", safeWorkerError(err))
	}
	_ = w.Repository.SetSystemMailHealth(ctx, delivery.ConfigurationID, false, safe)
	w.log().Warn("mail delivery failed", "delivery_id", delivery.ID, "provider", delivery.Provider, "purpose", delivery.Purpose, "terminal", terminal, "error", safe)
}

func (w Worker) workerID() string {
	if strings.TrimSpace(w.WorkerID) == "" {
		return "silicon-mail"
	}
	return w.WorkerID
}

func (w Worker) log() *slog.Logger {
	if w.Logger == nil {
		return slog.Default()
	}
	return w.Logger
}

func retryDelay(attempt int) time.Duration {
	switch attempt {
	case 0, 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	case 3:
		return 30 * time.Minute
	default:
		return 2 * time.Hour
	}
}

func safeWorkerError(err error, sensitive ...string) string {
	if err == nil {
		return ""
	}
	value := strings.TrimSpace(err.Error())
	for _, secret := range sensitive {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[redacted]")
		}
	}
	if len(value) > 240 {
		value = value[:240]
	}
	return value
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
