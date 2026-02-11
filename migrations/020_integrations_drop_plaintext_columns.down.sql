DO $$
BEGIN
  IF to_regclass('public.integrations') IS NULL THEN
    RETURN;
END IF;

ALTER TABLE integrations
    ADD COLUMN IF NOT EXISTS access_token  TEXT,
    ADD COLUMN IF NOT EXISTS refresh_token TEXT,
    ADD COLUMN IF NOT EXISTS api_key       TEXT;
END $$;
