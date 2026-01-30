ALTER TABLE transactions
ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_transactions_tags_gin
ON transactions USING GIN (tags);