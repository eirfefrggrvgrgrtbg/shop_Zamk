package notifications

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrMalformedAlert = errors.New("malformed seller alert")
)

const (
	RecipientKindCustomer = "customer"
	RecipientKindSeller   = "seller"
	RecipientKindStaff    = "staff"

	KindEvent = "event"
	KindAlert = "alert"

	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"

	StatusActive   = "active"
	StatusResolved = "resolved"

	// Customer types
	TypeCustomerOrderPaid         = "order_paid"
	TypeCustomerFulfillmentPacked = "fulfillment_packed"
	TypeCustomerShipmentCreated   = "shipment_created"
	TypeCustomerShipmentShipped   = "shipment_shipped"
	TypeCustomerShipmentDelivered = "shipment_delivered"

	// Seller types
	TypeSellerFulfillmentPaid   = "fulfillment_paid"
	TypeSellerShipmentCreated   = "shipment_created"
	TypeSellerShipmentShipped   = "shipment_shipped"
	TypeSellerShipmentDelivered = "shipment_delivered"

	// Admin/Staff types
	TypeStaffFulfillmentPacked     = "fulfillment_packed"
	TypeStaffShipmentProblem       = "shipment_problem"
	TypeSellerOnboardingCompleted  = "seller_onboarding_completed"
	TypeProductModerationSubmitted = "product_moderation_submitted"
	TypePayoutRequested            = "payout_requested"

	// Moderation types (to seller)
	TypeProductApproved = "product_approved"
	TypeProductRejected = "product_rejected"

	// Payout types (to seller)
	TypePayoutApproved = "payout_approved"
	TypePayoutRejected = "payout_rejected"
	TypePayoutPaid     = "payout_paid"

	// Return/Refund types
	TypeReturnCreated   = "return_created"
	TypeReturnNeedsInfo = "return_needs_info"
	TypeReturnApproved  = "return_approved"
	TypeReturnRejected  = "return_rejected"
	TypeRefundCreated   = "refund_created"

	// Stock alert types (to seller)
	TypeStockCriticalHidden = "stock_critical_hidden"
	TypeStockForecastRisk   = "stock_forecast_risk"
)

type Notification struct {
	ID                uuid.UUID              `json:"id"`
	RecipientUserID   *uuid.UUID             `json:"recipientUserId,omitempty"`
	RecipientSellerID *uuid.UUID             `json:"recipientSellerId,omitempty"`
	RecipientKind     string                 `json:"recipientKind"`
	Kind              string                 `json:"kind"`
	Severity          string                 `json:"severity"`
	Status            *string                `json:"status,omitempty"`
	Type              string                 `json:"type"`
	Title             string                 `json:"title"`
	Body              string                 `json:"body"`
	EntityType        string                 `json:"entityType"`
	EntityID          uuid.UUID              `json:"entityId"`
	DedupeKey         *string                `json:"dedupeKey,omitempty"`
	ActionURL         *string                `json:"actionUrl,omitempty"`
	Metadata          map[string]interface{} `json:"metadata"`
	ReadAt            *time.Time             `json:"readAt,omitempty"`
	ResolvedAt        *time.Time             `json:"resolvedAt,omitempty"`
	CreatedAt         time.Time              `json:"createdAt"`
}

type PaginatedNotifications struct {
	Items      []Notification `json:"items"`
	TotalCount int            `json:"totalCount"`
}

type UnreadCountResponse struct {
	UnreadCount int `json:"unreadCount"`
}
