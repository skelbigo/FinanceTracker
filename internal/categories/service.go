package categories

import (
	"context"
	"strings"
)

type Service struct{ repo *Repo }

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, workspaceID, name string, t Type) (Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, ErrInvalidType
	}

	return s.repo.CreateCategory(ctx, workspaceID, name, t)
}

func (s *Service) List(ctx context.Context, workspaceID string) ([]Category, error) {
	return s.repo.ListCategories(ctx, workspaceID)
}
