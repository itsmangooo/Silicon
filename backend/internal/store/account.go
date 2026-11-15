package store

import (
	"context"
	"net"
	"time"

	"github.com/google/uuid"
)

func (r Repository) AccountAppearance(ctx context.Context, userID uuid.UUID) (Appearance, bool, bool, error) {
	var value Appearance
	var avatar, background bool
	err := r.Pool.QueryRow(ctx, `SELECT appearance_style,appearance_mode,appearance_density,appearance_studio,avatar IS NOT NULL,appearance_background IS NOT NULL FROM users WHERE id=$1`, userID).
		Scan(&value.Style, &value.Mode, &value.Density, &value.Studio, &avatar, &background)
	return value, avatar, background, notFound(err)
}

func (r Repository) SetAccountAppearance(ctx context.Context, userID uuid.UUID, value Appearance, replaceBackground bool, background []byte) error {
	_, err := r.Pool.Exec(ctx, `UPDATE users SET appearance_style=$2,appearance_mode=$3,appearance_density=$4,appearance_studio=COALESCE($5,appearance_studio), appearance_background=CASE WHEN $6 THEN $7 ELSE appearance_background END WHERE id=$1`, userID, value.Style, value.Mode, value.Density, value.Studio, replaceBackground, background)
	return err
}

func (r Repository) AccountBackground(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	var image []byte
	err := r.Pool.QueryRow(ctx, `SELECT appearance_background FROM users WHERE id=$1 AND appearance_background IS NOT NULL`, userID).Scan(&image)
	return image, notFound(err)
}

func (r Repository) SetAccountBackground(ctx context.Context, userID uuid.UUID, image []byte) error {
	_, err := r.Pool.Exec(ctx, `UPDATE users SET appearance_background=$2 WHERE id=$1`, userID, image)
	return err
}

func (r Repository) SetDisplayName(ctx context.Context, userID uuid.UUID, name string) error {
	_, err := r.Pool.Exec(ctx, `UPDATE users SET display_name=$2 WHERE id=$1`, userID, name)
	return err
}

func (r Repository) AccountAvatar(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	var avatar []byte
	err := r.Pool.QueryRow(ctx, `SELECT avatar FROM users WHERE id=$1 AND avatar IS NOT NULL`, userID).Scan(&avatar)
	return avatar, notFound(err)
}

func (r Repository) SetAccountAvatar(ctx context.Context, userID uuid.UUID, avatar []byte) error {
	_, err := r.Pool.Exec(ctx, `UPDATE users SET avatar=$2 WHERE id=$1`, userID, avatar)
	return err
}

func (r Repository) ChangeAccountPassword(ctx context.Context, userID uuid.UUID, previous, next string, requestID uuid.UUID, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `UPDATE users SET password_hash=$3 WHERE id=$1 AND password_hash=$2 AND status='active'`, userID, previous, next)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrConflict
	}
	if _, err = tx.Exec(ctx, `DELETE FROM account_email_changes WHERE user_id=$1`, userID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE password_reset_tokens SET superseded_at=now() WHERE user_id=$1 AND used_at IS NULL AND superseded_at IS NULL`, userID); err != nil {
		return err
	}
	if err = insertAudit(ctx, tx, nil, &userID, "auth.password_changed", "user", &userID, requestID, nil, ip); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO account_notifications(user_id,kind) VALUES($1,'password_changed')`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) QueueAccountEmailChange(ctx context.Context, userID, deliveryID, configurationID uuid.UUID, email, provider string, hash, message []byte, expiresAt time.Time) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO account_email_changes(user_id,email,token_hash,expires_at) VALUES($1,$2,$3,$4)
		ON CONFLICT (user_id) DO UPDATE SET email=excluded.email,token_hash=excluded.token_hash,expires_at=excluded.expires_at`, userID, email, hash, expiresAt); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO mail_deliveries(id,configuration_id,purpose,recipient,provider,encrypted_message)
		VALUES($1,$2,'account_email_verification',$3,$4,$5)`, deliveryID, configurationID, email, provider, message); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) VerifyAccountEmail(ctx context.Context, userID uuid.UUID, hash []byte, requestID uuid.UUID, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var email string
	err = tx.QueryRow(ctx, `DELETE FROM account_email_changes WHERE user_id=$1 AND token_hash=$2 AND expires_at>now() RETURNING email`, userID, hash).Scan(&email)
	if err != nil {
		return notFound(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET email=$2 WHERE id=$1`, userID, email); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE password_reset_tokens SET superseded_at=now() WHERE user_id=$1 AND used_at IS NULL AND superseded_at IS NULL`, userID); err != nil {
		return err
	}
	if err = insertAudit(ctx, tx, nil, &userID, "auth.email_changed", "user", &userID, requestID, nil, ip); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO account_notifications(user_id,kind) VALUES($1,'email_changed')`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
