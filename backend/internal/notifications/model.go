package notifications

import (
	"time"

	"github.com/google/uuid"
)

const (
	RecipientKindCustomer = "customer"
	RecipientKindSeller   = "seller"
	RecipientKindStaff    = "staff"

	// Customer types
	TypeCustomerOrderPaid             = "order_paid"
	TypeCustomerFulfillmentAssembling = "fulfillment_assembling"
	TypeCustomerFulfillmentPacked     = "fulfillment_packed"
	TypeCustomerShipmentCreated       = "shipment_created"
	TypeCustomerShipmentShipped       = "shipment_shipped"
	TypeCustomerShipmentDelivered     = "shipment_delivered"

	// Seller types
	TypeSellerFulfillmentPaid   = "fulfillment_paid"
	TypeSellerShipmentCreated   = "shipment_created"
	TypeSellerShipmentShipped   = "shipment_shipped"
	TypeSellerShipmentDelivered = "shipment_delivered"

	// Admin/Staff types
	TypeStaffFulfillmentPacked = "fulfillment_packed"
	TypeStaffShipmentProblem   = "shipment_problem"
)

type Notification struct {
	ID                uuid.UUID              `json:"id"`
	RecipientUserID   *uuid.UUID             `json:"recipientUserId,omitempty"`
	RecipientSellerID *uuid.UUID             `json:"recipientSellerId,omitempty"`
	RecipientKind     string                 `json:"recipientKind"`
	Type              string                 `json:"type"`
	Title             string                 `json:"title"`
	Body              string                 `json:"body"`
	EntityType        string                 `json:"entityType"`
	EntityID          uuid.UUID              `json:"entityId"`
	Metadata          map[string]interface{} `json:"metadata"`
	ReadAt            *time.Time             `json:"readAt,omitempty"`
	CreatedAt         time.Time              `json:"createdAt"`
}

type PaginatedNotifications struct {
	Items      []Notification `json:"items"`
	TotalCount int            `json:"totalCount"`
}

type UnreadCountResponse struct {
	UnreadCount int `json:"unreadCount"`
}
