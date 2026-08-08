package payments

import "github.com/google/uuid"

type CreatePaymentRequest struct {
	Method string `json:"method"`
}

type CreatePaymentResponse struct {
	PaymentID       uuid.UUID `json:"paymentId"`
	PaymentNumber   string    `json:"paymentNumber"`
	Provider        string    `json:"provider"`
	PaymentMethod   string    `json:"paymentMethod"`
	Status          string    `json:"status"`
	AmountCents     int64     `json:"amountCents"`
	Currency        string    `json:"currency"`
	PaymentURL      string    `json:"paymentUrl"`
	IntegrationMode string    `json:"integrationMode"`
}

type CustomerSummaryDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Phone string    `json:"phone"`
}

type OrderSummaryDTO struct {
	OrderID         uuid.UUID           `json:"orderId"`
	OrderNumber     string              `json:"orderNumber"`
	OrderStatus     string              `json:"orderStatus"`
	OrderTotalCents int64               `json:"orderTotalCents"`
	Customer        *CustomerSummaryDTO `json:"customer"`
	CreatedAt       string              `json:"createdAt"`
}

type PaymentProblem struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
}

type AdminPaymentDTO struct {
	PaymentID         uuid.UUID `json:"paymentId"`
	PaymentNumber     string    `json:"paymentNumber"`
	ProviderPaymentID *string   `json:"providerPaymentId"`

	OrderID     uuid.UUID `json:"orderId"`
	OrderNumber string    `json:"orderNumber"`

	Customer *CustomerSummaryDTO `json:"customer"`

	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`

	Status          string  `json:"status"`
	Provider        *string `json:"provider"`
	PaymentMethod   *string `json:"paymentMethod"`
	IntegrationMode *string `json:"integrationMode"`

	AttemptNumber int `json:"attemptNumber"`
	AttemptsCount int `json:"attemptsCount"`

	RefundState                  string `json:"refundState"`
	PaidAmountCents              int64  `json:"paidAmountCents"`
	SucceededRefundedAmountCents int64  `json:"succeededRefundedAmountCents"`
	PendingRefundAmountCents     int64  `json:"pendingRefundAmountCents"`
	ReservedRefundAmountCents    int64  `json:"reservedRefundAmountCents"`
	NetAmountCents               int64  `json:"netAmountCents"`
	AvailableToRefundCents       int64  `json:"availableToRefundCents"`

	Problems []PaymentProblem `json:"problems"`

	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	PaidAt      *string `json:"paidAt"`
	FailedAt    *string `json:"failedAt"`
	CancelledAt *string `json:"cancelledAt"`
}

type AdminPaymentListResponse struct {
	Items      []AdminPaymentDTO `json:"items"`
	TotalCount int               `json:"totalCount"`
}

type AdminPaymentAttemptDTO struct {
	PaymentID         uuid.UUID `json:"paymentId"`
	PaymentNumber     string    `json:"paymentNumber"`
	AttemptNumber     int       `json:"attemptNumber"`
	Status            string    `json:"status"`
	Provider          *string   `json:"provider"`
	PaymentMethod     *string   `json:"paymentMethod"`
	AmountCents       int64     `json:"amountCents"`
	ProviderPaymentID *string   `json:"providerPaymentId"`
	CreatedAt         string    `json:"createdAt"`
	TerminalAt        *string   `json:"terminalAt"`
}

type SafePaymentEventDTO struct {
	EventID           uuid.UUID      `json:"eventId"`
	EventType         string         `json:"eventType"`
	SignatureValid    bool           `json:"signatureValid"`
	ProcessedAt       *string        `json:"processedAt"`
	CreatedAt         string         `json:"createdAt"`
	EventKey          string         `json:"eventKey"`
	SafePayloadSummary map[string]any `json:"safePayloadSummary"`
}

type AdminRefundDTO struct {
	RefundID         uuid.UUID `json:"refundId"`
	Status           string    `json:"status"`
	AmountCents      int64     `json:"amountCents"`
	ProviderRefundID *string   `json:"providerRefundId"`
	CreatedAt        string    `json:"createdAt"`
	ProcessedAt      *string   `json:"processedAt"`
}

type AdminPaymentDetailDTO struct {
	Payment        AdminPaymentDTO          `json:"payment"`
	Order          OrderSummaryDTO          `json:"order"`
	Attempts       []AdminPaymentAttemptDTO `json:"attempts"`
	ProviderEvents []SafePaymentEventDTO    `json:"providerEvents"`
	Refunds        []AdminRefundDTO         `json:"refunds"`
	Problems       []PaymentProblem         `json:"problems"`
}
