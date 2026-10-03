package marketing

import (
	"time"

	"github.com/google/uuid"
)

type FundingMode string

const (
	FundingModeSeller   FundingMode = "seller"
	FundingModeZamk     FundingMode = "zamk"
	FundingModeCofunded FundingMode = "cofunded"
)

type CampaignStatus string

const (
	CampaignStatusDraft          CampaignStatus = "draft"
	CampaignStatusSubmitted      CampaignStatus = "submitted"
	CampaignStatusCounterOffered CampaignStatus = "counter_offered"
	CampaignStatusApproved       CampaignStatus = "approved"
	CampaignStatusActive         CampaignStatus = "active"
	CampaignStatusEnded          CampaignStatus = "ended"
	CampaignStatusRejected       CampaignStatus = "rejected"
	CampaignStatusCancelled      CampaignStatus = "cancelled"
)

type DiscountType string

const (
	DiscountTypePercent DiscountType = "percent"
	DiscountTypeFixed   DiscountType = "fixed"
)

type UsageStatus string

const (
	UsageStatusReserved UsageStatus = "reserved"
	UsageStatusConsumed UsageStatus = "consumed"
	UsageStatusReleased UsageStatus = "released"
	UsageStatusExpired  UsageStatus = "expired"
)

type PriceChangeSource string

const (
	PriceChangeSourceSeller PriceChangeSource = "seller"
	PriceChangeSourceAdmin  PriceChangeSource = "admin"
	PriceChangeSourceSystem PriceChangeSource = "system"
	PriceChangeSourceImport PriceChangeSource = "import"
)

// MarketingCampaign represents a unified marketing promotion or co-funding application.
type MarketingCampaign struct {
	ID                         uuid.UUID      `json:"id" db:"id"`
	SellerID                   uuid.UUID      `json:"sellerId" db:"seller_id"`
	Title                      string         `json:"title" db:"title"`
	Description                *string        `json:"description,omitempty" db:"description"`
	FundingMode                FundingMode    `json:"fundingMode" db:"funding_mode"`
	Status                     CampaignStatus `json:"status" db:"status"`
	DiscountType               DiscountType   `json:"discountType" db:"discount_type"`
	SellerDiscountBps          int            `json:"sellerDiscountBps" db:"seller_discount_bps"`
	SellerDiscountFixedCents   int64          `json:"sellerDiscountFixedCents" db:"seller_discount_fixed_cents"`
	RequestedZamkShareBps      int            `json:"requestedZamkShareBps" db:"requested_zamk_share_bps"`
	RequestedZamkBudgetCapCents int64         `json:"requestedZamkBudgetCapCents" db:"requested_zamk_budget_cap_cents"`
	ApprovedZamkShareBps       int            `json:"approvedZamkShareBps" db:"approved_zamk_share_bps"`
	ApprovedZamkBudgetCapCents int64          `json:"approvedZamkBudgetCapCents" db:"approved_zamk_budget_cap_cents"`
	ZamkReservedCents          int64          `json:"zamkReservedCents" db:"zamk_reserved_cents"`
	ZamkSpentCents             int64          `json:"zamkSpentCents" db:"zamk_spent_cents"`
	RejectionReason            *string        `json:"rejectionReason,omitempty" db:"rejection_reason"`
	AdminComment               *string        `json:"adminComment,omitempty" db:"admin_comment"`
	StartsAt                   *time.Time     `json:"startsAt,omitempty" db:"starts_at"`
	EndsAt                     *time.Time     `json:"endsAt,omitempty" db:"ends_at"`
	SubmittedAt                *time.Time     `json:"submittedAt,omitempty" db:"submitted_at"`
	DecidedAt                  *time.Time     `json:"decidedAt,omitempty" db:"decided_at"`
	DecidedByStaffID           *uuid.UUID     `json:"decidedByStaffId,omitempty" db:"decided_by_staff_id"`
	CreatedAt                  time.Time      `json:"createdAt" db:"created_at"`
	UpdatedAt                  time.Time      `json:"updatedAt" db:"updated_at"`
}

type ProductScope string

