DROP TABLE IF EXISTS budgets CASCADE;
DROP FUNCTION IF EXISTS budgets_set_updated_at();

CREATE TABLE IF NOT EXISTS budgets (
                                       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    period TEXT NOT NULL CHECK (period IN ('week', 'month')),
    amount_limit_minor BIGINT NOT NULL CHECK (amount_limit_minor > 0),
    currency CHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT budgets_unique_ws_cat_period_currency UNIQUE (workspace_id, category_id, period, currency)
    );

CREATE INDEX IF NOT EXISTS idx_budgets_workspace_id ON budgets(workspace_id);
CREATE INDEX IF NOT EXISTS idx_budgets_category_id ON budgets(category_id);

CREATE OR REPLACE FUNCTION budgets_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_budgets_set_updated_at ON budgets;

CREATE TRIGGER trg_budgets_set_updated_at
    BEFORE UPDATE ON budgets
    FOR EACH ROW
    EXECUTE FUNCTION budgets_set_updated_at();
