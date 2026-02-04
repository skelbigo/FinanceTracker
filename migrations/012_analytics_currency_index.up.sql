CREATE INDEX IF NOT EXISTS idx_transactions_ws_currency_occurred
    ON transactions(workspace_id, currency, occurred_at DESC);