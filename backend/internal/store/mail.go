package store

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"github.com/google/uuid"
)

const passwordResetWindow = 15 * time.Minute

type SystemMailConfiguration struct {
	ID                   uuid.UUID       `json:"id"`
	Provider             string          `json:"provider"`
	FromName             string          `json:"fromName"`
	FromAddress          string          `json:"fromAddress"`
	ReplyTo              string          `json:"replyTo"`
	Settings             json.RawMessage `json:"settings"`
	CredentialConfigured bool            `json:"credentialConfigured"`
	Status               string          `json:"status"`
	SafeLastError        string          `json:"lastError,omitempty"`
	LastSuccessAt        *time.Time      `json:"lastSuccessAt,omitempty"`
	LastFailureAt        *time.Time      `json:"lastFailureAt,omitempty"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
	EncryptedCredentials []byte          `json:"-"`
}

type SystemMailConfigurationInput struct {
	ID                   uuid.UUID
	Provider             string
	FromName             string
	FromAddress          string
	ReplyTo              string
	Settings             json.RawMessage
	EncryptedCredentials []byte
	CreatedBy            uuid.UUID
}

type MailDelivery struct {
	ID                   uuid.UUID
	ConfigurationID      uuid.UUID
	Purpose              string
	Recipient            string
	Provider             string
	EncryptedMessage     []byte
	Status               string
	Attempts             int
	EncryptedCredentials []byte
	Settings             json.RawMessage
	FromName             string
	FromAddress          string
	ReplyTo              string
}

func (r Repository) ActiveSystemMailConfiguration(ctx context.Context) (SystemMailConfiguration, error) {
	return r.systemMailConfiguration(ctx, `WHERE active=true`)
}

func (r Repository) SystemMailConfigurationByID(ctx context.Context, id uuid.UUID) (SystemMailConfiguration, error) {
	return r.systemMailConfiguration(ctx, `WHERE id=$1`, id)
}

func (r Repository) systemMailConfiguration(ctx context.Context, where string, args ...any) (SystemMailConfiguration, error) {
	var item SystemMailConfiguration
	err := r.Pool.QueryRow(ctx, `SELECT id,provider,from_name,from_address,COALESCE(reply_to::text,''),settings,octet_length(encrypted_credentials)>0,status,safe_last_error,last_success_at,last_failure_at,created_at,updated_at,encrypted_credentials FROM system_mail_configurations `+where, args...).Scan(
		&item.ID, &item.Provider, &item.FromName, &item.FromAddress, &item.ReplyTo, &item.Settings, &item.CredentialConfigured,
		&item.Status, &item.SafeLastError, &item.LastSuccessAt, &item.LastFailureAt, &item.CreatedAt, &item.UpdatedAt, &item.EncryptedCredentials,
	)
	return item, notFound(err)
}

func (r Repository) SaveSystemMailConfiguration(ctx context.Context, input SystemMailConfigurationInput) (SystemMailConfiguration, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return SystemMailConfiguration{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE system_mail_configurations SET active=false,updated_at=now() WHERE active=true AND id<>$1`, input.ID); err != nil {
		return SystemMailConfiguration{}, err
	}
	var item SystemMailConfiguration
	err = tx.QueryRow(ctx, `INSERT INTO system_mail_configurations(id,provider,from_name,from_address,reply_to,settings,encrypted_credentials,active,created_by)
		VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,true,$8)
		ON CONFLICT(id) DO UPDATE SET provider=EXCLUDED.provider,from_name=EXCLUDED.from_name,from_address=EXCLUDED.from_address,reply_to=EXCLUDED.reply_to,settings=EXCLUDED.settings,encrypted_credentials=EXCLUDED.encrypted_credentials,active=true,status='not_tested',safe_last_error='',updated_at=now()
		RETURNING id,provider,from_name,from_address,COALESCE(reply_to::text,''),settings,octet_length(encrypted_credentials)>0,status,safe_last_error,last_success_at,last_failure_at,created_at,updated_at,encrypted_credentials`,
		input.ID, input.Provider, input.FromName, input.FromAddress, input.ReplyTo, input.Settings, input.EncryptedCredentials, input.CreatedBy).Scan(
		&item.ID, &item.Provider, &item.FromName, &item.FromAddress, &item.ReplyTo, &item.Settings, &item.CredentialConfigured,
		&item.Status, &item.SafeLastError, &item.LastSuccessAt, &item.LastFailureAt, &item.CreatedAt, &item.UpdatedAt, &item.EncryptedCredentials,
	)
	if err != nil {
		return SystemMailConfiguration{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SystemMailConfiguration{}, err
	}
	return item, nil
}

