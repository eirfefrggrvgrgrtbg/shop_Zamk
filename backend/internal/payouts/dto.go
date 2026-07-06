package payouts

type BalanceResponse struct {
	PendingBalanceCents   int64  `json:"pendingBalanceCents"`
	AvailableBalanceCents int64  `json:"availableBalanceCents"`
	RequestedPayoutsCents int64  `json:"requestedPayoutsCents"`
	PaidPayoutsCents      int64  `json:"paidPayoutsCents"`
	Currency              string `json:"currency"`
}

type PayoutRequestDto struct {
	AmountCents int64   `json:"amountCents" validate:"required,gt=0"`
	Comment     *string `json:"comment"`
}

type UpdatePayoutStatusRequest struct {
	Status  string  `json:"status" validate:"required,oneof=approved rejected paid cancelled"`
	Comment *string `json:"comment"`
}

type PayoutListResponse struct {
	Items      []Payout `json:"items"`
	TotalCount int      `json:"totalCount"`
}

type PayoutResponse struct {
	Payout
}

type AdminPayoutSummary struct {
	TotalAvailableCents   int64 `json:"totalAvailableCents"`
	TotalPendingCents     int64 `json:"totalPendingCents"`
	TotalPaidCents        int64 `json:"totalPaidCents"`
	TotalRejectedCents    int64 `json:"totalRejectedCents"`
	TotalCommissionCents  int64 `json:"totalCommissionCents"`
	Currency              string `json:"currency"`
}

type AdminSellerBalance struct {
	SellerID              string `json:"sellerId"`
	SellerName            string `json:"sellerName"`
	PendingBalanceCents   int64  `json:"pendingBalanceCents"`
	AvailableBalanceCents int64  `json:"availableBalanceCents"`
	Currency              string `json:"currency"`
}

type AdminSellerBalanceListResponse struct {
	Items      []AdminSellerBalance `json:"items"`
	TotalCount int                  `json:"totalCount"`
}

type PayoutFilter struct {
	Q        string
	SellerID string
	Status   string
}
