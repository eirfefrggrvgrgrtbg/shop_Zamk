package payments

import "context"

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

type Provider interface {
	CreatePayment(ctx context.Context, input CreatePaymentInput) (ProviderCreatePaymentResult, error)
	VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) error
	ParseWebhook(ctx context.Context, body []byte) (ProviderWebhookEvent, error)
	GetMode(method string) string
}
