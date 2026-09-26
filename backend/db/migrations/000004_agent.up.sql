ALTER TABLE servers
    ADD CONSTRAINT servers_id_organization_unique UNIQUE (id, organization_id),
    ADD COLUMN docker_available boolean NOT NULL DEFAULT false,
    ADD COLUMN docker_version text NOT NULL DEFAULT '',
    ADD COLUMN cpu_usage_percent double precision,
    ADD COLUMN memory_used_bytes bigint,
    ADD COLUMN disk_total_bytes bigint,
    ADD COLUMN disk_used_bytes bigint,
    ADD COLUMN uptime_seconds bigint,
    ADD COLUMN agent_version text NOT NULL DEFAULT '',
    ADD COLUMN agent_compatibility text NOT NULL DEFAULT 'unknown'
        CHECK (agent_compatibility IN ('unknown','compatible','outdated','incompatible')),
    ADD COLUMN agent_capabilities text[] NOT NULL DEFAULT ARRAY[]::text[];

ALTER TABLE servers DROP CONSTRAINT servers_connection_status_check;
ALTER TABLE servers ADD CONSTRAINT servers_connection_status_check
    CHECK (connection_status IN ('unknown','connected','disconnected','degraded','docker_unavailable'));

ALTER TABLE applications
    ADD COLUMN server_id uuid,
    ADD CONSTRAINT applications_server_scope_fk
        FOREIGN KEY (server_id, organization_id)
        REFERENCES servers(id, organization_id);
CREATE INDEX applications_server_idx ON applications(organization_id, server_id)
    WHERE server_id IS NOT NULL;

ALTER TABLE runtime_instances
    ADD COLUMN server_id uuid,
    ADD CONSTRAINT runtime_instances_server_scope_fk
        FOREIGN KEY (server_id, organization_id)
        REFERENCES servers(id, organization_id);

CREATE TABLE agent_enrollment_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    server_id uuid NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (server_id, organization_id)
        REFERENCES servers(id, organization_id) ON DELETE CASCADE
);
CREATE INDEX agent_enrollment_tokens_server_idx
    ON agent_enrollment_tokens(server_id, created_at DESC);

CREATE TABLE server_agents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    server_id uuid NOT NULL,
    credential_hash bytea NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','revoked')),
    protocol_version integer NOT NULL,
    version text NOT NULL,
    compatibility text NOT NULL CHECK (compatibility IN ('compatible','outdated','incompatible')),
    capabilities text[] NOT NULL DEFAULT ARRAY[]::text[],
    enrolled_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz,
    revoked_at timestamptz,
    FOREIGN KEY (server_id, organization_id)
        REFERENCES servers(id, organization_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX server_agents_one_active_idx ON server_agents(server_id)
    WHERE status = 'active';
CREATE INDEX server_agents_org_idx ON server_agents(organization_id, server_id);
