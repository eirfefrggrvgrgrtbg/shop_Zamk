package fulfillment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DeliveryResult struct {
	ShipmentID        uuid.UUID `json:"shipmentId"`
	FulfillmentID     uuid.UUID `json:"fulfillmentId"`
	OrderID           uuid.UUID `json:"orderId"`
	ShipmentStatus    string    `json:"shipmentStatus"`
	FulfillmentStatus string    `json:"fulfillmentStatus"`
	OrderStatus       string    `json:"orderStatus"`
	DeliveredAt       time.Time `json:"deliveredAt"`
}

func (r *Repository) DeliverShipmentTx(ctx context.Context, tx pgx.Tx, adminID, shipmentID uuid.UUID, comment *string) (*DeliveryResult, error) {
	// 1. Resolve preliminary linkage from shipment without row lock
	var preOrderID uuid.UUID
	var preFulfillmentID *uuid.UUID

	queryPre := `
		SELECT order_id, fulfillment_id
		FROM shipments
		WHERE id = $1
	`
	err := tx.QueryRow(ctx, queryPre, shipmentID).Scan(&preOrderID, &preFulfillmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, fmt.Errorf("failed to lookup shipment linkage: %w", err)
	}

	if preFulfillmentID == nil {
		return nil, ErrShipmentNotLinkedToFulfillment
	}

	// 2. Lock parent order and linked fulfillment (matching canonical lifecycle lock order: orders/fulfillments -> shipments)
	var orderStatus string
	var fulfillmentStatus string
	var realOrderID uuid.UUID

	queryHeader := `
		SELECT o.id, o.status, of.status
		FROM order_fulfillments of
		JOIN orders o ON o.id = of.order_id
		WHERE of.id = $1 AND of.order_id = $2
		FOR UPDATE OF of, o
	`
	err = tx.QueryRow(ctx, queryHeader, *preFulfillmentID, preOrderID).Scan(&realOrderID, &orderStatus, &fulfillmentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to lock fulfillment and order for delivery: %w", err)
	}

	// 3. Lock shipment row
	var lockedOrderID uuid.UUID
	var lockedFulfillmentID *uuid.UUID
	var shipmentStatus string
	var shippedAt *time.Time
	var deliveredAt *time.Time

	queryShipment := `
		SELECT order_id, fulfillment_id, status, shipped_at, delivered_at
		FROM shipments
		WHERE id = $1
		FOR UPDATE
	`
	err = tx.QueryRow(ctx, queryShipment, shipmentID).Scan(&lockedOrderID, &lockedFulfillmentID, &shipmentStatus, &shippedAt, &deliveredAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, fmt.Errorf("failed to lock shipment for delivery: %w", err)
	}

	// 4. RE-VALIDATE all linkages and statuses under active locks
	if lockedFulfillmentID == nil || *lockedFulfillmentID != *preFulfillmentID || lockedOrderID != preOrderID {
		return nil, ErrShipmentContradictoryState
	}

	if shipmentStatus == "delivered" {
		return nil, ErrShipmentAlreadyDelivered
	}

	if shipmentStatus != "shipped" {
		return nil, ErrDeliveryNotAllowed
	}

	if fulfillmentStatus == "delivered" {
		return nil, ErrShipmentAlreadyDelivered
	}

	if fulfillmentStatus != "shipped" {
		return nil, ErrFulfillmentNotShipped
	}

	if orderStatus == "cancelled" {
		return nil, ErrOrderCancelled
	}
	if orderStatus == "delivered" {
		return nil, ErrShipmentContradictoryState
	}
	if orderStatus != "assembling" && orderStatus != "packed" && orderStatus != "shipped" {
		return nil, ErrDeliveryNotAllowed
	}

	now := time.Now().UTC()

	// 5. Update shipment status and delivered_at (preserving shipped_at, carrier, tracking)
	updateShipment := `
		UPDATE shipments
		SET status = 'delivered',
		    delivered_at = $1,
		    updated_at = $1
		WHERE id = $2
	`
	if _, err := tx.Exec(ctx, updateShipment, now, shipmentID); err != nil {
		return nil, fmt.Errorf("failed to update shipment to delivered: %w", err)
	}

	// 6. Update fulfillment status
	updateFulfillment := `
		UPDATE order_fulfillments
		SET status = 'delivered',
		    updated_at = $1
		WHERE id = $2
	`
	if _, err := tx.Exec(ctx, updateFulfillment, now, *lockedFulfillmentID); err != nil {
		return nil, fmt.Errorf("failed to update fulfillment to delivered: %w", err)
	}

	// 7. Insert shipment_event
	eventComment := comment
	if eventComment == nil {
		defaultComment := "Доставлено получателю"
		eventComment = &defaultComment
	}

	insertEvent := `
		INSERT INTO shipment_events (id, shipment_id, from_status, to_status, actor_user_id, comment, created_at)
		VALUES ($1, $2, 'shipped', 'delivered', $3, $4, $5)
	`
	if _, err := tx.Exec(ctx, insertEvent, uuid.New(), shipmentID, &adminID, eventComment, now); err != nil {
		return nil, fmt.Errorf("failed to insert delivered shipment event: %w", err)
	}

	return &DeliveryResult{
		ShipmentID:        shipmentID,
		FulfillmentID:     *lockedFulfillmentID,
		OrderID:           lockedOrderID,
		ShipmentStatus:    "delivered",
		FulfillmentStatus: "delivered",
		DeliveredAt:       now,
	}, nil
}
