ALTER TABLE applications
    ADD COLUMN host_bind_address inet,
    ADD COLUMN published_port integer CHECK (published_port BETWEEN 1 AND 65535),
    ADD CONSTRAINT applications_port_binding_check CHECK (
        (host_bind_address IS NULL AND published_port IS NULL) OR
        (host_bind_address IS NOT NULL AND published_port IS NOT NULL AND internal_port IS NOT NULL)
    );

ALTER TABLE applications
    ADD CONSTRAINT applications_id_environment_org_unique
    UNIQUE (id, environment_id, organization_id);

ALTER TABLE deployments
    ADD CONSTRAINT deployments_id_application_org_unique
    UNIQUE (id, application_id, organization_id);

ALTER TABLE secrets
    ADD CONSTRAINT secrets_application_environment_scope_fk
    FOREIGN KEY (application_id, environment_id, organization_id)
    REFERENCES applications(id, environment_id, organization_id) ON DELETE CASCADE;

CREATE TABLE application_environment_variables (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    environment_id uuid NOT NULL,
    application_id uuid NOT NULL,
    name text NOT NULL CHECK (name ~ '^[A-Za-z_][A-Za-z0-9_]*$'),
    value text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (application_id, name),
    FOREIGN KEY (environment_id, organization_id)
        REFERENCES environments(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (application_id, environment_id, organization_id)
        REFERENCES applications(id, environment_id, organization_id) ON DELETE CASCADE
);
CREATE INDEX application_environment_variables_scope_idx
    ON application_environment_variables(organization_id, application_id);

CREATE UNIQUE INDEX secrets_application_name_idx
    ON secrets(organization_id, application_id, name)
    WHERE application_id IS NOT NULL;

CREATE TABLE runtime_instances (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    application_id uuid NOT NULL,
    deployment_id uuid NOT NULL,
    provider text NOT NULL CHECK (provider IN ('docker')),
    external_id text NOT NULL,
    image text NOT NULL,
    state text NOT NULL,
    health text NOT NULL,
    container_port integer CHECK (container_port BETWEEN 1 AND 65535),
    host_address inet,
    host_port integer CHECK (host_port BETWEEN 1 AND 65535),
    runtime_created_at timestamptz,
    runtime_started_at timestamptz,
    runtime_finished_at timestamptz,
    last_inspected_at timestamptz NOT NULL DEFAULT now(),
    removed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (deployment_id),
    UNIQUE (provider, external_id),
    UNIQUE (id, organization_id),
    FOREIGN KEY (application_id, organization_id)
        REFERENCES applications(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (deployment_id, organization_id)
        REFERENCES deployments(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (deployment_id, application_id, organization_id)
        REFERENCES deployments(id, application_id, organization_id) ON DELETE CASCADE,
    CHECK ((host_address IS NULL AND host_port IS NULL) OR (host_address IS NOT NULL AND host_port IS NOT NULL))
);
CREATE INDEX runtime_instances_application_idx
    ON runtime_instances(organization_id, application_id, created_at DESC);
CREATE INDEX runtime_instances_active_idx
    ON runtime_instances(application_id, removed_at)
    WHERE removed_at IS NULL;
