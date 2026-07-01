package orders

import (
	"context"
	"os"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestC8FulfillmentCreation(t *testing.T) {
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

	// Clean up tables
	db.Exec(ctx, "DELETE FROM order_status_history")
	db.Exec(ctx, "DELETE FROM order_reservations")
	db.Exec(ctx, "DELETE FROM order_items")
	db.Exec(ctx, "DELETE FROM order_fulfillments")
	db.Exec(ctx, "DELETE FROM orders")

	repo := NewRepository(db)

	buyer := uuid.New()
	sellerA := uuid.New()
	sellerB := uuid.New()

	productA := uuid.New()
	variantA := uuid.New()

	productB := uuid.New()
	variantB := uuid.New()

	// Insert mock users
	if _, err := db.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'Buyer', $2, 'hash', 'customer', 'active', false, now(), now())", buyer, buyer.String()+"@ex.com"); err != nil { t.Fatalf("buyer: %v", err) }
	if _, err := db.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'Seller A', $2, 'hash', 'seller', 'active', false, now(), now())", sellerA, sellerA.String()+"@ex.com"); err != nil { t.Fatalf("sellerA: %v", err) }
	if _, err := db.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at) VALUES ($1, 'Seller B', $2, 'hash', 'seller', 'active', false, now(), now())", sellerB, sellerB.String()+"@ex.com"); err != nil { t.Fatalf("sellerB: %v", err) }
	
	if _, err := db.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, $2, $3, $4, 'active')", sellerA, "Brand A", sellerA.String(), "contactA@ex.com"); err != nil { t.Fatalf("sellerA profile: %v", err) }
	if _, err := db.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, $2, $3, $4, 'active')", sellerB, "Brand B", sellerB.String(), "contactB@ex.com"); err != nil { t.Fatalf("sellerB profile: %v", err) }
	
	catID := uuid.New()
	brandID := uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'cat', $2)", catID, catID.String()); err != nil { t.Fatalf("cat: %v", err) }
	if _, err := db.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, 'br', $2)", brandID, brandID.String()); err != nil { t.Fatalf("br: %v", err) }

	if _, err := db.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents) VALUES ($1, $2, $3, $4, 'P A', $5, 'desc', 'published', 100000)", productA, sellerA, catID, brandID, productA.String()); err != nil { t.Fatalf("prodA: %v", err) }
	if _, err := db.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents) VALUES ($1, $2, $3, $4, 'P B', $5, 'desc', 'published', 200000)", productB, sellerB, catID, brandID, productB.String()); err != nil { t.Fatalf("prodB: %v", err) }

	if _, err := db.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, $3, 100000, true)", variantA, productA, variantA.String()); err != nil { t.Fatalf("varA: %v", err) }
	if _, err := db.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, $3, 200000, true)", variantB, productB, variantB.String()); err != nil { t.Fatalf("varB: %v", err) }

	// Mock cart directly into db so real cartRepo can find it
	cID := uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO carts (id, user_id) VALUES ($1, $2)", cID, buyer); err != nil {
		t.Fatalf("insert cart: %v", err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO cart_items (id, cart_id, product_id, product_variant_id, quantity) VALUES ($1, $2, $3, $4, 2)", uuid.New(), cID, productA, variantA); err != nil {
		t.Fatalf("insert cart item A: %v", err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO cart_items (id, cart_id, product_id, product_variant_id, quantity) VALUES ($1, $2, $3, $4, 1)", uuid.New(), cID, productB, variantB); err != nil {
		t.Fatalf("insert cart item B: %v", err)
	}

	// Add stock so inventory svc can reserve
	if _, err := db.Exec(ctx, "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 100, 0)", uuid.New(), productA, variantA, sellerA); err != nil {
		t.Fatalf("insert inventory A: %v", err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 100, 0)", uuid.New(), productB, variantB, sellerB); err != nil {
		t.Fatalf("insert inventory B: %v", err)
	}

	pgClient := &postgres.Client{Pool: db}
	
	cartRepo := cart.NewRepository(db)
	invRepo := inventory.NewRepository(db)
	// We pass nil for sellers.Repository since inventory doesn't strictly need it to run just reservations
	invSvc := inventory.NewService(invRepo, nil, pgClient)

	svc := NewService(repo, cartRepo, invSvc, pgClient)

	// Action: Create Order
	req := CreateOrderRequest{
		CustomerName: "Test",
	}
	order, err := svc.CreateOrder(ctx, buyer, req)
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// Assertions
	if len(order.Items) != 2 {
		t.Fatalf("expected 2 items")
	}

	// DB Check Fulfillments
	var fCount int
	err = db.QueryRow(ctx, "SELECT count(*) FROM order_fulfillments WHERE order_id = $1", order.ID).Scan(&fCount)
	if err != nil || fCount != 2 {
		t.Fatalf("expected 2 fulfillments, got %d", fCount)
	}

	// Verify links
	for _, item := range order.Items {
		if item.OrderFulfillmentID == nil {
			t.Fatalf("item missing fulfillment link")
		}
	}

	// Verify Sync
	err = pgClient.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := repo.MarkOrderFulfillmentsStatusTx(ctx, tx, order.ID, "awaiting_payment", "paid")
		return err
	})
	if err != nil {
		t.Fatalf("failed to mark paid: %v", err)
	}

	var paidCount int
	db.QueryRow(ctx, "SELECT count(*) FROM order_fulfillments WHERE order_id = $1 AND status = 'paid'", order.ID).Scan(&paidCount)
	if paidCount != 2 {
		t.Fatalf("expected 2 paid fulfillments, got %d", paidCount)
	}
}
