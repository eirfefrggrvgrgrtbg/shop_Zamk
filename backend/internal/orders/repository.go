package orders

import (
	"context"
	"errors"
	"fmt"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/common"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateOrderTx(ctx context.Context, tx pgx.Tx, order *Order) error {
	query := `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, delivery_method_id, delivery_method_code, delivery_method_name, delivery_price_cents, delivery_estimated_days_min, delivery_estimated_days_max, checkout_idempotency_key, checkout_request_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING created_at, updated_at
	`
	return tx.QueryRow(ctx, query, order.ID, order.UserID, order.Status, order.TotalPriceCents, order.Currency, order.CustomerName, order.CustomerPhone, order.CustomerEmail, order.DeliveryAddress, order.DeliveryMethodID, order.DeliveryMethodCode, order.DeliveryMethodName, order.DeliveryPriceCents, order.DeliveryEstimatedDaysMin, order.DeliveryEstimatedDaysMax, order.CheckoutIdempotencyKey, order.CheckoutRequestHash).Scan(&order.CreatedAt, &order.UpdatedAt)
}

func (r *Repository) CreateOrderItemTx(ctx context.Context, tx pgx.Tx, item *OrderItem) error {
	query := `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, image_url, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING created_at
	`
	return tx.QueryRow(ctx, query, item.ID, item.OrderID, item.OrderFulfillmentID, item.ProductID, item.ProductVariantID, item.SellerID, item.Title, item.ProductSlug, item.VariantSize, item.VariantColor, item.Sku, item.ImageURL, item.PriceCents, item.Quantity, item.SubtotalPriceCents).Scan(&item.CreatedAt)
}

func (r *Repository) CreateOrderFulfillmentTx(ctx context.Context, tx pgx.Tx, f *OrderFulfillment) error {
	query := `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`
	return tx.QueryRow(ctx, query, f.ID, f.OrderID, f.SellerID, f.Status, f.SubtotalCents, f.CommissionBps, f.SellerAmountCents).Scan(&f.CreatedAt, &f.UpdatedAt)
}

func (r *Repository) MarkOrderFulfillmentsStatusTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, fromStatus, toStatus string) (int64, error) {
	query := `UPDATE order_fulfillments SET status = $1, updated_at = now() WHERE order_id = $2 AND status = $3`
	cmd, err := tx.Exec(ctx, query, toStatus, orderID, fromStatus)
	if err != nil {
		return 0, err
	}
	return cmd.RowsAffected(), nil
}

func (r *Repository) CreateOrderReservationTx(ctx context.Context, tx pgx.Tx, res *OrderReservation) error {
	query := `INSERT INTO order_reservations (id, order_id, reservation_id) VALUES ($1, $2, $3) RETURNING created_at`
	return tx.QueryRow(ctx, query, res.ID, res.OrderID, res.ReservationID).Scan(&res.CreatedAt)
}

func (r *Repository) CreateOrderStatusHistoryTx(ctx context.Context, tx pgx.Tx, h *OrderStatusHistory) error {
	query := `INSERT INTO order_status_history (id, order_id, from_status, to_status, actor_user_id, comment) VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at`
	return tx.QueryRow(ctx, query, h.ID, h.OrderID, h.FromStatus, h.ToStatus, h.ActorUserID, h.Comment).Scan(&h.CreatedAt)
}

