package inventory_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		fmt.Println("TEST_DATABASE_URL not set, skipping integration tests")
		os.Exit(0)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	testDB = pool
	defer pool.Close()

	os.Exit(m.Run())
}

type TestContext struct {
	Ctx      context.Context
	Repo     *inventory.Repository
	Service  *inventory.Service
	SellerID uuid.UUID
}

func setupTestContext(t *testing.T) *TestContext {
	ctx := context.Background()
	repo := inventory.NewRepository(testDB)
	sellersRepo := sellers.NewRepository(testDB)
	pgClient, err := postgres.NewClient(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("failed to create pgClient: %v", err)
	}
	service := inventory.NewService(repo, sellersRepo, pgClient)

	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE stock_movements CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE inventory_items CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE seller_supply_items CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE seller_supplies CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE sellers CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE users CASCADE")

	// Create seller
	sellerID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'seller', 'Seller')", sellerID, "seller_inv@example.com")
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Test Seller', 'test-seller', 'contact@example.com', 'active')", sellerID)
	if err != nil {
		t.Fatalf("failed to insert seller: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerID, sellerID)

	return &TestContext{
		Ctx:      ctx,
		Repo:     repo,
		Service:  service,
		SellerID: sellerID,
	}
}

func TestListSellerInventory(t *testing.T) {
	tc := setupTestContext(t)

	// Create Product & Variant
	productID := uuid.New()
	variantID := uuid.New()
	_, err := testDB.Exec(tc.Ctx, "INSERT INTO products (id, title, slug, price_cents, status, seller_id) VALUES ($1, 'Test Product', 'test-product', 1000, 'published', $2)", productID, tc.SellerID)
	if err != nil {
		t.Fatalf("failed to insert product: %v", err)
	}
	_, err = testDB.Exec(tc.Ctx, "INSERT INTO product_variants (id, product_id, sku, is_active) VALUES ($1, $2, 'SKU123', true)", variantID, productID)
	if err != nil {
		t.Fatalf("failed to insert variant: %v", err)
	}

	// Create Inventory Item
	itemID := uuid.New()
	item := &inventory.Item{
		ID:               itemID,
		ProductID:        productID,
		ProductVariantID: variantID,
		SellerID:         tc.SellerID,
		TotalStock:       15,
		ReservedStock:    5,
	}
	err = tc.Repo.CreateItem(tc.Ctx, item)
	if err != nil {
		t.Fatalf("failed to create inventory item: %v", err)
	}

	// Create Inbound Supply
	supplyID := uuid.New()
	_, err = testDB.Exec(tc.Ctx, "INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, qr_token, created_at, updated_at) VALUES ($1, $2, 'ready_to_ship', 'SUP-123', 'pvz', 'token', now(), now())", supplyID, tc.SellerID)
	if err != nil {
		t.Fatalf("failed to insert supply: %v", err)
	}
	supplyItemID := uuid.New()
	_, err = testDB.Exec(tc.Ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, created_at, updated_at) VALUES ($1, $2, $3, 20, 0, now(), now())", supplyItemID, supplyID, variantID)
	if err != nil {
		t.Fatalf("failed to insert supply item: %v", err)
	}

	// Fetch Seller Inventory
	res, err := tc.Service.ListSellerInventory(tc.Ctx, tc.SellerID, 10, 0)
	if err != nil {
		t.Fatalf("failed to list inventory: %v", err)
	}

	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}

	inv := res.Items[0]
	if inv.OnHand != 15 {
		t.Errorf("expected OnHand 15, got %d", inv.OnHand)
	}
	if inv.Reserved != 5 {
		t.Errorf("expected Reserved 5, got %d", inv.Reserved)
	}
	if inv.Available != 10 {
		t.Errorf("expected Available 10, got %d", inv.Available)
	}
	if inv.Inbound != 20 {
		t.Errorf("expected Inbound 20, got %d", inv.Inbound)
	}
	if inv.AvailabilityStatus != "Заканчивается" {
		t.Errorf("expected status 'Заканчивается', got '%s'", inv.AvailabilityStatus)
	}
}
