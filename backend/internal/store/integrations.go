package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type GitHubIntegration struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	InstallationID int64     `json:"installationId"`
	AccountLogin   string    `json:"accountLogin"`
	Status         string    `json:"status"`
	LastError      string    `json:"lastError,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type GitSource struct {
	ApplicationID      uuid.UUID `json:"applicationId"`
	OrganizationID     uuid.UUID `json:"organizationId"`
	IntegrationID      uuid.UUID `json:"integrationId"`
	Provider           string    `json:"provider"`
	RepositoryID       int64     `json:"repositoryId"`
	RepositoryFullName string    `json:"repositoryFullName"`
	Branch             string    `json:"branch"`
	AutoDeploy         bool      `json:"autoDeploy"`
	UpdatedAt          time.Time `json:"updatedAt"`
}
type CloudflareIntegration struct {
	ID                uuid.UUID  `json:"id"`
	OrganizationID    uuid.UUID  `json:"organizationId"`
	AccountID         string     `json:"accountId"`
	Status            string     `json:"status"`
	LastError         string     `json:"lastError,omitempty"`
	LastCheckedAt     *time.Time `json:"lastCheckedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	EncryptedAPIToken []byte     `json:"-"`
}
type CloudflareZone struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	IntegrationID  uuid.UUID `json:"integrationId"`
	ProviderZoneID string    `json:"providerZoneId"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	Selected       bool      `json:"selected"`
}
type Domain struct {
	ID                      uuid.UUID  `json:"id"`
	OrganizationID          uuid.UUID  `json:"organizationId"`
	EnvironmentID           uuid.UUID  `json:"environmentId"`
	ApplicationID           uuid.UUID  `json:"applicationId"`
	TargetType              string     `json:"targetType"`
	TargetServerID          *uuid.UUID `json:"targetServerId"`
	TargetRuntimeInstanceID *uuid.UUID `json:"targetRuntimeInstanceId"`
	Hostname                string     `json:"hostname"`
	TargetPort              int        `json:"targetPort"`
	Protocol                string     `json:"protocol"`
	RoutingMode             string     `json:"routingMode"`
	RoutingProvider         string     `json:"routingProvider"`
	DNSProvider             string     `json:"dnsProvider"`
	ProviderZoneID          *string    `json:"providerZoneId"`
	ProviderRecordID        *string    `json:"providerRecordId"`
	DNSRecordType           string     `json:"dnsRecordType"`
	DNSContent              string     `json:"dnsContent"`
	Proxied                 bool       `json:"proxied"`
	DNSState                string     `json:"dnsState"`
	SiliconManaged          bool       `json:"siliconManaged"`
	LastSyncError           string     `json:"lastSyncError,omitempty"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}
