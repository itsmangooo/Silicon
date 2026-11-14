package store

import (
	"context"
	"net"

	"github.com/google/uuid"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
)

func (r Repository) ApplicationRuntimeSettings(ctx context.Context, org, app uuid.UUID) (runtimeprovider.Settings, error) {
	var settings runtimeprovider.Settings
	err := r.Pool.QueryRow(ctx, `SELECT runtime_settings FROM applications WHERE organization_id=$1 AND id=$2`, org, app).Scan(&settings)
	return settings, notFound(err)
}

func (r Repository) SetApplicationRuntimeSettings(ctx context.Context, org, app, actor uuid.UUID, settings runtimeprovider.Settings, requestID uuid.UUID, ip net.IP) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE applications SET runtime_settings=$3 WHERE organization_id=$1 AND id=$2`, org, app, settings)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := insertAudit(ctx, tx, &org, &actor, "application.runtime_settings_updated", "application", &app, requestID, nil, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
