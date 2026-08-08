package orders

import (
	"time"

	"github.com/google/uuid"
)

type CreateOrderRequest struct {
	CustomerName    string `json:"customerName" validate:"required"`
	CustomerPhone   string `json:"customerPhone" validate:"required"`
	CustomerEmail    string    `json:"customerEmail" validate:"required,email"`
	DeliveryAddress  string    `json:"deliveryAddress" validate:"required"`
	DeliveryMethodID uuid.UUID `json:"deliveryMethodId" validate:"required"`
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
	ID                 uuid.UUID    `json:"id"`
	OrderNumber        *string      `json:"orderNumber,omitempty"`
	CreatedAt          time.Time    `json:"createdAt"`
	CommercialStatus   string       `json:"commercialStatus"`
	DeliveryStatus     string       `json:"deliveryStatus"`
	PayoutStatus       *string      `json:"payoutStatus,omitempty"`
	SellerItemCount    int          `json:"sellerItemCount"`
	SellerUnits        int          `json:"sellerUnits"`
	SellerGrossAmount  int64        `json:"sellerGrossAmount"`
	SellerRefundAmount int64        `json:"sellerRefundAmount"`
	SellerNetAmount    int64        `json:"sellerNetAmount"`
	Items              []OrderItem  `json:"items"`
}

type SellerOrderSummary struct {
	TodayUnits     int   `json:"todayUnits"`
	TodayOrders    int   `json:"todayOrders"`
	Last7dGross    int64 `json:"last7dGross"`
	Last30dGross   int64 `json:"last30dGross"`
	ReturnsCount   int   `json:"returnsCount"`
	ReturnsAmount  int64 `json:"returnsAmount"`
}

type AdminOrderListResponse struct {
	Items      []AdminOrder `json:"items"`
	TotalCount int          `json:"totalCount"`
}

type AdminOrder struct {
	ID                 uuid.UUID  `json:"id"`
	UserID             uuid.UUID  `json:"userId"`
	Status             string     `json:"status"` // payment/overall status
	FulfillmentStatus  string     `json:"fulfillmentStatus"`
	FulfillmentsCount  int        `json:"fulfillmentsCount"`
	ItemPositionsCount int        `json:"itemPositionsCount"`
	UnitsCount         int        `json:"unitsCount"`
	SourceType         string     `json:"sourceType"` // 'auction', 'direct_sale', 'normal'
	OrderNumber        *string    `json:"orderNumber,omitempty"`
	TotalPriceCents    int64      `json:"totalPriceCents"`
	Currency           string     `json:"currency"`
	CustomerName       string     `json:"customerName"`
	CustomerEmail      string     `json:"customerEmail"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	CancelledAt        *time.Time `json:"cancelledAt,omitempty"`
}

type AdminOrderDetail struct {
	AdminOrder
	CustomerPhone              string               `json:"customerPhone"`
	DeliveryAddress            string               `json:"deliveryAddress"`
	DeliveryMethodID           *uuid.UUID           `json:"deliveryMethodId"`
	DeliveryMethodCode         *string              `json:"deliveryMethodCode"`
	DeliveryMethodName         *string              `json:"deliveryMethodName"`
	DeliveryPriceCents         *int64               `json:"deliveryPriceCents"`
	DeliveryEstimatedDaysMin   *int                 `json:"deliveryEstimatedDaysMin"`
	DeliveryEstimatedDaysMax   *int                 `json:"deliveryEstimatedDaysMax"`
	Items                      []OrderItem          `json:"items"`
	Fulfillments               []OrderFulfillment   `json:"fulfillments"`
}
