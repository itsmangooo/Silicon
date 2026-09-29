package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SystemUpdate struct {
	ID            uuid.UUID  `json:"id"`
	RequestedBy   *uuid.UUID `json:"requestedBy"`
	FromVersion   string     `json:"fromVersion"`
	TargetVersion string     `json:"targetVersion"`
	Status        string     `json:"status"`
	Message       string     `json:"message"`
	ReleaseNotes  string     `json:"releaseNotes"`
	StartedAt     *time.Time `json:"startedAt"`
	CompletedAt   *time.Time `json:"completedAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

const systemUpdateColumns = `id,requested_by,from_version,target_version,status,message,release_notes,started_at,completed_at,created_at,updated_at`

func scanSystemUpdate(row pgx.Row) (SystemUpdate, error) {
	var item SystemUpdate
	err := row.Scan(&item.ID, &item.RequestedBy, &item.FromVersion, &item.TargetVersion, &item.Status, &item.Message, &item.ReleaseNotes, &item.StartedAt, &item.CompletedAt, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func (r Repository) LatestSystemUpdate(ctx context.Context) (SystemUpdate, error) {
	item, err := scanSystemUpdate(r.Pool.QueryRow(ctx, `SELECT `+systemUpdateColumns+` FROM system_updates ORDER BY created_at DESC LIMIT 1`))
	return item, notFound(err)
}

func (r Repository) CreateSystemUpdate(ctx context.Context, requestedBy uuid.UUID, fromVersion, targetVersion, releaseNotes string) (SystemUpdate, error) {
	return scanSystemUpdate(r.Pool.QueryRow(ctx, `INSERT INTO system_updates(requested_by,from_version,target_version,release_notes) VALUES($1,$2,$3,$4) RETURNING `+systemUpdateColumns, requestedBy, fromVersion, targetVersion, releaseNotes))
}

func (r Repository) ClaimSystemUpdate(ctx context.Context) (SystemUpdate, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return SystemUpdate{}, err
	}
	defer tx.Rollback(ctx)
	item, err := scanSystemUpdate(tx.QueryRow(ctx, `SELECT `+systemUpdateColumns+` FROM system_updates WHERE status='queued' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`))
	if err != nil {
		return SystemUpdate{}, notFound(err)
	}
	item, err = scanSystemUpdate(tx.QueryRow(ctx, `UPDATE system_updates SET status='checking',message='Verifying the exact GitHub release',started_at=COALESCE(started_at,now()),updated_at=now() WHERE id=$1 RETURNING `+systemUpdateColumns, item.ID))
	if err != nil {
		return SystemUpdate{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SystemUpdate{}, err
	}
	return item, nil
}

func (r Repository) RequeueInterruptedSystemUpdates(ctx context.Context) error {
	_, err := r.Pool.Exec(ctx, `UPDATE system_updates
		SET status='queued',message='The update runner restarted; safely retrying the exact release.',started_at=NULL,updated_at=now()
		WHERE status IN ('checking','preparing','updating','migrating','restarting','waiting_for_health')`)
	return err
}

func (r Repository) SetSystemUpdateStatus(ctx context.Context, id uuid.UUID, status, message string) error {
	completed := status == "completed" || status == "failed"
	command := `UPDATE system_updates SET status=$2,message=$3,updated_at=now(),completed_at=CASE WHEN $4 THEN now() ELSE completed_at END WHERE id=$1`
	result, err := r.Pool.Exec(ctx, command, id, status, message, completed)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}
