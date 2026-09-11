package inventory_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	dbURL := testutil.GetTestDatabaseURL()

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
	testutil.AssertTestDatabase(t, testDB)

	ctx := context.Background()
	repo := inventory.NewRepository(testDB)
	sellersRepo := sellers.NewRepository(testDB)
	pgClient, err := postgres.NewClient(ctx, testutil.GetTestDatabaseURL())
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

func TestListSellerInventory_ForecastSnapshot(t *testing.T) {
	ctx := context.Background()

	var dbName string
	err := testDB.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	if err != nil || dbName != "zamk_test" {
		t.Fatalf("database safety guard failed: must be exactly zamk_test, got %s (err: %v)", dbName, err)
	}

	var (
		createdUserIDs    []uuid.UUID
		createdSellerIDs  []uuid.UUID
		createdProductIDs []uuid.UUID
		createdVariantIDs []uuid.UUID
	)

	t.Cleanup(func() {
		if len(createdVariantIDs) > 0 {
			_, _ = testDB.Exec(ctx, "DELETE FROM variant_stock_forecasts WHERE product_variant_id = ANY($1)", createdVariantIDs)
			_, _ = testDB.Exec(ctx, "DELETE FROM inventory_items WHERE product_variant_id = ANY($1)", createdVariantIDs)
			_, _ = testDB.Exec(ctx, "DELETE FROM product_variants WHERE id = ANY($1)", createdVariantIDs)
		}
		if len(createdProductIDs) > 0 {
			_, _ = testDB.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", createdProductIDs)
		}
		if len(createdSellerIDs) > 0 {
			_, _ = testDB.Exec(ctx, "DELETE FROM seller_users WHERE seller_id = ANY($1)", createdSellerIDs)
			_, _ = testDB.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", createdSellerIDs)
		}
		if len(createdUserIDs) > 0 {
			_, _ = testDB.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", createdUserIDs)
		}
	})

	repo := inventory.NewRepository(testDB)
	sellersRepo := sellers.NewRepository(testDB)
	pgClient, err := postgres.NewClient(ctx, testutil.GetTestDatabaseURL())
	if err != nil {
		t.Fatalf("failed to create pgClient: %v", err)
	}
	defer pgClient.Close()
	service := inventory.NewService(repo, sellersRepo, pgClient)

	// Seller A
	userAID := uuid.New()
	sellerAID := uuid.New()
	createdUserIDs = append(createdUserIDs, userAID)
	createdSellerIDs = append(createdSellerIDs, sellerAID)

	_, err = testDB.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'seller', 'Seller A')", userAID, "seller_a@example.com")
	if err != nil {
		t.Fatalf("failed to insert user A: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Brand A', $2, 'a@example.com', 'active')", sellerAID, "brand-a-"+sellerAID.String()[:8])
	if err != nil {
		t.Fatalf("failed to insert seller A: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerAID, userAID)
	if err != nil {
		t.Fatalf("failed to insert seller user A: %v", err)
	}

	// Product A with Variant A1 (with forecast) and Variant A2 (without forecast)
	productAID := uuid.New()
	variantA1ID := uuid.New()
	variantA2ID := uuid.New()
	createdProductIDs = append(createdProductIDs, productAID)
	createdVariantIDs = append(createdVariantIDs, variantA1ID, variantA2ID)

	_, err = testDB.Exec(ctx, "INSERT INTO products (id, title, slug, price_cents, status, seller_id) VALUES ($1, 'Product A', $2, 1000, 'published', $3)", productAID, "prod-a-"+productAID.String()[:8], sellerAID)
	if err != nil {
		t.Fatalf("failed to insert product A: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, is_active) VALUES ($1, $2, 'SKU-A1', true)", variantA1ID, productAID)
	if err != nil {
		t.Fatalf("failed to insert variant A1: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, is_active) VALUES ($1, $2, 'SKU-A2', true)", variantA2ID, productAID)
	if err != nil {
		t.Fatalf("failed to insert variant A2: %v", err)
	}

	// Inventory for A1 and A2
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 10, 2)", uuid.New(), productAID, variantA1ID, sellerAID)
	if err != nil {
		t.Fatalf("failed to insert inventory A1: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 15, 0)", uuid.New(), productAID, variantA2ID, sellerAID)
	if err != nil {
		t.Fatalf("failed to insert inventory A2: %v", err)
	}

	// Insert forecast snapshot for Variant A1
	nowTime := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	_, err = testDB.Exec(ctx, `
		INSERT INTO variant_stock_forecasts (product_variant_id, state, days_of_cover, calculated_at)
		VALUES ($1, 'warning', 9.1, $2)
	`, variantA1ID, nowTime)
	if err != nil {
		t.Fatalf("failed to insert forecast snapshot A1: %v", err)
	}

	// Seller B (Isolation check)
	userBID := uuid.New()
	sellerBID := uuid.New()
	productBID := uuid.New()
	variantB1ID := uuid.New()
	createdUserIDs = append(createdUserIDs, userBID)
	createdSellerIDs = append(createdSellerIDs, sellerBID)
	createdProductIDs = append(createdProductIDs, productBID)
	createdVariantIDs = append(createdVariantIDs, variantB1ID)

	_, err = testDB.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'seller', 'Seller B')", userBID, "seller_b@example.com")
	if err != nil {
		t.Fatalf("failed to insert user B: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Brand B', $2, 'b@example.com', 'active')", sellerBID, "brand-b-"+sellerBID.String()[:8])
	if err != nil {
		t.Fatalf("failed to insert seller B: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerBID, userBID)
	if err != nil {
		t.Fatalf("failed to insert seller user B: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO products (id, title, slug, price_cents, status, seller_id) VALUES ($1, 'Product B', $2, 1000, 'published', $3)", productBID, "prod-b-"+productBID.String()[:8], sellerBID)
	if err != nil {
		t.Fatalf("failed to insert product B: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, is_active) VALUES ($1, $2, 'SKU-B1', true)", variantB1ID, productBID)
	if err != nil {
		t.Fatalf("failed to insert variant B1: %v", err)
	}
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 20, 0)", uuid.New(), productBID, variantB1ID, sellerBID)
	if err != nil {
		t.Fatalf("failed to insert inventory B1: %v", err)
	}
	_, err = testDB.Exec(ctx, `
		INSERT INTO variant_stock_forecasts (product_variant_id, state, days_of_cover, calculated_at)
		VALUES ($1, 'critical', 3.5, $2)
	`, variantB1ID, nowTime)
	if err != nil {
		t.Fatalf("failed to insert forecast snapshot B1: %v", err)
	}

	// Fetch Seller A's Inventory
	res, err := service.ListSellerInventory(ctx, userAID, 50, 0)
	if err != nil {
		t.Fatalf("failed to list seller inventory: %v", err)
	}

	// 1. Check item count (exactly 2 for Seller A)
	if len(res.Items) != 2 {
		t.Fatalf("expected 2 items for Seller A, got %d", len(res.Items))
	}

	var itemA1, itemA2 *inventory.SellerInventoryItem
	for i := range res.Items {
		item := &res.Items[i]
		if item.VariantID == variantA1ID {
			itemA1 = item
		} else if item.VariantID == variantA2ID {
			itemA2 = item
		} else if item.VariantID == variantB1ID {
			t.Fatalf("Seller B's variant %s appeared in Seller A's inventory (SELLER ISOLATION BROKEN)", variantB1ID)
		}
	}

	if itemA1 == nil {
		t.Fatalf("Variant A1 was not returned in inventory")
	}
	if itemA2 == nil {
		t.Fatalf("Variant A2 was not returned in inventory")
	}

	// 2. Check Variant A1 forecast snapshot
	if itemA1.Forecast == nil {
		t.Fatalf("expected forecast for Variant A1, got nil")
	}
	if itemA1.Forecast.State != "warning" {
		t.Errorf("expected forecast.state 'warning', got '%s'", itemA1.Forecast.State)
	}
	if itemA1.Forecast.DaysOfCover == nil || *itemA1.Forecast.DaysOfCover != 9.1 {
		t.Errorf("expected forecast.daysOfCover 9.1, got %v", itemA1.Forecast.DaysOfCover)
	}
	if !itemA1.Forecast.CalculatedAt.Equal(nowTime) {
		t.Errorf("expected forecast.calculatedAt %v, got %v", nowTime, itemA1.Forecast.CalculatedAt)
	}

	// 3. Check Variant A2 forecast is safely nil
	if itemA2.Forecast != nil {
		t.Errorf("expected forecast for Variant A2 to be nil, got %+v", itemA2.Forecast)
	}
}
