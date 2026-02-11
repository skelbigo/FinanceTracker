DO $$
BEGIN
  IF to_regclass('public.integrations') IS NULL THEN
    RETURN;
END IF;

ALTER TABLE integrations
DROP COLUMN IF EXISTS access_token_enc,
    DROP COLUMN IF EXISTS refresh_token_enc,
    DROP COLUMN IF EXISTS api_key_enc;
END $$;
