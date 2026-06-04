package catalog

import "errors"

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrBrandNotFound    = errors.New("brand not found")
	ErrDuplicateSlug    = errors.New("slug already exists")
)
