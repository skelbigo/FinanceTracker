DO $$
DECLARE
has_plain boolean;
  has_enc boolean;
BEGIN
  IF to_regclass('public.integrations') IS NULL THEN
    RETURN;
END IF;

SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='integrations'
      AND column_name IN ('access_token','refresh_token','api_key')
) INTO has_plain;

IF NOT has_plain THEN
    RETURN;
END IF;

SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='integrations'
      AND column_name IN ('access_token_enc','refresh_token_enc','api_key_enc')
) INTO has_enc;

IF NOT has_enc THEN
    RAISE EXCEPTION 'integrations encrypted columns are missing; run migration 019 first';
END IF;

  IF EXISTS (
    SELECT 1 FROM integrations
    WHERE access_token IS NOT NULL OR refresh_token IS NOT NULL OR api_key IS NOT NULL
  ) THEN
    RAISE EXCEPTION 'plaintext integration secrets still present; run cmd/crypto-migrate first';
END IF;

ALTER TABLE integrations
DROP COLUMN IF EXISTS access_token,
    DROP COLUMN IF EXISTS refresh_token,
    DROP COLUMN IF EXISTS api_key;
END $$;
