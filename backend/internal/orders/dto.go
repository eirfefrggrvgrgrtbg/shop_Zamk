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
	ID             uuid.UUID   `json:"id"`
	Status         string      `json:"status"`
	CreatedAt      time.Time   `json:"createdAt"`
	ShipmentStatus *string     `json:"shipmentStatus,omitempty"`
	Items          []OrderItem `json:"items"`
}

type AdminOrderListResponse struct {
	Items      []AdminOrder `json:"items"`
	TotalCount int          `json:"totalCount"`
}

type AdminOrder struct {
	ID                uuid.UUID  `json:"id"`
	UserID            uuid.UUID  `json:"userId"`
	Status            string     `json:"status"` // payment/overall status
	FulfillmentStatus string     `json:"fulfillmentStatus"`
	SourceType        string     `json:"sourceType"` // 'auction', 'direct_sale', 'normal'
	TotalPriceCents   int64      `json:"totalPriceCents"`
	Currency          string     `json:"currency"`
	CustomerName      string     `json:"customerName"`
	CustomerEmail     string     `json:"customerEmail"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	CancelledAt       *time.Time `json:"cancelledAt,omitempty"`
}

type AdminOrderDetail struct {
	AdminOrder
	CustomerPhone   string             `json:"customerPhone"`
	DeliveryAddress string             `json:"deliveryAddress"`
	Items           []OrderItem        `json:"items"`
	Fulfillments    []OrderFulfillment `json:"fulfillments"`
}
