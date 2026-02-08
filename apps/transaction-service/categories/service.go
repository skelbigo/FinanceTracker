package categories

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Service struct{ repo *Repo }

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

func validateType(t Type) bool {
	return t == TypeIncome || t == TypeExpense
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalidName
	}
	if utf8.RuneCountInString(name) > 60 {
		return "", ErrInvalidName
	}
	return name, nil
}

func (s *Service) Create(ctx context.Context, workspaceID, name string, t Type) (Category, error) {
	name, err := normalizeName(name)
	if err != nil {
		return Category{}, err
	}
	if !validateType(t) {
		return Category{}, ErrInvalidType
	}

	return s.repo.CreateCategory(ctx, workspaceID, name, t)
}

func (s *Service) Update(ctx context.Context, workspaceID, categoryID, name string, t Type) (Category, error) {
	name, err := normalizeName(name)
	if err != nil {
		return Category{}, err
	}
	if !validateType(t) {
		return Category{}, ErrInvalidType
	}
	if strings.TrimSpace(categoryID) == "" {
		return Category{}, ErrCategoryNotFound
	}
	return s.repo.UpdateCategory(ctx, workspaceID, categoryID, name, t)
}

func (s *Service) Delete(ctx context.Context, workspaceID, categoryID string, reassignTo *string) error {
	if strings.TrimSpace(categoryID) == "" {
		return ErrCategoryNotFound
	}
	if reassignTo != nil {
		v := strings.TrimSpace(*reassignTo)
		if v == "" {
			reassignTo = nil
		} else if v == "none" {
			reassignTo = &v
		} else {
			if _, err := uuid.Parse(v); err != nil {
				return ErrCategoryNotFound
			}
			reassignTo = &v
		}
	}
	return s.repo.DeleteCategoryWithReassign(ctx, workspaceID, categoryID, reassignTo)
}

func (s *Service) List(ctx context.Context, workspaceID string) ([]Category, error) {
	return s.repo.ListCategories(ctx, workspaceID)
}
