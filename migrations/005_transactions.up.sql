CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    category_id UUID NULL REFERENCES categories(id),
    type TEXT NOT NULL CHECK (type IN ('income', 'expense')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency CHAR(3) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    note TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_transactions_workspace_occurred
ON transactions(workspace_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_transactions_workspace_category
ON transactions(workspace_id, category_id);

CREATE INDEX IF NOT EXISTS idx_transactions_workspace_type
ON transactions(workspace_id, type);