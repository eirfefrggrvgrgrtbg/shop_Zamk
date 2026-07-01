package cart

import (
	"context"
	"database/sql"
	"errors"

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

func (r *Repository) GetCartByUserID(ctx context.Context, userID uuid.UUID) (*Cart, error) {
	query := `SELECT id, user_id, created_at, updated_at FROM carts WHERE user_id = $1`
	var cart Cart
	err := r.db.QueryRow(ctx, query, userID).Scan(&cart.ID, &cart.UserID, &cart.CreatedAt, &cart.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCartNotFound
		}
		return nil, err
	}

	// Fetch items
	itemsQuery := `
		SELECT 
			ci.id, ci.cart_id, ci.product_id, ci.product_variant_id, ci.quantity, ci.created_at, ci.updated_at,
			p.title, COALESCE(pv.price_cents, p.price_cents), 
			COALESCE(ii.total_stock - ii.reserved_stock, 0) > 0 AS in_stock
		FROM cart_items ci
		JOIN products p ON ci.product_id = p.id
		JOIN product_variants pv ON ci.product_variant_id = pv.id
		LEFT JOIN inventory_items ii ON pv.id = ii.product_variant_id
		WHERE ci.cart_id = $1
		ORDER BY ci.created_at ASC
	`
	rows, err := r.db.Query(ctx, itemsQuery, cart.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item CartItem
		if err := rows.Scan(
			&item.ID, &item.CartID, &item.ProductID, &item.ProductVariantID, &item.Quantity, &item.CreatedAt, &item.UpdatedAt,
			&item.Title, &item.PriceCents, &item.InStock,
		); err != nil {
			return nil, err
		}
		cart.Items = append(cart.Items, item)
	}

	if cart.Items == nil {
		cart.Items = make([]CartItem, 0)
	}

	return &cart, nil
}

func (r *Repository) CreateCart(ctx context.Context, userID uuid.UUID) (*Cart, error) {
	cart := &Cart{
		ID:     uuid.New(),
		UserID: userID,
	}
	query := `INSERT INTO carts (id, user_id) VALUES ($1, $2) RETURNING created_at, updated_at`
	err := r.db.QueryRow(ctx, query, cart.ID, cart.UserID).Scan(&cart.CreatedAt, &cart.UpdatedAt)
	if err != nil {
		return nil, err
	}
	cart.Items = make([]CartItem, 0)
	return cart, nil
}

func (r *Repository) GetCartItem(ctx context.Context, cartID, productVariantID uuid.UUID) (*CartItem, error) {
	query := `
		SELECT id, cart_id, product_id, product_variant_id, quantity, created_at, updated_at
		FROM cart_items 
		WHERE cart_id = $1 AND product_variant_id = $2
	`
	var item CartItem
	err := r.db.QueryRow(ctx, query, cartID, productVariantID).Scan(
		&item.ID, &item.CartID, &item.ProductID, &item.ProductVariantID, &item.Quantity, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCartItemNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) AddItem(ctx context.Context, item *CartItem) error {
	query := `
		INSERT INTO cart_items (id, cart_id, product_id, product_variant_id, quantity)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query, item.ID, item.CartID, item.ProductID, item.ProductVariantID, item.Quantity)
	return err
}

func (r *Repository) UpdateItemQuantity(ctx context.Context, itemID uuid.UUID, quantity int) error {
	query := `UPDATE cart_items SET quantity = $1, updated_at = now() WHERE id = $2`
	_, err := r.db.Exec(ctx, query, quantity, itemID)
	return err
}

func (r *Repository) RemoveItem(ctx context.Context, cartID, itemID uuid.UUID) error {
	query := `DELETE FROM cart_items WHERE id = $1 AND cart_id = $2`
	res, err := r.db.Exec(ctx, query, itemID, cartID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrCartItemNotFound
	}
	return nil
}

func (r *Repository) ClearCart(ctx context.Context, cartID uuid.UUID) error {
	query := `DELETE FROM cart_items WHERE cart_id = $1`
	_, err := r.db.Exec(ctx, query, cartID)
	return err
}

func (r *Repository) ClearCartTx(ctx context.Context, tx pgx.Tx, cartID uuid.UUID) error {
	query := `DELETE FROM cart_items WHERE cart_id = $1`
	_, err := tx.Exec(ctx, query, cartID)
	return err
}

func (r *Repository) DeleteCart(ctx context.Context, cartID uuid.UUID) error {
	query := `DELETE FROM carts WHERE id = $1`
	_, err := r.db.Exec(ctx, query, cartID)
	return err
}

type ProductValidationInfo struct {
	Status        string
	VariantActive bool
	Available     int
}

func (r *Repository) GetProductValidationInfo(ctx context.Context, productID, variantID uuid.UUID) (*ProductValidationInfo, error) {
	query := `
		SELECT 
			p.status, 
			pv.is_active, 
			COALESCE(ii.total_stock - ii.reserved_stock, 0) AS available
		FROM products p
		JOIN product_variants pv ON p.id = pv.product_id
		LEFT JOIN inventory_items ii ON pv.id = ii.product_variant_id
		WHERE p.id = $1 AND pv.id = $2
	`
	var info ProductValidationInfo
	var status sql.NullString
	var active sql.NullBool
	err := r.db.QueryRow(ctx, query, productID, variantID).Scan(&status, &active, &info.Available)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("product or variant not found")
		}
		return nil, err
	}
	info.Status = status.String
	info.VariantActive = active.Bool
	return &info, nil
}
