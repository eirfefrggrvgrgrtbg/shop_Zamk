package sellers

import (
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
	"github.com/google/uuid"
)

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
	BrandName    string    `json:"brandName"`
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
