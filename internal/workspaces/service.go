package workspaces

import (
	"context"
	"strings"
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateWorkspace(ctx context.Context, creatorID, name, currency string) (Workspace, Role, error) {
	name = strings.TrimSpace(name)
	currency = strings.TrimSpace(currency)

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
	return s.repo.ListMembersInfo(ctx, workspaceID)
}

func (s *Service) AddMemberByEmail(ctx context.Context, workspaceID, email string, role Role) error {
	email = strings.TrimSpace(strings.ToLower(email))

	if role != RoleOwner && role != RoleMember && role != RoleViewer {
		return ErrInvalidRole
	}

	userID, err := s.repo.FindUserIDByEmail(ctx, email)
	if err != nil {
		return err
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
