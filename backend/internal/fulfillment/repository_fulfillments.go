package fulfillment

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListSellerFulfillments(ctx context.Context, sellerID uuid.UUID, limit, offset int, status *string) ([]Fulfillment, error) {
	query := `
		SELECT 
			f.id, f.order_id, f.seller_id, f.status, f.subtotal_cents, f.commission_bps, f.seller_amount_cents, f.created_at, f.updated_at,
			s.status as shipment_status, s.id as shipment_id,
			o.delivery_address, o.customer_name, o.customer_phone
		FROM order_fulfillments f
		JOIN orders o ON o.id = f.order_id
		LEFT JOIN shipments s ON (s.fulfillment_id = f.id) OR (s.fulfillment_id IS NULL AND s.order_id = f.order_id AND (SELECT COUNT(*) FROM order_fulfillments WHERE order_id = f.order_id) = 1)
		WHERE f.seller_id = $1
	`
	var args []interface{}
	args = append(args, sellerID)
	
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND f.status = $%d", len(args)+1)
		args = append(args, *status)
	}

	query += fmt.Sprintf(" ORDER BY f.created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Fulfillment
	for rows.Next() {
		var f Fulfillment
		if err := rows.Scan(
			&f.ID, &f.OrderID, &f.SellerID, &f.Status, &f.SubtotalCents, &f.CommissionBps, &f.SellerAmountCents, &f.CreatedAt, &f.UpdatedAt,
			&f.ShipmentStatus, &f.ShipmentID,
			&f.DeliveryAddress, &f.CustomerName, &f.CustomerPhone,
		); err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	if list == nil {
		list = make([]Fulfillment, 0)
	}

	for i := range list {
		items, err := r.GetFulfillmentItems(ctx, list[i].ID)
		if err != nil {
			return nil, err
		}
		list[i].Items = items
	}

	return list, nil
}

func (r *Repository) GetFulfillmentItems(ctx context.Context, fulfillmentID uuid.UUID) ([]FulfillmentItem, error) {
	query := `
		SELECT 
			id, product_id, title, product_variant_id, sku, quantity, price_cents, subtotal_price_cents, image_url
		FROM order_items
		WHERE order_fulfillment_id = $1
	`
	rows, err := r.db.Query(ctx, query, fulfillmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []FulfillmentItem
	for rows.Next() {
		var item FulfillmentItem
		if err := rows.Scan(&item.OrderItemID, &item.ProductID, &item.ProductTitle, &item.VariantID, &item.SKU, &item.Quantity, &item.UnitPriceCents, &item.LineTotalCents, &item.ImageURL); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = make([]FulfillmentItem, 0)
	}
	return items, nil
}

func (r *Repository) GetSellerFulfillment(ctx context.Context, sellerID, fulfillmentID uuid.UUID) (*Fulfillment, error) {
	query := `
		SELECT 
			f.id, f.order_id, f.seller_id, f.status, f.subtotal_cents, f.commission_bps, f.seller_amount_cents, f.created_at, f.updated_at,
			s.status as shipment_status, s.id as shipment_id,
			o.delivery_address, o.customer_name, o.customer_phone
		FROM order_fulfillments f
		JOIN orders o ON o.id = f.order_id
		LEFT JOIN shipments s ON (s.fulfillment_id = f.id) OR (s.fulfillment_id IS NULL AND s.order_id = f.order_id AND (SELECT COUNT(*) FROM order_fulfillments WHERE order_id = f.order_id) = 1)
		WHERE f.seller_id = $1 AND f.id = $2
	`
	var f Fulfillment
	err := r.db.QueryRow(ctx, query, sellerID, fulfillmentID).Scan(
		&f.ID, &f.OrderID, &f.SellerID, &f.Status, &f.SubtotalCents, &f.CommissionBps, &f.SellerAmountCents, &f.CreatedAt, &f.UpdatedAt,
		&f.ShipmentStatus, &f.ShipmentID,
		&f.DeliveryAddress, &f.CustomerName, &f.CustomerPhone,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, err
	}

	f.Items, err = r.GetFulfillmentItems(ctx, f.ID)
	if err != nil {
		return nil, err
	}

	return &f, nil
}

func (r *Repository) ListAdminFulfillments(ctx context.Context, limit, offset int, status *string) ([]Fulfillment, error) {
	query := `
		SELECT 
			f.id, f.order_id, f.seller_id, f.status, f.subtotal_cents, f.commission_bps, f.seller_amount_cents, f.created_at, f.updated_at,
			s.status as shipment_status, s.id as shipment_id,
			o.delivery_address, o.customer_name, o.customer_phone,
			sel.brand_name as seller_name
		FROM order_fulfillments f
		JOIN orders o ON o.id = f.order_id
		JOIN sellers sel ON sel.id = f.seller_id
		LEFT JOIN shipments s ON (s.fulfillment_id = f.id) OR (s.fulfillment_id IS NULL AND s.order_id = f.order_id AND (SELECT COUNT(*) FROM order_fulfillments WHERE order_id = f.order_id) = 1)
	`
	var args []interface{}
	
	if status != nil && *status != "" {
		query += fmt.Sprintf(" WHERE f.status = $%d", len(args)+1)
		args = append(args, *status)
	}

	query += fmt.Sprintf(" ORDER BY f.created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Fulfillment
	for rows.Next() {
		var f Fulfillment
		if err := rows.Scan(
			&f.ID, &f.OrderID, &f.SellerID, &f.Status, &f.SubtotalCents, &f.CommissionBps, &f.SellerAmountCents, &f.CreatedAt, &f.UpdatedAt,
			&f.ShipmentStatus, &f.ShipmentID,
			&f.DeliveryAddress, &f.CustomerName, &f.CustomerPhone,
			&f.SellerName,
		); err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	if list == nil {
		list = make([]Fulfillment, 0)
	}

	for i := range list {
		items, err := r.GetFulfillmentItems(ctx, list[i].ID)
		if err != nil {
			return nil, err
		}
		list[i].Items = items
	}

	return list, nil
}

func (r *Repository) GetAdminFulfillment(ctx context.Context, id uuid.UUID) (*Fulfillment, error) {
	query := `
		SELECT 
			f.id, f.order_id, f.seller_id, f.status, f.subtotal_cents, f.commission_bps, f.seller_amount_cents, f.created_at, f.updated_at,
			s.status as shipment_status, s.id as shipment_id,
			o.delivery_address, o.customer_name, o.customer_phone,
			sel.brand_name as seller_name
		FROM order_fulfillments f
		JOIN orders o ON o.id = f.order_id
		JOIN sellers sel ON sel.id = f.seller_id
		LEFT JOIN shipments s ON (s.fulfillment_id = f.id) OR (s.fulfillment_id IS NULL AND s.order_id = f.order_id AND (SELECT COUNT(*) FROM order_fulfillments WHERE order_id = f.order_id) = 1)
		WHERE f.id = $1
	`
	var f Fulfillment
	err := r.db.QueryRow(ctx, query, id).Scan(
		&f.ID, &f.OrderID, &f.SellerID, &f.Status, &f.SubtotalCents, &f.CommissionBps, &f.SellerAmountCents, &f.CreatedAt, &f.UpdatedAt,
		&f.ShipmentStatus, &f.ShipmentID,
		&f.DeliveryAddress, &f.CustomerName, &f.CustomerPhone,
		&f.SellerName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, err
	}

	f.Items, err = r.GetFulfillmentItems(ctx, f.ID)
	if err != nil {
		return nil, err
	}

	return &f, nil
}

func (r *Repository) GetOrderFulfillments(ctx context.Context, orderID uuid.UUID) ([]Fulfillment, error) {
	query := `
		SELECT 
			f.id, f.order_id, f.seller_id, f.status, f.subtotal_cents, f.commission_bps, f.seller_amount_cents, f.created_at, f.updated_at,
			s.status as shipment_status, s.id as shipment_id,
			sel.brand_name as seller_name
		FROM order_fulfillments f
		JOIN sellers sel ON sel.id = f.seller_id
		LEFT JOIN shipments s ON (s.fulfillment_id = f.id) OR (s.fulfillment_id IS NULL AND s.order_id = f.order_id AND (SELECT COUNT(*) FROM order_fulfillments WHERE order_id = f.order_id) = 1)
		WHERE f.order_id = $1
		ORDER BY f.created_at ASC
	`
	rows, err := r.db.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Fulfillment
	for rows.Next() {
		var f Fulfillment
		if err := rows.Scan(
			&f.ID, &f.OrderID, &f.SellerID, &f.Status, &f.SubtotalCents, &f.CommissionBps, &f.SellerAmountCents, &f.CreatedAt, &f.UpdatedAt,
			&f.ShipmentStatus, &f.ShipmentID,
			&f.SellerName,
		); err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	if list == nil {
		list = make([]Fulfillment, 0)
	}

	for i := range list {
		items, err := r.GetFulfillmentItems(ctx, list[i].ID)
		if err != nil {
			return nil, err
		}
		list[i].Items = items
	}

	return list, nil
}
