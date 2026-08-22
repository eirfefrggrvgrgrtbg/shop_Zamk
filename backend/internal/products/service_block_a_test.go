package products_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
)

func setupBlockATestDB(t *testing.T) (*postgres.Client, *products.Service, uuid.UUID) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	db, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)

	var dbName string
	err = db.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	if !strings.Contains(dbName, "zamk_test") {
		t.Fatalf("Refusing to run tests against non-test database: %s", dbName)
	}

	repo := products.NewRepository(db.Pool)
	sellerRepo := sellers.NewRepository(db.Pool)
	svc := products.NewService(repo, sellerRepo, db, nil, nil)

	userID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'seller', 'Test User')", userID, fmt.Sprintf("test-%s@test.com", userID))
	require.NoError(t, err)

	sellerID := uuid.New()
	slug := fmt.Sprintf("test-brand-%s", userID)
	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Test Brand', $2, 'test@test.com', 'active')", sellerID, slug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerID, userID)
	require.NoError(t, err)

	brandID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, $2, $3)", brandID, "Test Brand Name", slug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO seller_brands (id, seller_id, brand_id, is_primary, relationship_type, status) VALUES ($1, $2, $3, true, 'owner', 'active')", uuid.New(), sellerID, brandID)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Pool.Exec(context.Background(), "DELETE FROM product_revisions WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID)
		db.Pool.Exec(context.Background(), "DELETE FROM product_material_composition WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID)
		db.Pool.Exec(context.Background(), "DELETE FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID)
		db.Pool.Exec(context.Background(), "DELETE FROM products WHERE seller_id = $1", sellerID)
		db.Pool.Exec(context.Background(), "DELETE FROM seller_brands WHERE seller_id = $1", sellerID)
		db.Pool.Exec(context.Background(), "DELETE FROM brands WHERE id = $1", brandID)
		db.Pool.Exec(context.Background(), "DELETE FROM seller_users WHERE user_id = $1", userID)
		db.Pool.Exec(context.Background(), "DELETE FROM sellers WHERE id = $1", sellerID)
		db.Pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", userID)
	})

	return db, svc, userID
}

func setupNoBrandSeller(t *testing.T, db *postgres.Client) uuid.UUID {
    ctx := context.Background()
    userID := uuid.New()
	_, err := db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'seller', 'Test User 2')", userID, fmt.Sprintf("test-%s@test.com", userID))
	require.NoError(t, err)

	sellerID := uuid.New()
	slug := fmt.Sprintf("test-nobrand-%s", userID)
	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Test NoBrand', $2, 'test@test.com', 'active')", sellerID, slug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerID, userID)
	require.NoError(t, err)
    
    t.Cleanup(func() {
        db.Pool.Exec(ctx, "DELETE FROM seller_users WHERE user_id = $1", userID)
        db.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
        db.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
    })
    return userID
}

func TestBlockAPrimaryBrandRequired(t *testing.T) {
	db, svc, _ := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

    noBrandUserID := setupNoBrandSeller(t, db)

	req := products.CreateProductRequest{
		Title: "No Brand Product " + uuid.New().String(),
	}

	_, err := svc.CreateProductForSeller(ctx, noBrandUserID, req)
	if err == nil { t.Log("Barcode in DB:")
var b *string
db.Pool.QueryRow(ctx, "SELECT barcode FROM product_variants WHERE seller_sku = 'DEF-457'").Scan(&b)
t.Logf("DB Barcode is: %v", b) }
require.Error(t, err)
	assert.Contains(t, err.Error(), "no active primary brand")
}

func TestBlockAProductCreationNoInitialStock(t *testing.T) {
    db, svc, userID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

    req := products.CreateProductRequest{
		Title: "Test Stock Product " + uuid.New().String(),
        Variants: []products.ProductVariantRequest{
            { SKU: ptr("STK-1"), PriceCents: func() *int64 { v := int64(1000); return &v }(), InitialStock: func() *int { v := 100; return &v }() },
        },
	}
    p, err := svc.CreateProductForSeller(ctx, userID, req)
    require.NoError(t, err)
    
    var count int
    err = db.Pool.QueryRow(ctx, "SELECT count(*) FROM inventory_items WHERE product_id = $1", p.ID).Scan(&count)
    require.NoError(t, err)
    assert.Equal(t, 0, count, "Product creation must NOT seed stock in V1")
}

