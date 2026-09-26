package store

import (
	"context"
	"net"
	"sort"
	"time"

	"github.com/google/uuid"
)

type AgentIdentity struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	ServerID        uuid.UUID
	ProtocolVersion int
	Version         string
	Compatibility   string
	Capabilities    []string
}

type AgentHeartbeat struct {
	ProtocolVersion int
	Version         string
	Compatibility   string
	Capabilities    []string
	Hostname        string
	OperatingSystem string
	Architecture    string
	UptimeSeconds   int64
	DockerAvailable bool
	DockerVersion   string
	CPUCount        int
	CPUUsagePercent float64
	MemoryTotal     int64
	MemoryUsed      int64
	DiskTotal       int64
	DiskUsed        int64
}

func (r Repository) CreateEnrollmentToken(ctx context.Context, organizationID, serverID, actorID uuid.UUID, tokenHash []byte, expiresAt time.Time, requestID uuid.UUID, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE agent_enrollment_tokens SET expires_at=LEAST(expires_at,now()) WHERE organization_id=$1 AND server_id=$2 AND used_at IS NULL`, organizationID, serverID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO agent_enrollment_tokens(organization_id,server_id,token_hash,expires_at,created_by) SELECT $1,s.id,$3,$4,$5 FROM servers s WHERE s.organization_id=$1 AND s.id=$2`, organizationID, serverID, tokenHash, expiresAt, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "server.enrollment_created", "server", &serverID, requestID, map[string]any{"expiresAt": expiresAt.UTC()}, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) EnrollAgent(ctx context.Context, serverID uuid.UUID, tokenHash, credentialHash []byte, protocolVersion int, version, compatibility string, capabilities []string) (AgentIdentity, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return AgentIdentity{}, err
	}
	defer tx.Rollback(ctx)
	var organizationID uuid.UUID
	err = tx.QueryRow(ctx, `UPDATE agent_enrollment_tokens SET used_at=now() WHERE server_id=$1 AND token_hash=$2 AND used_at IS NULL AND expires_at>now() RETURNING organization_id`, serverID, tokenHash).Scan(&organizationID)
	if err != nil {
		return AgentIdentity{}, notFound(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE server_agents SET status='revoked',revoked_at=now() WHERE server_id=$1 AND status='active'`, serverID); err != nil {
		return AgentIdentity{}, err
	}
	capabilities = cleanCapabilities(capabilities)
	var identity AgentIdentity
	err = tx.QueryRow(ctx, `INSERT INTO server_agents(organization_id,server_id,credential_hash,protocol_version,version,compatibility,capabilities) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,organization_id,server_id,protocol_version,version,compatibility,capabilities`, organizationID, serverID, credentialHash, protocolVersion, version, compatibility, capabilities).Scan(&identity.ID, &identity.OrganizationID, &identity.ServerID, &identity.ProtocolVersion, &identity.Version, &identity.Compatibility, &identity.Capabilities)
	if err != nil {
		return AgentIdentity{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE servers SET connection_status='disconnected',health='unknown',agent_version=$2,agent_compatibility=$3,agent_capabilities=$4,updated_at=now() WHERE id=$1 AND organization_id=$5`, serverID, version, compatibility, capabilities, organizationID); err != nil {
		return AgentIdentity{}, err
	}
	if err = insertAudit(ctx, tx, &organizationID, nil, "server.agent_enrolled", "server", &serverID, uuid.New(), map[string]any{"agentId": identity.ID, "version": version}, nil); err != nil {
		return AgentIdentity{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AgentIdentity{}, err
	}
	return identity, nil
}

func (r Repository) AuthenticateAgent(ctx context.Context, agentID uuid.UUID, credentialHash []byte) (AgentIdentity, error) {
	var identity AgentIdentity
	err := r.Pool.QueryRow(ctx, `SELECT id,organization_id,server_id,protocol_version,version,compatibility,capabilities FROM server_agents WHERE id=$1 AND credential_hash=$2 AND status='active'`, agentID, credentialHash).Scan(&identity.ID, &identity.OrganizationID, &identity.ServerID, &identity.ProtocolVersion, &identity.Version, &identity.Compatibility, &identity.Capabilities)
	return identity, notFound(err)
}

func (r Repository) RecordAgentHeartbeat(ctx context.Context, agentID, serverID uuid.UUID, heartbeat AgentHeartbeat) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	heartbeat.Capabilities = cleanCapabilities(heartbeat.Capabilities)
	tag, err := tx.Exec(ctx, `UPDATE server_agents SET protocol_version=$3,version=$4,compatibility=$5,capabilities=$6,last_seen_at=now() WHERE id=$1 AND server_id=$2 AND status='active'`, agentID, serverID, heartbeat.ProtocolVersion, heartbeat.Version, heartbeat.Compatibility, heartbeat.Capabilities)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	status, health := "connected", "healthy"
	if heartbeat.Compatibility == "incompatible" {
		status, health = "degraded", "degraded"
	} else if !heartbeat.DockerAvailable {
		status, health = "docker_unavailable", "degraded"
	}
	tag, err = tx.Exec(ctx, `UPDATE servers SET hostname=$2,operating_system=$3,architecture=$4,connection_status=$5,health=$6,cpu_capacity=$7,cpu_usage_percent=$8,memory_bytes=NULLIF($9::bigint,0),memory_used_bytes=NULLIF($10::bigint,0),disk_total_bytes=NULLIF($11::bigint,0),disk_used_bytes=NULLIF($12::bigint,0),uptime_seconds=$13,docker_available=$14,docker_version=$15,agent_version=$16,agent_compatibility=$17,agent_capabilities=$18,last_seen_at=now(),updated_at=now() WHERE id=$1`, serverID, heartbeat.Hostname, heartbeat.OperatingSystem, heartbeat.Architecture, status, health, heartbeat.CPUCount, heartbeat.CPUUsagePercent, heartbeat.MemoryTotal, heartbeat.MemoryUsed, heartbeat.DiskTotal, heartbeat.DiskUsed, heartbeat.UptimeSeconds, heartbeat.DockerAvailable, heartbeat.DockerVersion, heartbeat.Version, heartbeat.Compatibility, heartbeat.Capabilities)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (r Repository) RevokeAgent(ctx context.Context, organizationID, serverID, actorID, requestID uuid.UUID, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE server_agents SET status='revoked',revoked_at=now() WHERE organization_id=$1 AND server_id=$2 AND status='active'`, organizationID, serverID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err = tx.Exec(ctx, `UPDATE servers SET connection_status='disconnected',health='unknown',updated_at=now() WHERE organization_id=$1 AND id=$2`, organizationID, serverID); err != nil {
		return err
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "server.agent_revoked", "server", &serverID, requestID, nil, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) MarkDisconnectedAgents(ctx context.Context, staleBefore time.Time) (int64, error) {
	tag, err := r.Pool.Exec(ctx, `UPDATE servers s SET connection_status='disconnected',health='unknown',updated_at=now() WHERE s.connection_status<>'disconnected' AND EXISTS(SELECT 1 FROM server_agents a WHERE a.server_id=s.id AND a.status='active' AND (a.last_seen_at IS NULL OR a.last_seen_at<$1))`, staleBefore)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func cleanCapabilities(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "docker.runtime" || value == "runtime.logs" || value == "node.metrics" {
			if !seen[value] {
				seen[value] = true
				result = append(result, value)
			}
		}
	}
	sort.Strings(result)
	return result
}
