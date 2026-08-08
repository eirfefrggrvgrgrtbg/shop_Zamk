package sellers

import (
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
	"github.com/google/uuid"
)

type CreateSellerRequest struct {
	OwnerName         string `json:"ownerName" validate:"required"`
	OwnerEmail        string `json:"ownerEmail" validate:"required,email"`
	GrantExistingUser bool   `json:"grantExistingUser"`
}

type CreateSellerResponse struct {
	Status                    string     `json:"status"` // created_new, granted_existing
	Seller                    Seller     `json:"seller"`
	OwnerUser                 users.User `json:"ownerUser"`
	TemporaryPassword         string     `json:"temporaryPassword,omitempty"`
	TemporaryPasswordReturned bool       `json:"temporaryPasswordReturned"`
}

type UpdateSellerStatusRequest struct {
	Status SellerStatus `json:"status" validate:"required"`
}


type PerformanceComponent struct {
	Code        string   `json:"code"`
	Label       string   `json:"label"`
	RawValue    *float64 `json:"rawValue"`
	Unit        string   `json:"unit"`
	Score       *int     `json:"score"`
	Weight      float64  `json:"weight"`
	Explanation string   `json:"explanation"`
}

type AdminListSeller struct {
	Seller
	OwnerName          string   `json:"ownerName"`
	OwnerEmail         string   `json:"ownerEmail"`
	WarningsActive     int      `json:"warningsActive"`
	PerformanceScore   *int     `json:"performanceScore"`
	PerformanceCategory    string                 `json:"performanceCategory"` // high, stable, attention, low, no_data
	PerformanceComponents  []PerformanceComponent `json:"performanceComponents"`
	PerformanceReasons     []string               `json:"performanceReasons"`
	GrossSales30d      int64    `json:"grossSales30d"`
	OrdersCount30d     int      `json:"ordersCount30d"`
	CancelRate30d      int      `json:"cancelRate30d"`
	Violations         int      `json:"violations"`
}

type ListSellersResponse struct {
	Items        []AdminListSeller `json:"items"`
	TotalCount   int               `json:"total"`
	Page         int               `json:"page"`
	Limit        int               `json:"limit"`
	TotalPages   int               `json:"totalPages"`
	StatusCounts map[string]int    `json:"statusCounts"`
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

type PublicSeller struct {
	ID          uuid.UUID `json:"id"`
	BrandName   string    `json:"brandName"`
	Slug        string    `json:"slug"`
	Description *string   `json:"description,omitempty"`
	LogoURL     *string   `json:"logoUrl,omitempty"`
}

// ---- Phase E: Seller Management DTOs ----

// UpdateSellerStatusRequest already exists but needs Reason added via pointer override below.
// We extend the existing struct via a separate type for history-aware update.
type UpdateSellerStatusWithReasonRequest struct {
	Status string  `json:"status"`
	Reason *string `json:"reason,omitempty"`
}

type VerifySellerRequest struct{}

type VerifySellerResponse struct {
	SellerID uuid.UUID `json:"sellerId"`
	Status   string    `json:"status"`
}

type VerifySellerMissingFieldsError struct {
	Error         string   `json:"error"`
	MissingFields []string `json:"missingFields"`
}

type SellerStatusHistoryItem struct {
	ID          uuid.UUID  `json:"id"`
	OldStatus   *string    `json:"oldStatus,omitempty"`
	NewStatus   string     `json:"newStatus"`
	Reason      *string    `json:"reason,omitempty"`
	ActorUserID *uuid.UUID `json:"actorUserId,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// Warning DTOs
type CreateWarningRequest struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type ResolveWarningRequest struct {
	ResolutionNote *string `json:"resolutionNote,omitempty"`
}

type WarningResponse struct {
	ID             uuid.UUID  `json:"id"`
	SellerID       uuid.UUID  `json:"sellerId"`
	Type           string     `json:"type"`
	Title          string     `json:"title"`
	Message        string     `json:"message"`
	Severity       string     `json:"severity"`
	Status         string     `json:"status"`
	ActorUserID    *uuid.UUID `json:"actorUserId,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`
	ResolvedBy     *uuid.UUID `json:"resolvedBy,omitempty"`
	ResolutionNote *string    `json:"resolutionNote,omitempty"`
}

// Violation DTOs
type CreateViolationRequest struct {
	Type             string `json:"type"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Severity         string `json:"severity"`
	CountsForPenalty bool   `json:"countsForPenalty"`
}

type ResolveViolationRequest struct {
	ResolutionNote *string `json:"resolutionNote,omitempty"`
}

type ViolationResponse struct {
	ID               uuid.UUID  `json:"id"`
	SellerID         uuid.UUID  `json:"sellerId"`
	Type             string     `json:"type"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Severity         string     `json:"severity"`
	Status           string     `json:"status"`
	CountsForPenalty bool       `json:"countsForPenalty"`
	ActorUserID      *uuid.UUID `json:"actorUserId,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	ResolvedAt       *time.Time `json:"resolvedAt,omitempty"`
	ResolvedBy       *uuid.UUID `json:"resolvedBy,omitempty"`
	ResolutionNote   *string    `json:"resolutionNote,omitempty"`
}

// CreateWarningInput is the internal input for repo.CreateWarning
type CreateWarningInput struct {
	SellerID    uuid.UUID
	Type        string
	Title       string
	Message     string
	Severity    string
	ActorUserID *uuid.UUID
}

// CreateViolationInput is the internal input for repo.CreateViolation
type CreateViolationInput struct {
	SellerID         uuid.UUID
	Type             string
	Title            string
	Description      string
	Severity         string
	CountsForPenalty bool
	ActorUserID      *uuid.UUID
}

// SellerDetailResponse is the extended admin view of a seller.
type SellerDetailResponse struct {
	ID           uuid.UUID `json:"id"`
	BrandName    *string   `json:"brandName,omitempty"`
	Slug         *string   `json:"slug,omitempty"`
	Description  *string   `json:"description,omitempty"`
	LogoURL      *string   `json:"logoUrl,omitempty"`
	ContactEmail *string   `json:"contactEmail,omitempty"`
	ContactPhone *string   `json:"contactPhone,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	Owner        struct {
		ID     uuid.UUID `json:"id"`
		Name   string    `json:"name"`
		Email  string    `json:"email"`
		Status string    `json:"status"`
	} `json:"owner"`
	Counts struct {
		WarningsActive          int `json:"warningsActive"`
		ViolationsActive        int `json:"violationsActive"`
		ActivePenaltyViolations int `json:"activePenaltyViolations"`
	} `json:"counts"`
	CommissionPolicy struct {
		BaseCommissionBps           int    `json:"baseCommissionBps"`
		PenaltyCommissionBps        int    `json:"penaltyCommissionBps"`
		PenaltyRule                 string `json:"penaltyRule"`
		CurrentAppliedCommissionBps int    `json:"currentAppliedCommissionBps"`
		AutomaticPenaltyEnabled     bool   `json:"automaticPenaltyEnabled"`
	} `json:"commissionPolicy"`
}

