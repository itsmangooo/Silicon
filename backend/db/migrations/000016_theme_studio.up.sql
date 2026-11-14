ALTER TABLE users
    ADD COLUMN appearance_studio jsonb CHECK (
        appearance_studio IS NULL OR
        (jsonb_typeof(appearance_studio) = 'object' AND appearance_studio->>'version' IS NOT DISTINCT FROM '2')
    ),
    ADD COLUMN appearance_background bytea CHECK (octet_length(appearance_background) <= 4194304);
