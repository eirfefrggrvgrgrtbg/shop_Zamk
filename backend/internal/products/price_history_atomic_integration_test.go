package products_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func setupPriceHistoryTestDB(t *testing.T) (*postgres.Client, *products.Service, uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	require.True(t, strings.Contains(dsn, "zamk_test"), "MUST run only against zamk_test")

	db, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)

	testutil.AssertTestDatabase(t, db.Pool)

	// Ensure migration 96 is applied if table doesn't exist
	var exists bool
	_ = db.Pool.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'product_price_history')").Scan(&exists)
	if !exists {
		root := findRepoRoot()
		require.NotEmpty(t, root)
		mig96Path := filepath.Join(root, "migrations", "000096_create_product_price_history.up.sql")
		content, err := os.ReadFile(mig96Path)
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, string(content))
		require.NoError(t, err)
	}

	repo := products.NewRepository(db.Pool)
	sellerRepo := sellers.NewRepository(db.Pool)
	svc := products.NewService(repo, sellerRepo, db, nil, nil)

	userID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'seller', 'Price History Test User')", userID, fmt.Sprintf("test-%s@test.com", userID))
	require.NoError(t, err)

	sellerID := uuid.New()
	slug := fmt.Sprintf("test-brand-%s", userID.String()[:8])
	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Test Brand', $2, 'test@test.com', 'active')", sellerID, slug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerID, userID)
	require.NoError(t, err)

	return db, svc, sellerID, userID
}

func cleanupPriceHistoryFixtures(client *postgres.Client, sellerID uuid.UUID, userID uuid.UUID, productIDs []uuid.UUID) {
	ctx := context.Background()
	for _, pid := range productIDs {
		client.Pool.Exec(ctx, "DELETE FROM product_price_history WHERE product_id = $1", pid)
		client.Pool.Exec(ctx, "DELETE FROM product_variants WHERE product_id = $1", pid)
		client.Pool.Exec(ctx, "DELETE FROM products WHERE id = $1", pid)
	}
	client.Pool.Exec(ctx, "DELETE FROM seller_users WHERE user_id = $1", userID)
	client.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
	client.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
}

