DROP INDEX IF EXISTS idx_transactions_tags_gin;
ALTER TABLE transactions
DROP COLUMN IF EXISTS tags;