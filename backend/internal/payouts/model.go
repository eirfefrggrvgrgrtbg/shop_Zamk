package payouts

import (
	"time"

	"github.com/google/uuid"
)

type SellerCommissionRule struct {
	ID        uuid.UUID `json:"id" db:"id"`
	SellerID  uuid.UUID `json:"sellerId" db:"seller_id"`
	RateBPS   int       `json:"rateBps" db:"rate_bps"`
	Reason    string    `json:"reason" db:"reason"`
	CreatedBy uuid.UUID `json:"createdBy" db:"created_by"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

type PayoutBatch struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	SellerID      uuid.UUID  `json:"sellerId" db:"seller_id"`
	AmountCents   int64      `json:"amountCents" db:"amount_cents"`
	Status        string     `json:"status" db:"status"`
	ScheduledFor  time.Time  `json:"scheduledFor" db:"scheduled_for"`
	ProcessedAt   *time.Time `json:"processedAt" db:"processed_at"`
	FailureReason *string    `json:"failureReason" db:"failure_reason"`
	CreatedAt     time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt     time.Time  `json:"updatedAt" db:"updated_at"`
}

type SellerLedgerEntry struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	SellerID      uuid.UUID  `json:"sellerId" db:"seller_id"`
	OrderID       *uuid.UUID `json:"orderId" db:"order_id"`
	OrderItemID   *uuid.UUID `json:"orderItemId" db:"order_item_id"`
	PayoutBatchID *uuid.UUID `json:"payoutBatchId" db:"payout_batch_id"`
	Type          string     `json:"type" db:"type"`
	AmountCents   int64      `json:"amountCents" db:"amount_cents"`
	Currency      string     `json:"currency" db:"currency"`
	AvailableAt   *time.Time `json:"availableAt" db:"available_at"`
	Metadata      []byte     `json:"metadata" db:"metadata"`
	CreatedAt     time.Time  `json:"createdAt" db:"created_at"`
}
