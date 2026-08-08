package products_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string {
	return &s
}

func setupTestDB(t *testing.T) (*postgres.Client, *products.Service, uuid.UUID) {
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
	
	// Create minimal dependencies (if products service requires them, they can be nil if unused in CreateProduct)
	svc := products.NewService(repo, sellerRepo, db, nil, nil)

	// Create a dummy user and seller for the tests
	userID := uuid.New()
	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'seller', 'Test User')", userID, fmt.Sprintf("test-%s@test.com", userID))
	require.NoError(t, err)

	sellerID := uuid.New()
	slug := fmt.Sprintf("test-brand-%s", userID)
	_, err = db.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Test Brand', $2, 'test@test.com', 'active')", sellerID, slug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerID, userID)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Pool.Exec(context.Background(), "DELETE FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID)
		db.Pool.Exec(context.Background(), "DELETE FROM products WHERE seller_id = $1", sellerID)
		db.Pool.Exec(context.Background(), "DELETE FROM seller_users WHERE user_id = $1", userID)
		db.Pool.Exec(context.Background(), "DELETE FROM sellers WHERE id = $1", sellerID)
		db.Pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", userID)
	})

	return db, svc, userID
}

func TestSKUUniqueness(t *testing.T) {
	db, svc, seller1UserID := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	_, _, seller2UserID := setupTestDB(t)

	// A. duplicate SKU inside one Product request -> reject
	t.Run("A. Duplicate in one request", func(t *testing.T) {
		req := products.CreateProductRequest{
			Title:      "Test Product A",
			Currency:   "RUB",
			PriceCents: 100,
			Variants: []products.ProductVariantRequest{
				{SKU: ptr("ABC-001")},
				{SKU: ptr(" ABC-001 ")},
			},
		}
		_, err := svc.CreateProductForSeller(ctx, seller1UserID, req)
		require.Error(t, err)
		var skuErr *products.DuplicateSKUError
		require.ErrorAs(t, err, &skuErr)
		assert.Equal(t, "ABC-001", skuErr.SKU)
	})

	// B. same Seller: Product A SKU ABC-001, Product B SKU ABC-001 -> reject
	t.Run("B. Same seller cross-product", func(t *testing.T) {
		req1 := products.CreateProductRequest{
			Title:      "Test Product B1",
			Slug:       ptr(uuid.New().String()),
			Currency:   "RUB",
			PriceCents: 100,
			Variants: []products.ProductVariantRequest{
				{SKU: ptr("ABC-002")},
			},
		}
		_, err := svc.CreateProductForSeller(ctx, seller1UserID, req1)
		require.NoError(t, err)

		req2 := products.CreateProductRequest{
			Title:      "Test Product B2",
			Slug:       ptr(uuid.New().String()),
			Currency:   "RUB",
			PriceCents: 100,
			Variants: []products.ProductVariantRequest{
				{SKU: ptr("abc-002")},
			},
		}
		_, err = svc.CreateProductForSeller(ctx, seller1UserID, req2)
		require.Error(t, err)
		var skuErr *products.DuplicateSKUError
		require.ErrorAs(t, err, &skuErr)
	})

	// C. different Sellers: Seller A SKU ABC-001, Seller B SKU ABC-001 -> allow
	t.Run("C. Different sellers same SKU", func(t *testing.T) {
		req1 := products.CreateProductRequest{
			Title:      "Test Product C1",
			Slug:       ptr(uuid.New().String()),
			Currency:   "RUB",
			PriceCents: 100,
			Variants: []products.ProductVariantRequest{
				{SKU: ptr("ABC-003")},
			},
		}
		_, err := svc.CreateProductForSeller(ctx, seller1UserID, req1)
		require.NoError(t, err)

		req2 := products.CreateProductRequest{
			Title:      "Test Product C2",
			Slug:       ptr(uuid.New().String()),
			Currency:   "RUB",
			PriceCents: 100,
			Variants: []products.ProductVariantRequest{
				{SKU: ptr("ABC-003")},
			},
		}
		_, err = svc.CreateProductForSeller(ctx, seller2UserID, req2)
		require.NoError(t, err)
	})

	// D. update Variant keeping its own SKU -> allow
	t.Run("D. Update variant keeping own SKU", func(t *testing.T) {
		req := products.CreateProductRequest{
			Title:      "Test Product D1",
			Slug:       ptr(uuid.New().String()),
			Currency:   "RUB",
			PriceCents: 100,
			Variants: []products.ProductVariantRequest{
				{SKU: ptr("ABC-004")},
			},
		}
		p, err := svc.CreateProductForSeller(ctx, seller1UserID, req)
		require.NoError(t, err)

		reqUpdate := products.UpdateProductRequest{
			Variants: []products.ProductVariantRequest{
				{SKU: ptr("ABC-004")},
			},
		}
		_, err = svc.UpdateProductForSeller(ctx, seller1UserID, p.ID, reqUpdate)
		require.NoError(t, err)
	})

	// E. update Variant to another existing SKU of same Seller -> reject
	t.Run("E. Update variant to another existing SKU", func(t *testing.T) {
		req1 := products.CreateProductRequest{
			Title:      "Test Product E1",
			Slug:       ptr(uuid.New().String()),
			Currency:   "RUB",
			PriceCents: 100,
			Variants: []products.ProductVariantRequest{
				{SKU: ptr("ABC-005")},
			},
		}
		_, err := svc.CreateProductForSeller(ctx, seller1UserID, req1)
		require.NoError(t, err)

		req2 := products.CreateProductRequest{
			Title:      "Test Product E2",
			Slug:       ptr(uuid.New().String()),
			Currency:   "RUB",
			PriceCents: 100,
			Variants: []products.ProductVariantRequest{
				{SKU: ptr("ABC-006")},
			},
		}
		p2, err := svc.CreateProductForSeller(ctx, seller1UserID, req2)
		require.NoError(t, err)

		reqUpdate := products.UpdateProductRequest{
			Variants: []products.ProductVariantRequest{
				{SKU: ptr("ABC-005")},
			},
		}
		_, err = svc.UpdateProductForSeller(ctx, seller1UserID, p2.ID, reqUpdate)
		require.Error(t, err)
	})

	t.Run("F. Concurrent duplicate creation", func(t *testing.T) {
		var wg sync.WaitGroup
		successCount := 0
		errCount := 0
		var mu sync.Mutex

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				req := products.CreateProductRequest{
					Title:      "Test Product F",
					Slug:       ptr(uuid.New().String()),
					Currency:   "RUB",
					PriceCents: 100,
					Variants: []products.ProductVariantRequest{
						{SKU: ptr("CONCURRENT-001")},
					},
				}
				_, err := svc.CreateProductForSeller(context.Background(), seller1UserID, req)
				mu.Lock()
				if err == nil {
					successCount++
				} else {
					if strings.Contains(err.Error(), "already") || (err != nil) {
						errCount++
					}
				}
				mu.Unlock()
			}(i)
		}
		wg.Wait()
		
		assert.Equal(t, 1, successCount, "Only one concurrent request should succeed")
		assert.Equal(t, 4, errCount, "The others should fail with duplicate SKU error")
	})
}
