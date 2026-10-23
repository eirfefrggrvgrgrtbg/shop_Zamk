package payouts

import (
	"time"

	"github.com/google/uuid"
)

type BalanceResponse struct {
	GrossSalesCents  int64  `json:"grossSalesCents"`
	CommissionCents  int64  `json:"commissionCents"`
	AdjustmentsCents int64  `json:"adjustmentsCents"`
	FrozenCents      int64  `json:"frozenCents"`
	AvailableCents   int64  `json:"availableCents"`
	PaidCents        int64  `json:"paidCents"`
	ReturnHoldCents  int64  `json:"returnHoldCents"` // earning under an open return: neither available nor paid
	Currency         string `json:"currency"`
	NextPayoutAt     *time.Time `json:"nextPayoutAt"`
}

type PayoutBatchListResponse struct {
	Items      []PayoutBatch `json:"items"`
	TotalCount int           `json:"totalCount"`
}

type LedgerListResponse struct {
	Items      []SellerLedgerEntry `json:"items"`
	TotalCount int                 `json:"totalCount"`
}

type AdminSellerCommissionRequest struct {
	RateBPS int    `json:"rateBps" validate:"required,min=0,max=10000"`
	Reason  string `json:"reason" validate:"required"`
}

type PayoutFilter struct {
	Q        string
	SellerID string
	Status   string
}

type AdminPayoutSummary struct {
	TotalAvailableCents  int64  `json:"totalAvailableCents"`
	TotalFrozenCents     int64  `json:"totalFrozenCents"`
	TotalPendingCents    int64  `json:"totalPendingCents"`
	TotalReturnHoldCents int64  `json:"totalReturnHoldCents"`
	TotalPaidCents       int64  `json:"totalPaidCents"`
	TotalCommissionCents int64  `json:"totalCommissionCents"`
	Currency             string `json:"currency"`
}

type AdminSellerBalance struct {
	SellerID              string `json:"sellerId"`
	SellerName            string `json:"sellerName"`
	FrozenBalanceCents    int64  `json:"frozenBalanceCents"`
	AvailableBalanceCents int64  `json:"availableBalanceCents"`
	Currency              string `json:"currency"`
}

type AdminSellerBalanceListResponse struct {
	Items      []AdminSellerBalance `json:"items"`
	TotalCount int                  `json:"totalCount"`
}

// AdminPayoutListItem is one payout batch as the admin sees it (PAYOUT.ADMIN.1). The list shows BATCHES:
// statuses are the payout_batches statuses (scheduled, processing, paid, failed, held).
type AdminPayoutListItem struct {
	PayoutBatch
	SellerName      *string `json:"sellerName"`
	Currency        string  `json:"currency"`
	EarningsCount   int     `json:"earningsCount"`
	EntriesCount    int     `json:"entriesCount"`
	ReturnHoldCents int64   `json:"returnHoldCents"`
}

// AdminPayoutEntry is one ledger row linked to a batch.
type AdminPayoutEntry struct {
	ID           uuid.UUID  `json:"id"`
	Type         string     `json:"type"`
	Reason       *string    `json:"reason"`
	AmountCents  int64      `json:"amountCents"`
	OrderID      *uuid.UUID `json:"orderId"`
	OrderNumber  *string    `json:"orderNumber"`
	OrderItemID  *uuid.UUID `json:"orderItemId"`
	ProductTitle *string    `json:"productTitle"`
	AvailableAt  *time.Time `json:"availableAt"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// AdminPayoutTotals splits a batch by ledger meaning; earnings + holds + adjustments = the batch amount at
// formation, payout is the manually confirmed transfer (negative), if any.
type AdminPayoutTotals struct {
	EarningsCents    int64 `json:"earningsCents"`
	ReturnHoldCents  int64 `json:"returnHoldCents"`
	AdjustmentsCents int64 `json:"adjustmentsCents"`
	PayoutCents      int64 `json:"payoutCents"`
}

type AdminPayoutDetail struct {
	Payout                AdminPayoutListItem `json:"payout"`
	Entries               []AdminPayoutEntry  `json:"entries"`
	Totals                AdminPayoutTotals   `json:"totals"`
	UncoveredDisputeCents int64               `json:"uncoveredDisputeCents"`
	CanConfirmPaid        bool                `json:"canConfirmPaid"`
	CanHold               bool                `json:"canHold"`
	ConfirmBlockedReason  string              `json:"confirmBlockedReason,omitempty"`
}
