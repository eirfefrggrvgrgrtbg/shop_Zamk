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