const (
	ProductScopeEntireStore        ProductScope = "ENTIRE_STORE"
	ProductScopeSelectedProducts   ProductScope = "SELECTED_PRODUCTS"
	ProductScopeSelectedCategories ProductScope = "SELECTED_CATEGORIES"
)

type TargetType string

const (
	TargetTypeInclude TargetType = "INCLUDE"
	TargetTypeExclude TargetType = "EXCLUDE"
)

// PromoCodeProductTarget maps promo codes to explicitly included or excluded products.
type PromoCodeProductTarget struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	PromoCodeID uuid.UUID  `json:"promoCodeId" db:"promo_code_id"`
	ProductID   uuid.UUID  `json:"productId" db:"product_id"`
	TargetType  TargetType `json:"targetType" db:"target_type"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
}

// PromoCodeCategoryTarget maps promo codes to explicitly included or excluded categories.
type PromoCodeCategoryTarget struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	PromoCodeID uuid.UUID  `json:"promoCodeId" db:"promo_code_id"`
	CategoryID  uuid.UUID  `json:"categoryId" db:"category_id"`
	TargetType  TargetType `json:"targetType" db:"target_type"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
}

// PromoCode represents a customer-facing promotional voucher linked to a campaign.
type PromoCode struct {
	ID                      uuid.UUID    `json:"id" db:"id"`
	CampaignID              uuid.UUID    `json:"campaignId" db:"campaign_id"`
	SellerID                uuid.UUID    `json:"sellerId" db:"seller_id"`
	Code                    string       `json:"code" db:"code"`
	DiscountType            DiscountType `json:"discountType" db:"discount_type"`
	DiscountValueBps        int          `json:"discountValueBps" db:"discount_value_bps"`
	DiscountValueFixedCents int64        `json:"discountValueFixedCents" db:"discount_value_fixed_cents"`
	MinOrderSubtotalCents   int64        `json:"minOrderSubtotalCents" db:"min_order_subtotal_cents"`
	GlobalUsageLimit        *int         `json:"globalUsageLimit,omitempty" db:"global_usage_limit"`
	PerCustomerUsageLimit   int          `json:"perCustomerUsageLimit" db:"per_customer_usage_limit"`
	FirstPaidOrderOnly      bool         `json:"firstPaidOrderOnly" db:"first_paid_order_only"`
	ProductScope            ProductScope `json:"productScope" db:"product_scope"`
	MaxDiscountCents        *int64       `json:"maxDiscountCents,omitempty" db:"max_discount_cents"`
	IsActive                bool         `json:"isActive" db:"is_active"`
	StartsAt                *time.Time   `json:"startsAt,omitempty" db:"starts_at"`
	EndsAt                  *time.Time   `json:"endsAt,omitempty" db:"ends_at"`
	CreatedAt               time.Time    `json:"createdAt" db:"created_at"`
	UpdatedAt               time.Time    `json:"updatedAt" db:"updated_at"`
}

// PromoUsage tracks the lifecycle of a promo code application for a specific order.
type PromoUsage struct {
	ID                  uuid.UUID   `json:"id" db:"id"`
	PromoCodeID         uuid.UUID   `json:"promoCodeId" db:"promo_code_id"`
	CampaignID          uuid.UUID   `json:"campaignId" db:"campaign_id"`
	OrderID             uuid.UUID   `json:"orderId" db:"order_id"`
	UserID              uuid.UUID   `json:"userId" db:"user_id"`
	Status              UsageStatus `json:"status" db:"status"`
	SubsidyCents        int64       `json:"subsidyCents" db:"subsidy_cents"`
	SellerDiscountCents int64       `json:"sellerDiscountCents" db:"seller_discount_cents"`
	ReservedAt          time.Time   `json:"reservedAt" db:"reserved_at"`
	ConsumedAt          *time.Time  `json:"consumedAt,omitempty" db:"consumed_at"`
	ReleasedAt          *time.Time  `json:"releasedAt,omitempty" db:"released_at"`
	ExpiresAt           time.Time   `json:"expiresAt" db:"expires_at"`
	IsFirstOrder        bool        `json:"isFirstOrder" db:"is_first_order"`
}

