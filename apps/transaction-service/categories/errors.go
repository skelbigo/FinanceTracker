package categories

import "errors"

var (
	ErrCategoryExists   = errors.New("category already exists")
	ErrInvalidType      = errors.New("invalid category type")
	ErrInvalidName      = errors.New("invalid category name")
	ErrCategoryNotFound = errors.New("category not found")
	ErrCategoryInUse    = errors.New("category is used by transactions")
)
