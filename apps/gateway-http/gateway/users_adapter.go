package gateway

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/skelbigo/FinanceTracker/apps/auth-service/auth"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
	"github.com/skelbigo/FinanceTracker/apps/notification-service/notifications"
)

type AuthUsersAdapter struct {
	repo *auth.Repo
}

func NewAuthUsersAdapter(repo *auth.Repo) *AuthUsersAdapter {
	return &AuthUsersAdapter{repo: repo}
}

var _ notifications.UserLookup = (*AuthUsersAdapter)(nil)
var _ workspaces.UserDirectory = (*AuthUsersAdapter)(nil)

func (a *AuthUsersAdapter) GetUserEmail(ctx context.Context, userID string) (string, error) {
	u, err := a.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return u.Email, nil
}

func (a *AuthUsersAdapter) FindUserIDByEmail(ctx context.Context, email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := a.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", workspaces.ErrUserNotFound
		}
		return "", err
	}
	return u.ID, nil
}

func (a *AuthUsersAdapter) GetUserPublicByID(ctx context.Context, userID string) (workspaces.UserPublic, error) {
	u, err := a.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return workspaces.UserPublic{}, workspaces.ErrUserNotFound
		}
		return workspaces.UserPublic{}, err
	}
	return workspaces.UserPublic{Email: u.Email, Name: u.Name}, nil
}
