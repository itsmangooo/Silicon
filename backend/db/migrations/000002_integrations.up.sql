ALTER TABLE deployments
    ADD COLUMN repository text NOT NULL DEFAULT '',
    ADD COLUMN branch text NOT NULL DEFAULT '',
    ADD COLUMN commit_sha text NOT NULL DEFAULT '',
    ADD COLUMN trigger_type text NOT NULL DEFAULT 'manual'
        CHECK (trigger_type IN ('manual','github_push','rollback','redeploy')),
    ADD COLUMN external_delivery_id text;

CREATE UNIQUE INDEX deployments_external_delivery_idx
    ON deployments(application_id, external_delivery_id)
    WHERE external_delivery_id IS NOT NULL;

CREATE TABLE github_integrations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    installation_id bigint NOT NULL CHECK (installation_id > 0),
    account_login text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'connected' CHECK (status IN ('connected','disconnected','error')),
    last_error text NOT NULL DEFAULT '',
    connected_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id),
    UNIQUE (installation_id),
    UNIQUE (id, organization_id)
);

CREATE TABLE application_git_sources (
    application_id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    integration_id uuid NOT NULL REFERENCES github_integrations(id) ON DELETE CASCADE,
    provider text NOT NULL DEFAULT 'github' CHECK (provider = 'github'),
    repository_id bigint NOT NULL CHECK (repository_id > 0),
    repository_full_name text NOT NULL,
    branch text NOT NULL DEFAULT 'main',
    auto_deploy boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (application_id, organization_id)
        REFERENCES applications(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (integration_id, organization_id)
        REFERENCES github_integrations(id, organization_id) ON DELETE CASCADE,
    UNIQUE (organization_id, repository_id, application_id)
);
CREATE INDEX application_git_sources_webhook_idx
    ON application_git_sources(repository_id, branch)
    WHERE auto_deploy;

CREATE TABLE github_webhook_deliveries (
    delivery_id text PRIMARY KEY,
    event_type text NOT NULL,
    installation_id bigint NOT NULL,
    repository_id bigint NOT NULL,
    payload_sha256 text NOT NULL,
    status text NOT NULL DEFAULT 'processing' CHECK (status IN ('processing','accepted','ignored','failed')),
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz
);

CREATE TABLE cloudflare_integrations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    account_id text NOT NULL,
    encrypted_api_token bytea NOT NULL,
    status text NOT NULL DEFAULT 'connected' CHECK (status IN ('connected','disconnected','error')),
    last_error text NOT NULL DEFAULT '',
    connected_by uuid REFERENCES users(id) ON DELETE SET NULL,
    last_checked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id),
    UNIQUE (id, organization_id)
);

CREATE TABLE cloudflare_zones (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    integration_id uuid NOT NULL REFERENCES cloudflare_integrations(id) ON DELETE CASCADE,
    provider_zone_id text NOT NULL,
    name citext NOT NULL,
    status text NOT NULL DEFAULT 'active',
    selected boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, provider_zone_id),
    UNIQUE (organization_id, name),
    FOREIGN KEY (integration_id, organization_id)
        REFERENCES cloudflare_integrations(id, organization_id) ON DELETE CASCADE
);

ALTER TABLE domains
    ADD COLUMN dns_provider text NOT NULL DEFAULT 'external' CHECK (dns_provider IN ('external','cloudflare')),
    ADD COLUMN provider_zone_id text,
    ADD COLUMN provider_record_id text,
    ADD COLUMN dns_record_type text NOT NULL DEFAULT 'A' CHECK (dns_record_type IN ('A','AAAA','CNAME')),
    ADD COLUMN dns_content text NOT NULL DEFAULT '',
    ADD COLUMN proxied boolean NOT NULL DEFAULT false,
    ADD COLUMN dns_state text NOT NULL DEFAULT 'external' CHECK (dns_state IN ('pending','active','conflict','error','external')),
    ADD COLUMN silicon_managed boolean NOT NULL DEFAULT false,
    ADD COLUMN last_sync_error text NOT NULL DEFAULT '',
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE domains ADD CONSTRAINT domains_id_org_unique UNIQUE(id, organization_id);

CREATE TABLE cloudflare_tunnels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    integration_id uuid NOT NULL REFERENCES cloudflare_integrations(id) ON DELETE CASCADE,
    provider_tunnel_id text NOT NULL,
    name text NOT NULL,
    ownership text NOT NULL CHECK (ownership IN ('silicon','imported','external')),
    status text NOT NULL DEFAULT 'inactive',
    encrypted_tunnel_token bytea,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, provider_tunnel_id),
    UNIQUE (id, organization_id),
    FOREIGN KEY (integration_id, organization_id)
        REFERENCES cloudflare_integrations(id, organization_id) ON DELETE CASCADE
);

CREATE TABLE cloudflare_tunnel_routes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    tunnel_id uuid NOT NULL,
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    hostname citext NOT NULL,
    service_url text NOT NULL,
    silicon_managed boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tunnel_id, hostname),
    UNIQUE (domain_id),
    FOREIGN KEY (domain_id, organization_id)
        REFERENCES domains(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (tunnel_id, organization_id)
        REFERENCES cloudflare_tunnels(id, organization_id) ON DELETE CASCADE
);

ALTER TABLE servers
    ADD COLUMN connectivity_type text NOT NULL DEFAULT 'public'
        CHECK (connectivity_type IN ('public','self_hosted','private'));

ALTER TABLE jobs DROP CONSTRAINT jobs_job_type_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_job_type_check
    CHECK (job_type IN ('build_application','deploy_application','rollback_deployment','collect_runtime_status','sync_domain','configure_tunnel'));