func TestBlockAMaterialCompositionValidation(t *testing.T) {
    db, svc, userID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()
    
    var materialID uuid.UUID
    err := db.Pool.QueryRow(ctx, "SELECT id FROM materials WHERE code = 'COTTON'").Scan(&materialID)
    require.NoError(t, err)

    req := products.CreateProductRequest{
		Title: "Composition Product " + uuid.New().String(),
        MaterialComposition: []products.ProductMaterialCompositionRequest{
            {MaterialID: materialID, Percentage: 50.0},
        },
	}
    _, err = svc.CreateProductForSeller(ctx, userID, req)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "must sum to 100")
    
    req.MaterialComposition[0].Percentage = 100.0
    _, err = svc.CreateProductForSeller(ctx, userID, req)
    require.NoError(t, err)
}

func TestBlockACanonicalVariantFields(t *testing.T) {
    db, svc, userID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()
    
    var colorID uuid.UUID
    err := db.Pool.QueryRow(ctx, "SELECT id FROM colors WHERE code = 'BLACK'").Scan(&colorID)
    require.NoError(t, err)
    
    var sizeID uuid.UUID
    err = db.Pool.QueryRow(ctx, "SELECT id FROM size_values WHERE value = 'M' LIMIT 1").Scan(&sizeID)
    require.NoError(t, err)

    req := products.CreateProductRequest{
		Title: "Canonical Product " + uuid.New().String(),
        Variants: []products.ProductVariantRequest{
            { 
                SellerSKU: ptr("CAN-1"), 
                ColorID: &colorID, 
                SizeValueID: &sizeID, 
                ShadeName: ptr("Dark Black"), 
                PriceCents: func() *int64 { v := int64(2000); return &v }(),
            },
        },
	}
    p, err := svc.CreateProductForSeller(ctx, userID, req)
    require.NoError(t, err)
    require.Len(t, p.Variants, 1)
    
    v := p.Variants[0]
    assert.Equal(t, "CAN-1", *v.SellerSKU)
    assert.Equal(t, colorID, *v.ColorID)
    assert.Equal(t, sizeID, *v.SizeValueID)
    assert.Equal(t, "Dark Black", *v.ShadeName)
    assert.NotNil(t, v.Barcode, "Server generated internal barcode")
    assert.True(t, strings.HasPrefix(*v.Barcode, "ZMK-"))
    
    // Check DB
    var dbSku, dbShade string
    err = db.Pool.QueryRow(ctx, "SELECT seller_sku, shade_name FROM product_variants WHERE id = $1", v.ID).Scan(&dbSku, &dbShade)
    require.NoError(t, err)
    assert.Equal(t, "CAN-1", dbSku)
    assert.Equal(t, "Dark Black", dbShade)
}

func TestBlockAUpdatePriceOnly(t *testing.T) {
    db, svc, userID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

    req := products.CreateProductRequest{
		Title: "Price Update Product " + uuid.New().String(),
        Variants: []products.ProductVariantRequest{
            { SellerSKU: ptr("PRC-1"), PriceCents: func() *int64 { v := int64(1000); return &v }() },
        },
	}
    p, err := svc.CreateProductForSeller(ctx, userID, req)
    require.NoError(t, err)
    
    // Admin approves it
    _, err = db.Pool.Exec(ctx, "UPDATE products SET status = 'published' WHERE id = $1", p.ID)
    require.NoError(t, err)
    
    // Update price only
    priceReq := products.UpdateVariantPricesRequest{
        Prices: map[uuid.UUID]int64{
            p.Variants[0].ID: 2500,
        },
    }
    
    p2, err := svc.UpdateVariantPricesForSeller(ctx, userID, p.ID, priceReq)
    require.NoError(t, err)
    assert.Equal(t, "published", p2.Status, "Product must remain published without moderation reset")
    assert.Equal(t, int64(2500), *p2.Variants[0].PriceCents)
    
    var revCount int
    db.Pool.QueryRow(ctx, "SELECT count(*) FROM product_revisions WHERE product_id = $1", p.ID).Scan(&revCount)
    assert.Equal(t, 0, revCount, "No revision should be created for price-only update")
}

func TestBlockARevisionFoundation(t *testing.T) {
    db, svc, userID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

    req := products.CreateProductRequest{
		Title: "Revision Product " + uuid.New().String(),
	}
    p, err := svc.CreateProductForSeller(ctx, userID, req)
    require.NoError(t, err)
    
    _, err = db.Pool.Exec(ctx, "UPDATE products SET status = 'published' WHERE id = $1", p.ID)
    require.NoError(t, err)
    
    trueVal := true
    updReq := products.UpdateProductRequest{
        Title: ptr("Updated Title"),
        ContinueSelling: &trueVal,
    }
    
    p2, err := svc.UpdateProductForSeller(ctx, userID, p.ID, updReq)
    require.NoError(t, err)
    
    // Live product remains published
    assert.Equal(t, "published", p2.Status)
    assert.NotNil(t, p2.LiveRevisionID)
    
    // DB check: Live product title should NOT change
    var liveTitle string
    err = db.Pool.QueryRow(ctx, "SELECT title FROM products WHERE id = $1", p.ID).Scan(&liveTitle)
    require.NoError(t, err)
    assert.True(t, strings.HasPrefix(liveTitle, "Revision Product"), "Live product title must NOT change")
    
    // DB check: Revision should exist and contain the updated title
    var contentSnapshot map[string]interface{}
    err = db.Pool.QueryRow(ctx, "SELECT content_snapshot FROM product_revisions WHERE id = $1", *p2.LiveRevisionID).Scan(&contentSnapshot)
    require.NoError(t, err)
    assert.Equal(t, "Updated Title", contentSnapshot["title"], "Revision must contain the proposed changes")
}

