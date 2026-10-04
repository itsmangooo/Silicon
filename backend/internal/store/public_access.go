package store

import (
	"context"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SystemPublicAccess struct {
	OrganizationID              uuid.UUID `json:"organizationId"`
	IntegrationID               uuid.UUID `json:"integrationId"`
	ZoneID                      uuid.UUID `json:"zoneId"`
	TunnelID                    uuid.UUID `json:"tunnelId"`
	Hostname                    string    `json:"hostname"`
	LocalOrigin                 string    `json:"localOrigin"`
	ProviderRecordID            string    `json:"-"`
	PreviousPublicURL           string    `json:"-"`
	PreviousCookieSecure        bool      `json:"-"`
	PreviousTrustForwardedProto bool      `json:"-"`
	PreviousBindAddress         string    `json:"-"`
	CreatedAt                   time.Time `json:"createdAt"`
	UpdatedAt                   time.Time `json:"updatedAt"`
}

type SystemPublicAccessOperation struct {
	ID                          uuid.UUID  `json:"id"`
	Action                      string     `json:"action"`
	Status                      string     `json:"status"`
	RequestedBy                 *uuid.UUID `json:"requestedBy,omitempty"`
	OrganizationID              *uuid.UUID `json:"organizationId,omitempty"`
	IntegrationID               *uuid.UUID `json:"integrationId,omitempty"`
	ZoneID                      *uuid.UUID `json:"zoneId,omitempty"`
	TunnelID                    *uuid.UUID `json:"tunnelId,omitempty"`
	Hostname                    string     `json:"hostname"`
	LocalOrigin                 string     `json:"localOrigin"`
	ProviderRecordID            string     `json:"-"`
	PreviousPublicURL           string     `json:"-"`
	PreviousCookieSecure        *bool      `json:"-"`
	PreviousTrustForwardedProto *bool      `json:"-"`
	PreviousBindAddress         *string    `json:"-"`
	Message                     string     `json:"message"`
	FailedStage                 string     `json:"failedStage,omitempty"`
	CreatedAt                   time.Time  `json:"createdAt"`
	StartedAt                   *time.Time `json:"startedAt,omitempty"`
	CompletedAt                 *time.Time `json:"completedAt,omitempty"`
	UpdatedAt                   time.Time  `json:"updatedAt"`
}

type PublicAccessResources struct {
	Integration                CloudflareIntegration
	Zone                       CloudflareZone
	Tunnel                     CloudflareTunnel
	TunnelServerConnectionType string
}

type PublicAccessConnectionOption struct {
	OrganizationID   uuid.UUID             `json:"organizationId"`
	OrganizationName string                `json:"organizationName"`
	Integration      CloudflareIntegration `json:"integration"`
	Zones            []CloudflareZone      `json:"zones"`
	Tunnels          []CloudflareTunnel    `json:"tunnels"`
}

func scanPublicAccess(row pgx.Row) (SystemPublicAccess, error) {
	var item SystemPublicAccess
	err := row.Scan(&item.OrganizationID, &item.IntegrationID, &item.ZoneID, &item.TunnelID,
		&item.Hostname, &item.LocalOrigin, &item.ProviderRecordID, &item.PreviousPublicURL,
		&item.PreviousCookieSecure, &item.PreviousTrustForwardedProto, &item.PreviousBindAddress,
		&item.CreatedAt, &item.UpdatedAt)
	return item, notFound(err)
}

func (r Repository) SystemPublicAccess(ctx context.Context) (SystemPublicAccess, error) {
	return scanPublicAccess(r.Pool.QueryRow(ctx, `SELECT organization_id,integration_id,zone_id,tunnel_id,hostname,local_origin,provider_record_id,previous_public_url,previous_cookie_secure,previous_trust_forwarded_proto,host(previous_bind_address),created_at,updated_at FROM system_public_access WHERE singleton=true`))
}

func scanPublicAccessOperation(row pgx.Row) (SystemPublicAccessOperation, error) {
	var item SystemPublicAccessOperation
	err := row.Scan(&item.ID, &item.Action, &item.Status, &item.RequestedBy, &item.OrganizationID,
		&item.IntegrationID, &item.ZoneID, &item.TunnelID, &item.Hostname, &item.LocalOrigin,
		&item.ProviderRecordID, &item.PreviousPublicURL, &item.PreviousCookieSecure, &item.PreviousTrustForwardedProto, &item.PreviousBindAddress,
		&item.Message, &item.FailedStage, &item.CreatedAt, &item.StartedAt,
		&item.CompletedAt, &item.UpdatedAt)
	return item, notFound(err)
}

func (r Repository) LatestSystemPublicAccessOperation(ctx context.Context) (SystemPublicAccessOperation, error) {
	return scanPublicAccessOperation(r.Pool.QueryRow(ctx, `SELECT id,action,status,requested_by,organization_id,integration_id,zone_id,tunnel_id,hostname,local_origin,provider_record_id,previous_public_url,previous_cookie_secure,previous_trust_forwarded_proto,host(previous_bind_address),message,failed_stage,created_at,started_at,completed_at,updated_at FROM system_public_access_operations ORDER BY created_at DESC LIMIT 1`))
}

func (r Repository) PublicAccessConnectionOptions(ctx context.Context, userID uuid.UUID) ([]PublicAccessConnectionOption, error) {
	rows, err := r.Pool.Query(ctx, `SELECT o.id,o.name,i.id,i.organization_id,i.account_id,i.status,i.last_error,i.last_checked_at,i.created_at,i.updated_at,i.encrypted_api_token FROM organizations o JOIN memberships m ON m.organization_id=o.id JOIN cloudflare_integrations i ON i.organization_id=o.id WHERE m.user_id=$1 AND m.role IN ('owner','admin') AND i.status='connected' ORDER BY o.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PublicAccessConnectionOption{}
	for rows.Next() {
		var item PublicAccessConnectionOption
		if err := rows.Scan(&item.OrganizationID, &item.OrganizationName, &item.Integration.ID,
			&item.Integration.OrganizationID, &item.Integration.AccountID, &item.Integration.Status,
			&item.Integration.LastError, &item.Integration.LastCheckedAt, &item.Integration.CreatedAt,
			&item.Integration.UpdatedAt, &item.Integration.EncryptedAPIToken); err != nil {
			return nil, err
		}
		item.Integration.EncryptedAPIToken = nil
		item.Zones, err = r.ListCloudflareZones(ctx, item.OrganizationID)
		if err != nil {
			return nil, err
		}
		item.Tunnels, err = r.ListTunnels(ctx, item.OrganizationID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) PublicAccessResources(ctx context.Context, orgID, integrationID, zoneID, tunnelID uuid.UUID) (PublicAccessResources, error) {
	var x PublicAccessResources
	err := r.Pool.QueryRow(ctx, `SELECT
		i.id,i.organization_id,i.account_id,i.status,i.last_error,i.last_checked_at,i.created_at,i.updated_at,i.encrypted_api_token,
		z.id,z.organization_id,z.integration_id,z.provider_zone_id,z.name,z.status,z.selected,
		t.id,t.organization_id,t.integration_id,t.provider_tunnel_id,t.name,t.ownership,t.status,t.encrypted_tunnel_token,t.server_id,t.installation_status,t.installation_error,t.created_at,
		s.connection_type
		FROM cloudflare_integrations i
		JOIN cloudflare_zones z ON z.integration_id=i.id AND z.organization_id=i.organization_id
		JOIN cloudflare_tunnels t ON t.integration_id=i.id AND t.organization_id=i.organization_id
		JOIN servers s ON s.id=t.server_id AND s.organization_id=t.organization_id
		WHERE i.organization_id=$1 AND i.id=$2 AND z.id=$3 AND t.id=$4`, orgID, integrationID, zoneID, tunnelID).Scan(
		&x.Integration.ID, &x.Integration.OrganizationID, &x.Integration.AccountID, &x.Integration.Status, &x.Integration.LastError, &x.Integration.LastCheckedAt, &x.Integration.CreatedAt, &x.Integration.UpdatedAt, &x.Integration.EncryptedAPIToken,
		&x.Zone.ID, &x.Zone.OrganizationID, &x.Zone.IntegrationID, &x.Zone.ProviderZoneID, &x.Zone.Name, &x.Zone.Status, &x.Zone.Selected,
		&x.Tunnel.ID, &x.Tunnel.OrganizationID, &x.Tunnel.IntegrationID, &x.Tunnel.ProviderTunnelID, &x.Tunnel.Name, &x.Tunnel.Ownership, &x.Tunnel.Status, &x.Tunnel.EncryptedTunnelToken, &x.Tunnel.ServerID, &x.Tunnel.InstallationStatus, &x.Tunnel.InstallationError, &x.Tunnel.CreatedAt,
		&x.TunnelServerConnectionType)
	return x, notFound(err)
}

func (r Repository) CreateSystemPublicAccessOperation(ctx context.Context, actorID, orgID, integrationID, zoneID, tunnelID uuid.UUID, hostname string, requestID uuid.UUID, ip net.IP) (SystemPublicAccessOperation, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return SystemPublicAccessOperation{}, err
	}
	defer tx.Rollback(ctx)
	var role string
	if err = tx.QueryRow(ctx, `SELECT role FROM memberships WHERE organization_id=$1 AND user_id=$2`, orgID, actorID).Scan(&role); err != nil || (role != "owner" && role != "admin") {
		return SystemPublicAccessOperation{}, ErrNotFound
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cloudflare_integrations i JOIN cloudflare_zones z ON z.integration_id=i.id AND z.organization_id=i.organization_id JOIN cloudflare_tunnels t ON t.integration_id=i.id AND t.organization_id=i.organization_id JOIN servers s ON s.id=t.server_id AND s.organization_id=t.organization_id WHERE i.organization_id=$1 AND i.id=$2 AND z.id=$3 AND t.id=$4 AND i.status='connected' AND z.status='active' AND t.installation_status='installed' AND s.connection_type='local')`, orgID, integrationID, zoneID, tunnelID).Scan(&exists); err != nil || !exists {
		return SystemPublicAccessOperation{}, ErrNotFound
	}
	var item SystemPublicAccessOperation
	err = tx.QueryRow(ctx, `INSERT INTO system_public_access_operations(action,requested_by,organization_id,integration_id,zone_id,tunnel_id,hostname,message) VALUES('configure',$1,$2,$3,$4,$5,$6,'Public access configuration queued.') RETURNING id,action,status,requested_by,organization_id,integration_id,zone_id,tunnel_id,hostname,local_origin,provider_record_id,previous_public_url,previous_cookie_secure,previous_trust_forwarded_proto,host(previous_bind_address),message,failed_stage,created_at,started_at,completed_at,updated_at`, actorID, orgID, integrationID, zoneID, tunnelID, hostname).Scan(&item.ID, &item.Action, &item.Status, &item.RequestedBy, &item.OrganizationID, &item.IntegrationID, &item.ZoneID, &item.TunnelID, &item.Hostname, &item.LocalOrigin, &item.ProviderRecordID, &item.PreviousPublicURL, &item.PreviousCookieSecure, &item.PreviousTrustForwardedProto, &item.PreviousBindAddress, &item.Message, &item.FailedStage, &item.CreatedAt, &item.StartedAt, &item.CompletedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	if err = insertAudit(ctx, tx, &orgID, &actorID, "system.public_access.configure_requested", "system_public_access", &item.ID, requestID, map[string]any{"hostname": hostname}, ip); err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) CreateDisableSystemPublicAccessOperation(ctx context.Context, actorID, requestID uuid.UUID, ip net.IP) (SystemPublicAccessOperation, error) {
	active, err := r.SystemPublicAccess(ctx)
	if err != nil {
		return SystemPublicAccessOperation{}, err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return SystemPublicAccessOperation{}, err
	}
	defer tx.Rollback(ctx)
	var item SystemPublicAccessOperation
	err = tx.QueryRow(ctx, `INSERT INTO system_public_access_operations(action,requested_by,organization_id,integration_id,zone_id,tunnel_id,hostname,message) VALUES('disable',$1,$2,$3,$4,$5,$6,'Public access disable queued.') RETURNING id,action,status,requested_by,organization_id,integration_id,zone_id,tunnel_id,hostname,local_origin,provider_record_id,previous_public_url,previous_cookie_secure,previous_trust_forwarded_proto,host(previous_bind_address),message,failed_stage,created_at,started_at,completed_at,updated_at`, actorID, active.OrganizationID, active.IntegrationID, active.ZoneID, active.TunnelID, active.Hostname).Scan(&item.ID, &item.Action, &item.Status, &item.RequestedBy, &item.OrganizationID, &item.IntegrationID, &item.ZoneID, &item.TunnelID, &item.Hostname, &item.LocalOrigin, &item.ProviderRecordID, &item.PreviousPublicURL, &item.PreviousCookieSecure, &item.PreviousTrustForwardedProto, &item.PreviousBindAddress, &item.Message, &item.FailedStage, &item.CreatedAt, &item.StartedAt, &item.CompletedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	if err = insertAudit(ctx, tx, &active.OrganizationID, &actorID, "system.public_access.disable_requested", "system_public_access", &item.ID, requestID, nil, ip); err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) RequeueInterruptedSystemPublicAccessOperations(ctx context.Context) error {
	_, err := r.Pool.Exec(ctx, `UPDATE system_public_access_operations SET status='pending',message='Operation resumed after helper restart.',updated_at=now() WHERE status IN ('validating','configuring_cloudflare','updating_configuration','restarting','waiting_for_health')`)
	return err
}

func (r Repository) ClaimSystemPublicAccessOperation(ctx context.Context) (SystemPublicAccessOperation, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return SystemPublicAccessOperation{}, err
	}
	defer tx.Rollback(ctx)
	item, err := scanPublicAccessOperation(tx.QueryRow(ctx, `SELECT id,action,status,requested_by,organization_id,integration_id,zone_id,tunnel_id,hostname,local_origin,provider_record_id,previous_public_url,previous_cookie_secure,previous_trust_forwarded_proto,host(previous_bind_address),message,failed_stage,created_at,started_at,completed_at,updated_at FROM system_public_access_operations WHERE status='pending' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`))
	if err != nil {
		return item, err
	}
	if _, err = tx.Exec(ctx, `UPDATE system_public_access_operations SET status='validating',message='Validating hostname and Cloudflare resources.',started_at=COALESCE(started_at,now()),updated_at=now() WHERE id=$1`, item.ID); err != nil {
		return item, err
	}
	item.Status = "validating"
	item.Message = "Validating hostname and Cloudflare resources."
	return item, tx.Commit(ctx)
}

func (r Repository) SetSystemPublicAccessOperationStatus(ctx context.Context, id uuid.UUID, status, message, failedStage, recordID, origin string) error {
	_, err := r.Pool.Exec(ctx, `UPDATE system_public_access_operations SET status=$2,message=$3,failed_stage=$4,provider_record_id=CASE WHEN $5<>'' THEN $5 ELSE provider_record_id END,local_origin=CASE WHEN $6<>'' THEN $6 ELSE local_origin END,completed_at=CASE WHEN $2 IN ('active','disabled','failed') THEN now() ELSE completed_at END,updated_at=now() WHERE id=$1`, id, status, message, failedStage, recordID, origin)
	return err
}

func (r Repository) SetSystemPublicAccessOperationSnapshot(ctx context.Context, id uuid.UUID, publicURL string, cookieSecure, forwarded bool, bind string) error {
	_, err := r.Pool.Exec(ctx, `UPDATE system_public_access_operations SET previous_public_url=$2,previous_cookie_secure=$3,previous_trust_forwarded_proto=$4,previous_bind_address=$5,updated_at=now() WHERE id=$1`, id, publicURL, cookieSecure, forwarded, bind)
	return err
}

func (r Repository) ActivateSystemPublicAccess(ctx context.Context, op SystemPublicAccessOperation, recordID, origin, previousURL string, previousSecure, previousForwarded bool, previousBind string) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO system_public_access(singleton,organization_id,integration_id,zone_id,tunnel_id,hostname,local_origin,provider_record_id,previous_public_url,previous_cookie_secure,previous_trust_forwarded_proto,previous_bind_address,configured_by) VALUES(true,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(singleton) DO UPDATE SET organization_id=excluded.organization_id,integration_id=excluded.integration_id,zone_id=excluded.zone_id,tunnel_id=excluded.tunnel_id,hostname=excluded.hostname,local_origin=excluded.local_origin,provider_record_id=excluded.provider_record_id,updated_at=now(),configured_by=excluded.configured_by`, *op.OrganizationID, *op.IntegrationID, *op.ZoneID, *op.TunnelID, op.Hostname, origin, recordID, previousURL, previousSecure, previousForwarded, previousBind, op.RequestedBy)
	return err
}

func (r Repository) DeleteSystemPublicAccess(ctx context.Context) error {
	_, err := r.Pool.Exec(ctx, `DELETE FROM system_public_access WHERE singleton=true`)
	return err
}
