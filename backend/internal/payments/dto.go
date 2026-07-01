package payments

import "github.com/google/uuid"

type CreatePaymentResponse struct {
	PaymentID   uuid.UUID `json:"paymentId"`
	Provider    string    `json:"provider"`
	Status      string    `json:"status"`
	AmountCents int64     `json:"amountCents"`
	Currency    string    `json:"currency"`
	PaymentURL  string    `json:"paymentUrl"`
}

type AdminPaymentListResponse struct {
	Items      []Payment `json:"items"`
	TotalCount int       `json:"totalCount"`
}
