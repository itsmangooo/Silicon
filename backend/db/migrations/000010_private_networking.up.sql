-- Silicon private networks are organization-scoped overlays. WireGuard private
-- keys stay on target hosts; the control plane persists public identity only.
CREATE TABLE silicon_networks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    cidr cidr NOT NULL,
    provider text NOT NULL DEFAULT 'wireguard' CHECK (provider IN ('wireguard')),
    topology text NOT NULL DEFAULT 'hub_spoke' CHECK (topology IN ('hub_spoke')),
    hub_server_id uuid NOT NULL,
    listen_port integer NOT NULL DEFAULT 51820 CHECK (listen_port BETWEEN 1 AND 65535),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','reconciling','active','degraded','error','deleting')),
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    last_reconciled_at timestamptz,
    UNIQUE (id, organization_id),
    UNIQUE (organization_id, name),
    UNIQUE (organization_id, cidr),
    FOREIGN KEY (hub_server_id, organization_id)
        REFERENCES servers(id, organization_id) ON DELETE RESTRICT,
    CHECK (family(cidr) = 4 AND masklen(cidr) BETWEEN 16 AND 29)
);
CREATE INDEX silicon_networks_scope_idx ON silicon_networks(organization_id, created_at DESC);

CREATE TABLE silicon_network_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    network_id uuid NOT NULL,
    server_id uuid NOT NULL,
    address inet NOT NULL,
    public_key text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','configuring','active','error','removing')),
    last_error text NOT NULL DEFAULT '',
    configuration_hash text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    last_reconciled_at timestamptz,
    UNIQUE (id, organization_id),
    UNIQUE (network_id, server_id),
    UNIQUE (network_id, server_id, organization_id),
    UNIQUE (network_id, address),
    FOREIGN KEY (network_id, organization_id)
        REFERENCES silicon_networks(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (server_id, organization_id)
        REFERENCES servers(id, organization_id) ON DELETE RESTRICT
);
CREATE INDEX silicon_network_members_scope_idx ON silicon_network_members(organization_id, network_id);

ALTER TABLE applications
    ADD CONSTRAINT applications_id_server_organization_unique
    UNIQUE (id, server_id, organization_id);

CREATE TABLE silicon_network_services (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    network_id uuid NOT NULL,
    application_id uuid NOT NULL,
    server_id uuid NOT NULL,
    hostname text NOT NULL CHECK (hostname ~ '^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*\.internal$'),
    protocol text NOT NULL DEFAULT 'tcp' CHECK (protocol IN ('tcp','udp')),
    port integer NOT NULL CHECK (port BETWEEN 1 AND 65535),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','error')),
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id),
    UNIQUE (network_id, application_id),
    UNIQUE (organization_id, application_id),
    UNIQUE (network_id, hostname),
    UNIQUE (id, network_id, organization_id),
    UNIQUE (application_id, network_id, organization_id),
    FOREIGN KEY (network_id, organization_id)
        REFERENCES silicon_networks(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (application_id, server_id, organization_id)
        REFERENCES applications(id, server_id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (network_id, server_id, organization_id)
        REFERENCES silicon_network_members(network_id, server_id, organization_id) ON DELETE RESTRICT
);
CREATE INDEX silicon_network_services_scope_idx ON silicon_network_services(organization_id, network_id);

CREATE TABLE silicon_network_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    network_id uuid NOT NULL,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    source_application_id uuid NOT NULL,
    destination_service_id uuid NOT NULL,
    protocol text NOT NULL CHECK (protocol IN ('tcp','udp')),
    port integer NOT NULL CHECK (port BETWEEN 1 AND 65535),
    action text NOT NULL CHECK (action IN ('allow','deny')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id),
    UNIQUE (network_id, name),
    FOREIGN KEY (network_id, organization_id)
        REFERENCES silicon_networks(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (source_application_id, network_id, organization_id)
        REFERENCES silicon_network_services(application_id, network_id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (destination_service_id, network_id, organization_id)
        REFERENCES silicon_network_services(id, network_id, organization_id) ON DELETE CASCADE
);
CREATE INDEX silicon_network_policies_scope_idx ON silicon_network_policies(organization_id, network_id);

CREATE TABLE silicon_network_operations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    network_id uuid NOT NULL,
    job_id uuid,
    operation_type text NOT NULL CHECK (operation_type IN ('reconcile','delete')),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','succeeded','failed')),
    error text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    UNIQUE (id, organization_id),
    FOREIGN KEY (network_id, organization_id)
        REFERENCES silicon_networks(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (job_id, organization_id)
        REFERENCES jobs(id, organization_id) ON DELETE SET NULL (job_id)
);
CREATE INDEX silicon_network_operations_scope_idx ON silicon_network_operations(organization_id, network_id, created_at DESC);

ALTER TABLE jobs DROP CONSTRAINT jobs_job_type_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_job_type_check CHECK (job_type IN (
    'build_application','deploy_application','rollback_deployment','collect_runtime_status','sync_domain','configure_tunnel',
    'provision_aws_machine','terminate_aws_machine','create_aws_snapshot','prepare_aws_machine','reconcile_network'
));
