package fulfillment

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateShipmentTx(ctx context.Context, tx pgx.Tx, s *Shipment) error {
	query := `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`
	err := tx.QueryRow(ctx, query, s.ID, s.OrderID, s.FulfillmentID, s.Status, s.Carrier, s.TrackingNumber, s.TrackingUrl).Scan(&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrShipmentExists
		}
		return err
	}
	return nil
}

func (r *Repository) CreateShipmentEventTx(ctx context.Context, tx pgx.Tx, e *ShipmentEvent) error {
	query := `
		INSERT INTO shipment_events (id, shipment_id, from_status, to_status, actor_user_id, comment)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`
	return tx.QueryRow(ctx, query, e.ID, e.ShipmentID, e.FromStatus, e.ToStatus, e.ActorUserID, e.Comment).Scan(&e.CreatedAt)
}

func (r *Repository) UpdateShipmentTx(ctx context.Context, tx pgx.Tx, s *Shipment) error {
	query := `
		UPDATE shipments
		SET status = $1, carrier = $2, tracking_number = $3, tracking_url = $4, shipped_at = $5, delivered_at = $6, updated_at = now()
		WHERE id = $7
		RETURNING updated_at
	`
	return tx.QueryRow(ctx, query, s.Status, s.Carrier, s.TrackingNumber, s.TrackingUrl, s.ShippedAt, s.DeliveredAt, s.ID).Scan(&s.UpdatedAt)
}

func (r *Repository) GetShipment(ctx context.Context, id uuid.UUID) (*Shipment, error) {
	query := `
		SELECT id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, delivered_at, created_at, updated_at
		FROM shipments WHERE id = $1
	`
	var s Shipment
	err := r.db.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.OrderID, &s.FulfillmentID, &s.Status, &s.Carrier, &s.TrackingNumber, &s.TrackingUrl, &s.ShippedAt, &s.DeliveredAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *Repository) GetShipmentByOrderID(ctx context.Context, orderID uuid.UUID) (*Shipment, error) {
	query := `
		SELECT id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, delivered_at, created_at, updated_at
		FROM shipments WHERE order_id = $1 LIMIT 1
	`
	var s Shipment
	err := r.db.QueryRow(ctx, query, orderID).Scan(
		&s.ID, &s.OrderID, &s.FulfillmentID, &s.Status, &s.Carrier, &s.TrackingNumber, &s.TrackingUrl, &s.ShippedAt, &s.DeliveredAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *Repository) ListShipments(ctx context.Context, limit, offset int) ([]Shipment, error) {
	query := `
		SELECT id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, delivered_at, created_at, updated_at
		FROM shipments ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Shipment
	for rows.Next() {
		var s Shipment
		if err := rows.Scan(&s.ID, &s.OrderID, &s.FulfillmentID, &s.Status, &s.Carrier, &s.TrackingNumber, &s.TrackingUrl, &s.ShippedAt, &s.DeliveredAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	if list == nil {
		list = make([]Shipment, 0)
	}
	return list, nil
}

type LockedShipmentContext struct {
	Shipment          *Shipment
	OrderStatus       string
	FulfillmentStatus string
}

func (r *Repository) LockShipmentForUpdateTx(ctx context.Context, tx pgx.Tx, shipmentID uuid.UUID) (*LockedShipmentContext, error) {
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

	var orderStatus string
	var fulfillmentStatus string

	// 2. Lock parent order FIRST (authoritative serialization point, prevents deadlocks with cancellation)
	queryOrder := `
		SELECT status
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`
	err = tx.QueryRow(ctx, queryOrder, preOrderID).Scan(&orderStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("order not found")
		}
		return nil, fmt.Errorf("failed to lock parent order: %w", err)
	}

	// 3. Lock linked fulfillment SECOND (if present)
	if preFulfillmentID != nil {
		queryFulfillment := `
			SELECT status
			FROM order_fulfillments
			WHERE id = $1 AND order_id = $2
			FOR UPDATE
		`
		err = tx.QueryRow(ctx, queryFulfillment, *preFulfillmentID, preOrderID).Scan(&fulfillmentStatus)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrFulfillmentNotFound
			}
			return nil, fmt.Errorf("failed to lock fulfillment: %w", err)
		}
	}

	// 3. Lock shipment row
	queryShipment := `
		SELECT id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, delivered_at, created_at, updated_at
		FROM shipments
		WHERE id = $1
		FOR UPDATE
	`
	var s Shipment
	err = tx.QueryRow(ctx, queryShipment, shipmentID).Scan(
		&s.ID, &s.OrderID, &s.FulfillmentID, &s.Status, &s.Carrier, &s.TrackingNumber, &s.TrackingUrl, &s.ShippedAt, &s.DeliveredAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, fmt.Errorf("failed to lock shipment: %w", err)
	}

	// 4. Re-validate linkages under active locks
	if s.OrderID != preOrderID {
		return nil, ErrShipmentContradictoryState
	}
	if preFulfillmentID != nil {
		if s.FulfillmentID == nil || *s.FulfillmentID != *preFulfillmentID {
			return nil, ErrShipmentContradictoryState
		}
	} else if s.FulfillmentID != nil {
		return nil, ErrShipmentContradictoryState
	}

	return &LockedShipmentContext{
		Shipment:          &s,
		OrderStatus:       orderStatus,
		FulfillmentStatus: fulfillmentStatus,
	}, nil
}