// ---- Phase B: Seller Onboarding DTOs ----

type InviteSellerRequest struct {
	FirstName         string `json:"firstName" validate:"required"`
	LastName          string `json:"lastName" validate:"required"`
	MiddleName        string `json:"middleName,omitempty"`
	Email             string `json:"email" validate:"required,email"`
	Phone             string `json:"phone,omitempty"`
	TemporaryPassword string `json:"temporaryPassword" validate:"required,min=8"`
}

type InviteSellerResponse struct {
	SellerID                  uuid.UUID `json:"sellerId"`
	OwnerUserID               uuid.UUID `json:"ownerUserId"`
	TemporaryPasswordReturned bool      `json:"temporaryPasswordReturned"`
}

type UpdateOnboardingStepRequest struct {
	CurrentStep int               `json:"currentStep" validate:"required,min=1,max=6"`
	Payload     OnboardingPayload `json:"payload" validate:"required"`
}

type SubmitOnboardingRequest struct {
	Payload OnboardingPayload `json:"payload" validate:"required"`
}

type RejectOnboardingRequest struct {
	ReviewComment string `json:"reviewComment" validate:"required"`
}

type RequestChangesOnboardingRequest struct {
	ReviewComment string `json:"reviewComment" validate:"required"`
}

type ApproveOnboardingRequest struct {
	ReviewComment string `json:"reviewComment,omitempty"`
}

