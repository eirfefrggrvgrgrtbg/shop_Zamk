package orders

import (
	"context"
	"errors"
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
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`
	return tx.QueryRow(ctx, query, order.ID, order.UserID, order.Status, order.TotalPriceCents, order.Currency, order.CustomerName, order.CustomerPhone, order.CustomerEmail, order.DeliveryAddress).Scan(&order.CreatedAt, &order.UpdatedAt)
}

func (r *Repository) CreateOrderItemTx(ctx context.Context, tx pgx.Tx, item *OrderItem) error {
	query := `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, sku, image_url, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING created_at
	`
	return tx.QueryRow(ctx, query, item.ID, item.OrderID, item.ProductID, item.ProductVariantID, item.SellerID, item.Title, item.ProductSlug, item.VariantSize, item.VariantColor, item.Sku, item.ImageURL, item.PriceCents, item.Quantity, item.SubtotalPriceCents).Scan(&item.CreatedAt)
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
		SELECT id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at, cancelled_at
		FROM orders WHERE id = $1
	`
	var o Order
	err := r.db.QueryRow(ctx, query, id).Scan(&o.ID, &o.UserID, &o.Status, &o.TotalPriceCents, &o.Currency, &o.CustomerName, &o.CustomerPhone, &o.CustomerEmail, &o.DeliveryAddress, &o.CreatedAt, &o.UpdatedAt, &o.CancelledAt)
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
		SELECT id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at, cancelled_at
		FROM orders WHERE id = $1 FOR UPDATE
	`
	var o Order
	err := tx.QueryRow(ctx, query, id).Scan(&o.ID, &o.UserID, &o.Status, &o.TotalPriceCents, &o.Currency, &o.CustomerName, &o.CustomerPhone, &o.CustomerEmail, &o.DeliveryAddress, &o.CreatedAt, &o.UpdatedAt, &o.CancelledAt)
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
		SELECT id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at, cancelled_at
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
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalPriceCents, &o.Currency, &o.CustomerName, &o.CustomerPhone, &o.CustomerEmail, &o.DeliveryAddress, &o.CreatedAt, &o.UpdatedAt, &o.CancelledAt); err != nil {
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

func (r *Repository) ListAdminOrders(ctx context.Context, limit, offset int) ([]Order, error) {
	query := `
		SELECT id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at, cancelled_at
		FROM orders ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalPriceCents, &o.Currency, &o.CustomerName, &o.CustomerPhone, &o.CustomerEmail, &o.DeliveryAddress, &o.CreatedAt, &o.UpdatedAt, &o.CancelledAt); err != nil {
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

func (r *Repository) ListSellerOrders(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]SellerOrder, error) {
	query := `
		SELECT DISTINCT o.id, o.status, o.created_at
		FROM orders o
		JOIN order_items oi ON o.id = oi.order_id
		WHERE oi.seller_id = $1
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
		if err := rows.Scan(&o.ID, &o.Status, &o.CreatedAt); err != nil {
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
		SELECT o.id, o.status, o.created_at
		FROM orders o
		JOIN order_items oi ON o.id = oi.order_id
		WHERE o.id = $1 AND oi.seller_id = $2
		LIMIT 1
	`
	var o SellerOrder
	err := r.db.QueryRow(ctx, query, orderID, sellerID).Scan(&o.ID, &o.Status, &o.CreatedAt)
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
