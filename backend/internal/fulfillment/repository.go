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

func (r *Repository) ListAdminShipments(ctx context.Context, limit, offset int) ([]AdminShipmentListItem, error) {
	query := `
		SELECT
			s.id,
			s.order_id,
			o.order_number,
			s.fulfillment_id,
			of.status AS fulfillment_status,
			of.seller_id,
			sel.brand_name AS seller_name,
			s.status,
			s.carrier,
			s.tracking_number,
			s.tracking_url,
			o.delivery_method_name,
			COALESCE(item_counts.items_count, 0) AS items_count,
			COALESCE(item_counts.units_count, 0) AS units_count,
			of.packed_at,
			s.shipped_at,
			s.delivered_at,
			s.created_at,
			s.updated_at
		FROM shipments s
		JOIN orders o ON o.id = s.order_id
		LEFT JOIN order_fulfillments of ON of.id = s.fulfillment_id
		LEFT JOIN sellers sel ON sel.id = of.seller_id
		LEFT JOIN LATERAL (
			SELECT
				COUNT(oi.id)::int AS items_count,
				COALESCE(SUM(oi.quantity), 0)::int AS units_count
			FROM order_items oi
			WHERE (s.fulfillment_id IS NOT NULL AND oi.order_fulfillment_id = s.fulfillment_id)
			   OR (s.fulfillment_id IS NULL AND oi.order_id = s.order_id)
		) item_counts ON TRUE
		ORDER BY s.created_at DESC, s.id DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AdminShipmentListItem
	for rows.Next() {
		var s AdminShipmentListItem
		if err := rows.Scan(
			&s.ID,
			&s.OrderID,
			&s.OrderNumber,
			&s.FulfillmentID,
			&s.FulfillmentStatus,
			&s.SellerID,
			&s.SellerName,
			&s.Status,
			&s.Carrier,
			&s.TrackingNumber,
			&s.TrackingUrl,
			&s.DeliveryMethodName,
			&s.ItemsCount,
			&s.UnitsCount,
			&s.PackedAt,
			&s.ShippedAt,
			&s.DeliveredAt,
			&s.CreatedAt,
			&s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		s.ShipmentID = s.ID
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if list == nil {
		list = make([]AdminShipmentListItem, 0)
	}
	return list, nil
}

func (r *Repository) GetAdminShipmentDetail(ctx context.Context, id uuid.UUID) (*AdminShipmentDetail, error) {
	queryHeader := `
		SELECT
			s.id,
			s.order_id,
			o.order_number,
			s.fulfillment_id,
			of.status AS fulfillment_status,
			of.seller_id,
			sel.brand_name AS seller_name,
			s.status,
			s.carrier,
			s.tracking_number,
			s.tracking_url,
			o.delivery_method_name,
			o.customer_name,
			o.customer_phone,
			o.delivery_address,
			of.packed_at,
			s.shipped_at,
			s.delivered_at,
			s.created_at,
			s.updated_at
		FROM shipments s
		JOIN orders o ON o.id = s.order_id
		LEFT JOIN order_fulfillments of ON of.id = s.fulfillment_id
		LEFT JOIN sellers sel ON sel.id = of.seller_id
		WHERE s.id = $1
	`
	var d AdminShipmentDetail
	err := r.db.QueryRow(ctx, queryHeader, id).Scan(
		&d.ID,
		&d.OrderID,
		&d.OrderNumber,
		&d.FulfillmentID,
		&d.FulfillmentStatus,
		&d.SellerID,
		&d.SellerName,
		&d.Status,
		&d.Carrier,
		&d.TrackingNumber,
		&d.TrackingUrl,
		&d.DeliveryMethodName,
		&d.CustomerName,
		&d.CustomerPhone,
		&d.DeliveryAddress,
		&d.PackedAt,
		&d.ShippedAt,
		&d.DeliveredAt,
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, err
	}
	d.ShipmentID = d.ID

	queryItems := `
		SELECT
			oi.id,
			oi.product_id,
			oi.product_variant_id,
			oi.title,
			NULLIF(COALESCE(NULLIF(oi.image_url, ''), img.image_url, NULLIF(p.main_image_url, '')), '') AS image_url,
			NULLIF(COALESCE(NULLIF(oi.variant_color, ''), c.name_ru, NULLIF(pv.color, '')), '') AS variant_color,
			NULLIF(COALESCE(NULLIF(oi.variant_size, ''), sv.value, NULLIF(pv.size, '')), '') AS variant_size,
			NULLIF(COALESCE(NULLIF(oi.sku, ''), NULLIF(pv.sku, ''), NULLIF(pv.seller_sku, '')), '') AS sku,
			oi.quantity
		FROM order_items oi
		LEFT JOIN products p ON p.id = oi.product_id
		LEFT JOIN product_variants pv ON pv.id = oi.product_variant_id
		LEFT JOIN colors c ON c.id = pv.color_id
		LEFT JOIN size_values sv ON sv.id = pv.size_value_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(pi.rendition_url, pi.image_url) AS image_url
			FROM product_images pi
			WHERE pi.product_id = oi.product_id
			  AND (
				(pv.color_id IS NOT NULL AND pi.color_id = pv.color_id)
				OR pi.color_id IS NULL
			  )
			ORDER BY
			  CASE
				WHEN pv.color_id IS NOT NULL AND pi.color_id = pv.color_id THEN 1
				WHEN pi.color_id IS NULL THEN 2
				ELSE 3
			  END ASC,
			  pi.sort_order ASC,
			  pi.created_at ASC
			LIMIT 1
		) img ON TRUE
		WHERE ($1::uuid IS NOT NULL AND oi.order_fulfillment_id = $1)
		   OR ($1::uuid IS NULL AND oi.order_id = $2)
		ORDER BY oi.created_at ASC, oi.id ASC
	`
	rows, err := r.db.Query(ctx, queryItems, d.FulfillmentID, d.OrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []AdminShipmentItem
	unitsCount := 0
	for rows.Next() {
		var it AdminShipmentItem
		var variantID uuid.UUID
		if err := rows.Scan(
			&it.OrderItemID,
			&it.ProductID,
			&variantID,
			&it.ProductTitle,
			&it.ImageURL,
			&it.VariantColor,
			&it.VariantSize,
			&it.SKU,
			&it.Quantity,
		); err != nil {
			return nil, err
		}
		if variantID != uuid.Nil {
			v := variantID
			it.VariantID = &v
		}
		unitsCount += it.Quantity
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		items = make([]AdminShipmentItem, 0)
	}

	d.Items = items
	d.ItemsCount = len(items)
	d.UnitsCount = unitsCount
	return &d, nil
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
