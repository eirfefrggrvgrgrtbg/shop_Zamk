package payments

import (
	"time"

	"github.com/google/uuid"
)

type Payment struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	OrderID           uuid.UUID  `json:"orderId" db:"order_id"`
	Provider          string     `json:"provider" db:"provider"`
	ProviderPaymentID *string    `json:"providerPaymentId" db:"provider_payment_id"`
	Status            string     `json:"status" db:"status"`
	AmountCents       int64      `json:"amountCents" db:"amount_cents"`
	Currency          string     `json:"currency" db:"currency"`
	PaymentURL        *string    `json:"paymentUrl" db:"payment_url"`
	IdempotencyKey    string     `json:"idempotencyKey" db:"idempotency_key"`
	CreatedAt         time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time  `json:"updatedAt" db:"updated_at"`
	PaidAt            *time.Time `json:"paidAt" db:"paid_at"`
	FailedAt          *time.Time `json:"failedAt" db:"failed_at"`
	CancelledAt       *time.Time `json:"cancelledAt" db:"cancelled_at"`
}

type PaymentEvent struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	PaymentID         *uuid.UUID `json:"paymentId" db:"payment_id"`
	Provider          string     `json:"provider" db:"provider"`
	ProviderPaymentID *string    `json:"providerPaymentId" db:"provider_payment_id"`
	EventType         string     `json:"eventType" db:"event_type"`
	RawPayload        []byte     `json:"rawPayload" db:"raw_payload"`
	SignatureValid    bool       `json:"signatureValid" db:"signature_valid"`
	ProcessedAt       *time.Time `json:"processedAt" db:"processed_at"`
	CreatedAt         time.Time  `json:"createdAt" db:"created_at"`
}
