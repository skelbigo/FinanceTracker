\
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

VIOLATIONS=0

grep_go() {
  local pattern="$1"
  grep -R --line-number -E -i "$pattern" "$ROOT_DIR" \
    --include="*.go" \
    --exclude="*_test.go" \
    --exclude="*_integration_test.go" \
    || true
}

check_table() {
  local table="$1"
  local owner_path="$2"
  local allow_regex="${3:-}"

  echo
  echo "==> No SQL on ${table} table outside ${owner_path}"

  local pattern="\\b(from|join)\\s+${table}\\b"
  local matches
  matches="$(grep_go "$pattern")"

  matches="$(echo "$matches" | grep -v "$owner_path" || true)"

  if [[ -n "$allow_regex" ]]; then
    matches="$(echo "$matches" | grep -vE "$allow_regex" || true)"
  fi

  if [[ -n "$matches" ]]; then
    echo "$matches"
    VIOLATIONS=1
  else
    echo "OK"
  fi
}

check_table "users"                 "/apps/auth-service/" ""
check_table "refresh_tokens"         "/apps/auth-service/" ""
check_table "transactions"           "/apps/transaction-service/" "/apps/gateway-http/analytics/"
check_table "categories"             "/apps/transaction-service/" "/apps/gateway-http/analytics/"
check_table "budgets"                "/apps/budget-service/" ""
check_table "budget_events"          "/apps/budget-service/" ""
check_table "notifications"          "/apps/notification-service/" ""
check_table "notification_delivery"  "/apps/notification-service/" ""
check_table "workspaces"             "/apps/gateway-http/workspaces/" ""
check_table "workspace_members"      "/apps/gateway-http/workspaces/" ""

echo
if [[ "$VIOLATIONS" -ne 0 ]]; then
  echo "Ownership violations found (prod code only). Fix by moving the SQL into the owning service OR exposing a port/contract."
  exit 1
fi

echo "No ownership violations found (prod code only)."
