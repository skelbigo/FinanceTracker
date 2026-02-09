package workspaces

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/stringsx"
)

type Service struct {
	repo  *Repo
	users UserDirectory
}

type UserDirectory interface {
	FindUserIDByEmail(ctx context.Context, email string) (string, error)
	GetUserPublicByID(ctx context.Context, userID string) (UserPublic, error)
}

type UserPublic struct {
	Email string
	Name  *string
}

func NewService(repo *Repo, users UserDirectory) *Service {
	return &Service{repo: repo, users: users}
}

func normalizeWorkspaceName(name string) (string, error) {
	name = stringsx.TrimStrip(name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return "", ErrInvalidWorkspaceName
	}
	return name, nil
}

func normalizeCurrencyOptional(currency string) (string, error) {
	currency = strings.ToUpper(stringsx.TrimStrip(currency))
	if currency == "" {
		return "", nil
	}
	if len(currency) != 3 {
		return "", ErrInvalidCurrency
	}
	for i := 0; i < 3; i++ {
		ch := currency[i]
		if ch < 'A' || ch > 'Z' {
			return "", ErrInvalidCurrency
		}
	}
	return currency, nil
}

func (s *Service) CreateWorkspace(ctx context.Context, creatorID, name, currency string) (Workspace, Role, error) {
	var err error
	name, err = normalizeWorkspaceName(name)
	if err != nil {
		return Workspace{}, "", err
	}
	currency, err = normalizeCurrencyOptional(currency)
	if err != nil {
		return Workspace{}, "", err
	}

	w, err := s.repo.CreateWorkspaceWithOwner(ctx, creatorID, name, currency)
	if err != nil {
		return Workspace{}, "", err
	}
	return w, RoleOwner, nil
}

func (s *Service) ListMyWorkspaces(ctx context.Context, userID string) ([]WorkspaceListItem, error) {
	return s.repo.ListMyWorkspaces(ctx, userID)
}

func (s *Service) GetWorkspace(ctx context.Context, workspaceID, userID string) (Workspace, Role, error) {
	return s.repo.GetWorkspaceWithRole(ctx, workspaceID, userID)
}

func (s *Service) ListMembers(ctx context.Context, workspaceID string) ([]MemberInfo, error) {
	members, err := s.repo.ListMembersInfo(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if s.users == nil {
		return members, nil
	}
	for i := range members {
		u, err := s.users.GetUserPublicByID(ctx, members[i].UserID)
		if err == nil {
			members[i].Email = u.Email
			members[i].Name = u.Name
		}
	}
	return members, nil
}

func (s *Service) AddMemberByEmail(ctx context.Context, workspaceID, email string, role Role) error {
	email = stringsx.TrimStrip(strings.ToLower(email))

	if role != RoleOwner && role != RoleMember && role != RoleViewer {
		return ErrInvalidRole
	}

	if s.users == nil {
		return ErrUserNotFound
	}

	userID, err := s.users.FindUserIDByEmail(ctx, email)
	if err != nil {
		return err
	}

	if err := s.repo.AddMemberByUserID(ctx, workspaceID, userID, role); err != nil {
		return err
	}

	return nil
}

func (s *Service) AddMemberByUserID(ctx context.Context, workspaceID, userID string, role Role) error {
	userID = strings.TrimSpace(userID)

	if role != RoleOwner && role != RoleMember && role != RoleViewer {
		return ErrInvalidRole
	}
	if userID == "" {
		return ErrUserNotFound
	}

	if err := s.repo.AddMemberByUserID(ctx, workspaceID, userID, role); err != nil {
		return err
	}
	return nil
}

func (s *Service) UpdateMemberRole(ctx context.Context, workspaceID, actorUserID, targetUserID string, newRole Role) error {
	if newRole != RoleOwner && newRole != RoleMember && newRole != RoleViewer {
		return ErrInvalidRole
	}
	return s.repo.UpdateMemberRoleSafe(ctx, workspaceID, actorUserID, targetUserID, newRole)
}

func (s *Service) RemoveMember(ctx context.Context, workspaceID, actorUserID, targetUserID string) error {
	return s.repo.RemoveMemberSafe(ctx, workspaceID, actorUserID, targetUserID)
}
