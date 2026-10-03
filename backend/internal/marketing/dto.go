package marketing

import (
	"time"

	"github.com/google/uuid"
)

type CreateSellerCampaignRequest struct {
	Title                    string       `json:"title" validate:"required"`
	Description              *string      `json:"description,omitempty"`
	DiscountType             DiscountType `json:"discountType" validate:"required"`
	SellerDiscountBps        int          `json:"sellerDiscountBps"`
	SellerDiscountFixedCents int64        `json:"sellerDiscountFixedCents"`
	StartsAt                 *time.Time   `json:"startsAt,omitempty"`
	EndsAt                   *time.Time   `json:"endsAt,omitempty"`
}

type CreateCofundedApplicationRequest struct {
	Title                       string     `json:"title" validate:"required"`
	Description                 *string    `json:"description,omitempty"`
	SellerDiscountBps           int        `json:"sellerDiscountBps" validate:"required"`
	RequestedZamkShareBps       int        `json:"requestedZamkShareBps" validate:"required"`
	RequestedZamkBudgetCapCents int64      `json:"requestedZamkBudgetCapCents" validate:"required"`
	StartsAt                    *time.Time `json:"startsAt,omitempty"`
	EndsAt                      *time.Time `json:"endsAt,omitempty"`
}

type AdminReviewCampaignRequest struct {
	Action                 string  `json:"action" validate:"required"` // "approve", "reject"
	ApprovedZamkShareBps   *int    `json:"approvedZamkShareBps,omitempty"`
	ApprovedZamkBudgetCents *int64 `json:"approvedZamkBudgetCents,omitempty"`
	RejectionReason        *string `json:"rejectionReason,omitempty"`
	AdminComment           *string `json:"adminComment,omitempty"`
}

type CreatePromoCodeRequest struct {
	CampaignID              uuid.UUID    `json:"campaignId" validate:"required"`
	Code                    string       `json:"code" validate:"required"`
	DiscountType            DiscountType `json:"discountType" validate:"required"`
	DiscountValueBps        int          `json:"discountValueBps"`
	DiscountValueFixedCents int64        `json:"discountValueFixedCents"`
	MinOrderSubtotalCents   int64        `json:"minOrderSubtotalCents"`
	GlobalUsageLimit        *int         `json:"globalUsageLimit,omitempty"`
	PerCustomerUsageLimit   int          `json:"perCustomerUsageLimit"`
	FirstPaidOrderOnly      bool         `json:"firstPaidOrderOnly"`
	StartsAt                *time.Time   `json:"startsAt,omitempty"`
	EndsAt                  *time.Time   `json:"endsAt,omitempty"`
}

type SellerEligibilityResponse struct {
	SellerID               uuid.UUID `json:"sellerId"`
	SuccessfulSalesCount   int       `json:"successfulSalesCount"`
	RequiredSalesThreshold int       `json:"requiredSalesThreshold"`
	IsEligible             bool      `json:"isEligible"`
	HasOpenPlatformCampaign bool     `json:"hasOpenPlatformCampaign"`
}

// CreateSellerPromoRequest is the client payload for creating a seller-funded promotion.
type CreateSellerPromoRequest struct {
	Code                    string        `json:"code" validate:"required"`
	DiscountType            DiscountType  `json:"discountType" validate:"required"`
	DiscountValueBps        int           `json:"discountValueBps"`
	DiscountValueFixedCents int64         `json:"discountValueFixedCents"`
	MinOrderSubtotalCents   int64         `json:"minOrderSubtotalCents"`
	FirstPaidOrderOnly      bool          `json:"firstPaidOrderOnly"`
	ProductScope            *ProductScope `json:"productScope,omitempty"`
	IncludedProductIDs      []uuid.UUID   `json:"includedProductIds,omitempty"`
	ExcludedProductIDs      []uuid.UUID   `json:"excludedProductIds,omitempty"`
	MaxDiscountCents        *int64        `json:"maxDiscountCents,omitempty"`
	StartsAt                *time.Time    `json:"startsAt,omitempty"`
	EndsAt                  *time.Time    `json:"endsAt,omitempty"`
	GlobalUsageLimit        *int          `json:"globalUsageLimit,omitempty"`
	PerCustomerUsageLimit   int           `json:"perCustomerUsageLimit"`
	IsActive                *bool         `json:"isActive,omitempty"`
}

// UpdateSellerPromoRequest contains only the mutable operational fields for a promotion.
type UpdateSellerPromoRequest struct {
	IsActive              *bool      `json:"isActive,omitempty"`
	StartsAt              *time.Time `json:"startsAt,omitempty"`
	EndsAt                *time.Time `json:"endsAt,omitempty"`
	GlobalUsageLimit      *int       `json:"globalUsageLimit,omitempty"`
	PerCustomerUsageLimit *int       `json:"perCustomerUsageLimit,omitempty"`
}

// SellerPromoResponse represents a seller promo code item returned by the seller API.
type SellerPromoResponse struct {
	ID                      uuid.UUID    `json:"id"`
	CampaignID              uuid.UUID    `json:"campaignId"`
	SellerID                uuid.UUID    `json:"sellerId"`
	Code                    string       `json:"code"`
	DiscountType            DiscountType `json:"discountType"`
	DiscountValueBps        int          `json:"discountValueBps"`
	DiscountValueFixedCents int64        `json:"discountValueFixedCents"`
	MinOrderSubtotalCents   int64        `json:"minOrderSubtotalCents"`
	GlobalUsageLimit        *int         `json:"globalUsageLimit,omitempty"`
	PerCustomerUsageLimit   int          `json:"perCustomerUsageLimit"`
	FirstPaidOrderOnly      bool         `json:"firstPaidOrderOnly"`
	ProductScope            ProductScope `json:"productScope"`
	IncludedProductIDs      []uuid.UUID  `json:"includedProductIds"`
	ExcludedProductIDs      []uuid.UUID  `json:"excludedProductIds"`
	MaxDiscountCents        *int64       `json:"maxDiscountCents,omitempty"`
	IsActive                bool         `json:"isActive"`
	StartsAt                *time.Time   `json:"startsAt,omitempty"`
	EndsAt                  *time.Time   `json:"endsAt,omitempty"`
	ReservedUsageCount      int          `json:"reservedUsageCount"`
	ConsumedUsageCount      int          `json:"consumedUsageCount"`
	CreatedAt               time.Time    `json:"createdAt"`
	UpdatedAt               time.Time    `json:"updatedAt"`
	Status                  string       `json:"status"` // "scheduled", "active", "paused", "expired", "exhausted"
}

// SellerPromotionsResponse represents the response envelope for listing seller promotions.
type SellerPromotionsResponse struct {
	Items []SellerPromoResponse `json:"items"`
	Count int                   `json:"count"`
}

// SellerPromoItem holds database projection of promo_codes joined with usage counters.
type SellerPromoItem struct {
	PromoCode
	IncludedProductIDs []uuid.UUID
	ExcludedProductIDs []uuid.UUID
	ReservedCount      int
	ConsumedCount      int
}

// DerivePromoStatus calculates the exact derived display status matching checkout semantics.
func DerivePromoStatus(p *PromoCode, reservedCount, consumedCount int, now time.Time) string {
	if p.EndsAt != nil && now.After(*p.EndsAt) {
		return "expired"
	}
	if !p.IsActive {
		return "paused"
	}
	if p.StartsAt != nil && now.Before(*p.StartsAt) {
		return "scheduled"
	}
	if p.GlobalUsageLimit != nil && (reservedCount+consumedCount >= *p.GlobalUsageLimit) {
		return "exhausted"
	}
	return "active"
}
