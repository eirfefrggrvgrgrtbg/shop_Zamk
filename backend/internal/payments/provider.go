package payments

import (
	"context"
	"strings"
)

type ProviderStatusCategory string

const (
	ProviderStatusCategorySuccess         ProviderStatusCategory = "success"
	ProviderStatusCategoryTerminalFailure ProviderStatusCategory = "terminal_failure"
	ProviderStatusCategoryNonTerminal     ProviderStatusCategory = "non_terminal"
	ProviderStatusCategoryUnknown         ProviderStatusCategory = "unknown"
)

// ClassifyProviderStatus maps any raw provider status string to its canonical category.
// It is the single source of truth for both Webhook and CheckOrder reconciliation.
func ClassifyProviderStatus(status string) ProviderStatusCategory {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "CONFIRMED":
		return ProviderStatusCategorySuccess

	case "REJECTED", "CANCELED", "DEADLINE_EXPIRED":
		return ProviderStatusCategoryTerminalFailure

	case "AUTHORIZED", "AUTH_FAIL", "NEW", "FORMSHOWED", "FORM_SHOWED",
		"3DS_CHECKING", "3DS_CHECKED", "CHECKING", "CHECKED",
		"AUTHORIZING", "CONFIRMING":
		return ProviderStatusCategoryNonTerminal

	default:
		return ProviderStatusCategoryUnknown
	}
}

type CreatePaymentInput struct {
	OrderID         string
	AmountCents     int64
	Currency        string
	IdempotencyKey  string
	Description     string
	Method          string
	IntegrationMode string
}

type ProviderCreatePaymentResult struct {
	ProviderPaymentID string
	PaymentURL        string
	Status            string
}

type ProviderWebhookEvent struct {
	ProviderPaymentID string
	ProviderEventID   *string
	EventKey          string
	OrderID           string
	Status            string // "succeeded", "failed", "cancelled", etc.
	ProviderStatus    string // raw status from provider (e.g., "AUTHORIZED", "CONFIRMED")
	AmountCents       int64
	RawPayload        []byte
}

type ProviderPaymentItem struct {
	ProviderPaymentID string
	AmountCents       int64
	Status            string // "succeeded", "pending", "cancelled", etc.
	ProviderStatus    string // raw status from provider (e.g., "CONFIRMED", "AUTHORIZED", "REJECTED", "CANCELED", "DEADLINE_EXPIRED", "NEW")
	Success           bool
}

type CheckOrderResult struct {
	OrderID  string
	Payments []ProviderPaymentItem
}

type Provider interface {
	CreatePayment(ctx context.Context, input CreatePaymentInput) (ProviderCreatePaymentResult, error)
	VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) error
	ParseWebhook(ctx context.Context, body []byte) (ProviderWebhookEvent, error)
	GetMode(method string) string
	CheckOrder(ctx context.Context, orderID string) (CheckOrderResult, error)
}
