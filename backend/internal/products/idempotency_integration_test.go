package products_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func createSecondTestSeller(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	userID := uuid.New()
	sellerID := uuid.New()
	brandID := uuid.New()
	slug := fmt.Sprintf("test-brand-%s", userID)

	// Register cleanup BEFORE first mutation
	t.Cleanup(func() {
		cleanQueries := []struct {
			query string
			arg   uuid.UUID
		}{
			{"DELETE FROM variant_attribute_values WHERE product_variant_id IN (SELECT id FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1))", sellerID},
			{"DELETE FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID},
			{"DELETE FROM product_attribute_values WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID},
			{"DELETE FROM product_material_composition WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID},
			{"DELETE FROM product_size_chart_rows WHERE size_chart_id IN (SELECT id FROM product_size_charts WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1))", sellerID},
			{"DELETE FROM product_size_charts WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID},
			{"DELETE FROM product_images WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID},
			{"DELETE FROM product_revisions WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID},
			{"DELETE FROM products WHERE seller_id = $1", sellerID},
			{"DELETE FROM seller_brands WHERE seller_id = $1", sellerID},
			{"DELETE FROM brands WHERE id = $1", brandID},
			{"DELETE FROM seller_users WHERE user_id = $1", userID},
			{"DELETE FROM sellers WHERE id = $1", sellerID},
			{"DELETE FROM users WHERE id = $1", userID},
		}
		for _, cq := range cleanQueries {
			if _, err := pool.Exec(context.Background(), cq.query, cq.arg); err != nil {
				t.Errorf("cleanup error for %s (%s): %v", cq.query, cq.arg, err)
			}
		}
	})

	_, err := pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'seller', 'Second Seller')", userID, fmt.Sprintf("test-%s@test.com", userID))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Second Brand', $2, 'test2@test.com', 'active')", sellerID, slug)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')", uuid.New(), sellerID, userID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, $2, $3)", brandID, "Second Brand Name", slug)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO seller_brands (id, seller_id, brand_id, is_primary, relationship_type, status) VALUES ($1, $2, $3, true, 'owner', 'active')", uuid.New(), sellerID, brandID)
	require.NoError(t, err)

	return userID, sellerID
}

func TestProductIdempotency_Integration(t *testing.T) {
	dbClient, svc, sellerUserID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// HARD DB SAFETY GUARD
	var dbName string
	err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName)

	var sellerID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT seller_id FROM seller_users WHERE user_id = $1", sellerUserID).Scan(&sellerID)
	require.NoError(t, err)

	// Fetch reference fixtures
	var materialID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM materials WHERE is_active = true LIMIT 1").Scan(&materialID)
	require.NoError(t, err)

	var sizeValueID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM size_values LIMIT 1").Scan(&sizeValueID)
	require.NoError(t, err)

	// Pre-generate fixture IDs
	prodAttrDefID := uuid.New()
	catID := uuid.New()

	var mu sync.Mutex
	var createdProductIDs []uuid.UUID
	registerProduct := func(id uuid.UUID) {
		mu.Lock()
		createdProductIDs = append(createdProductIDs, id)
		mu.Unlock()
	}

	// Register ALL cleanups BEFORE any test mutations
	t.Cleanup(func() {
		mu.Lock()
		ids := make([]uuid.UUID, len(createdProductIDs))
		copy(ids, createdProductIDs)
		mu.Unlock()
		if len(ids) > 0 {
			queries := []string{
				"DELETE FROM variant_attribute_values WHERE product_variant_id IN (SELECT id FROM product_variants WHERE product_id = ANY($1))",
				"DELETE FROM product_variants WHERE product_id = ANY($1)",
				"DELETE FROM product_attribute_values WHERE product_id = ANY($1)",
				"DELETE FROM product_material_composition WHERE product_id = ANY($1)",
				"DELETE FROM product_size_chart_rows WHERE size_chart_id IN (SELECT id FROM product_size_charts WHERE product_id = ANY($1))",
				"DELETE FROM product_size_charts WHERE product_id = ANY($1)",
				"DELETE FROM product_images WHERE product_id = ANY($1)",
				"DELETE FROM product_revisions WHERE product_id = ANY($1)",
				"DELETE FROM products WHERE id = ANY($1)",
			}
			for _, q := range queries {
				if _, err := pool.Exec(context.Background(), q, ids); err != nil {
					t.Errorf("cleanup failed for query %s: %v", q, err)
				}
			}
		}

		if _, err := pool.Exec(context.Background(), "DELETE FROM category_size_chart_fields WHERE category_id = $1", catID); err != nil {
			t.Errorf("failed to clean up category_size_chart_fields %s: %v", catID, err)
		}
		if _, err := pool.Exec(context.Background(), "DELETE FROM category_attribute_definitions WHERE category_id = $1", catID); err != nil {
			t.Errorf("failed to clean up category_attribute_definitions %s: %v", catID, err)
		}
		if _, err := pool.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", catID); err != nil {
			t.Errorf("failed to clean up category %s: %v", catID, err)
		}
		if _, err := pool.Exec(context.Background(), "DELETE FROM attribute_definitions WHERE id = $1", prodAttrDefID); err != nil {
			t.Errorf("failed to clean up attribute definition %s: %v", prodAttrDefID, err)
		}
	})

	// Now perform setup inserts
	_, err = pool.Exec(ctx, "INSERT INTO attribute_definitions (id, code, name_ru, value_type, scope, is_active) VALUES ($1, $2, 'Test Prod Attr', 'TEXT', 'PRODUCT', true)", prodAttrDefID, "TEST_PROD_ATTR_"+prodAttrDefID.String())
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'Idem Integ Cat', $2)", catID, "idem-cat-"+uuid.New().String())
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO category_attribute_definitions (category_id, attribute_definition_id, required, filterable, variant_axis) VALUES ($1, $2, false, true, false)", catID, prodAttrDefID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO category_size_chart_fields (id, category_id, code, name, unit, is_required, sort_order) VALUES (gen_random_uuid(), $1, 'CHEST', 'Обхват груди', 'cm', false, 10)", catID)
	require.NoError(t, err)

	t.Run("first keyed request creates one product", func(t *testing.T) {
		idemKey := uuid.New()
		req := products.CreateProductRequest{
			Title:      "First Keyed Product " + uuid.New().String(),
			CategoryID: &catID,
			PriceCents: 1000,
			Currency:   "RUB",
		}
		prod, err := svc.CreateProductForSeller(ctx, sellerUserID, req, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, prod.ID)
		registerProduct(prod.ID)
	})

	t.Run("same seller/key/payload returns same productId", func(t *testing.T) {
		idemKey := uuid.New()
		req := products.CreateProductRequest{
			Title:      "Replay Test Product " + uuid.New().String(),
			CategoryID: &catID,
			PriceCents: 1200,
			Currency:   "RUB",
		}
		prod1, err := svc.CreateProductForSeller(ctx, sellerUserID, req, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err)
		registerProduct(prod1.ID)

		prod2, err := svc.CreateProductForSeller(ctx, sellerUserID, req, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err)
		require.Equal(t, prod1.ID, prod2.ID)

		var count int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM products WHERE id = $1", prod1.ID).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("replay with real SellerSKU does NOT return DuplicateSKUError and creates zero duplicates", func(t *testing.T) {
		idemKey := uuid.New()
		realSKU := fmt.Sprintf("SKU-IDEM-%s", uuid.New().String())
		textVal := "blue"
		req := products.CreateProductRequest{
			Title:      "Real SKU Replay Product " + uuid.New().String(),
			CategoryID: &catID,
			PriceCents: 3500,
			Currency:   "RUB",
			Variants: []products.ProductVariantRequest{
				{
					SellerSKU: &realSKU,
				},
			},
			Attributes: []products.ProductAttributeValueRequest{
				{
					AttributeDefinitionID: prodAttrDefID,
					TextValue:             &textVal,
				},
			},
			MaterialComposition: []products.ProductMaterialCompositionRequest{
				{
					MaterialID: materialID,
					Percentage: 100,
				},
			},
			SizeChartRows: []products.ProductSizeChartRowRequest{
				{
					SizeValueID:  sizeValueID,
					Measurements: map[string]interface{}{"CHEST": float64(96)},
				},
			},
		}

		// First call
		prod1, err := svc.CreateProductForSeller(ctx, sellerUserID, req, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err)
		registerProduct(prod1.ID)

		// Second call (replay)
		prod2, err := svc.CreateProductForSeller(ctx, sellerUserID, req, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err, "replay must not fail with duplicate SKU")
		require.Equal(t, prod1.ID, prod2.ID)

		// Verify zero-side-effect row counts across all touched tables
		var prodCount, variantCount, attrCount, matCount, chartCount, chartRowCount int

		err = pool.QueryRow(ctx, "SELECT count(*) FROM products WHERE id = $1", prod1.ID).Scan(&prodCount)
		require.NoError(t, err)
		require.Equal(t, 1, prodCount, "products row count")

		err = pool.QueryRow(ctx, "SELECT count(*) FROM product_variants WHERE product_id = $1", prod1.ID).Scan(&variantCount)
		require.NoError(t, err)
		require.Equal(t, 1, variantCount, "product_variants row count")

		err = pool.QueryRow(ctx, "SELECT count(*) FROM product_attribute_values WHERE product_id = $1", prod1.ID).Scan(&attrCount)
		require.NoError(t, err)
		require.Equal(t, 1, attrCount, "product_attribute_values row count")

		err = pool.QueryRow(ctx, "SELECT count(*) FROM product_material_composition WHERE product_id = $1", prod1.ID).Scan(&matCount)
		require.NoError(t, err)
		require.Equal(t, 1, matCount, "product_material_composition row count")

		err = pool.QueryRow(ctx, "SELECT count(*) FROM product_size_charts WHERE product_id = $1", prod1.ID).Scan(&chartCount)
		require.NoError(t, err)
		require.Equal(t, 1, chartCount, "product_size_charts row count")

		err = pool.QueryRow(ctx, "SELECT count(*) FROM product_size_chart_rows WHERE size_chart_id IN (SELECT id FROM product_size_charts WHERE product_id = $1)", prod1.ID).Scan(&chartRowCount)
		require.NoError(t, err)
		require.Equal(t, 1, chartRowCount, "product_size_chart_rows row count")
	})

	t.Run("same seller/key/different payload returns conflict", func(t *testing.T) {
		idemKey := uuid.New()
		req1 := products.CreateProductRequest{
			Title:      "Payload 1 " + uuid.New().String(),
			CategoryID: &catID,
			PriceCents: 1000,
			Currency:   "RUB",
		}
		prod1, err := svc.CreateProductForSeller(ctx, sellerUserID, req1, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err)
		registerProduct(prod1.ID)

		req2 := products.CreateProductRequest{
			Title:      "Payload 2 Differing " + uuid.New().String(),
			CategoryID: &catID,
			PriceCents: 2000,
			Currency:   "RUB",
		}
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req2, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.ErrorIs(t, err, products.ErrIdempotencyKeyConflict)
	})

	t.Run("same seller/key/different Slug returns conflict", func(t *testing.T) {
		idemKey := uuid.New()
		slug1 := "slug-branch-a-" + uuid.New().String()
		req1 := products.CreateProductRequest{
			Title:      "Slug Regression Product " + uuid.New().String(),
			Slug:       &slug1,
			CategoryID: &catID,
			PriceCents: 1500,
			Currency:   "RUB",
		}
		prod1, err := svc.CreateProductForSeller(ctx, sellerUserID, req1, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err)
		registerProduct(prod1.ID)

		slug2 := "slug-branch-b-" + uuid.New().String()
		req2 := req1
		req2.Slug = &slug2
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req2, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.ErrorIs(t, err, products.ErrIdempotencyKeyConflict, "different Slug with same key MUST return idempotency conflict")
	})

	t.Run("same seller/key/different Gender returns conflict", func(t *testing.T) {
		idemKey := uuid.New()
		gender1 := "unisex"
		req1 := products.CreateProductRequest{
			Title:      "Gender Regression Product " + uuid.New().String(),
			Gender:     &gender1,
			CategoryID: &catID,
			PriceCents: 1600,
			Currency:   "RUB",
		}
		prod1, err := svc.CreateProductForSeller(ctx, sellerUserID, req1, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err)
		registerProduct(prod1.ID)

		gender2 := "women"
		req2 := req1
		req2.Gender = &gender2
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req2, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.ErrorIs(t, err, products.ErrIdempotencyKeyConflict, "different Gender with same key MUST return idempotency conflict")
	})

	t.Run("same seller/key/different MainImageURL returns conflict", func(t *testing.T) {
		idemKey := uuid.New()
		img1 := "http://example.com/image1.jpg"
		req1 := products.CreateProductRequest{
			Title:        "MainImageURL Regression Product " + uuid.New().String(),
			MainImageURL: &img1,
			CategoryID:   &catID,
			PriceCents:   1700,
			Currency:     "RUB",
		}
		prod1, err := svc.CreateProductForSeller(ctx, sellerUserID, req1, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err)
		registerProduct(prod1.ID)

		img2 := "http://example.com/image2.jpg"
		req2 := req1
		req2.MainImageURL = &img2
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req2, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.ErrorIs(t, err, products.ErrIdempotencyKeyConflict, "different MainImageURL with same key MUST return idempotency conflict")
	})

	t.Run("same seller/key/different Images returns conflict", func(t *testing.T) {
		idemKey := uuid.New()
		req1 := products.CreateProductRequest{
			Title:      "Images Array Regression Product " + uuid.New().String(),
			CategoryID: &catID,
			PriceCents: 1750,
			Currency:   "RUB",
			Images: []products.ProductImageRequest{
				{ImageURL: "http://example.com/item1.jpg"},
			},
		}
		prod1, err := svc.CreateProductForSeller(ctx, sellerUserID, req1, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err)
		registerProduct(prod1.ID)

		req2 := req1
		req2.Images = []products.ProductImageRequest{
			{ImageURL: "http://example.com/item2.jpg"},
		}
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req2, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.ErrorIs(t, err, products.ErrIdempotencyKeyConflict, "different Images array with same key MUST return idempotency conflict")
	})

	t.Run("different seller/same key creates independent products", func(t *testing.T) {
		seller2UserID, seller2ID := createSecondTestSeller(t, pool)

		idemKey := uuid.New()
		slug1 := "cross-seller-slug-1-" + uuid.New().String()
		req1 := products.CreateProductRequest{
			Title:      "Cross Seller Same Key Product",
			Slug:       &slug1,
			CategoryID: &catID,
			PriceCents: 1800,
			Currency:   "RUB",
		}

		slug2 := "cross-seller-slug-2-" + uuid.New().String()
		req2 := products.CreateProductRequest{
			Title:      "Cross Seller Same Key Product",
			Slug:       &slug2,
			CategoryID: &catID,
			PriceCents: 1800,
			Currency:   "RUB",
		}

		prod1, err := svc.CreateProductForSeller(ctx, sellerUserID, req1, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err)
		registerProduct(prod1.ID)

		prod2, err := svc.CreateProductForSeller(ctx, seller2UserID, req2, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err)

		require.NotEqual(t, prod1.ID, prod2.ID)
		require.Equal(t, sellerID, prod1.SellerID)
		require.Equal(t, seller2ID, prod2.SellerID)
	})

	t.Run("concurrent same seller/key/same payload with real non-empty SellerSKU", func(t *testing.T) {
		idemKey := uuid.New()
		realSKU := fmt.Sprintf("CONCURRENT-SKU-%s", uuid.New().String())
		req := products.CreateProductRequest{
			Title:      "Concurrent Same Key Product " + uuid.New().String(),
			CategoryID: &catID,
			PriceCents: 2200,
			Currency:   "RUB",
			Variants: []products.ProductVariantRequest{
				{SellerSKU: &realSKU},
			},
		}

		concurrency := 5
		var wg sync.WaitGroup
		var successCount int32
		var errCount int32
		var returnedIDs sync.Map

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				prod, err := svc.CreateProductForSeller(ctx, sellerUserID, req, products.CreateProductOptions{IdempotencyKey: &idemKey})
				if err == nil {
					atomic.AddInt32(&successCount, 1)
					returnedIDs.Store(prod.ID, true)
				} else {
					atomic.AddInt32(&errCount, 1)
				}
			}()
		}
		wg.Wait()

		require.Equal(t, int32(concurrency), successCount)
		require.Equal(t, int32(0), errCount)

		var uniqueIDs []uuid.UUID
		returnedIDs.Range(func(k, v interface{}) bool {
			uniqueIDs = append(uniqueIDs, k.(uuid.UUID))
			return true
		})
		require.Len(t, uniqueIDs, 1, "all concurrent callers must receive the exact same productId")
		registerProduct(uniqueIDs[0])

		var prodCount, variantCount int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM products WHERE id = $1", uniqueIDs[0]).Scan(&prodCount)
		require.NoError(t, err)
		require.Equal(t, 1, prodCount)

		err = pool.QueryRow(ctx, "SELECT count(*) FROM product_variants WHERE product_id = $1", uniqueIDs[0]).Scan(&variantCount)
		require.NoError(t, err)
		require.Equal(t, 1, variantCount)
	})

	t.Run("concurrent same seller/key/different payload", func(t *testing.T) {
		idemKey := uuid.New()
		concurrency := 5
		var wg sync.WaitGroup
		var successCount int32
		var conflictCount int32
		var wonProductIDs sync.Map

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			price := int64(1000 + i*100)
			req := products.CreateProductRequest{
				Title:      fmt.Sprintf("Concurrent Divergent %d %s", i, uuid.New().String()),
				CategoryID: &catID,
				PriceCents: price,
				Currency:   "RUB",
			}
			go func() {
				defer wg.Done()
				prod, err := svc.CreateProductForSeller(ctx, sellerUserID, req, products.CreateProductOptions{IdempotencyKey: &idemKey})
				if err == nil {
					atomic.AddInt32(&successCount, 1)
					wonProductIDs.Store(prod.ID, true)
				} else if err == products.ErrIdempotencyKeyConflict {
					atomic.AddInt32(&conflictCount, 1)
				}
			}()
		}
		wg.Wait()

		require.Equal(t, int32(1), successCount, "exactly one create must succeed")
		require.Equal(t, int32(concurrency-1), conflictCount, "all others must receive idempotency conflict")

		var wonIDs []uuid.UUID
		wonProductIDs.Range(func(k, v interface{}) bool {
			wonIDs = append(wonIDs, k.(uuid.UUID))
			return true
		})
		require.Len(t, wonIDs, 1)
		registerProduct(wonIDs[0])

		var count int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM products WHERE id = $1", wonIDs[0]).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("forced later subdocument failure rolls back whole transaction with no reserved half-product", func(t *testing.T) {
		idemKey := uuid.New()
		fakeAttrDefID := uuid.New() // Not in attribute_definitions, causes DB FK error during subdocument insert
		textVal := "should fail"

		reqFailing := products.CreateProductRequest{
			Title:      "Failing Subdoc Product " + uuid.New().String(),
			PriceCents: 1900,
			Currency:   "RUB",
			Attributes: []products.ProductAttributeValueRequest{
				{
					AttributeDefinitionID: fakeAttrDefID,
					TextValue:             &textVal,
				},
			},
		}

		// Attempt create - must fail inside transaction during subdocument insert
		_, err := svc.CreateProductForSeller(ctx, sellerUserID, reqFailing, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.Error(t, err)
		// Explicit proof that failure occurred at the DB transaction level (foreign key violation on product_attribute_values)
		require.Contains(t, err.Error(), "product_attribute_values", "error must originate from subdocument insert inside transaction")

		// Assert no product exists with that idempotency key
		var count int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM products WHERE seller_id = $1 AND create_idempotency_key = $2", sellerID, idemKey).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count, "failed transaction must leave no product row for the idempotency key")

		// Assert zero orphaned child rows exist for this failed attempt
		var childCount int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM product_attribute_values WHERE text_value = $1", textVal).Scan(&childCount)
		require.NoError(t, err)
		require.Equal(t, 0, childCount, "failed transaction must leave zero orphaned child rows")

		// Retry with valid request using the EXACT SAME idempotency key
		reqSuccess := products.CreateProductRequest{
			Title:      "Corrected Product Same Key " + uuid.New().String(),
			PriceCents: 1900,
			Currency:   "RUB",
		}
		prod, err := svc.CreateProductForSeller(ctx, sellerUserID, reqSuccess, products.CreateProductOptions{IdempotencyKey: &idemKey})
		require.NoError(t, err, "key must not be poisoned by previous rolled back transaction")
		require.NotEqual(t, uuid.Nil, prod.ID)
		registerProduct(prod.ID)
	})
}
