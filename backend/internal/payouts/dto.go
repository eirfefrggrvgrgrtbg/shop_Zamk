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
