CREATE TABLE project_environment_variables (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id uuid NOT NULL,
    name text NOT NULL CHECK (name ~ '^[A-Za-z_][A-Za-z0-9_]*$'),
    value text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, name),
    FOREIGN KEY (project_id, organization_id)
        REFERENCES projects(id, organization_id) ON DELETE CASCADE
);
CREATE INDEX project_environment_variables_scope_idx
    ON project_environment_variables(organization_id, project_id);

CREATE TABLE environment_environment_variables (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id uuid NOT NULL,
    environment_id uuid NOT NULL,
    name text NOT NULL CHECK (name ~ '^[A-Za-z_][A-Za-z0-9_]*$'),
    value text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (environment_id, name),
    FOREIGN KEY (environment_id, project_id, organization_id)
        REFERENCES environments(id, project_id, organization_id) ON DELETE CASCADE
);
CREATE INDEX environment_environment_variables_scope_idx
    ON environment_environment_variables(organization_id, environment_id);

ALTER TABLE secrets ADD COLUMN project_id uuid;

-- Existing application secrets remain application overrides. No ciphertext is
-- rewritten, so the existing authenticated-encryption context remains valid.
UPDATE secrets AS secret
SET project_id = application.project_id,
    environment_id = application.environment_id
FROM applications AS application
WHERE secret.application_id = application.id
  AND secret.organization_id = application.organization_id;

UPDATE secrets AS secret
SET project_id = environment.project_id
FROM environments AS environment
WHERE secret.application_id IS NULL
  AND secret.environment_id = environment.id
  AND secret.organization_id = environment.organization_id;

ALTER TABLE secrets
    ADD CONSTRAINT secrets_project_organization_fk
    FOREIGN KEY (project_id, organization_id)
    REFERENCES projects(id, organization_id) ON DELETE CASCADE,
    ADD CONSTRAINT secrets_environment_project_organization_fk
    FOREIGN KEY (environment_id, project_id, organization_id)
    REFERENCES environments(id, project_id, organization_id) ON DELETE CASCADE,
    ADD CONSTRAINT secrets_scope_hierarchy_check CHECK (
        (project_id IS NULL AND environment_id IS NULL AND application_id IS NULL) OR
        (project_id IS NOT NULL AND environment_id IS NULL AND application_id IS NULL) OR
        (project_id IS NOT NULL AND environment_id IS NOT NULL AND application_id IS NULL) OR
        (project_id IS NOT NULL AND environment_id IS NOT NULL AND application_id IS NOT NULL)
    );

CREATE UNIQUE INDEX secrets_project_name_idx
    ON secrets(organization_id, project_id, name)
    WHERE project_id IS NOT NULL AND environment_id IS NULL AND application_id IS NULL;

CREATE UNIQUE INDEX secrets_environment_name_idx
    ON secrets(organization_id, environment_id, name)
    WHERE environment_id IS NOT NULL AND application_id IS NULL;
