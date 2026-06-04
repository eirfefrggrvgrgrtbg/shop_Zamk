package sellers

import (
	"time"

	"github.com/google/uuid"
)

type SellerStatus string

const (
	StatusPending  SellerStatus = "pending"
	StatusActive   SellerStatus = "active"
	StatusBlocked  SellerStatus = "blocked"
	StatusArchived SellerStatus = "archived"
)

type SellerRole string

const (
	RoleOwner   SellerRole = "owner"
	RoleManager SellerRole = "manager"
)

type Seller struct {
	ID           uuid.UUID    `json:"id"`
	BrandName    string       `json:"brandName"`
	Slug         string       `json:"slug"`
	Description  *string      `json:"description,omitempty"`
	ContactEmail string       `json:"contactEmail"`
	ContactPhone *string      `json:"contactPhone,omitempty"`
	Status       SellerStatus `json:"status"`
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`
}

type SellerUser struct {
	ID        uuid.UUID  `json:"id"`
	SellerID  uuid.UUID  `json:"sellerId"`
	UserID    uuid.UUID  `json:"userId"`
	Role      SellerRole `json:"role"`
	CreatedAt time.Time  `json:"createdAt"`
}