type SellerOverviewResponse struct {
	Period string `json:"period"` // "7d", "30d", "all"

	Sales struct {
		GrossSalesCents        int64 `json:"grossSalesCents"`
		OrdersCount            int   `json:"ordersCount"`
		ItemsSold              int   `json:"itemsSold"`
		AverageOrderValueCents int64 `json:"averageOrderValueCents"`
		DeliveredOrders        int   `json:"deliveredOrders"`
		CancelledOrders        int   `json:"cancelledOrders"`
		ReturnedOrders         int   `json:"returnedOrders"`
		SellerCausedCancellationRate int `json:"sellerCausedCancellationRate"`
		ReturnRateBySellerReason     int `json:"returnRateBySellerReason"`
		PendingPayoutCents     int64 `json:"pendingPayoutCents"`
	} `json:"sales"`

	Catalog struct {
		ProductsTotal      int `json:"productsTotal"`
		ProductsPublished  int `json:"productsPublished"`
		ProductsModeration int `json:"productsModeration"`
		ProductsRejected   int `json:"productsRejected"`
		ProductsDraft      int `json:"productsDraft"`
		ProductsOutOfStock int `json:"productsOutOfStock"`
		ProductsLowStock   int `json:"productsLowStock"`
		VariantsTotal      int `json:"variantsTotal"`
	} `json:"catalog"`

	Fulfillment struct {
		// Seller Execution
		AssemblyStartedOnTimeRate float64 `json:"assemblyStartedOnTimeRate"`
		AssemblyOnTimeRate        float64 `json:"assemblyOnTimeRate"`
		AverageAssemblyTimeHours  float64 `json:"averageAssemblyTimeHours"`
		OverdueFulfillments       int     `json:"overdueFulfillments"`
		PackedOnTimeRate          float64 `json:"packedOnTimeRate"`
		HandoverOnTimeRate        float64 `json:"handoverOnTimeRate"`
		PickingErrorRate          float64 `json:"pickingErrorRate"`
		PackingErrorRate          float64 `json:"packingErrorRate"`
		
		// Logistics ZAMK
		ShipmentsCreated   int `json:"shipmentsCreated"`
		HandedToCourier    int `json:"handedToCourier"`
		Delivered          int `json:"delivered"`
		PlatformDelays     int `json:"platformDelays"`
		LogisticsDelays    int `json:"logisticsDelays"`

		// Legacy fields (kept for compatibility)
		FulfillmentsNew          int     `json:"fulfillmentsNew"`
		FulfillmentsProcessing   int     `json:"fulfillmentsProcessing"`
		FulfillmentsPacked       int     `json:"fulfillmentsPacked"`
		FulfillmentsShipped      int     `json:"fulfillmentsShipped"`
		FulfillmentsDelivered    int     `json:"fulfillmentsDelivered"`
		FulfillmentsProblematic  int     `json:"fulfillmentsProblematic"`
	} `json:"fulfillment"`

	Finance struct {
		PaidByCustomersCents    int64 `json:"paidByCustomersCents"`
		RefundsCents            int64 `json:"refundsCents"`
		PendingPayoutCents      int64 `json:"pendingPayoutCents"`
		PaidPayoutCents         int64 `json:"paidPayoutCents"`
		FrozenCents             int64 `json:"frozenCents"`
		PlatformCommissionCents int64 `json:"platformCommissionCents"`
		CommissionConfigured    bool  `json:"commissionConfigured"`
	} `json:"finance"`

	Quality struct {
		Rating           float64 `json:"rating"`
		ReviewsCount     int     `json:"reviewsCount"`
		WarningsActive   int     `json:"warningsActive"`
		WarningsClosed   int     `json:"warningsClosed"`
		ViolationsActive int     `json:"violationsActive"`
		OpenReturns      int     `json:"openReturns"`
		RejectedProducts int     `json:"rejectedProducts"`
	} `json:"quality"`

	Activity struct {
		LastLoginAt          *string `json:"lastLoginAt"`
		LastProductUpdatedAt *string `json:"lastProductUpdatedAt"`
		LastOrderProcessedAt *string `json:"lastOrderProcessedAt"`
		DaysWithoutActivity  *int    `json:"daysWithoutActivity"`
	} `json:"activity"`

	Performance struct {
		PerformanceCategory string              `json:"performanceCategory"`
		PerformanceReasons  []PerformanceReason `json:"performanceReasons"`
	} `json:"performance"`

	Profile struct {
		StoreCreated        bool     `json:"storeCreated"`
		StoreStatus         string   `json:"storeStatus"`
		OwnerAccessStatus   string   `json:"ownerAccessStatus"`
		ProfileCompleteness int      `json:"profileCompleteness"`
		MissingFields       []string `json:"missingFields"`
	} `json:"profile"`
}

type PerformanceReason struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Value int    `json:"value"`
	Unit  string `json:"unit,omitempty"`
}


type SellerNoteDTO struct {
	ID         string     `json:"id"`
	SellerID   string     `json:"sellerId"`
	AuthorID   *string    `json:"authorId,omitempty"`
	AuthorName *string    `json:"authorName,omitempty"` // populated from admin_users
	NoteType   string     `json:"noteType"`
	Content    string     `json:"content"`
	Deadline   *time.Time `json:"deadline,omitempty"`
	IsArchived bool       `json:"isArchived"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type CreateSellerNoteRequest struct {
	NoteType string     `json:"noteType" validate:"required"`
	Content  string     `json:"content" validate:"required"`
	Deadline *time.Time `json:"deadline,omitempty"`
}

type ImprovementPlanDTO struct {
	ID              string     `json:"id"`
	SellerID        string     `json:"sellerId"`
	AssigneeID      *string    `json:"assigneeId,omitempty"`
	AssigneeName    *string    `json:"assigneeName,omitempty"`
	CreatorID       *string    `json:"creatorId,omitempty"`
	CreatorName     *string    `json:"creatorName,omitempty"`
	Status          string     `json:"status"`
	Reason          string     `json:"reason"`
	Actions         []string   `json:"actions"`
	InternalComment string     `json:"internalComment"`
	Deadline        *time.Time `json:"deadline,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
}

type CreateImprovementPlanRequest struct {
	AssigneeID      *string    `json:"assigneeId,omitempty"`
	Reason          string     `json:"reason" validate:"required"`
	Actions         []string   `json:"actions" validate:"required,min=1"`
	InternalComment string     `json:"internalComment"`
	Deadline        *time.Time `json:"deadline,omitempty"`
}

type UpdateImprovementPlanStatusRequest struct {
	Status string `json:"status" validate:"required"`
}

type ArchiveSellerNoteRequest struct {
	IsArchived bool `json:"isArchived"`
}
