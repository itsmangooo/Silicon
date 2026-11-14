ALTER TABLE applications
    ADD COLUMN runtime_settings jsonb NOT NULL DEFAULT '{}',
    ADD CONSTRAINT applications_runtime_settings_object CHECK (jsonb_typeof(runtime_settings) = 'object');
