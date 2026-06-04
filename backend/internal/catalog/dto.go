package catalog

import "github.com/google/uuid"

type CreateCategoryRequest struct {
	Name        string     `json:"name" validate:"required"`
	Slug        string     `json:"slug" validate:"required"`
	ParentID    *uuid.UUID `json:"parentId,omitempty"`
	Description *string    `json:"description,omitempty"`
	SortOrder   int        `json:"sortOrder"`
}

type CreateBrandRequest struct {
	Name        string  `json:"name" validate:"required"`
	Slug        string  `json:"slug" validate:"required"`
	Description *string `json:"description,omitempty"`
	LogoURL     *string `json:"logoUrl,omitempty"`
}

type CategoryListResponse struct {
	Items []Category `json:"items"`
}

type BrandListResponse struct {
	Items []Brand `json:"items"`
}
