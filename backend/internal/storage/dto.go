package storage

import "github.com/google/uuid"

type UploadImageResponse struct {
	ID        *uuid.UUID `json:"id,omitempty"`
	ImageURL  string     `json:"imageUrl"`
	ObjectKey string     `json:"objectKey,omitempty"` // For internal/admin use
	AltText   string     `json:"altText,omitempty"`
	SortOrder int        `json:"sortOrder,omitempty"`
	IsMain    bool       `json:"isMain,omitempty"`
}

type BrandLogoResponse struct {
	LogoURL string `json:"logoUrl"`
}

type SellerLogoResponse struct {
	LogoURL string `json:"logoUrl"`
}
