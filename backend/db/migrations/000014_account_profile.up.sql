ALTER TABLE users
    ADD COLUMN appearance_style text NOT NULL DEFAULT 'soft'
        CHECK (appearance_style IN ('soft','glass','playful','obsidian','metal','industrial','bento','terminal')),
    ADD COLUMN appearance_mode text NOT NULL DEFAULT 'dark'
        CHECK (appearance_mode IN ('dark','light','system')),
    ADD COLUMN appearance_density text NOT NULL DEFAULT 'comfortable'
        CHECK (appearance_density IN ('comfortable','compact')),
    ADD COLUMN avatar bytea CHECK (octet_length(avatar) <= 524288);

CREATE TABLE account_email_changes (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email citext NOT NULL,
    token_hash bytea UNIQUE NOT NULL,
    expires_at timestamptz NOT NULL
);
