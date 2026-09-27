package behavior

import (
	"context"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
)

type Repository struct {
	db *postgres.Client
}

func NewRepository(db *postgres.Client) *Repository {
	return &Repository{db: db}
}

// InsertEvents batch inserts events.
func (r *Repository) InsertEvents(ctx context.Context, events []BehavioralEvent) (int, int, error) {
	if len(events) == 0 {
		return 0, 0, nil
	}

	query := `
		INSERT INTO behavioral_events (
			id, event_type, source, visitor_id, user_id,
			product_id, variant_id, category_id, order_id, return_id, order_item_id,
			quantity, placement, route, occurred_at, received_at, metadata
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17
		)
		ON CONFLICT (id) DO NOTHING
	`

	// Start a transaction if needed, or just iterate. For performance on small batches, iterating is okay,
	// but a transaction is cleaner.
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	accepted := 0
	for _, e := range events {
		res, err := tx.Exec(ctx, query,
			e.ID, e.EventType, e.Source, e.VisitorID, e.UserID,
			e.ProductID, e.VariantID, e.CategoryID, e.OrderID, e.ReturnID, e.OrderItemID,
			e.Quantity, e.Placement, e.Route, e.OccurredAt, e.ReceivedAt, e.Metadata,
		)
		if err != nil {
			return 0, 0, err
		}
		if res.RowsAffected() == 1 {
			accepted++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}

	duplicates := len(events) - accepted
	return accepted, duplicates, nil
}

// VariantRecord represents an existing variant and its canonical product/category.
type VariantRecord struct {
	ProductID  uuid.UUID
	CategoryID *uuid.UUID
}

// ProductRecord represents an existing product and its canonical category.
type ProductRecord struct {
	CategoryID *uuid.UUID
}

// ValidationData contains preloaded entity records for batch validation.
type ValidationData struct {
	Variants map[uuid.UUID]VariantRecord
	Products map[uuid.UUID]ProductRecord
}

// ValidateProductsAndVariants performs batch queries to check variants and products, resolving canonical category IDs.
func (r *Repository) ValidateProductsAndVariants(ctx context.Context, variantIDs []uuid.UUID, productIDs []uuid.UUID) (ValidationData, error) {
	data := ValidationData{
		Variants: make(map[uuid.UUID]VariantRecord),
		Products: make(map[uuid.UUID]ProductRecord),
	}

	if len(variantIDs) > 0 {
		queryVariants := `
			SELECT v.id, v.product_id, p.category_id
			FROM product_variants v
			JOIN products p ON p.id = v.product_id
			WHERE v.id = ANY($1)
		`
		rows, err := r.db.Pool.Query(ctx, queryVariants, variantIDs)
		if err != nil {
			return data, err
		}
		defer rows.Close()

		for rows.Next() {
			var vID uuid.UUID
			var pID uuid.UUID
			var cID *uuid.UUID
			if err := rows.Scan(&vID, &pID, &cID); err != nil {
				return data, err
			}
			data.Variants[vID] = VariantRecord{
				ProductID:  pID,
				CategoryID: cID,
			}
		}
	}

	if len(productIDs) > 0 {
		queryProducts := `
			SELECT id, category_id
			FROM products
			WHERE id = ANY($1)
		`
		rows, err := r.db.Pool.Query(ctx, queryProducts, productIDs)
		if err != nil {
			return data, err
		}
		defer rows.Close()

		for rows.Next() {
			var pID uuid.UUID
			var cID *uuid.UUID
			if err := rows.Scan(&pID, &cID); err != nil {
				return data, err
			}
			data.Products[pID] = ProductRecord{
				CategoryID: cID,
			}
		}
	}

	return data, nil
}
