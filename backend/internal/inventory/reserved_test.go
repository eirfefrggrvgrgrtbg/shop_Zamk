package inventory_test

import (
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/google/uuid"
)

func TestReservedStockCalculation(t *testing.T) {
	tc := setupTestContext(t)
	
	// Create Product & Variant
	productID := uuid.New()
	variantID := uuid.New()
	_, err := testDB.Exec(tc.Ctx, "INSERT INTO products (id, title, slug, price_cents, status, seller_id) VALUES ($1, 'Res Product', 'res-product', 1000, 'published', $2)", productID, tc.SellerID)
	if err != nil {
		t.Fatalf("failed to insert product: %v", err)
	}
	_, err = testDB.Exec(tc.Ctx, "INSERT INTO product_variants (id, product_id, sku, is_active) VALUES ($1, $2, 'RES123', true)", variantID, productID)
	if err != nil {
		t.Fatalf("failed to insert product variant: %v", err)
	}

	// Create Inventory Item with 20 Total Stock
	itemID := uuid.New()
	item := &inventory.Item{
		ID:               itemID,
		ProductID:        productID,
		ProductVariantID: variantID,
		SellerID:         tc.SellerID,
		TotalStock:       20,
		ReservedStock:    0,
	}
	err = tc.Repo.CreateItem(tc.Ctx, item)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Reserve 4 units
	customerID := uuid.New()
	_, _ = testDB.Exec(tc.Ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'customer', 'Cust')", customerID, "cust@example.com")
	
	_, err = tc.Service.CreateReservation(tc.Ctx, customerID, variantID, 4, 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to create reservation: %v", err)
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
	if inv.OnHand != 20 {
		t.Errorf("expected OnHand 20, got %d", inv.OnHand)
	}
	if inv.Reserved != 4 {
		t.Errorf("expected Reserved 4, got %d", inv.Reserved)
	}
	if inv.Available != 16 {
		t.Errorf("expected Available 16, got %d", inv.Available)
	}
}
