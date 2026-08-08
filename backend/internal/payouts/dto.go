package payouts

import "time"

type BalanceResponse struct {
	GrossSalesCents  int64  `json:"grossSalesCents"`
	CommissionCents  int64  `json:"commissionCents"`
	AdjustmentsCents int64  `json:"adjustmentsCents"`
	FrozenCents      int64  `json:"frozenCents"`
	AvailableCents   int64  `json:"availableCents"`
	PaidCents        int64  `json:"paidCents"`
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
