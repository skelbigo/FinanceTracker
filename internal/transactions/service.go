package transactions

import "context"

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service { return &Service{repo: repo} }

type ListResult struct {
	Items   []Transaction `json:"items"`
	HasNext bool          `json:"has_next"`
	Limit   int           `json:"limit"`
	Offset  int           `json:"offset"`
}

func (s *Service) Create(ctx context.Context, t Transaction) (Transaction, error) {

	return s.repo.Create(ctx, t)
}

func (s *Service) List(ctx context.Context, workspaceID string, f ListFilter) (ListResult, error) {
	pageSize := f.Limit
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	f.Limit = pageSize + 1
	f.Offset = offset

	items, err := s.repo.List(ctx, workspaceID, f)
	if err != nil {
		return ListResult{}, err
	}

	hasNext := false
	if len(items) > pageSize {
		hasNext = true
		items = items[:pageSize]
	}

	return ListResult{
		Items:   items,
		HasNext: hasNext,
		Limit:   pageSize,
		Offset:  offset,
	}, nil
}

func (s *Service) GetByID(ctx context.Context, workspaceID, txID string) (Transaction, error) {
	return s.repo.GetByID(ctx, workspaceID, txID)
}

func (s *Service) Update(ctx context.Context, t Transaction) (Transaction, error) {
	return s.repo.Update(ctx, t)
}

func (s *Service) Delete(ctx context.Context, workspaceID, txID string) (bool, error) {
	return s.repo.Delete(ctx, workspaceID, txID)
}
