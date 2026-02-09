DROP INDEX IF EXISTS idx_refresh_tokens_user_active;

ALTER TABLE refresh_tokens
DROP CONSTRAINT IF EXISTS fk_refresh_tokens_replaced_by;

ALTER TABLE refresh_tokens
DROP COLUMN IF EXISTS replaced_by_token_id,
    DROP COLUMN IF EXISTS user_agent,
    DROP COLUMN IF EXISTS ip;