func (r *Repository) GetOrder(ctx context.Context, id uuid.UUID) (*Order, error) {
	query := `
		SELECT id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, delivery_method_id, delivery_method_code, delivery_method_name, delivery_price_cents, delivery_estimated_days_min, delivery_estimated_days_max, checkout_idempotency_key, checkout_request_hash, created_at, updated_at, cancelled_at
		FROM orders WHERE id = $1
	`
	var o Order
	err := r.db.QueryRow(ctx, query, id).Scan(&o.ID, &o.UserID, &o.Status, &o.TotalPriceCents, &o.Currency, &o.CustomerName, &o.CustomerPhone, &o.CustomerEmail, &o.DeliveryAddress, &o.DeliveryMethodID, &o.DeliveryMethodCode, &o.DeliveryMethodName, &o.DeliveryPriceCents, &o.DeliveryEstimatedDaysMin, &o.DeliveryEstimatedDaysMax, &o.CheckoutIdempotencyKey, &o.CheckoutRequestHash, &o.CreatedAt, &o.UpdatedAt, &o.CancelledAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	o.Items, err = r.GetOrderItems(ctx, o.ID)
	if err != nil {
		return nil, err
	}

	return &o, nil
}

func (r *Repository) GetOrderForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Order, error) {
	query := `
		SELECT id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, delivery_method_id, delivery_method_code, delivery_method_name, delivery_price_cents, delivery_estimated_days_min, delivery_estimated_days_max, checkout_idempotency_key, checkout_request_hash, created_at, updated_at, cancelled_at
		FROM orders WHERE id = $1 FOR UPDATE
	`
	var o Order
	err := tx.QueryRow(ctx, query, id).Scan(&o.ID, &o.UserID, &o.Status, &o.TotalPriceCents, &o.Currency, &o.CustomerName, &o.CustomerPhone, &o.CustomerEmail, &o.DeliveryAddress, &o.DeliveryMethodID, &o.DeliveryMethodCode, &o.DeliveryMethodName, &o.DeliveryPriceCents, &o.DeliveryEstimatedDaysMin, &o.DeliveryEstimatedDaysMax, &o.CheckoutIdempotencyKey, &o.CheckoutRequestHash, &o.CreatedAt, &o.UpdatedAt, &o.CancelledAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return &o, nil
}

func (r *Repository) GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]OrderItem, error) {
	query := `
		SELECT id, order_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, image_url, price_cents, quantity, subtotal_price_cents, created_at
		FROM order_items WHERE order_id = $1 ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []OrderItem
	for rows.Next() {
		var i OrderItem
		if err := rows.Scan(&i.ID, &i.OrderID, &i.ProductID, &i.ProductVariantID, &i.SellerID, &i.Title, &i.ProductSlug, &i.VariantSize, &i.VariantColor, &i.Sku, &i.ImageURL, &i.PriceCents, &i.Quantity, &i.SubtotalPriceCents, &i.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if items == nil {
		items = make([]OrderItem, 0)
	}
	return items, nil
}

func (r *Repository) ListCustomerOrders(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Order, error) {
	query := `
		SELECT id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, delivery_method_id, delivery_method_code, delivery_method_name, delivery_price_cents, delivery_estimated_days_min, delivery_estimated_days_max, checkout_idempotency_key, checkout_request_hash, created_at, updated_at, cancelled_at
		FROM orders WHERE user_id = $1 ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalPriceCents, &o.Currency, &o.CustomerName, &o.CustomerPhone, &o.CustomerEmail, &o.DeliveryAddress, &o.DeliveryMethodID, &o.DeliveryMethodCode, &o.DeliveryMethodName, &o.DeliveryPriceCents, &o.DeliveryEstimatedDaysMin, &o.DeliveryEstimatedDaysMax, &o.CheckoutIdempotencyKey, &o.CheckoutRequestHash, &o.CreatedAt, &o.UpdatedAt, &o.CancelledAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}

	for i := range orders {
		orders[i].Items, err = r.GetOrderItems(ctx, orders[i].ID)
		if err != nil {
			return nil, err
		}
	}

	if orders == nil {
		orders = make([]Order, 0)
	}
	return orders, nil
}