func (r Repository) SetSystemMailHealth(ctx context.Context, id uuid.UUID, healthy bool, safeError string) error {
	if healthy {
		_, err := r.Pool.Exec(ctx, `UPDATE system_mail_configurations SET status='healthy',safe_last_error='',last_success_at=now(),updated_at=now() WHERE id=$1`, id)
		return err
	}
	_, err := r.Pool.Exec(ctx, `UPDATE system_mail_configurations SET status='failed',safe_last_error=$2,last_failure_at=now(),updated_at=now() WHERE id=$1`, id, safeError)
	return err
}

func (r Repository) QueueMailDelivery(ctx context.Context, id, configurationID uuid.UUID, purpose, recipient, provider string, encryptedMessage []byte) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO mail_deliveries(id,configuration_id,purpose,recipient,provider,encrypted_message) VALUES($1,$2,$3,$4,$5,$6)`, id, configurationID, purpose, recipient, provider, encryptedMessage)
	return err
}

func (r Repository) AllowPasswordResetRequest(ctx context.Context, emailHash, ipHash []byte) (bool, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext(encode($1,'hex'))),pg_advisory_xact_lock(hashtext(encode($2,'hex')))`, emailHash, ipHash); err != nil {
		return false, err
	}
	cutoff := time.Now().Add(-passwordResetWindow)
	var emailCount, ipCount int
	if err = tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE email_hash=$1),count(*) FILTER (WHERE ip_hash=$2) FROM password_reset_request_limits WHERE created_at>$3`, emailHash, ipHash, cutoff).Scan(&emailCount, &ipCount); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM password_reset_request_limits WHERE created_at<now()-interval '24 hours'`); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO password_reset_request_limits(email_hash,ip_hash) VALUES($1,$2)`, emailHash, ipHash); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return emailCount < 3 && ipCount < 10, nil
}

func (r Repository) CreatePasswordResetDelivery(ctx context.Context, userID uuid.UUID, tokenHash, requestedIPHash []byte, expiresAt time.Time, deliveryID, configurationID uuid.UUID, recipient, provider string, encryptedMessage []byte) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE password_reset_tokens SET superseded_at=now() WHERE user_id=$1 AND used_at IS NULL AND superseded_at IS NULL`, userID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO password_reset_tokens(user_id,token_hash,expires_at,requested_ip_hash) VALUES($1,$2,$3,$4)`, userID, tokenHash, expiresAt, requestedIPHash); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO mail_deliveries(id,configuration_id,purpose,recipient,provider,encrypted_message) VALUES($1,$2,'password_reset',$3,$4,$5)`, deliveryID, configurationID, recipient, provider, encryptedMessage); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) RecordPasswordResetRequest(ctx context.Context, userID, requestID uuid.UUID, ip net.IP) error {
	return insertAudit(ctx, r.Pool, nil, nil, "auth.password_reset_requested", "user", &userID, requestID, nil, ip)
}

