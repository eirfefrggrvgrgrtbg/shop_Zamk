package supplies_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
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
	Repo     *supplies.Repository
	Service  *supplies.Service
	SellerID uuid.UUID
	AdminID  uuid.UUID
	Variant1 uuid.UUID
	Variant2 uuid.UUID
	VariantOther uuid.UUID
}

func setupTestContext(t *testing.T) *TestContext {
	ctx := context.Background()
	repo := supplies.NewRepository(testDB)
	service := supplies.NewService(testDB, repo)

	// Clean up tables
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE stock_movements CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE inventory_items CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE supply_receiving_scans CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE supply_receiving_items CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE supply_receiving_sessions CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE seller_supply_box_items CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE seller_supply_boxes CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE seller_supply_items CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE seller_supplies CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE product_variants CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE products CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE sellers CASCADE")
	_, _ = testDB.Exec(ctx, "TRUNCATE TABLE users CASCADE")

	// Insert test data
	adminID := uuid.New()
	testDB.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at) VALUES ($1, 'Admin', 'admin@test.com', 'hash', 'admin', now(), now())", adminID)

	sellerUserID := uuid.New()
	testDB.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at) VALUES ($1, 'Seller', 'seller@test.com', 'hash', 'seller', now(), now())", sellerUserID)

	sellerID := uuid.New()
	testDB.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Test Company', 'test-brand', 'test@test.com', 'active', now(), now())", sellerID)
	testDB.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role, created_at) VALUES ($1, $2, $3, 'owner', now())", uuid.New(), sellerID, sellerUserID)

	otherSellerID := uuid.New()
	testDB.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Other Company', 'other-brand', 'other@test.com', 'active', now(), now())", otherSellerID)

	productID := uuid.New()
	testDB.Exec(ctx, "INSERT INTO products (id, seller_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, 'Test Product', 'test-slug', 100, 'published', now(), now())", productID, sellerID)
	
	otherProductID := uuid.New()
	testDB.Exec(ctx, "INSERT INTO products (id, seller_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, 'Other Product', 'other-slug', 200, 'published', now(), now())", otherProductID, otherSellerID)

	v1, v2, vOther := uuid.New(), uuid.New(), uuid.New()
	testDB.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'SKU1', 100, now(), now())", v1, productID)
	testDB.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'SKU2', 200, now(), now())", v2, productID)
	testDB.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, created_at, updated_at) VALUES ($1, $2, 'SKU3', 300, now(), now())", vOther, otherProductID)

	// Create inventory records to avoid foreign key issues during finalization
	testDB.Exec(ctx, "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock) VALUES ($1, $2, $3, $4, 0)", uuid.New(), productID, v1, sellerID)
	testDB.Exec(ctx, "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock) VALUES ($1, $2, $3, $4, 0)", uuid.New(), productID, v2, sellerID)

	return &TestContext{
		Ctx:          ctx,
		Repo:         repo,
		Service:      service,
		SellerID:     sellerID,
		AdminID:      adminID,
		Variant1:     v1,
		Variant2:     v2,
		VariantOther: vOther,
	}
}

func TestSupplyCreateOwnVariants(t *testing.T) {
	tc := setupTestContext(t)
	req := supplies.CreateSupplyRequest{
		HandoffMethod: "pvz",
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 10},
		},
		Boxes: []supplies.CreateSupplyBoxRequest{
			{BoxNumber: "1", Items: []supplies.CreateSupplyBoxItemRequest{{VariantID: tc.Variant1, Quantity: 10}}},
		},
	}
	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if supply.Status != "draft" {
		t.Fatalf("expected draft, got %s", supply.Status)
	}
}

func TestSupplyRejectCrossSellerVariant(t *testing.T) {
	tc := setupTestContext(t)
	req := supplies.CreateSupplyRequest{
		HandoffMethod: "pvz",
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.VariantOther, ExpectedQuantity: 10},
		},
		Boxes: []supplies.CreateSupplyBoxRequest{
			{BoxNumber: "1", Items: []supplies.CreateSupplyBoxItemRequest{{VariantID: tc.VariantOther, Quantity: 10}}},
		},
	}
	_, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != supplies.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestSupplyBoxQuantities(t *testing.T) {
	tc := setupTestContext(t)
	req := supplies.CreateSupplyRequest{
		HandoffMethod: "pvz",
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 20},
		},
		Boxes: []supplies.CreateSupplyBoxRequest{
			{BoxNumber: "1", Items: []supplies.CreateSupplyBoxItemRequest{{VariantID: tc.Variant1, Quantity: 10}}},
			{BoxNumber: "2", Items: []supplies.CreateSupplyBoxItemRequest{{VariantID: tc.Variant1, Quantity: 15}}}, // Mismatch! 25 total vs 20 expected
		},
	}
	_, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != supplies.ErrInvalidQuantities {
		t.Fatalf("expected ErrInvalidQuantities, got %v", err)
	}
}

func TestSupplyTransitionValidation(t *testing.T) {
	tc := setupTestContext(t)
	req := supplies.CreateSupplyRequest{
		HandoffMethod: "pvz",
		Items: []supplies.CreateSupplyItemRequest{
			{VariantID: tc.Variant1, ExpectedQuantity: 10},
		},
		Boxes: []supplies.CreateSupplyBoxRequest{
			{BoxNumber: "1", Items: []supplies.CreateSupplyBoxItemRequest{{VariantID: tc.Variant1, Quantity: 10}}},
		},
	}
	supply, err := tc.Service.CreateSupply(tc.Ctx, tc.SellerID, req)
	if err != nil {
		t.Fatalf("failed to create supply: %v", err)
	}
	
	err = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supply.ID)
	if err != nil {
		t.Fatalf("expected nil err on MarkShipped, got %v", err)
	}
	
	// Double mark shipped should fail (invalid status transition)
	err = tc.Service.MarkShipped(tc.Ctx, tc.SellerID, supply.ID)
	if err != supplies.ErrInvalidStatus {
		t.Fatalf("expected ErrInvalidStatus on double mark shipped, got %v", err)
	}
}