func (r *Repository) ListAdminOrders(ctx context.Context, q, status, paymentStatus, fulfillmentStatus, sourceType, sellerId string, limit, offset int) ([]AdminOrder, int, error) {
	baseQuery := `
		FROM orders o
		LEFT JOIN order_items oi ON o.id = oi.order_id
		LEFT JOIN order_fulfillments f ON o.id = f.order_id
		LEFT JOIN auction_order_links aol ON o.id = aol.order_id
		WHERE 1=1
	`
	args := []interface{}{}
	argID := 1

	if q != "" {
		baseQuery += ` AND (o.customer_name ILIKE $` + fmt.Sprintf("%d", argID) + ` OR o.customer_email ILIKE $` + fmt.Sprintf("%d", argID+1) + ` OR o.id::text ILIKE $` + fmt.Sprintf("%d", argID+2) + `)`
		args = append(args, "%"+q+"%", "%"+q+"%", "%"+q+"%")
		argID += 3
	}
	if status != "" {
		baseQuery += ` AND o.status = $` + fmt.Sprintf("%d", argID)
		args = append(args, status)
		argID++
	}
	if paymentStatus != "" {
		baseQuery += ` AND o.status = $` + fmt.Sprintf("%d", argID) // In this system order status IS payment status
		args = append(args, paymentStatus)
		argID++
	}
	if fulfillmentStatus != "" {
		baseQuery += ` AND f.status = $` + fmt.Sprintf("%d", argID)
		args = append(args, fulfillmentStatus)
		argID++
	}
	if sellerId != "" {
		baseQuery += ` AND f.seller_id = $` + fmt.Sprintf("%d", argID)
		args = append(args, sellerId)
		argID++
	}
	if sourceType == "auction" {
		baseQuery += " AND aol.id IS NOT NULL"
	} else if sourceType == "direct_sale" {
		baseQuery += " AND aol.id IS NULL AND oi.seller_id = '" + common.PlatformSellerIDStr + "'"
	} else if sourceType == "normal" {
		baseQuery += " AND aol.id IS NULL AND oi.seller_id != '" + common.PlatformSellerIDStr + "'"
	}

	countQuery := "SELECT COUNT(DISTINCT o.id) " + baseQuery
	var totalCount int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	selectQuery := `
		SELECT 
			o.id, o.user_id, o.status, 
			MAX(COALESCE(f.status, 'pending')) as fulfillment_status,
			CASE 
				WHEN MAX(aol.id::text) IS NOT NULL THEN 'auction'
				WHEN MAX(oi.seller_id::text) = '` + common.PlatformSellerIDStr + `' THEN 'direct_sale'
				ELSE 'normal'
			END as source_type,
			o.total_price_cents, o.currency, o.customer_name, o.customer_email, o.created_at, o.updated_at, o.cancelled_at
	` + baseQuery + `
		GROUP BY o.id
		ORDER BY o.created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argID) + ` OFFSET $` + fmt.Sprintf("%d", argID+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []AdminOrder
	for rows.Next() {
		var o AdminOrder
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.FulfillmentStatus, &o.SourceType, &o.TotalPriceCents, &o.Currency, &o.CustomerName, &o.CustomerEmail, &o.CreatedAt, &o.UpdatedAt, &o.CancelledAt); err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}

	if orders == nil {
		orders = make([]AdminOrder, 0)
	}
	return orders, totalCount, nil
}

func (r *Repository) GetAdminOrderDetail(ctx context.Context, id uuid.UUID) (*AdminOrderDetail, error) {
	query := `
		SELECT 
			o.id, o.user_id, o.status, 
			(SELECT COALESCE(MAX(status), 'pending') FROM order_fulfillments WHERE order_id = o.id) as fulfillment_status,
			CASE 
				WHEN EXISTS(SELECT 1 FROM auction_order_links WHERE order_id = o.id) THEN 'auction'
				WHEN EXISTS(SELECT 1 FROM order_items WHERE order_id = o.id AND seller_id = '` + common.PlatformSellerIDStr + `') THEN 'direct_sale'
				ELSE 'normal'
			END as source_type,
			o.total_price_cents, o.currency, o.customer_name, o.customer_email, o.created_at, o.updated_at, o.cancelled_at,
			o.customer_phone, o.delivery_address, o.delivery_method_id, o.delivery_method_code, o.delivery_method_name, o.delivery_price_cents, o.delivery_estimated_days_min, o.delivery_estimated_days_max
		FROM orders o
		WHERE o.id = $1
	`
	var o AdminOrderDetail
	err := r.db.QueryRow(ctx, query, id).Scan(
		&o.ID, &o.UserID, &o.Status, &o.FulfillmentStatus, &o.SourceType,
		&o.TotalPriceCents, &o.Currency, &o.CustomerName, &o.CustomerEmail,
		&o.CreatedAt, &o.UpdatedAt, &o.CancelledAt,
		&o.CustomerPhone, &o.DeliveryAddress, &o.DeliveryMethodID, &o.DeliveryMethodCode, &o.DeliveryMethodName, &o.DeliveryPriceCents, &o.DeliveryEstimatedDaysMin, &o.DeliveryEstimatedDaysMax,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	o.Items, err = r.GetOrderItems(ctx, o.ID)
	if err != nil {
		return nil, err
	}

	fQuery := `
		SELECT id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at
		FROM order_fulfillments WHERE order_id = $1
	`
	fRows, err := r.db.Query(ctx, fQuery, o.ID)
	if err != nil {
		return nil, err
	}
	defer fRows.Close()

	for fRows.Next() {
		var f OrderFulfillment
		if err := fRows.Scan(&f.ID, &f.OrderID, &f.SellerID, &f.Status, &f.SubtotalCents, &f.CommissionBps, &f.SellerAmountCents, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		o.Fulfillments = append(o.Fulfillments, f)
	}

	if o.Fulfillments == nil {
		o.Fulfillments = make([]OrderFulfillment, 0)
	}

	return &o, nil
}

func (r *Repository) ListSellerOrders(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]SellerOrder, error) {
	query := `
		SELECT o.id, o.status, o.created_at, s.status
		FROM orders o
		JOIN order_items oi ON o.id = oi.order_id
		LEFT JOIN shipments s ON o.id = s.order_id
		WHERE oi.seller_id = $1
		GROUP BY o.id, o.status, o.created_at, s.status
		ORDER BY o.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, sellerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []SellerOrder
	for rows.Next() {
		var o SellerOrder
		if err := rows.Scan(&o.ID, &o.Status, &o.CreatedAt, &o.ShipmentStatus); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}

	for i := range orders {
		// Only fetch items belonging to this seller
		itemQuery := `
			SELECT id, order_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, image_url, price_cents, quantity, subtotal_price_cents, created_at
			FROM order_items WHERE order_id = $1 AND seller_id = $2 ORDER BY created_at ASC
		`
		itemRows, err := r.db.Query(ctx, itemQuery, orders[i].ID, sellerID)
		if err != nil {
			return nil, err
		}
		var items []OrderItem
		for itemRows.Next() {
			var it OrderItem
			if err := itemRows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.ProductVariantID, &it.SellerID, &it.Title, &it.ProductSlug, &it.VariantSize, &it.VariantColor, &it.Sku, &it.ImageURL, &it.PriceCents, &it.Quantity, &it.SubtotalPriceCents, &it.CreatedAt); err != nil {
				itemRows.Close()
				return nil, err
			}
			items = append(items, it)
		}
		itemRows.Close()
		if items == nil {
			items = make([]OrderItem, 0)
		}
		orders[i].Items = items
	}

	if orders == nil {
		orders = make([]SellerOrder, 0)
	}
	return orders, nil
}

