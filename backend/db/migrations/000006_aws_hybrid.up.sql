CREATE TABLE aws_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 120),
    account_id text NOT NULL CHECK (account_id ~ '^[0-9]{12}$'),
    role_arn text NOT NULL DEFAULT '',
    encrypted_external_id bytea,
    encrypted_access_key_id bytea,
    encrypted_secret_access_key bytea,
    default_region text NOT NULL,
    enabled_regions text[] NOT NULL DEFAULT ARRAY[]::text[],
    status text NOT NULL DEFAULT 'connected' CHECK (status IN ('connected','error','disabled')),
    last_error text NOT NULL DEFAULT '',
    last_checked_at timestamptz,
    connected_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, display_name),
    UNIQUE (organization_id, account_id, role_arn),
    UNIQUE (id, organization_id)
);
CREATE INDEX aws_accounts_organization_idx ON aws_accounts(organization_id, created_at);

ALTER TABLE projects
    ADD CONSTRAINT projects_id_organization_unique UNIQUE (id, organization_id);

CREATE TABLE aws_resource_ownership (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    account_id uuid NOT NULL,
    region text NOT NULL,
    resource_type text NOT NULL CHECK (resource_type IN ('instance','vpc','subnet','security_group','elastic_ip','volume','snapshot')),
    provider_resource_id text NOT NULL,
    ownership text NOT NULL CHECK (ownership IN ('external','imported','managed')),
    project_id uuid,
    environment_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (account_id, organization_id) REFERENCES aws_accounts(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, organization_id) REFERENCES projects(id, organization_id) ON DELETE SET NULL (project_id),
    FOREIGN KEY (environment_id, organization_id) REFERENCES environments(id, organization_id) ON DELETE SET NULL (environment_id),
    UNIQUE (organization_id, account_id, region, resource_type, provider_resource_id),
    UNIQUE (id, organization_id)
);
CREATE INDEX aws_resource_ownership_scope_idx ON aws_resource_ownership(organization_id, account_id, region, resource_type);

CREATE TABLE aws_instances (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    account_id uuid NOT NULL,
    server_id uuid,
    provider_instance_id text NOT NULL,
    region text NOT NULL,
    name text NOT NULL DEFAULT '',
    state text NOT NULL DEFAULT 'pending',
    instance_type text NOT NULL,
    architecture text NOT NULL DEFAULT '',
    availability_zone text NOT NULL DEFAULT '',
    image_id text NOT NULL,
    private_ip text NOT NULL DEFAULT '',
    public_ip text NOT NULL DEFAULT '',
    vpc_id text NOT NULL DEFAULT '',
    subnet_id text NOT NULL DEFAULT '',
    security_group_ids text[] NOT NULL DEFAULT ARRAY[]::text[],
    ownership text NOT NULL CHECK (ownership IN ('imported','managed')),
    connection_method text NOT NULL CHECK (connection_method IN ('ssh','aws_ssm')),
    launched_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (account_id, organization_id) REFERENCES aws_accounts(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (server_id, organization_id) REFERENCES servers(id, organization_id) ON DELETE SET NULL (server_id),
    UNIQUE (organization_id, account_id, region, provider_instance_id),
    UNIQUE (server_id)
);
CREATE INDEX aws_instances_scope_idx ON aws_instances(organization_id, account_id, region, state);

ALTER TABLE servers DROP CONSTRAINT servers_connection_type_check;
ALTER TABLE servers ADD CONSTRAINT servers_connection_type_check
    CHECK (connection_type IN ('unconfigured','local','ssh','aws_ssm'));
ALTER TABLE servers
    ADD COLUMN provider_type text NOT NULL DEFAULT 'generic' CHECK (provider_type IN ('generic','aws')),
    ADD COLUMN aws_account_id uuid,
    ADD COLUMN aws_instance_id text NOT NULL DEFAULT '',
    ADD COLUMN aws_region text NOT NULL DEFAULT '',
    ADD CONSTRAINT servers_aws_account_scope_fk FOREIGN KEY (aws_account_id, organization_id)
        REFERENCES aws_accounts(id, organization_id) ON DELETE RESTRICT,
    ADD CONSTRAINT servers_aws_configuration_check CHECK (
        (provider_type='generic' AND aws_instance_id='' AND aws_region='') OR
        (provider_type='aws' AND aws_account_id IS NOT NULL AND aws_instance_id<>'' AND aws_region<>'')
    );

CREATE TABLE aws_budgets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    account_id uuid,
    project_id uuid,
    environment_id uuid,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    monthly_amount numeric(14,2) NOT NULL CHECK (monthly_amount > 0),
    currency char(3) NOT NULL DEFAULT 'USD',
    thresholds numeric(5,2)[] NOT NULL DEFAULT ARRAY[50,80,90,100]::numeric[],
    prevent_new_provisioning boolean NOT NULL DEFAULT false,
    provider_type text NOT NULL DEFAULT 'silicon' CHECK (provider_type IN ('silicon','aws')),
    provider_budget_name text NOT NULL DEFAULT '',
    last_evaluated_amount numeric(14,2),
    last_evaluated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (account_id, organization_id) REFERENCES aws_accounts(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, organization_id) REFERENCES projects(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id, organization_id) REFERENCES environments(id, organization_id) ON DELETE CASCADE,
    CONSTRAINT aws_budgets_single_scope_check CHECK (num_nonnulls(account_id, project_id, environment_id) <= 1),
    UNIQUE (organization_id, name)
);
CREATE INDEX aws_budgets_scope_idx ON aws_budgets(organization_id, account_id, project_id, environment_id);

CREATE TABLE aws_cost_snapshots (
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    account_id uuid NOT NULL,
    period_start date NOT NULL,
    period_end date NOT NULL,
    total numeric(14,4) NOT NULL CHECK (total >= 0),
    project_costs jsonb NOT NULL DEFAULT '{}'::jsonb,
    environment_costs jsonb NOT NULL DEFAULT '{}'::jsonb,
    project_attribution_available boolean NOT NULL DEFAULT false,
    environment_attribution_available boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, account_id, period_start),
    FOREIGN KEY (account_id, organization_id) REFERENCES aws_accounts(id, organization_id) ON DELETE CASCADE
);

CREATE TABLE aws_operations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    account_id uuid NOT NULL,
    job_id uuid REFERENCES jobs(id) ON DELETE SET NULL,
    operation_type text NOT NULL CHECK (operation_type IN ('provision_machine','terminate_machine','create_snapshot','prepare_machine')),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','waiting_for_aws','waiting_for_connection','succeeded','failed')),
    resource_type text NOT NULL DEFAULT '',
    provider_resource_id text NOT NULL DEFAULT '',
    request jsonb NOT NULL DEFAULT '{}'::jsonb,
    error text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    FOREIGN KEY (account_id, organization_id) REFERENCES aws_accounts(id, organization_id) ON DELETE CASCADE
);
CREATE INDEX aws_operations_scope_idx ON aws_operations(organization_id, created_at DESC);

ALTER TABLE jobs DROP CONSTRAINT jobs_job_type_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_job_type_check CHECK (job_type IN (
    'build_application','deploy_application','rollback_deployment','collect_runtime_status','sync_domain','configure_tunnel',
    'provision_aws_machine','terminate_aws_machine','create_aws_snapshot','prepare_aws_machine'
));
