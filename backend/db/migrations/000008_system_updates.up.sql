ALTER TABLE users
    ADD COLUMN is_system_admin boolean NOT NULL DEFAULT false;

-- Existing installations need one installation-level administrator. Preserve
-- every account and grant the capability only to the oldest account when no
-- explicit system administrator exists yet.
UPDATE users
SET is_system_admin = true
WHERE id = (
    SELECT id
    FROM users
    ORDER BY created_at, id
    LIMIT 1
)
AND NOT EXISTS (SELECT 1 FROM users WHERE is_system_admin);

CREATE TABLE system_updates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    requested_by uuid REFERENCES users(id) ON DELETE SET NULL,
    from_version text NOT NULL,
    target_version text NOT NULL CHECK (target_version ~ '^v[0-9]+\.[0-9]+\.[0-9]+$'),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN (
        'queued',
        'checking',
        'preparing',
        'updating',
        'migrating',
        'restarting',
        'waiting_for_health',
        'completed',
        'failed'
    )),
    message text NOT NULL DEFAULT '',
    release_notes text NOT NULL DEFAULT '',
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX system_updates_one_active_idx
    ON system_updates ((true))
    WHERE status IN ('queued','checking','preparing','updating','migrating','restarting','waiting_for_health');

CREATE INDEX system_updates_created_idx ON system_updates(created_at DESC);
