DROP TRIGGER IF EXISTS trg_budgets_set_updated_at ON budgets;
DROP FUNCTION IF EXISTS budgets_set_updated_at();

DROP INDEX IF EXISTS idx_budgets_category;
DROP INDEX IF EXISTS idx_budgets_workspace_year_month;

DROP TABLE IF EXISTS budgets;