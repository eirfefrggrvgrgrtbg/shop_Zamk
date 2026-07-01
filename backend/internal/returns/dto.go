package returns

import "github.com/google/uuid"

type CreateReturnRequest struct {
	Reason  string                   `json:"reason" validate:"required"`
	Comment *string                  `json:"comment"`
	Items   []CreateReturnItemRequest `json:"items" validate:"required,min=1"`
}

type CreateReturnItemRequest struct {
	OrderItemID uuid.UUID `json:"orderItemId" validate:"required"`
	Quantity    int       `json:"quantity" validate:"required,min=1"`
	Reason      *string   `json:"reason"`
	Condition   *string   `json:"condition"`
}

type UpdateReturnStatusRequest struct {
	Status       string  `json:"status" validate:"required,oneof=approved rejected item_received completed cancelled"`
	AdminComment *string `json:"adminComment"`
	// For item_received/completed status, optionally specify restock decision per item
	ItemRestock []UpdateReturnItemRestockRequest `json:"itemRestock"`
}

type UpdateReturnItemRestockRequest struct {
	ReturnItemID uuid.UUID `json:"returnItemId" validate:"required"`
	Restock      bool      `json:"restock"`
}

type CreateRefundRequest struct {
	Reason *string `json:"reason"`
}

type ReturnResponse struct {
	Return
	Items []ReturnItem `json:"items"`
}

type ReturnListResponse struct {
	Items      []ReturnResponse `json:"items"`
	TotalCount int              `json:"totalCount"`
}

type RefundListResponse struct {
	Items      []Refund `json:"items"`
	TotalCount int      `json:"totalCount"`
}

type SellerReturnItem struct {
	ReturnItemID uuid.UUID `json:"returnItemId"`
	ReturnID     uuid.UUID `json:"returnId"`
	OrderID      uuid.UUID `json:"orderId"`
	OrderItemID  uuid.UUID `json:"orderItemId"`
	Status       string    `json:"status"` // return status
	Quantity     int       `json:"quantity"`
	Reason       *string   `json:"reason"`
	Condition    *string   `json:"condition"`
}

type SellerReturnListResponse struct {
	Items      []SellerReturnItem `json:"items"`
	TotalCount int                `json:"totalCount"`
}
