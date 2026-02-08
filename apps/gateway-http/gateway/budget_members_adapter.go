package gateway

import (
	"context"

	"github.com/skelbigo/FinanceTracker/apps/budget-service/budgets"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
)

type budgetMembersAdapter struct {
	wsRepo *workspaces.Repo
}

func (a budgetMembersAdapter) ListMembers(ctx context.Context, workspaceID string) ([]budgets.MemberInfo, error) {
	ms, err := a.wsRepo.ListMembersInfo(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	out := make([]budgets.MemberInfo, 0, len(ms))
	for _, m := range ms {
		out = append(out, budgets.MemberInfo{
			UserID: m.UserID,
			Role:   budgets.Role(m.Role),
		})
	}
	return out, nil
}
