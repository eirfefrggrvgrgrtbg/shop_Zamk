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

	// Insert mock products
	db.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, created_at, updated_at) VALUES ($1, 'sA@ex.com', 'hash', 'seller', now(), now())", sellerA)
	db.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, created_at, updated_at) VALUES ($1, 'sB@ex.com', 'hash', 'seller', now(), now())", sellerB)
	
	db.Exec(ctx, "INSERT INTO sellers (id, brand_name, legal_name, inn, kpp, ogrn, legal_address, status) VALUES ($1, 'Brand A', 'Legal A', '1', '1', '1', '1', 'approved')", sellerA)
	db.Exec(ctx, "INSERT INTO sellers (id, brand_name, legal_name, inn, kpp, ogrn, legal_address, status) VALUES ($1, 'Brand B', 'Legal B', '2', '2', '2', '2', 'approved')", sellerB)
	
	catID := uuid.New()
	brandID := uuid.New()
	db.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'cat', 'cat')", catID)
	db.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, 'br', 'br')", brandID)

	db.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status) VALUES ($1, $2, $3, $4, 'P A', 'p-a', 'desc', 'published')", productA, sellerA, catID, brandID)
	db.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status) VALUES ($1, $2, $3, $4, 'P B', 'p-b', 'desc', 'published')", productB, sellerB, catID, brandID)

	db.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, compare_at_price_cents, is_active) VALUES ($1, $2, 'skuA', 100000, 100000, true)", variantA, productA)
	db.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, compare_at_price_cents, is_active) VALUES ($1, $2, 'skuB', 200000, 200000, true)", variantB, productB)

	// Mock cart directly into db so real cartRepo can find it
	cID := uuid.New()
	db.Exec(ctx, "INSERT INTO carts (id, user_id, status) VALUES ($1, $2, 'active')", cID, buyer)
	db.Exec(ctx, "INSERT INTO cart_items (cart_id, product_id, product_variant_id, quantity) VALUES ($1, $2, $3, 2)", cID, productA, variantA)
	db.Exec(ctx, "INSERT INTO cart_items (cart_id, product_id, product_variant_id, quantity) VALUES ($1, $2, $3, 1)", cID, productB, variantB)

	// Add stock so inventory svc can reserve
	db.Exec(ctx, "INSERT INTO inventory_records (id, product_variant_id, stock_available, stock_reserved) VALUES ($1, $2, 100, 0)", uuid.New(), variantA)
	db.Exec(ctx, "INSERT INTO inventory_records (id, product_variant_id, stock_available, stock_reserved) VALUES ($1, $2, 100, 0)", uuid.New(), variantB)

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
		return repo.MarkOrderFulfillmentsStatusTx(ctx, tx, order.ID, "awaiting_payment", "paid")
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
