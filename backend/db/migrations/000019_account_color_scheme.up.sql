ALTER TABLE users ADD COLUMN appearance_scheme text NOT NULL DEFAULT 'catppuccin'
    CHECK (appearance_scheme IN ('catppuccin', 'nord', 'tokyo-night', 'gruvbox'));
