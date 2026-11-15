ALTER TABLE users DROP CONSTRAINT users_appearance_style_check,
    DROP CONSTRAINT users_appearance_studio_check;

UPDATE users
SET appearance_style = CASE appearance_style WHEN 'industrial' THEN 'flat' ELSE 'obsidian' END
WHERE appearance_style IN ('industrial', 'terminal');

UPDATE users
SET appearance_studio = jsonb_set(
    appearance_studio || '{"version":3}'::jsonb,
    '{presets}',
    COALESCE((
        SELECT jsonb_agg(
            CASE preset->>'style'
                WHEN 'industrial' THEN preset || '{"style":"flat"}'::jsonb
                WHEN 'terminal' THEN preset || '{"style":"obsidian"}'::jsonb
                ELSE preset
            END ORDER BY position)
        FROM jsonb_array_elements(
            CASE WHEN jsonb_typeof(appearance_studio->'presets') = 'array'
                THEN appearance_studio->'presets' ELSE '[]'::jsonb END
        ) WITH ORDINALITY AS items(preset, position)
    ), '[]'::jsonb)
)
WHERE appearance_studio IS NOT NULL;

ALTER TABLE users ADD CONSTRAINT users_appearance_style_check CHECK (
    appearance_style IN ('glass','playful','obsidian','soft','neumorphism','clay','bento','flat')
), ADD CONSTRAINT users_appearance_studio_check CHECK (
    appearance_studio IS NULL OR (
        jsonb_typeof(appearance_studio) = 'object' AND
        COALESCE(appearance_studio->>'version', '') IN ('2','3')
    )
);
