package sellers

import (
	"time"

	"github.com/google/uuid"
)

type SellerStatus string

const (
	StatusPendingSetup SellerStatus = "pending_setup"
	StatusPending      SellerStatus = "pending"
	StatusActive       SellerStatus = "active"
	StatusBlocked      SellerStatus = "blocked"
	StatusArchived     SellerStatus = "archived"
)

type SellerRole string

const (
	RoleOwner   SellerRole = "owner"
	RoleManager SellerRole = "manager"
)

type Seller struct {
	ID            uuid.UUID    `json:"id"`
	BrandName     *string      `json:"brandName,omitempty"`
	Slug          *string      `json:"slug,omitempty"`
	Description   *string      `json:"description,omitempty"`
	ContactEmail  *string      `json:"contactEmail,omitempty"`
	ContactPhone  *string      `json:"contactPhone,omitempty"`
	AverageRating float64      `json:"averageRating"`
	ReviewsCount  int          `json:"reviewsCount"`
	Status        SellerStatus `json:"status"`
	IsPlatform    bool         `json:"isPlatform"`
	LogoURL       *string      `json:"logoUrl,omitempty"`
	LogoObjectKey *string      `json:"logoObjectKey,omitempty"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

type SellerUser struct {
	ID        uuid.UUID  `json:"id"`
	SellerID  uuid.UUID  `json:"sellerId"`
	UserID    uuid.UUID  `json:"userId"`
	Role      SellerRole `json:"role"`
	CreatedAt time.Time  `json:"createdAt"`
}

// SellerDetail is the internal aggregate used by GetSellerDetailByID.
type SellerDetail struct {
	ID            uuid.UUID
	BrandName     *string
	Slug          *string
	Description   *string
	ContactEmail  *string
	ContactPhone  *string
	AverageRating float64 `json:"averageRating"`
	ReviewsCount  int     `json:"reviewsCount"`
	LogoURL       *string
	Status        SellerStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time

	OwnerID     uuid.UUID
	OwnerName   string
	OwnerEmail  string
	OwnerStatus string

	WarningsActive          int
	ViolationsActive        int
	ActivePenaltyViolations int
}

type SellersFilter struct {
	Query       string
	Status      []string
	Store       string
	Problems    string
	RatingMin   *float64
	RatingMax   *float64
	HasReviews  *bool
	PerformanceMin *int
	PerformanceMax *int
	PerformanceCategory string
	SalesGrossMin *int64
	SalesGrossMax *int64
	OrdersCountMin *int
	OrdersCountMax *int
	HasWarnings *bool
	HasViolations *bool
	Blocked     *bool
	Sort        string
	Direction   string
	Limit       int
	Offset      int
}
