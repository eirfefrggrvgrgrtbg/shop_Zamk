package orders

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	UserID          uuid.UUID  `json:"userId" db:"user_id"`
	Status          string     `json:"status" db:"status"`
	TotalPriceCents int64      `json:"totalPriceCents" db:"total_price_cents"`
	Currency        string     `json:"currency" db:"currency"`
	CustomerName    string     `json:"customerName" db:"customer_name"`
	CustomerPhone   string     `json:"customerPhone" db:"customer_phone"`
	CustomerEmail   string     `json:"customerEmail" db:"customer_email"`
	DeliveryAddress string     `json:"deliveryAddress" db:"delivery_address"`
	CreatedAt       time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time  `json:"updatedAt" db:"updated_at"`
	CancelledAt     *time.Time `json:"cancelledAt" db:"cancelled_at"`

	Items []OrderItem `json:"items" db:"-"`
}

type OrderItem struct {
	ID                 uuid.UUID  `json:"id" db:"id"`
	OrderID            uuid.UUID  `json:"orderId" db:"order_id"`
	OrderFulfillmentID *uuid.UUID `json:"orderFulfillmentId" db:"order_fulfillment_id"`
	ProductID          uuid.UUID  `json:"productId" db:"product_id"`
	ProductVariantID   uuid.UUID  `json:"productVariantId" db:"product_variant_id"`
	SellerID           uuid.UUID  `json:"sellerId" db:"seller_id"`
	Title              string     `json:"title" db:"title"`
	ProductSlug        string     `json:"productSlug" db:"product_slug"`
	VariantSize        *string    `json:"variantSize" db:"variant_size"`
	VariantColor       *string    `json:"variantColor" db:"variant_color"`
	Sku                *string    `json:"sku" db:"sku"`
	ImageURL           *string    `json:"imageUrl" db:"image_url"`
	PriceCents         int64      `json:"priceCents" db:"price_cents"`
	Quantity           int        `json:"quantity" db:"quantity"`
	SubtotalPriceCents int64      `json:"subtotalPriceCents" db:"subtotal_price_cents"`
	CreatedAt          time.Time  `json:"createdAt" db:"created_at"`
}

type OrderFulfillment struct {
	ID                uuid.UUID `json:"id" db:"id"`
	OrderID           uuid.UUID `json:"orderId" db:"order_id"`
	SellerID          uuid.UUID `json:"sellerId" db:"seller_id"`
	Status            string    `json:"status" db:"status"`
	SubtotalCents     int64     `json:"subtotalCents" db:"subtotal_cents"`
	CommissionBps     int       `json:"commissionBps" db:"commission_bps"`
	SellerAmountCents int64     `json:"sellerAmountCents" db:"seller_amount_cents"`
	CreatedAt         time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time `json:"updatedAt" db:"updated_at"`
}

type OrderReservation struct {
	ID            uuid.UUID `json:"id" db:"id"`
	OrderID       uuid.UUID `json:"orderId" db:"order_id"`
	ReservationID uuid.UUID `json:"reservationId" db:"reservation_id"`
	CreatedAt     time.Time `json:"createdAt" db:"created_at"`
}

type OrderStatusHistory struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	OrderID     uuid.UUID  `json:"orderId" db:"order_id"`
	FromStatus  *string    `json:"fromStatus" db:"from_status"`
	ToStatus    string     `json:"toStatus" db:"to_status"`
	ActorUserID *uuid.UUID `json:"actorUserId" db:"actor_user_id"`
	Comment     *string    `json:"comment" db:"comment"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
}
