package categories

import "errors"

var (
	ErrCategoryExists = errors.New("category already exists")
	ErrInvalidType    = errors.New("invalid category type")
)
