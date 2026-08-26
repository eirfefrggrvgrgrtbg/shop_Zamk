package orders

import (
	"context"
	"os"
	"testing"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSerializedAllocationPrimitives(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("Skipping test: TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	// Create test data directly
	sellerID := uuid.New()
	buyerID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	variantID := uuid.New()

	db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1, 'alloc@example.com', 'hash', 'buyer') ON CONFLICT DO NOTHING`, buyerID)
	db.Exec(ctx, `INSERT INTO sellers (id, user_id, company_name) VALUES ($1, $2, 'Seller') ON CONFLICT DO NOTHING`, sellerID, buyerID)
	db.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', 'cat') ON CONFLICT DO NOTHING`, catID)
	db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, description) VALUES ($1, $2, $3, 'Prod', 'prod', 'published', '') ON CONFLICT DO NOTHING`, prodID, sellerID, catID)
	db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, 'alloc-sku', 1000, true) ON CONFLICT DO NOTHING`, variantID, prodID)

	orderID := uuid.New()
	db.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'awaiting_payment', 1000, 'N', 'P', 'E', 'A')
	`, orderID, buyerID)

	orderItemID := uuid.New()
	db.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, 'Title', 'slug', 1000, 2, 2000)
	`, orderItemID, orderID, prodID, variantID, sellerID)

	orderItem2ID := uuid.New()
	db.Exec(ctx, `
		INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, 'Title2', 'slug2', 1000, 2, 2000)
	`, orderItem2ID, orderID, prodID, variantID, sellerID)

	supplyID := uuid.New()
	db.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, human_id)
		VALUES ($1, $2, 'created', 'SUP-ALLOC', 'ALLOC')
	`, supplyID, sellerID)

	supplyItemID := uuid.New()
	db.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, product_variant_id, expected_quantity)
		VALUES ($1, $2, $3, 5)
	`, supplyItemID, supplyID, variantID)

	// Create 5 warehouse ZMUs for variantID
	for i := 1; i <= 5; i++ {
		uid := uuid.New()
		db.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
			VALUES ($1, $2, $3, $4, $5, $6, 'warehouse')
		`, uid, "ZMU-ALLOC-"+uuid.New().String()[:6], variantID, supplyID, supplyItemID, i)
	}

	// Variant mismatch test setup
	variantID2 := uuid.New()
	db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, 'alloc-sku2', 1000, true) ON CONFLICT DO NOTHING`, variantID2, prodID)
	uidMismatch := uuid.New()
	db.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'warehouse')
	`, uidMismatch, "ZMU-ALLOC-MISMATCH", variantID2, supplyID, supplyItemID, 6)

	t.Run("A_Allocate2", func(t *testing.T) {
		tx, _ := db.Begin(ctx)
		allocated, err := repo.AllocateUnitsForOrderItem(ctx, tx, orderItemID, variantID, 2, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(allocated) != 2 {
			t.Fatalf("expected 2 allocated, got %d", len(allocated))
		}
		tx.Commit(ctx)

		allocs, _ := repo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
		if len(allocs) != 2 {
			t.Fatalf("expected 2 active allocations")
		}
		if allocs[0].InventoryUnitID == allocs[1].InventoryUnitID {
			t.Fatalf("expected distinct units")
		}
	})

	t.Run("B_AllocateAnother2", func(t *testing.T) {
		tx, _ := db.Begin(ctx)
		allocated, err := repo.AllocateUnitsForOrderItem(ctx, tx, orderItem2ID, variantID, 2, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(allocated) != 2 {
			t.Fatalf("expected 2 allocated, got %d", len(allocated))
		}
		tx.Commit(ctx)

		allocs, _ := repo.ListActiveAllocationsForOrderItem(ctx, orderItem2ID)
		if len(allocs) != 2 {
			t.Fatalf("expected 2 active allocations")
		}
	})

	t.Run("C_AllocateFailsNotEnough", func(t *testing.T) {
		tx, _ := db.Begin(ctx)
		_, err := repo.AllocateUnitsForOrderItem(ctx, tx, uuid.New(), variantID, 2, nil)
		if err != ErrInsufficientWarehouseUnits {
			t.Fatalf("expected ErrInsufficientWarehouseUnits, got %v", err)
		}
		tx.Rollback(ctx)
	})

	t.Run("D_ReleaseAllocations", func(t *testing.T) {
		tx, _ := db.Begin(ctx)
		err := repo.ReleaseAllocationsForOrderItem(ctx, tx, orderItemID, "cancelled")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		tx.Commit(ctx)

		allocs, _ := repo.ListActiveAllocationsForOrderItem(ctx, orderItemID)
		if len(allocs) != 0 {
			t.Fatalf("expected 0 active allocations")
		}

		// Now we can allocate 2 more
		tx, _ = db.Begin(ctx)
		allocated, err := repo.AllocateUnitsForOrderItem(ctx, tx, uuid.New(), variantID, 2, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(allocated) != 2 {
			t.Fatalf("expected 2 allocated, got %d", len(allocated))
		}
		tx.Commit(ctx)
	})

	t.Run("F_VariantMismatch", func(t *testing.T) {
		tx, _ := db.Begin(ctx)
		allocated, err := repo.AllocateUnitsForOrderItem(ctx, tx, uuid.New(), variantID2, 1, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(allocated) != 1 {
			t.Fatalf("expected 1 allocated, got %d", len(allocated))
		}
		if allocated[0] != uidMismatch {
			t.Fatalf("expected uidMismatch")
		}
		tx.Commit(ctx)
	})
}
