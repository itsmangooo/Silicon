ALTER TABLE cloudflare_zones
    ADD CONSTRAINT cloudflare_zones_id_org_unique UNIQUE (id, organization_id);

CREATE TABLE system_public_access (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    organization_id uuid NOT NULL REFERENCES organizations(id),
    integration_id uuid NOT NULL,
    zone_id uuid NOT NULL,
    tunnel_id uuid NOT NULL,
    hostname citext NOT NULL,
    local_origin text NOT NULL,
    provider_record_id text NOT NULL,
    previous_public_url text NOT NULL,
    previous_cookie_secure boolean NOT NULL,
    previous_trust_forwarded_proto boolean NOT NULL,
    previous_bind_address inet NOT NULL,
    configured_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (integration_id, organization_id)
        REFERENCES cloudflare_integrations(id, organization_id),
    FOREIGN KEY (zone_id, organization_id)
        REFERENCES cloudflare_zones(id, organization_id),
    FOREIGN KEY (tunnel_id, organization_id)
        REFERENCES cloudflare_tunnels(id, organization_id)
);

CREATE TABLE system_public_access_operations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    action text NOT NULL CHECK (action IN ('configure','disable')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending','validating','configuring_cloudflare','updating_configuration',
        'restarting','waiting_for_health','active','disabled','failed'
    )),
    requested_by uuid REFERENCES users(id) ON DELETE SET NULL,
    organization_id uuid,
    integration_id uuid,
    zone_id uuid,
    tunnel_id uuid,
    hostname citext NOT NULL DEFAULT '',
    local_origin text NOT NULL DEFAULT '',
    provider_record_id text NOT NULL DEFAULT '',
    previous_public_url text NOT NULL DEFAULT '',
    previous_cookie_secure boolean,
    previous_trust_forwarded_proto boolean,
    previous_bind_address inet,
    message text NOT NULL DEFAULT '',
    failed_stage text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    completed_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (integration_id, organization_id)
        REFERENCES cloudflare_integrations(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (zone_id, organization_id)
        REFERENCES cloudflare_zones(id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (tunnel_id, organization_id)
        REFERENCES cloudflare_tunnels(id, organization_id) ON DELETE CASCADE,
    CHECK (
        (action='configure' AND organization_id IS NOT NULL AND integration_id IS NOT NULL
            AND zone_id IS NOT NULL AND tunnel_id IS NOT NULL AND hostname <> '')
        OR action='disable'
    )
);

CREATE UNIQUE INDEX system_public_access_one_active_operation_idx
    ON system_public_access_operations ((true))
    WHERE status IN ('pending','validating','configuring_cloudflare','updating_configuration','restarting','waiting_for_health');

CREATE INDEX system_public_access_operations_created_idx
    ON system_public_access_operations(created_at DESC);
