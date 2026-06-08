package sellers

import "github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"

type CreateSellerRequest struct {
	BrandName         string  `json:"brandName" validate:"required"`
	Slug              *string `json:"slug,omitempty"`
	Description       *string `json:"description,omitempty"`
	ContactEmail      string  `json:"contactEmail" validate:"required,email"`
	ContactPhone      *string `json:"contactPhone,omitempty"`
	OwnerName         string  `json:"ownerName" validate:"required"`
	OwnerEmail        string  `json:"ownerEmail" validate:"required,email"`
	TemporaryPassword string  `json:"temporaryPassword" validate:"required,min=8"`
}

type CreateSellerResponse struct {
	Seller                   Seller     `json:"seller"`
	OwnerUser                users.User `json:"ownerUser"`
	TemporaryPasswordReturned bool       `json:"temporaryPasswordReturned"`
}

type UpdateSellerStatusRequest struct {
	Status SellerStatus `json:"status" validate:"required"`
}

type ListSellersResponse struct {
	Items      []Seller `json:"items"`
	TotalCount int      `json:"totalCount"`
}

type SellerMeResponse struct {
	Seller     Seller     `json:"seller"`
	SellerUser SellerUser `json:"sellerUser"`
	User       users.User `json:"user"`
}

type UpdateSellerProfileRequest struct {
	BrandName    *string `json:"brandName,omitempty"`
	Description  *string `json:"description,omitempty"`
	ContactEmail *string `json:"contactEmail,omitempty"`
	ContactPhone *string `json:"contactPhone,omitempty"`
	Slug         *string `json:"slug,omitempty"`
}
