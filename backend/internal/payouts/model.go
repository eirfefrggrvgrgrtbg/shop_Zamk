package payouts

import (
	"time"

	"github.com/google/uuid"
)

type Payout struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	SellerID    uuid.UUID  `json:"sellerId" db:"seller_id"`
	Status      string     `json:"status" db:"status"`
	AmountCents int64      `json:"amountCents" db:"amount_cents"`
	Currency    string     `json:"currency" db:"currency"`
	RequestedAt time.Time  `json:"requestedAt" db:"requested_at"`
	ApprovedAt  *time.Time `json:"approvedAt" db:"approved_at"`
	RejectedAt  *time.Time `json:"rejectedAt" db:"rejected_at"`
	PaidAt      *time.Time `json:"paidAt" db:"paid_at"`
	AdminUserID *uuid.UUID `json:"adminUserId" db:"admin_user_id"`
	Comment     *string    `json:"comment" db:"comment"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time  `json:"updatedAt" db:"updated_at"`
}

type SellerBalanceLedger struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	SellerID    uuid.UUID  `json:"sellerId" db:"seller_id"`
	OrderID     *uuid.UUID `json:"orderId" db:"order_id"`
	OrderItemID *uuid.UUID `json:"orderItemId" db:"order_item_id"`
	ReturnID    *uuid.UUID `json:"returnId" db:"return_id"`
	RefundID    *uuid.UUID `json:"refundId" db:"refund_id"`
	PayoutID    *uuid.UUID `json:"payoutId" db:"payout_id"`
	Type        string     `json:"type" db:"type"`
	AmountCents int64      `json:"amountCents" db:"amount_cents"`
	Currency    string     `json:"currency" db:"currency"`
	AvailableAt *time.Time `json:"availableAt" db:"available_at"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
	Comment     *string    `json:"comment" db:"comment"`
}
