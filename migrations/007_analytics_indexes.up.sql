CREATE INDEX IF NOT EXISTS idx_transactions_ws_type_occurred
    ON transactions(workspace_id, type, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_transactions_ws_category_occurred
    ON transactions(workspace_id, category_id, occurred_at DESC);
