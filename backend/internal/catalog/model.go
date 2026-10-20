package catalog

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID          uuid.UUID  `json:"id"`
	ParentID    *uuid.UUID `json:"parentId,omitempty"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description *string    `json:"description,omitempty"`
	SortOrder   int        `json:"sortOrder"`
	IsActive    bool       `json:"isActive"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	// SizeSystem is the category's default size system with its active values (listing only; omitted when none).
	SizeSystem *CategorySizeSystem `json:"sizeSystem,omitempty"`
}

type CategorySizeSystem struct {
	Code   string              `json:"code"`
	Name   string              `json:"name"`
	Values []CategorySizeValue `json:"values"`
}

type CategorySizeValue struct {
	Value     string `json:"value"`
	SortOrder int    `json:"sortOrder"`
}

type Brand struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Description   *string   `json:"description,omitempty"`
	LogoURL       *string   `json:"logoUrl,omitempty"`
	LogoObjectKey *string   `json:"logoObjectKey,omitempty"`
	IsActive      bool      `json:"isActive"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
