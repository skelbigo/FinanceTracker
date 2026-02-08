package gateway

import (
	"context"

	"github.com/skelbigo/FinanceTracker/internal/budgets"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

type budgetMembersAdapter struct {
	wsRepo interface {
		ListMembersInfo(ctx context.Context, workspaceID string) ([]workspaces.MemberInfo, error)
	}
}

func (a budgetMembersAdapter) ListMembers(ctx context.Context, workspaceID string) ([]budgets.MemberInfo, error) {
	items, err := a.wsRepo.ListMembersInfo(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]budgets.MemberInfo, 0, len(items))
	for _, m := range items {
		out = append(out, budgets.MemberInfo{
			UserID: m.UserID,
			Role:   budgets.Role(m.Role),
		})
	}
	return out, nil
}
