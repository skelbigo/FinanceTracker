package transactions

import "context"

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service { return &Service{repo: repo} }

func (s *Service) Create(ctx context.Context, t Transaction) (Transaction, error) {
	return s.repo.Create(ctx, t)
}

func (s *Service) List(ctx context.Context, workspaceID string, f ListFilter) ([]Transaction, error) {
	return s.repo.List(ctx, workspaceID, f)
}
