-- Remove the retired Silicon Agent identity and enrollment model without
-- deleting servers, applications, deployments, or runtime history.
DROP TABLE IF EXISTS agent_enrollment_tokens;
DROP TABLE IF EXISTS server_agents;

ALTER TABLE servers
    DROP COLUMN IF EXISTS agent_version,
    DROP COLUMN IF EXISTS agent_compatibility,
    DROP COLUMN IF EXISTS agent_capabilities;

ALTER TABLE servers RENAME COLUMN last_seen_at TO last_checked_at;

ALTER TABLE servers DROP CONSTRAINT servers_connection_status_check;
ALTER TABLE servers ADD CONSTRAINT servers_connection_status_check CHECK (
    connection_status IN (
        'connection_not_configured',
        'connected',
        'unreachable',
        'docker_unavailable',
        'authentication_failed',
        'host_key_changed'
    )
);

ALTER TABLE servers
    ADD COLUMN connection_type text NOT NULL DEFAULT 'unconfigured'
        CHECK (connection_type IN ('unconfigured','local','ssh')),
    ADD COLUMN public_address text NOT NULL DEFAULT '',
    ADD COLUMN ssh_port integer NOT NULL DEFAULT 22 CHECK (ssh_port BETWEEN 1 AND 65535),
    ADD COLUMN ssh_username text NOT NULL DEFAULT '',
    ADD COLUMN ssh_private_key_encrypted bytea,
    ADD COLUMN ssh_host_key_fingerprint text NOT NULL DEFAULT '',
    ADD COLUMN connection_error text NOT NULL DEFAULT '';

UPDATE servers
SET connection_status='connection_not_configured',
    health='unknown',
    docker_available=false,
    docker_version='',
    connection_type='unconfigured',
    connection_error='The retired Silicon Agent connection must be replaced with local or SSH configuration.';

ALTER TABLE domains
    ADD COLUMN target_type text NOT NULL DEFAULT 'application'
        CHECK (target_type IN ('application','server','runtime_instance')),
    ADD COLUMN target_server_id uuid,
    ADD COLUMN target_runtime_instance_id uuid,
    ADD COLUMN protocol text NOT NULL DEFAULT 'http'
        CHECK (protocol IN ('http','https','tcp')),
    ADD COLUMN routing_mode text NOT NULL DEFAULT 'dns_only'
        CHECK (routing_mode IN ('dns_only','cloudflare_proxied','cloudflare_tunnel')),
    ADD CONSTRAINT domains_target_server_scope_fk
        FOREIGN KEY (target_server_id, organization_id)
        REFERENCES servers(id, organization_id),
    ADD CONSTRAINT domains_target_runtime_scope_fk
        FOREIGN KEY (target_runtime_instance_id, organization_id)
        REFERENCES runtime_instances(id, organization_id);

UPDATE domains
SET target_server_id = applications.server_id,
    routing_mode = CASE WHEN proxied THEN 'cloudflare_proxied' ELSE 'dns_only' END
FROM applications
WHERE applications.id=domains.application_id
  AND applications.organization_id=domains.organization_id;

ALTER TABLE cloudflare_tunnels
    ADD COLUMN server_id uuid,
    ADD COLUMN installation_status text NOT NULL DEFAULT 'not_installed'
        CHECK (installation_status IN ('not_installed','installed','error','external')),
    ADD COLUMN installation_error text NOT NULL DEFAULT '',
    ADD CONSTRAINT cloudflare_tunnels_server_scope_fk
        FOREIGN KEY (server_id, organization_id)
        REFERENCES servers(id, organization_id);