func TestBlockATemporaryUnpublish(t *testing.T) {
    db, svc, userID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

    req := products.CreateProductRequest{
		Title: "Unpublish Product " + uuid.New().String(),
	}
    p, err := svc.CreateProductForSeller(ctx, userID, req)
    require.NoError(t, err)
    
    _, err = db.Pool.Exec(ctx, "UPDATE products SET status = 'published' WHERE id = $1", p.ID)
    require.NoError(t, err)
    
    falseVal := false
    updReq := products.UpdateProductRequest{
        Title: ptr("Hidden Title"),
        ContinueSelling: &falseVal,
    }
    
    p2, err := svc.UpdateProductForSeller(ctx, userID, p.ID, updReq)
    require.NoError(t, err)
    
    assert.Equal(t, "pending_moderation", p2.Status)
    
    // Live title remains unchanged in DB even though it's hidden, or at least the logic resets status
    // Wait, the current logic for ContinueSelling=false actually calls UpdateProduct!
    // If it calls UpdateProduct, it OVERWRITES live content. The user said: "Do NOT overwrite live content with unmoderated content."
    // Let me check my implementation. I implemented:
    // } else {
    //    txRepo.UpdateProduct(ctx, p)
    // }
    // Oh! If `ContinueSelling=false`, it overwrites the DB directly, violating "Do NOT overwrite live content with unmoderated content."
    
    var hiddenTitle string
    err = db.Pool.QueryRow(ctx, "SELECT title FROM products WHERE id = $1", p.ID).Scan(&hiddenTitle)
    require.NoError(t, err)
    assert.True(t, strings.HasPrefix(hiddenTitle, "Unpublish Product"), "Live approved content must be preserved even if hidden")
}

func TestBlockABarcodeAndSKUUniqueness(t *testing.T) {
    db, svc, userID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

    // Same Seller SKU -> conflict
    req1 := products.CreateProductRequest{
		Title: "Prod 1 " + uuid.New().String(),
        Variants: []products.ProductVariantRequest{
            { SellerSKU: ptr("ABC-123") },
        },
	}
    _, err := svc.CreateProductForSeller(ctx, userID, req1)
    require.NoError(t, err)

    req2 := products.CreateProductRequest{
		Title: "Prod 2 " + uuid.New().String(),
        Variants: []products.ProductVariantRequest{
            { SellerSKU: ptr("abc-123") },
        },
	}
    _, err = svc.CreateProductForSeller(ctx, userID, req2)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "exists")

    // Barcode globally unique
	bcode := "BARCODE-" + uuid.New().String()
    req3 := products.CreateProductRequest{
		Title: "Prod 3 " + uuid.New().String(),
        Variants: []products.ProductVariantRequest{
            { SellerSKU: ptr("DEF-456"), Barcode: &bcode },
        },
	}
    _, err = svc.CreateProductForSeller(ctx, userID, req3)
    require.NoError(t, err)

    req4 := products.CreateProductRequest{
		Title: "Prod 4 " + uuid.New().String(),
        Variants: []products.ProductVariantRequest{
            { SellerSKU: ptr("DEF-457"), Barcode: &bcode },
        },
	}
    _, err = svc.CreateProductForSeller(ctx, userID, req4)
    require.Error(t, err)
}

func TestBlockASizeChartValidation(t *testing.T) {
    db, svc, userID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()
    
    // Create a category requiring size chart
    catID := uuid.New()
    _, err := db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Shoes', $2, true)\", catID, \"shoes-\" + uuid.New().String())", catID)
	_, err = db.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Shoes', $2, true)", catID, "shoes-" + uuid.New().String())

    
    _, err = db.Pool.Exec(ctx, "INSERT INTO category_size_chart_fields (id, category_id, code, name, is_required) VALUES ($1, $2, 'length_cm', 'Length CM', true)", uuid.New(), catID)
    require.NoError(t, err)
    
    req := products.CreateProductRequest{
		Title: "Shoe Prod " + uuid.New().String(),
        CategoryID: &catID,
	}
    _, err = svc.CreateProductForSeller(ctx, userID, req)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "category requires a size chart")
}
