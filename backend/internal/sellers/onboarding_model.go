package sellers

import (
	"time"

	"github.com/google/uuid"
)

type OnboardingStatus string

const (
	OnboardingStatusNotStarted       OnboardingStatus = "not_started"
	OnboardingStatusDraft            OnboardingStatus = "draft"
	OnboardingStatusPendingReview    OnboardingStatus = "pending_review"
	OnboardingStatusChangesRequested OnboardingStatus = "changes_requested"
	OnboardingStatusApproved         OnboardingStatus = "approved"
	OnboardingStatusRejected         OnboardingStatus = "rejected"
)

type SellerOnboardingApplication struct {
	ID            uuid.UUID         `json:"id"`
	SellerID      uuid.UUID         `json:"sellerId"`
	Status        OnboardingStatus  `json:"status"`
	CurrentStep   int               `json:"currentStep"`
	Payload       OnboardingPayload `json:"payload"`
	ReviewComment *string           `json:"reviewComment,omitempty"`
	SubmittedAt   *time.Time        `json:"submittedAt,omitempty"`
	ReviewedAt    *time.Time        `json:"reviewedAt,omitempty"`
	ReviewedBy    *uuid.UUID        `json:"reviewedBy,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

type OnboardingPayload struct {
	Owner OwnerPayload `json:"owner"`
	Store StorePayload `json:"store"`
	Brand BrandPayload `json:"brand"`
	Legal LegalPayload `json:"legal"`
}

type OwnerPayload struct {
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	MiddleName string `json:"middleName,omitempty"`
	Phone      string `json:"phone,omitempty"`
	Contact    string `json:"contact,omitempty"`
}

type StorePayload struct {
	Name             string            `json:"name"`
	Slug             string            `json:"slug"`
	ShortDescription string            `json:"shortDescription,omitempty"`
	Description      string            `json:"description,omitempty"`
	LogoURL          string            `json:"logoUrl,omitempty"`
	CoverURL         string            `json:"coverUrl,omitempty"`
	PublicEmail      string            `json:"publicEmail,omitempty"`
	PublicPhone      string            `json:"publicPhone,omitempty"`
	SocialLinks      map[string]string `json:"socialLinks,omitempty"`
}

type BrandPayload struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	LogoURL     string `json:"logoUrl,omitempty"`
	Country     string `json:"country,omitempty"`
}

type LegalPayload struct {
	SellerType              string `json:"sellerType"`
	LegalName               string `json:"legalName"`
	TaxID                   string `json:"taxId"`
	RegistrationNumber      string `json:"registrationNumber"`
	PayoutDetailsConfigured bool   `json:"payoutDetailsConfigured"`
}

type SellerBrand struct {
	ID               uuid.UUID `json:"id"`
	SellerID         uuid.UUID `json:"sellerId"`
	BrandID          uuid.UUID `json:"brandId"`
	IsPrimary        bool      `json:"isPrimary"`
	RelationshipType string    `json:"relationshipType"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}