func TestPriceHistory_PriceChangeCreatesExactlyOneRecord(t *testing.T) {
	client, svc, sellerID, sellerUserID := setupPriceHistoryTestDB(t)
	var productIDs []uuid.UUID

	defer func() {
		cleanupPriceHistoryFixtures(client, sellerID, sellerUserID, productIDs)
		client.Close()
	}()

	ctx := context.Background()

	// 1. Create a published product with 1 variant priced at 1000 cents
	productID := uuid.New()
	variantID := uuid.New()
	productIDs = append(productIDs, productID)

	_, err := client.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, price_cents, status)
		VALUES ($1, $2, 'Audited T-Shirt', $3, 1000, 'published')
	`, productID, sellerID, fmt.Sprintf("audited-tshirt-%s", productID.String()[:8]))
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents, is_active)
		VALUES ($1, $2, $3, 1000, true)
	`, variantID, productID, fmt.Sprintf("SKU-AUDIT-%s", variantID.String()[:8]))
	require.NoError(t, err)

	// Verify no history exists initially
	var initialCount int
	err = client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_price_history WHERE product_id = $1", productID).Scan(&initialCount)
	require.NoError(t, err)
	assert.Equal(t, 0, initialCount, "Product creation must not backfill fake history")

	// 2. Perform price change: 1000 cents -> 1500 cents via UpdateProductPrices
	req := products.UpdateProductPricesRequest{
		Variants: []products.VariantPriceUpdateRequest{
			{ID: variantID, PriceCents: 1500},
		},
	}
	err = svc.UpdateProductPrices(ctx, sellerUserID, productID, req)
	require.NoError(t, err)

	// Verify variant price changed
	var vPrice int64
	err = client.Pool.QueryRow(ctx, "SELECT price_cents FROM product_variants WHERE id = $1", variantID).Scan(&vPrice)
	require.NoError(t, err)
	assert.Equal(t, int64(1500), vPrice)

	// Verify product price changed
	var pPrice int64
	err = client.Pool.QueryRow(ctx, "SELECT price_cents FROM products WHERE id = $1", productID).Scan(&pPrice)
	require.NoError(t, err)
	assert.Equal(t, int64(1500), pPrice)

	// 3. Verify exactly ONE variant history record exists with truthful actor and source
	var varHistCount int
	var oldPrice, newPrice int64
	var changedBy uuid.UUID
	var source string
	err = client.Pool.QueryRow(ctx, `
		SELECT COUNT(*), old_price_cents, new_price_cents, changed_by_user_id, source
		FROM product_price_history
		WHERE product_variant_id = $1
		GROUP BY old_price_cents, new_price_cents, changed_by_user_id, source
	`, variantID).Scan(&varHistCount, &oldPrice, &newPrice, &changedBy, &source)
	require.NoError(t, err)

	assert.Equal(t, 1, varHistCount, "price A -> price B must create exactly one variant history event")
	assert.Equal(t, int64(1000), oldPrice)
	assert.Equal(t, int64(1500), newPrice)
	assert.Equal(t, sellerUserID, changedBy, "Actor identity must be truthful seller user ID")
	assert.Equal(t, "seller", source)

	// 4. Verify exactly ONE product history record exists for the minimum price sync
	var prodHistCount int
	err = client.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM product_price_history
		WHERE product_id = $1 AND product_variant_id IS NULL
	`, productID).Scan(&prodHistCount)
	require.NoError(t, err)
	assert.Equal(t, 1, prodHistCount, "Product price sync must create exactly one product price history event")
}

func TestPriceHistory_NoOpCreatesNoRecord(t *testing.T) {
	client, svc, sellerID, sellerUserID := setupPriceHistoryTestDB(t)
	var productIDs []uuid.UUID

	defer func() {
		cleanupPriceHistoryFixtures(client, sellerID, sellerUserID, productIDs)
		client.Close()
	}()

	ctx := context.Background()

	productID := uuid.New()
	variantID := uuid.New()
	productIDs = append(productIDs, productID)

	_, err := client.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, price_cents, status)
		VALUES ($1, $2, 'No-Op T-Shirt', $3, 2000, 'published')
	`, productID, sellerID, fmt.Sprintf("noop-tshirt-%s", productID.String()[:8]))
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents, is_active)
		VALUES ($1, $2, $3, 2000, true)
	`, variantID, productID, fmt.Sprintf("SKU-NOOP-%s", variantID.String()[:8]))
	require.NoError(t, err)

	// Update to SAME price (2000 -> 2000): No-Op
	req := products.UpdateProductPricesRequest{
		Variants: []products.VariantPriceUpdateRequest{
			{ID: variantID, PriceCents: 2000},
		},
	}
	err = svc.UpdateProductPrices(ctx, sellerUserID, productID, req)
	require.NoError(t, err)

	// Verify NO history record is created for a no-op price update
	var count int
	err = client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_price_history WHERE product_id = $1", productID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "No-op price change (A -> A) must not create meaningless history")
}

func TestPriceHistory_FailedTransactionRollsBackHistory(t *testing.T) {
	client, svc, sellerID, sellerUserID := setupPriceHistoryTestDB(t)
	var productIDs []uuid.UUID

	defer func() {
		cleanupPriceHistoryFixtures(client, sellerID, sellerUserID, productIDs)
		client.Close()
	}()

	ctx := context.Background()

	productID := uuid.New()
	variantID := uuid.New()
	productIDs = append(productIDs, productID)

	_, err := client.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, price_cents, status)
		VALUES ($1, $2, 'Atomic Rollback T-Shirt', $3, 3000, 'published')
	`, productID, sellerID, fmt.Sprintf("atomic-tshirt-%s", productID.String()[:8]))
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents, is_active)
		VALUES ($1, $2, $3, 3000, true)
	`, variantID, productID, fmt.Sprintf("SKU-ATOMIC-%s", variantID.String()[:8]))
	require.NoError(t, err)

	// Attempt update containing one valid variant and one foreign/invalid variant ID
	foreignVariantID := uuid.New()
	req := products.UpdateProductPricesRequest{
		Variants: []products.VariantPriceUpdateRequest{
			{ID: variantID, PriceCents: 4000},
			{ID: foreignVariantID, PriceCents: 5000},
		},
	}

	err = svc.UpdateProductPrices(ctx, sellerUserID, productID, req)
	require.Error(t, err, "Update must fail because foreign variant does not belong to product")

	// Verify variant price is unchanged
	var vPrice int64
	err = client.Pool.QueryRow(ctx, "SELECT price_cents FROM product_variants WHERE id = $1", variantID).Scan(&vPrice)
	require.NoError(t, err)
	assert.Equal(t, int64(3000), vPrice, "Variant price must remain unchanged after failed transaction")

	// Verify NO orphan history rows were left behind
	var count int
	err = client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_price_history WHERE product_id = $1", productID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Failed transaction must roll back cleanly and not leave orphan history")
}