func (r Repository) ClaimMailDelivery(ctx context.Context, workerID string) (MailDelivery, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return MailDelivery{}, err
	}
	defer tx.Rollback(ctx)
	var item MailDelivery
	err = tx.QueryRow(ctx, `SELECT d.id,d.configuration_id,d.purpose,d.recipient,d.provider,d.encrypted_message,d.status,d.attempts,c.encrypted_credentials,c.settings,c.from_name,c.from_address,COALESCE(c.reply_to::text,'')
		FROM mail_deliveries d JOIN system_mail_configurations c ON c.id=d.configuration_id
		WHERE (d.status='queued' OR (d.status='sending' AND d.locked_at<now()-interval '10 minutes')) AND d.available_at<=now()
		ORDER BY d.created_at FOR UPDATE OF d SKIP LOCKED LIMIT 1`).Scan(
		&item.ID, &item.ConfigurationID, &item.Purpose, &item.Recipient, &item.Provider, &item.EncryptedMessage, &item.Status, &item.Attempts,
		&item.EncryptedCredentials, &item.Settings, &item.FromName, &item.FromAddress, &item.ReplyTo,
	)
	if err != nil {
		return MailDelivery{}, notFound(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE mail_deliveries SET status='sending',attempts=attempts+1,locked_at=now(),locked_by=$2,updated_at=now() WHERE id=$1`, item.ID, workerID); err != nil {
		return MailDelivery{}, err
	}
	item.Attempts++
	if err = tx.Commit(ctx); err != nil {
		return MailDelivery{}, err
	}
	return item, nil
}

func (r Repository) CompleteMailDelivery(ctx context.Context, id uuid.UUID) error {
	_, err := r.Pool.Exec(ctx, `UPDATE mail_deliveries SET status='sent',sent_at=now(),locked_at=NULL,locked_by=NULL,safe_last_error='',updated_at=now() WHERE id=$1`, id)
	return err
}

func (r Repository) RetryMailDelivery(ctx context.Context, id uuid.UUID, terminal bool, availableAt time.Time, safeError string) error {
	status := "queued"
	if terminal {
		status = "failed"
	}
	_, err := r.Pool.Exec(ctx, `UPDATE mail_deliveries SET status=$2,available_at=$3,locked_at=NULL,locked_by=NULL,safe_last_error=$4,updated_at=now() WHERE id=$1`, id, status, availableAt, safeError)
	return err
}

func (r Repository) ConsumePasswordResetToken(ctx context.Context, tokenHash []byte, passwordHash string, requestID uuid.UUID, ip net.IP) (uuid.UUID, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var userID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT user_id FROM password_reset_tokens WHERE token_hash=$1 AND used_at IS NULL AND superseded_at IS NULL AND expires_at>now() FOR UPDATE`, tokenHash).Scan(&userID)
	if err != nil {
		return uuid.Nil, notFound(err)
	}
	result, err := tx.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1 AND status='active'`, userID, passwordHash)
	if err != nil {
		return uuid.Nil, err
	}
	if result.RowsAffected() != 1 {
		return uuid.Nil, ErrNotFound
	}
	if _, err = tx.Exec(ctx, `UPDATE password_reset_tokens SET used_at=now() WHERE user_id=$1 AND used_at IS NULL`, userID); err != nil {
		return uuid.Nil, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID); err != nil {
		return uuid.Nil, err
	}
	if err = insertAudit(ctx, tx, nil, &userID, "auth.password_reset_completed", "user", &userID, requestID, nil, ip); err != nil {
		return uuid.Nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

func (r Repository) AdminResetPassword(ctx context.Context, email, passwordHash string, requestID uuid.UUID) (uuid.UUID, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var userID uuid.UUID
	if err = tx.QueryRow(ctx, `UPDATE users SET password_hash=$2 WHERE email=$1 AND status='active' RETURNING id`, email, passwordHash).Scan(&userID); err != nil {
		return uuid.Nil, notFound(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID); err != nil {
		return uuid.Nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE password_reset_tokens SET used_at=COALESCE(used_at,now()) WHERE user_id=$1`, userID); err != nil {
		return uuid.Nil, err
	}
	if err = insertAudit(ctx, tx, nil, &userID, "auth.password_reset_admin", "user", &userID, requestID, map[string]any{"method": "host_cli"}, nil); err != nil {
		return uuid.Nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}
