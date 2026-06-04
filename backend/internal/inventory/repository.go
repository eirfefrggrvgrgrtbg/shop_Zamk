package inventory

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{db: tx}
}

func (r *Repository) GetItemForUpdateByVariant(ctx context.Context, variantID uuid.UUID) (*Item, error) {
	query := `
		SELECT id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at
		FROM inventory_items
		WHERE product_variant_id = $1
		FOR UPDATE
	`
	var i Item
	err := r.db.QueryRow(ctx, query, variantID).Scan(
		&i.ID, &i.ProductID, &i.ProductVariantID, &i.SellerID, &i.TotalStock, &i.ReservedStock, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInventoryItemNotFound
		}
		return nil, fmt.Errorf("failed to lock inventory item: %w", err)
	}
	i.ComputeAvailable()
	return &i, nil
}

func (r *Repository) GetItemByID(ctx context.Context, itemID uuid.UUID) (*Item, error) {
	query := `
		SELECT id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at
		FROM inventory_items
		WHERE id = $1
	`
	var i Item
	err := r.db.QueryRow(ctx, query, itemID).Scan(
		&i.ID, &i.ProductID, &i.ProductVariantID, &i.SellerID, &i.TotalStock, &i.ReservedStock, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInventoryItemNotFound
		}
		return nil, fmt.Errorf("failed to get inventory item: %w", err)
	}
	i.ComputeAvailable()
	return &i, nil
}

func (r *Repository) GetItemByVariantID(ctx context.Context, variantID uuid.UUID) (*Item, error) {
	query := `
		SELECT id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at
		FROM inventory_items
		WHERE product_variant_id = $1
	`
	var i Item
	err := r.db.QueryRow(ctx, query, variantID).Scan(
		&i.ID, &i.ProductID, &i.ProductVariantID, &i.SellerID, &i.TotalStock, &i.ReservedStock, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInventoryItemNotFound
		}
		return nil, fmt.Errorf("failed to get inventory item: %w", err)
	}
	i.ComputeAvailable()
	return &i, nil
}

func (r *Repository) CreateItem(ctx context.Context, i *Item) error {
	query := `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		i.ID, i.ProductID, i.ProductVariantID, i.SellerID, i.TotalStock, i.ReservedStock, i.CreatedAt, i.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create inventory item: %w", err)
	}
	return nil
}

func (r *Repository) UpdateItemStock(ctx context.Context, i *Item) error {
	query := `
		UPDATE inventory_items
		SET total_stock = $1, reserved_stock = $2, updated_at = now()
		WHERE id = $3
	`
	res, err := r.db.Exec(ctx, query, i.TotalStock, i.ReservedStock, i.ID)
	if err != nil {
		return fmt.Errorf("failed to update inventory item: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrInventoryItemNotFound
	}
	return nil
}

func (r *Repository) RecordMovement(ctx context.Context, m *StockMovement) error {
	query := `
		INSERT INTO stock_movements (id, inventory_item_id, product_id, product_variant_id, seller_id, type, quantity, reason, actor_user_id, reference_type, reference_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.db.Exec(ctx, query,
		m.ID, m.InventoryItemID, m.ProductID, m.ProductVariantID, m.SellerID, m.Type, m.Quantity, m.Reason, m.ActorUserID, m.ReferenceType, m.ReferenceID, m.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to record stock movement: %w", err)
	}
	return nil
}

func (r *Repository) CreateReservation(ctx context.Context, res *Reservation) error {
	query := `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		res.ID, res.InventoryItemID, res.ProductID, res.ProductVariantID, res.UserID, res.Quantity, res.Status, res.ExpiresAt, res.OrderID, res.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create reservation: %w", err)
	}
	return nil
}

func (r *Repository) UpdateReservationStatus(ctx context.Context, res *Reservation) error {
	query := `
		UPDATE reservations
		SET status = $1, released_at = $2, order_id = $3
		WHERE id = $4
	`
	result, err := r.db.Exec(ctx, query, res.Status, res.ReleasedAt, res.OrderID, res.ID)
	if err != nil {
		return fmt.Errorf("failed to update reservation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrReservationNotFound
	}
	return nil
}

func (r *Repository) GetReservationByIDForUpdate(ctx context.Context, id uuid.UUID) (*Reservation, error) {
	query := `
		SELECT id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id, created_at, released_at
		FROM reservations
		WHERE id = $1
		FOR UPDATE
	`
	var res Reservation
	err := r.db.QueryRow(ctx, query, id).Scan(
		&res.ID, &res.InventoryItemID, &res.ProductID, &res.ProductVariantID, &res.UserID, &res.Quantity, &res.Status, &res.ExpiresAt, &res.OrderID, &res.CreatedAt, &res.ReleasedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReservationNotFound
		}
		return nil, fmt.Errorf("failed to get reservation: %w", err)
	}
	return &res, nil
}

// Listing operations

func (r *Repository) ListInventory(ctx context.Context) ([]Item, error) {
	query := `
		SELECT id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at
		FROM inventory_items
		ORDER BY created_at DESC
	`
	return r.listInventoryItems(ctx, query)
}

func (r *Repository) ListInventoryBySeller(ctx context.Context, sellerID uuid.UUID) ([]Item, error) {
	query := `
		SELECT id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at
		FROM inventory_items
		WHERE seller_id = $1
		ORDER BY created_at DESC
	`
	return r.listInventoryItems(ctx, query, sellerID)
}

func (r *Repository) listInventoryItems(ctx context.Context, query string, args ...any) ([]Item, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list inventory items: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var i Item
		if err := rows.Scan(&i.ID, &i.ProductID, &i.ProductVariantID, &i.SellerID, &i.TotalStock, &i.ReservedStock, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		i.ComputeAvailable()
		items = append(items, i)
	}
	return items, nil
}

func (r *Repository) ListMovementsByInventoryItemID(ctx context.Context, itemID uuid.UUID) ([]StockMovement, error) {
	query := `
		SELECT id, inventory_item_id, product_id, product_variant_id, seller_id, type, quantity, reason, actor_user_id, reference_type, reference_id, created_at
		FROM stock_movements
		WHERE inventory_item_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to list stock movements: %w", err)
	}
	defer rows.Close()

	var movs []StockMovement
	for rows.Next() {
		var m StockMovement
		if err := rows.Scan(
			&m.ID, &m.InventoryItemID, &m.ProductID, &m.ProductVariantID, &m.SellerID,
			&m.Type, &m.Quantity, &m.Reason, &m.ActorUserID, &m.ReferenceType, &m.ReferenceID, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		movs = append(movs, m)
	}
	return movs, nil
}