type CloudflareTunnel struct {
	ID                   uuid.UUID  `json:"id"`
	OrganizationID       uuid.UUID  `json:"organizationId"`
	IntegrationID        uuid.UUID  `json:"integrationId"`
	ProviderTunnelID     string     `json:"providerTunnelId"`
	Name                 string     `json:"name"`
	Ownership            string     `json:"ownership"`
	Status               string     `json:"status"`
	EncryptedTunnelToken []byte     `json:"-"`
	ServerID             *uuid.UUID `json:"serverId"`
	InstallationStatus   string     `json:"installationStatus"`
	InstallationError    string     `json:"installationError,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
}
type TunnelRoute struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	TunnelID       uuid.UUID `json:"tunnelId"`
	DomainID       uuid.UUID `json:"domainId"`
	Hostname       string    `json:"hostname"`
	ServiceURL     string    `json:"serviceUrl"`
	SiliconManaged bool      `json:"siliconManaged"`
}

func (r Repository) UpsertGitHubIntegration(ctx context.Context, orgID, actorID uuid.UUID, installationID int64, account string, requestID uuid.UUID, ip net.IP) (GitHubIntegration, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return GitHubIntegration{}, err
	}
	defer tx.Rollback(ctx)
	var item GitHubIntegration
	err = tx.QueryRow(ctx, `INSERT INTO github_integrations(organization_id,installation_id,account_login,status,last_error,connected_by) VALUES($1,$2,$3,'connected','',$4) ON CONFLICT(organization_id) DO UPDATE SET installation_id=excluded.installation_id,account_login=excluded.account_login,status='connected',last_error='',connected_by=excluded.connected_by,updated_at=now() RETURNING id,organization_id,installation_id,account_login,status,last_error,created_at,updated_at`, orgID, installationID, account, actorID).Scan(&item.ID, &item.OrganizationID, &item.InstallationID, &item.AccountLogin, &item.Status, &item.LastError, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	if err = insertAudit(ctx, tx, &orgID, &actorID, "integration.github.connected", "github_integration", &item.ID, requestID, map[string]any{"installationId": installationID, "account": account}, ip); err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}
func (r Repository) GitHubIntegration(ctx context.Context, orgID uuid.UUID) (GitHubIntegration, error) {
	var i GitHubIntegration
	err := r.Pool.QueryRow(ctx, `SELECT id,organization_id,installation_id,account_login,status,last_error,created_at,updated_at FROM github_integrations WHERE organization_id=$1`, orgID).Scan(&i.ID, &i.OrganizationID, &i.InstallationID, &i.AccountLogin, &i.Status, &i.LastError, &i.CreatedAt, &i.UpdatedAt)
	return i, notFound(err)
}
func (r Repository) DisconnectGitHub(ctx context.Context, orgID, actorID, requestID uuid.UUID, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE github_integrations SET status='disconnected',updated_at=now() WHERE organization_id=$1`, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := insertAudit(ctx, tx, &orgID, &actorID, "integration.github.disconnected", "github_integration", nil, requestID, nil, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r Repository) UpsertGitSource(ctx context.Context, orgID, appID, actorID uuid.UUID, repoID int64, fullName, branch string, auto bool, requestID uuid.UUID, ip net.IP) (GitSource, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return GitSource{}, err
	}
	defer tx.Rollback(ctx)
	var i GitSource
	err = tx.QueryRow(ctx, `INSERT INTO application_git_sources(application_id,organization_id,integration_id,repository_id,repository_full_name,branch,auto_deploy) SELECT a.id,a.organization_id,g.id,$3,$4,$5,$6 FROM applications a JOIN github_integrations g ON g.organization_id=a.organization_id AND g.status='connected' WHERE a.id=$2 AND a.organization_id=$1 ON CONFLICT(application_id) DO UPDATE SET integration_id=excluded.integration_id,repository_id=excluded.repository_id,repository_full_name=excluded.repository_full_name,branch=excluded.branch,auto_deploy=excluded.auto_deploy,updated_at=now() RETURNING application_id,organization_id,integration_id,provider,repository_id,repository_full_name,branch,auto_deploy,updated_at`, orgID, appID, repoID, fullName, branch, auto).Scan(&i.ApplicationID, &i.OrganizationID, &i.IntegrationID, &i.Provider, &i.RepositoryID, &i.RepositoryFullName, &i.Branch, &i.AutoDeploy, &i.UpdatedAt)
	if err != nil {
		return i, notFound(err)
	}
	if err = insertAudit(ctx, tx, &orgID, &actorID, "application.git_source.updated", "application", &appID, requestID, map[string]any{"repository": fullName, "branch": branch, "autoDeploy": auto}, ip); err != nil {
		return i, err
	}
	return i, tx.Commit(ctx)
}
func (r Repository) GitSource(ctx context.Context, orgID, appID uuid.UUID) (GitSource, error) {
	var i GitSource
	err := r.Pool.QueryRow(ctx, `SELECT application_id,organization_id,integration_id,provider,repository_id,repository_full_name,branch,auto_deploy,updated_at FROM application_git_sources WHERE organization_id=$1 AND application_id=$2`, orgID, appID).Scan(&i.ApplicationID, &i.OrganizationID, &i.IntegrationID, &i.Provider, &i.RepositoryID, &i.RepositoryFullName, &i.Branch, &i.AutoDeploy, &i.UpdatedAt)
	return i, notFound(err)
}

var ErrDuplicateDelivery = errors.New("duplicate webhook delivery")

