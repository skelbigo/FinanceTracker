CREATE TABLE IF NOT EXISTS budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    year INT NOT NULL,
    month INT NOT NULL CHECK (month BETWEEN 1 AND 12),
    amount BIGINT NOT NULL CHECK (amount >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT budgets_unique_ws_cat_year_month UNIQUE (workspace_id, category_id, year, month)
);

CREATE INDEX IF NOT EXISTS idx_budgets_workspace_year_month ON budgets (workspace_id, year, month);
CREATE INDEX IF NOT EXISTS idx_budgets_category ON budgets (category_id);

CREATE OR REPLACE FUNCTION budgets_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.apdated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_budgets_set_updated_at ON budgets;

CREATE TRIGGER trg_budgets_set_updated_at
BEFORE UPDATE ON budgets
FOR EACH ROW
EXECUTE FUNCTION budgets_set_updated_at();