func (r *Repository) GetSellerOrder(ctx context.Context, sellerID, orderID uuid.UUID) (*SellerOrder, error) {
	query := `
		SELECT o.id, o.status, o.created_at, s.status
		FROM orders o
		JOIN order_items oi ON o.id = oi.order_id
		LEFT JOIN shipments s ON o.id = s.order_id
		WHERE o.id = $1 AND oi.seller_id = $2
		LIMIT 1
	`
	var o SellerOrder
	err := r.db.QueryRow(ctx, query, orderID, sellerID).Scan(&o.ID, &o.Status, &o.CreatedAt, &o.ShipmentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	itemQuery := `
		SELECT id, order_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, image_url, price_cents, quantity, subtotal_price_cents, created_at
		FROM order_items WHERE order_id = $1 AND seller_id = $2 ORDER BY created_at ASC
	`
	itemRows, err := r.db.Query(ctx, itemQuery, o.ID, sellerID)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	var items []OrderItem
	for itemRows.Next() {
		var it OrderItem
		if err := itemRows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.ProductVariantID, &it.SellerID, &it.Title, &it.ProductSlug, &it.VariantSize, &it.VariantColor, &it.Sku, &it.ImageURL, &it.PriceCents, &it.Quantity, &it.SubtotalPriceCents, &it.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if items == nil {
		items = make([]OrderItem, 0)
	}
	o.Items = items

	return &o, nil
}

func (r *Repository) UpdateOrderStatusTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, status string) error {
	query := `UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`
	_, err := tx.Exec(ctx, query, status, orderID)
	return err
}

func (r *Repository) SetOrderCancelledTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	query := `UPDATE orders SET status = 'cancelled', updated_at = now(), cancelled_at = now() WHERE id = $1`
	_, err := tx.Exec(ctx, query, orderID)
	return err
}