func PayloadDigest(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func (r Repository) AcceptGitHubPush(ctx context.Context, deliveryID string, installationID, repoID int64, repoName, branch, commitSHA, payloadHash string) ([]Deployment, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO github_webhook_deliveries(delivery_id,event_type,installation_id,repository_id,payload_sha256) VALUES($1,'push',$2,$3,$4)`, deliveryID, installationID, repoID, payloadHash); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrDuplicateDelivery
		}
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT s.application_id,s.organization_id,s.repository_full_name FROM application_git_sources s JOIN github_integrations g ON g.id=s.integration_id AND g.organization_id=s.organization_id WHERE g.installation_id=$1 AND g.status='connected' AND s.repository_id=$2 AND lower(s.repository_full_name)=lower($3) AND s.branch=$4 AND s.auto_deploy`, installationID, repoID, repoName, branch)
	if err != nil {
		return nil, err
	}
	type match struct {
		app, org uuid.UUID
		repo     string
	}
	matches := []match{}
	for rows.Next() {
		var m match
		if err := rows.Scan(&m.app, &m.org, &m.repo); err != nil {
			rows.Close()
			return nil, err
		}
		matches = append(matches, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]Deployment, 0, len(matches))
	for _, m := range matches {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, m.app.String()); err != nil {
			return nil, err
		}
		var d Deployment
		err = tx.QueryRow(ctx, `INSERT INTO deployments(organization_id,application_id,number,source,source_revision,status,repository,branch,commit_sha,trigger_type,external_delivery_id,previous_deployment_id) SELECT $1,a.id,COALESCE((SELECT max(number)+1 FROM deployments WHERE application_id=a.id),1),$3,$4,'queued',$3,$5,$4,'github_push',$6,(SELECT id FROM deployments WHERE application_id=a.id ORDER BY number DESC LIMIT 1) FROM applications a WHERE a.organization_id=$1 AND a.id=$2 RETURNING id,organization_id,application_id,number,source,source_revision,image,status,triggered_by,previous_deployment_id,repository,branch,commit_sha,trigger_type,external_delivery_id,created_at,updated_at`, m.org, m.app, m.repo, commitSHA, branch, deliveryID).Scan(&d.ID, &d.OrganizationID, &d.ApplicationID, &d.Number, &d.Source, &d.SourceRevision, &d.Image, &d.Status, &d.TriggeredBy, &d.PreviousDeploymentID, &d.Repository, &d.Branch, &d.CommitSHA, &d.TriggerType, &d.ExternalDeliveryID, &d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			return nil, err
		}
		payload, _ := json.Marshal(map[string]any{"deploymentId": d.ID, "repository": m.repo, "branch": branch, "commitSha": commitSHA})
		if _, err = tx.Exec(ctx, `INSERT INTO jobs(organization_id,job_type,payload) VALUES($1,'deploy_application',$2)`, m.org, payload); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO deployment_events(deployment_id,to_status,message) VALUES($1,'queued','Verified GitHub push queued exact revision')`, d.ID); err != nil {
			return nil, err
		}
		if err = insertAudit(ctx, tx, &m.org, nil, "deployment.github_push", "deployment", &d.ID, uuid.New(), map[string]any{"repository": m.repo, "branch": branch, "commitSha": commitSHA, "deliveryId": deliveryID}, nil); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	status := "accepted"
	if len(matches) == 0 {
		status = "ignored"
	}
	if _, err = tx.Exec(ctx, `UPDATE github_webhook_deliveries SET status=$2,processed_at=now() WHERE delivery_id=$1`, deliveryID, status); err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}

func (r Repository) UpsertCloudflareIntegration(ctx context.Context, orgID, actorID uuid.UUID, accountID string, encrypted []byte, requestID uuid.UUID, ip net.IP) (CloudflareIntegration, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return CloudflareIntegration{}, err
	}
	defer tx.Rollback(ctx)
	var i CloudflareIntegration
	err = tx.QueryRow(ctx, `INSERT INTO cloudflare_integrations(organization_id,account_id,encrypted_api_token,status,last_error,connected_by,last_checked_at) VALUES($1,$2,$3,'connected','',$4,now()) ON CONFLICT(organization_id) DO UPDATE SET account_id=excluded.account_id,encrypted_api_token=excluded.encrypted_api_token,status='connected',last_error='',connected_by=excluded.connected_by,last_checked_at=now(),updated_at=now() RETURNING id,organization_id,account_id,status,last_error,last_checked_at,created_at,updated_at`, orgID, accountID, encrypted, actorID).Scan(&i.ID, &i.OrganizationID, &i.AccountID, &i.Status, &i.LastError, &i.LastCheckedAt, &i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		return i, err
	}
	if err = insertAudit(ctx, tx, &orgID, &actorID, "integration.cloudflare.connected", "cloudflare_integration", &i.ID, requestID, map[string]any{"accountId": accountID}, ip); err != nil {
		return i, err
	}
	return i, tx.Commit(ctx)
}
func (r Repository) CloudflareIntegration(ctx context.Context, orgID uuid.UUID) (CloudflareIntegration, error) {
	var i CloudflareIntegration
	err := r.Pool.QueryRow(ctx, `SELECT id,organization_id,account_id,status,last_error,last_checked_at,created_at,updated_at,encrypted_api_token FROM cloudflare_integrations WHERE organization_id=$1`, orgID).Scan(&i.ID, &i.OrganizationID, &i.AccountID, &i.Status, &i.LastError, &i.LastCheckedAt, &i.CreatedAt, &i.UpdatedAt, &i.EncryptedAPIToken)
	return i, notFound(err)
}

func (r Repository) SetCloudflareHealth(ctx context.Context, orgID uuid.UUID, status, lastError string) error {
	tag, err := r.Pool.Exec(ctx, `UPDATE cloudflare_integrations SET status=$2,last_error=$3,last_checked_at=now(),updated_at=now() WHERE organization_id=$1`, orgID, status, lastError)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
func (r Repository) ReplaceCloudflareZones(ctx context.Context, orgID, integrationID uuid.UUID, zones []CloudflareZone) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	providerIDs := make([]string, 0, len(zones))
	for _, z := range zones {
		providerIDs = append(providerIDs, z.ProviderZoneID)
		if _, err = tx.Exec(ctx, `INSERT INTO cloudflare_zones(organization_id,integration_id,provider_zone_id,name,status,selected) VALUES($1,$2,$3,$4,$5,true) ON CONFLICT(organization_id,provider_zone_id) DO UPDATE SET name=excluded.name,status=excluded.status,updated_at=now()`, orgID, integrationID, z.ProviderZoneID, z.Name, z.Status); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM cloudflare_zones WHERE organization_id=$1 AND NOT(provider_zone_id=ANY($2::text[]))`, orgID, providerIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r Repository) ListCloudflareZones(ctx context.Context, orgID uuid.UUID) ([]CloudflareZone, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,integration_id,provider_zone_id,name,status,selected FROM cloudflare_zones WHERE organization_id=$1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CloudflareZone{}
	for rows.Next() {
		var i CloudflareZone
		if err := rows.Scan(&i.ID, &i.OrganizationID, &i.IntegrationID, &i.ProviderZoneID, &i.Name, &i.Status, &i.Selected); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (r Repository) SetCloudflareZoneSelected(ctx context.Context, orgID, zoneID uuid.UUID, selected bool) error {
	tag, err := r.Pool.Exec(ctx, `UPDATE cloudflare_zones SET selected=$3,updated_at=now() WHERE organization_id=$1 AND id=$2`, orgID, zoneID, selected)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (r Repository) CreateDomain(ctx context.Context, orgID, appID, actorID uuid.UUID, hostname string, targetPort int, protocol, routingMode string, requestID uuid.UUID, ip net.IP) (Domain, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Domain{}, err
	}
	defer tx.Rollback(ctx)
	var d Domain
	err = tx.QueryRow(ctx, `INSERT INTO domains(organization_id,environment_id,application_id,target_server_id,hostname,target_port,protocol,routing_mode,dns_provider,proxied,dns_state) SELECT a.organization_id,a.environment_id,a.id,a.server_id,$3,$4,$5,$6,'cloudflare',($6='cloudflare_proxied'),'pending' FROM applications a WHERE a.organization_id=$1 AND a.id=$2 AND a.server_id IS NOT NULL RETURNING `+domainColumns, orgID, appID, hostname, targetPort, protocol, routingMode).Scan(domainScan(&d)...)
	if err != nil {
		return d, notFound(err)
	}
	if err = insertAudit(ctx, tx, &orgID, &actorID, "domain.created", "domain", &d.ID, requestID, map[string]any{"hostname": hostname}, ip); err != nil {
		return d, err
	}
	return d, tx.Commit(ctx)
}
func (r Repository) ListDomains(ctx context.Context, orgID uuid.UUID) ([]Domain, error) {
	rows, err := r.Pool.Query(ctx, `SELECT `+domainColumns+` FROM domains WHERE organization_id=$1 ORDER BY hostname`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Domain{}
	for rows.Next() {
		var d Domain
		if err := rows.Scan(domainScan(&d)...); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}
func (r Repository) Domain(ctx context.Context, orgID, domainID uuid.UUID) (Domain, error) {
	var d Domain
	err := r.Pool.QueryRow(ctx, `SELECT `+domainColumns+` FROM domains WHERE organization_id=$1 AND id=$2`, orgID, domainID).Scan(domainScan(&d)...)
	return d, notFound(err)
}

const domainColumns = `id,organization_id,environment_id,application_id,target_type,target_server_id,target_runtime_instance_id,hostname,target_port,protocol,routing_mode,routing_provider,dns_provider,provider_zone_id,provider_record_id,dns_record_type,dns_content,proxied,dns_state,silicon_managed,last_sync_error,created_at,updated_at`

func domainScan(d *Domain) []any {
	return []any{&d.ID, &d.OrganizationID, &d.EnvironmentID, &d.ApplicationID, &d.TargetType, &d.TargetServerID, &d.TargetRuntimeInstanceID, &d.Hostname, &d.TargetPort, &d.Protocol, &d.RoutingMode, &d.RoutingProvider, &d.DNSProvider, &d.ProviderZoneID, &d.ProviderRecordID, &d.DNSRecordType, &d.DNSContent, &d.Proxied, &d.DNSState, &d.SiliconManaged, &d.LastSyncError, &d.CreatedAt, &d.UpdatedAt}
}
func (r Repository) SetDomainSync(ctx context.Context, orgID, domainID uuid.UUID, zoneID, recordID *string, state string, owned bool, syncError string) error {
	tag, err := r.Pool.Exec(ctx, `UPDATE domains SET provider_zone_id=$3,provider_record_id=$4,dns_state=$5,silicon_managed=$6,last_sync_error=$7,updated_at=now() WHERE organization_id=$1 AND id=$2`, orgID, domainID, zoneID, recordID, state, owned, syncError)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (r Repository) SetDomainOriginDesired(ctx context.Context, orgID, domainID uuid.UUID, recordType, content string, proxied bool) error {
	tag, err := r.Pool.Exec(ctx, `UPDATE domains SET dns_record_type=$3,dns_content=$4,proxied=$5,updated_at=now() WHERE organization_id=$1 AND id=$2`, orgID, domainID, recordType, content, proxied)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (r Repository) UpdateDomainDesired(ctx context.Context, orgID, domainID, actorID uuid.UUID, targetPort int, protocol, routingMode, recordType, content string, proxied bool, requestID uuid.UUID, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE domains SET target_port=$3,protocol=$4,routing_mode=$5,dns_record_type=$6,dns_content=$7,proxied=$8,dns_state='pending',last_sync_error='',updated_at=now() WHERE organization_id=$1 AND id=$2`, orgID, domainID, targetPort, protocol, routingMode, recordType, content, proxied)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err = insertAudit(ctx, tx, &orgID, &actorID, "domain.changed", "domain", &domainID, requestID, map[string]any{"routingMode": routingMode, "recordType": recordType, "proxied": proxied, "targetPort": targetPort, "protocol": protocol}, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r Repository) DeleteDomainRecord(ctx context.Context, orgID, domainID, actorID, requestID uuid.UUID, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `DELETE FROM domains WHERE organization_id=$1 AND id=$2`, orgID, domainID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err = insertAudit(ctx, tx, &orgID, &actorID, "domain.deleted", "domain", &domainID, requestID, nil, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) SaveTunnel(ctx context.Context, orgID, integrationID uuid.UUID, providerID, name, ownership, status string, encryptedToken []byte, serverID *uuid.UUID) (CloudflareTunnel, error) {
	var t CloudflareTunnel
	err := r.Pool.QueryRow(ctx, `INSERT INTO cloudflare_tunnels(organization_id,integration_id,provider_tunnel_id,name,ownership,status,encrypted_tunnel_token,server_id,installation_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,CASE WHEN $5 IN ('external','imported') THEN 'external' ELSE 'not_installed' END) ON CONFLICT(organization_id,provider_tunnel_id) DO UPDATE SET name=excluded.name,ownership=excluded.ownership,status=excluded.status,server_id=excluded.server_id,updated_at=now() RETURNING id,organization_id,integration_id,provider_tunnel_id,name,ownership,status,encrypted_tunnel_token,server_id,installation_status,installation_error,created_at`, orgID, integrationID, providerID, name, ownership, status, encryptedToken, serverID).Scan(tunnelScan(&t)...)
	return t, err
}
func (r Repository) ListTunnels(ctx context.Context, orgID uuid.UUID) ([]CloudflareTunnel, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,integration_id,provider_tunnel_id,name,ownership,status,encrypted_tunnel_token,server_id,installation_status,installation_error,created_at FROM cloudflare_tunnels WHERE organization_id=$1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CloudflareTunnel{}
	for rows.Next() {
		var t CloudflareTunnel
		if err := rows.Scan(tunnelScan(&t)...); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}
func (r Repository) Tunnel(ctx context.Context, orgID, tunnelID uuid.UUID) (CloudflareTunnel, error) {
	var t CloudflareTunnel
	err := r.Pool.QueryRow(ctx, `SELECT id,organization_id,integration_id,provider_tunnel_id,name,ownership,status,encrypted_tunnel_token,server_id,installation_status,installation_error,created_at FROM cloudflare_tunnels WHERE organization_id=$1 AND id=$2`, orgID, tunnelID).Scan(tunnelScan(&t)...)
	return t, notFound(err)
}

func tunnelScan(t *CloudflareTunnel) []any {
	return []any{&t.ID, &t.OrganizationID, &t.IntegrationID, &t.ProviderTunnelID, &t.Name, &t.Ownership, &t.Status, &t.EncryptedTunnelToken, &t.ServerID, &t.InstallationStatus, &t.InstallationError, &t.CreatedAt}
}

func (r Repository) SetTunnelInstallation(ctx context.Context, orgID, tunnelID uuid.UUID, status, installationError string) error {
	tag, err := r.Pool.Exec(ctx, `UPDATE cloudflare_tunnels SET installation_status=$3,installation_error=$4,updated_at=now() WHERE organization_id=$1 AND id=$2`, orgID, tunnelID, status, installationError)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
func (r Repository) SaveTunnelRoute(ctx context.Context, orgID, tunnelID, domainID uuid.UUID, hostname, service string) (TunnelRoute, error) {
	var route TunnelRoute
	err := r.Pool.QueryRow(ctx, `INSERT INTO cloudflare_tunnel_routes(organization_id,tunnel_id,domain_id,hostname,service_url) SELECT $1,t.id,d.id,$4,$5 FROM cloudflare_tunnels t JOIN domains d ON d.organization_id=t.organization_id WHERE t.organization_id=$1 AND t.id=$2 AND d.id=$3 ON CONFLICT(domain_id) DO UPDATE SET tunnel_id=excluded.tunnel_id,hostname=excluded.hostname,service_url=excluded.service_url,updated_at=now() RETURNING id,organization_id,tunnel_id,domain_id,hostname,service_url,silicon_managed`, orgID, tunnelID, domainID, hostname, service).Scan(&route.ID, &route.OrganizationID, &route.TunnelID, &route.DomainID, &route.Hostname, &route.ServiceURL, &route.SiliconManaged)
	return route, notFound(err)
}
func (r Repository) TunnelRoutes(ctx context.Context, orgID, tunnelID uuid.UUID) ([]TunnelRoute, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,tunnel_id,domain_id,hostname,service_url,silicon_managed FROM cloudflare_tunnel_routes WHERE organization_id=$1 AND tunnel_id=$2 ORDER BY hostname`, orgID, tunnelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []TunnelRoute{}
	for rows.Next() {
		var x TunnelRoute
		if err := rows.Scan(&x.ID, &x.OrganizationID, &x.TunnelID, &x.DomainID, &x.Hostname, &x.ServiceURL, &x.SiliconManaged); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
