UPDATE users SET appearance_style = 'soft' WHERE appearance_style = 'metal';

ALTER TABLE users
    DROP CONSTRAINT users_appearance_style_check,
    ADD CONSTRAINT users_appearance_style_check
        CHECK (appearance_style IN ('soft','glass','playful','obsidian','industrial','bento','terminal'));

CREATE FUNCTION pg_temp.remove_metal_effects(theme jsonb) RETURNS jsonb
LANGUAGE sql IMMUTABLE STRICT AS $$
    SELECT CASE WHEN jsonb_typeof(theme->'effects') = 'object'
        THEN jsonb_set(theme - 'quality', '{effects}',
            (theme->'effects') - ARRAY['metalMotion','reflection','distortion','highlight','metalSpeed'])
        ELSE theme - 'quality'
    END
$$;

WITH migrated AS (
    SELECT id,
        appearance_studio || jsonb_build_object('theme', pg_temp.remove_metal_effects(appearance_studio->'theme')) ||
        CASE WHEN jsonb_typeof(appearance_studio->'presets') = 'array' THEN
            jsonb_build_object('presets', (
                SELECT COALESCE(jsonb_agg(
                    preset || jsonb_build_object('theme', pg_temp.remove_metal_effects(preset->'theme')) ||
                    CASE WHEN preset->>'style' = 'metal' THEN '{"style":"soft"}'::jsonb ELSE '{}'::jsonb END
                    ORDER BY position), '[]'::jsonb)
                FROM jsonb_array_elements(appearance_studio->'presets') WITH ORDINALITY AS items(preset, position)
            ))
        ELSE '{}'::jsonb END AS studio
    FROM users
    WHERE appearance_studio IS NOT NULL
)
UPDATE users SET appearance_studio = migrated.studio
FROM migrated
WHERE users.id = migrated.id AND users.appearance_studio IS DISTINCT FROM migrated.studio;

DROP FUNCTION pg_temp.remove_metal_effects(jsonb);