func (r *Repository) GetOrderReservations(ctx context.Context, orderID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT reservation_id FROM order_reservations WHERE order_id = $1`
	rows, err := r.db.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *Repository) GetExpiredAwaitingPaymentOrdersTx(ctx context.Context, tx pgx.Tx, olderThan time.Time, limit int) ([]uuid.UUID, error) {
	query := `
		SELECT id
		FROM orders
		WHERE status = 'awaiting_payment' AND created_at < $1
		ORDER BY created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.Query(ctx, query, olderThan, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *Repository) GetSellerIDByUserID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var sellerID uuid.UUID
	query := `SELECT seller_id FROM seller_users WHERE user_id = $1 LIMIT 1`
	err := r.db.QueryRow(ctx, query, userID).Scan(&sellerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, errors.New("not a seller")
		}
		return uuid.Nil, err
	}
	return sellerID, nil
}

type DeliveryMethodSnapshot struct {
	ID               uuid.UUID
	Code             string
	Name             string
	PriceCents       int64
	EstimatedDaysMin *int
	EstimatedDaysMax *int
	IsActive         bool
}

func (r *Repository) GetDeliveryMethod(ctx context.Context, id uuid.UUID) (*DeliveryMethodSnapshot, error) {
	query := `SELECT id, code, name, price_cents, estimated_days_min, estimated_days_max, is_active FROM delivery_methods WHERE id = $1`
	var m DeliveryMethodSnapshot
	err := r.db.QueryRow(ctx, query, id).Scan(&m.ID, &m.Code, &m.Name, &m.PriceCents, &m.EstimatedDaysMin, &m.EstimatedDaysMax, &m.IsActive)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repository) GetOrderByIdempotencyKey(ctx context.Context, userID uuid.UUID, idempotencyKey uuid.UUID) (*Order, error) {
	query := `
		SELECT id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, delivery_method_id, delivery_method_code, delivery_method_name, delivery_price_cents, delivery_estimated_days_min, delivery_estimated_days_max, checkout_idempotency_key, checkout_request_hash, created_at, updated_at, cancelled_at
		FROM orders WHERE user_id = $1 AND checkout_idempotency_key = $2
	`
	var o Order
	err := r.db.QueryRow(ctx, query, userID, idempotencyKey).Scan(&o.ID, &o.UserID, &o.Status, &o.TotalPriceCents, &o.Currency, &o.CustomerName, &o.CustomerPhone, &o.CustomerEmail, &o.DeliveryAddress, &o.DeliveryMethodID, &o.DeliveryMethodCode, &o.DeliveryMethodName, &o.DeliveryPriceCents, &o.DeliveryEstimatedDaysMin, &o.DeliveryEstimatedDaysMax, &o.CheckoutIdempotencyKey, &o.CheckoutRequestHash, &o.CreatedAt, &o.UpdatedAt, &o.CancelledAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	o.Items, err = r.GetOrderItems(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	return &o, nil
}
