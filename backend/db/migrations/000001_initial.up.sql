CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email citext NOT NULL UNIQUE,
    display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 120),
    password_hash text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE organizations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('owner', 'admin', 'developer', 'viewer')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, user_id)
);
CREATE INDEX memberships_user_idx ON memberships(user_id);

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash bytea NOT NULL UNIQUE,
    csrf_token_hash bytea NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    user_agent text NOT NULL DEFAULT '',
    ip_address inet
);
CREATE INDEX sessions_user_idx ON sessions(user_id);
CREATE INDEX sessions_expiry_idx ON sessions(expires_at);

CREATE TABLE password_reset_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX password_reset_tokens_user_idx ON password_reset_tokens(user_id, created_at DESC);

CREATE TABLE projects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    slug text NOT NULL CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, slug)
);
CREATE INDEX projects_organization_idx ON projects(organization_id);

CREATE TABLE environments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    slug text NOT NULL CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, slug),
    UNIQUE (id, organization_id),
    UNIQUE (id, project_id, organization_id)
);
CREATE INDEX environments_scope_idx ON environments(organization_id, project_id);

CREATE TABLE applications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id uuid NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    source_type text NOT NULL DEFAULT 'docker_image' CHECK (source_type IN ('docker_image', 'git_dockerfile', 'compose')),
    image text,
    internal_port integer CHECK (internal_port BETWEEN 1 AND 65535),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (environment_id, name),
    UNIQUE (id, organization_id),
    FOREIGN KEY (environment_id, project_id, organization_id)
        REFERENCES environments(id, project_id, organization_id) ON DELETE CASCADE
);
CREATE INDEX applications_scope_idx ON applications(organization_id, project_id, environment_id);

CREATE TABLE deployments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    application_id uuid NOT NULL,
    number bigint NOT NULL CHECK (number > 0),
    source text NOT NULL DEFAULT '',
    source_revision text NOT NULL DEFAULT '',
    image text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('queued','preparing','building','deploying','starting','healthy','failed','cancelled','superseded','rolled_back')),
    triggered_by uuid REFERENCES users(id) ON DELETE SET NULL,
    previous_deployment_id uuid,
    rollback_of_deployment_id uuid,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (application_id, number),
    UNIQUE (id, organization_id),
    FOREIGN KEY (application_id, organization_id)
        REFERENCES applications(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (previous_deployment_id, organization_id)
        REFERENCES deployments(id, organization_id),
    FOREIGN KEY (rollback_of_deployment_id, organization_id)
        REFERENCES deployments(id, organization_id)
);
CREATE INDEX deployments_scope_idx ON deployments(organization_id, application_id, created_at DESC);

CREATE TABLE deployment_events (
    id bigserial PRIMARY KEY,
    deployment_id uuid NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    from_status text,
    to_status text NOT NULL,
    message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX deployment_events_deployment_idx ON deployment_events(deployment_id, created_at);

CREATE TABLE servers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    hostname text NOT NULL,
    operating_system text NOT NULL DEFAULT '',
    architecture text NOT NULL DEFAULT '',
    runtime text NOT NULL DEFAULT 'docker',
    connection_status text NOT NULL DEFAULT 'unknown' CHECK (connection_status IN ('unknown','connected','disconnected')),
    health text NOT NULL DEFAULT 'unknown' CHECK (health IN ('unknown','healthy','degraded','unhealthy')),
    cpu_capacity integer CHECK (cpu_capacity IS NULL OR cpu_capacity > 0),
    memory_bytes bigint CHECK (memory_bytes IS NULL OR memory_bytes > 0),
    last_seen_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);
CREATE INDEX servers_organization_idx ON servers(organization_id);

CREATE TABLE domains (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    environment_id uuid NOT NULL,
    application_id uuid NOT NULL,
    hostname citext NOT NULL,
    target_port integer NOT NULL CHECK (target_port BETWEEN 1 AND 65535),
    routing_provider text NOT NULL DEFAULT 'external',
    tls_mode text NOT NULL DEFAULT 'external' CHECK (tls_mode = 'external'),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, hostname),
    FOREIGN KEY (environment_id, organization_id)
        REFERENCES environments(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (application_id, organization_id)
        REFERENCES applications(id, organization_id) ON DELETE CASCADE
);
CREATE INDEX domains_scope_idx ON domains(organization_id, application_id);

CREATE TABLE identity_providers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name text NOT NULL,
    provider_type text NOT NULL CHECK (provider_type IN ('oidc', 'authentik')),
    issuer_url text NOT NULL,
    client_id text NOT NULL,
    encrypted_client_secret bytea,
    scopes text[] NOT NULL DEFAULT ARRAY['openid','profile','email']::text[],
    callback_url text NOT NULL,
    claim_mapping jsonb NOT NULL DEFAULT '{}'::jsonb,
    enabled boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

CREATE TABLE external_identities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_provider_id uuid NOT NULL REFERENCES identity_providers(id) ON DELETE CASCADE,
    provider_subject text NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (identity_provider_id, provider_subject),
    UNIQUE (identity_provider_id, user_id)
);

CREATE TABLE secrets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    environment_id uuid,
    application_id uuid,
    name text NOT NULL,
    provider text NOT NULL DEFAULT 'local',
    encrypted_value bytea NOT NULL,
    key_version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, environment_id, application_id, name),
    FOREIGN KEY (environment_id, organization_id)
        REFERENCES environments(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (application_id, organization_id)
        REFERENCES applications(id, organization_id) ON DELETE CASCADE
);
CREATE INDEX secrets_scope_idx ON secrets(organization_id, application_id);

CREATE TABLE audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid REFERENCES organizations(id) ON DELETE CASCADE,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid,
    request_id uuid NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    ip_address inet,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_scope_idx ON audit_events(organization_id, created_at DESC);
CREATE INDEX audit_events_actor_idx ON audit_events(actor_user_id, created_at DESC);

CREATE TABLE jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    job_type text NOT NULL CHECK (job_type IN ('build_application','deploy_application','rollback_deployment','collect_runtime_status')),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','succeeded','failed','cancelled')),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    locked_at timestamptz,
    locked_by text,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_claim_idx ON jobs(status, available_at, created_at) WHERE status = 'queued';
CREATE INDEX jobs_organization_idx ON jobs(organization_id, created_at DESC);
