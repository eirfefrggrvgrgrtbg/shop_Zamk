package orders

import (
	"time"

	"github.com/google/uuid"
)

type CreateOrderRequest struct {
	CustomerName    string `json:"customerName" validate:"required"`
	CustomerPhone   string `json:"customerPhone" validate:"required"`
	CustomerEmail   string `json:"customerEmail" validate:"required,email"`
	DeliveryAddress string `json:"deliveryAddress" validate:"required"`
}

type UpdateOrderStatusRequest struct {
	Status  string  `json:"status" validate:"required"`
	Comment *string `json:"comment,omitempty"`
}

type OrderListResponse struct {
	Items      []Order `json:"items"`
	TotalCount int     `json:"totalCount"`
}

type SellerOrderListResponse struct {
	Items      []SellerOrder `json:"items"`
	TotalCount int           `json:"totalCount"`
}

type SellerOrder struct {
	ID              uuid.UUID     `json:"id"`
	Status          string        `json:"status"`
	CreatedAt       time.Time     `json:"createdAt"`
	DeliveryAddress string        `json:"deliveryAddress"`
	CustomerName    string        `json:"customerName"`
	CustomerPhone   string        `json:"customerPhone"`
	ShipmentStatus  *string       `json:"shipmentStatus,omitempty"`
	Items           []OrderItem   `json:"items"`
}