// ProductPriceHistory records historical product and variant price changes for reference verification.
type ProductPriceHistory struct {
	ID               uuid.UUID         `json:"id" db:"id"`
	ProductID        uuid.UUID         `json:"productId" db:"product_id"`
	ProductVariantID *uuid.UUID        `json:"productVariantId,omitempty" db:"product_variant_id"`
	OldPriceCents    int64             `json:"oldPriceCents" db:"old_price_cents"`
	NewPriceCents    int64             `json:"newPriceCents" db:"new_price_cents"`
	ChangedByUserID  *uuid.UUID        `json:"changedByUserId,omitempty" db:"changed_by_user_id"`
	Source           PriceChangeSource `json:"source" db:"source"`
	Reason           *string           `json:"reason,omitempty" db:"reason"`
	CreatedAt        time.Time         `json:"createdAt" db:"created_at"`
}

// OrderItemPromotion preserves an immutable financial split snapshot for a promotional order item.
type OrderItemPromotion struct {
	ID                          uuid.UUID `json:"id" db:"id"`
	OrderItemID                 uuid.UUID `json:"orderItemId" db:"order_item_id"`
	OrderID                     uuid.UUID `json:"orderId" db:"order_id"`
	SellerID                    uuid.UUID `json:"sellerId" db:"seller_id"`
	CampaignID                  uuid.UUID `json:"campaignId" db:"campaign_id"`
	PromoCodeID                 uuid.UUID `json:"promoCodeId" db:"promo_code_id"`
	BaseUnitPriceCents          int64     `json:"baseUnitPriceCents" db:"base_unit_price_cents"`
	SellerDiscountUnitCents     int64     `json:"sellerDiscountUnitCents" db:"seller_discount_unit_cents"`
	ZamkSubsidyUnitCents        int64     `json:"zamkSubsidyUnitCents" db:"zamk_subsidy_unit_cents"`
	CustomerPaidUnitPriceCents  int64     `json:"customerPaidUnitPriceCents" db:"customer_paid_unit_price_cents"`
	CommissionBaseUnitCents     int64     `json:"commissionBaseUnitCents" db:"commission_base_unit_cents"`
	Quantity                    int       `json:"quantity" db:"quantity"`
	TotalSellerDiscountCents    int64     `json:"totalSellerDiscountCents" db:"total_seller_discount_cents"`
	TotalZamkSubsidyCents       int64     `json:"totalZamkSubsidyCents" db:"total_zamk_subsidy_cents"`
	TotalCustomerPaidCents      int64     `json:"totalCustomerPaidCents" db:"total_customer_paid_cents"`
	TotalCommissionBaseCents    int64     `json:"totalCommissionBaseCents" db:"total_commission_base_cents"`
	CommissionRateBps           int       `json:"commissionRateBps" db:"commission_rate_bps"`
	TotalCommissionChargedCents int64     `json:"totalCommissionChargedCents" db:"total_commission_charged_cents"`
	CreatedAt                   time.Time `json:"createdAt" db:"created_at"`
}

// PromotedLineResult holds the calculated promotional economics for a single promoted order line.
type PromotedLineResult struct {
	OrderItemID                 uuid.UUID
	ProductID                   uuid.UUID
	ProductVariantID            uuid.UUID
	SellerID                    uuid.UUID
	BaseUnitPriceCents          int64
	Quantity                    int
	SellerDiscountUnitCents     int64
	ZamkSubsidyUnitCents        int64
	CustomerPaidUnitPriceCents  int64
	CommissionBaseUnitCents     int64
	TotalSellerDiscountCents    int64
	TotalZamkSubsidyCents       int64
	TotalCustomerPaidCents      int64
	TotalCommissionBaseCents    int64
	CommissionRateBps           int
	TotalCommissionChargedCents int64
}

// CheckoutPromoCalculation contains the fully verified and computed checkout promotional breakdown.
type CheckoutPromoCalculation struct {
	PromoCodeID              uuid.UUID
	CampaignID               uuid.UUID
	SellerID                 uuid.UUID
	FundingMode              FundingMode
	DiscountType             DiscountType
	Code                     string
	IsFirstOrder             bool
	TotalSellerDiscountCents int64
	TotalZamkSubsidyCents    int64
	TotalCustomerPaidCents   int64
	TotalOrderDiscountCents  int64
	PromotedLines            []PromotedLineResult
}
