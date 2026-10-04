CREATE TABLE system_mail_configurations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider text NOT NULL CHECK (provider IN ('resend','postmark','mailgun','ses','smtp')),
    from_name text NOT NULL CHECK (char_length(from_name) BETWEEN 1 AND 120),
    from_address citext NOT NULL,
    reply_to citext,
    settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    encrypted_credentials bytea NOT NULL,
    active boolean NOT NULL DEFAULT true,
    status text NOT NULL DEFAULT 'not_tested' CHECK (status IN ('not_tested','healthy','failed')),
    safe_last_error text NOT NULL DEFAULT '',
    last_success_at timestamptz,
    last_failure_at timestamptz,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, provider)
);
CREATE UNIQUE INDEX system_mail_one_active_idx ON system_mail_configurations(active) WHERE active;

CREATE TABLE mail_deliveries (
    id uuid PRIMARY KEY,
    configuration_id uuid NOT NULL REFERENCES system_mail_configurations(id) ON DELETE RESTRICT,
    purpose text NOT NULL CHECK (char_length(purpose) BETWEEN 1 AND 100),
    recipient citext NOT NULL,
    provider text NOT NULL,
    encrypted_message bytea NOT NULL,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','sending','sent','failed','cancelled')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 10),
    available_at timestamptz NOT NULL DEFAULT now(),
    locked_at timestamptz,
    locked_by text,
    sent_at timestamptz,
    safe_last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (configuration_id, provider)
        REFERENCES system_mail_configurations(id, provider)
);
CREATE INDEX mail_deliveries_claim_idx ON mail_deliveries(status, available_at, created_at)
    WHERE status IN ('queued','sending');
CREATE INDEX mail_deliveries_created_idx ON mail_deliveries(created_at DESC);

CREATE TABLE password_reset_request_limits (
    id bigserial PRIMARY KEY,
    email_hash bytea NOT NULL,
    ip_hash bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX password_reset_request_email_idx ON password_reset_request_limits(email_hash, created_at DESC);
CREATE INDEX password_reset_request_ip_idx ON password_reset_request_limits(ip_hash, created_at DESC);

ALTER TABLE password_reset_tokens
    ADD COLUMN requested_ip_hash bytea,
    ADD COLUMN superseded_at timestamptz;
