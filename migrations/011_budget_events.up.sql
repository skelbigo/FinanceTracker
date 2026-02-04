CREATE TABLE IF NOT EXISTS budget_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    spent_minor BIGINT NOT NULL,
    limit_minor BIGINT NOT NULL,
    currency TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (budget_id, period_start, period_end)
);

CREATE INDEX IF NOT EXISTS idx_budget_events_workspace_id ON budget_events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_budget_events_budget_id ON budget_events(budget_id);