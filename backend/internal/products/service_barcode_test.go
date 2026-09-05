package products_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestProductBarcode_PlaceholderSanitization(t *testing.T) {
	dbClient, svc, sellerUserID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	catID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Barcode Test Cat', $2, false)", catID, "cat-"+uuid.New().String())
	require.NoError(t, err)

	// A. Create variant with empty/no barcode -> receives valid canonical ZMK
	p1, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Product A - Auto ZMK",
		CategoryID: &catID,
		Variants: []products.ProductVariantRequest{
			{
				PriceCents: func(i int64) *int64 { return &i }(1500),
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, p1.Variants, 1)
	v1 := p1.Variants[0]
	require.NotNil(t, v1.Barcode)
	assert.True(t, strings.HasPrefix(*v1.Barcode, "ZMK-"), "empty barcode must receive canonical ZMK prefix, got: %s", *v1.Barcode)
	assert.Greater(t, len(*v1.Barcode), 4)

	// B. Variant with placeholder barcode "000000000000" in DB -> UpdateProductForSeller replaces it with canonical ZMK
	p2, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Product B - Placeholder In DB",
		CategoryID: &catID,
		Variants: []products.ProductVariantRequest{
			{
				PriceCents: func(i int64) *int64 { return &i }(2000),
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, p2.Variants, 1)
	v2 := p2.Variants[0]

	// Simulate existing legacy placeholder barcode in DB
	placeholder := "000000000000"
	_, err = pool.Exec(ctx, "UPDATE product_variants SET barcode = $1 WHERE id = $2", placeholder, v2.ID)
	require.NoError(t, err)

	// Verify DB currently has the placeholder
	var dbBarcode string
	err = pool.QueryRow(ctx, "SELECT barcode FROM product_variants WHERE id = $1", v2.ID).Scan(&dbBarcode)
	require.NoError(t, err)
	require.Equal(t, placeholder, dbBarcode)

	// Update the product -> service must detect placeholder barcode and replace with canonical ZMK
	updatedTitleB := "Product B - Updated"
	p2Updated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p2.ID, products.UpdateProductRequest{
		Title: &updatedTitleB,
		Variants: []products.ProductVariantRequest{
			{
				ID:         &v2.ID,
				PriceCents: func(i int64) *int64 { return &i }(2200),
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, p2Updated.Variants, 1)
	newBarcodeB := p2Updated.Variants[0].Barcode
	require.NotNil(t, newBarcodeB)
	assert.NotEqual(t, placeholder, *newBarcodeB, "placeholder barcode must NOT be persisted")
	assert.True(t, strings.HasPrefix(*newBarcodeB, "ZMK-"), "placeholder barcode must be replaced with canonical ZMK, got: %s", *newBarcodeB)

	// C. Existing valid barcode/ZMK -> is preserved, is NOT regenerated unexpectedly
	validBarcode := "4607001234567"
	p3, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Product C - Valid Barcode",
		CategoryID: &catID,
		Variants: []products.ProductVariantRequest{
			{
				PriceCents: func(i int64) *int64 { return &i }(2500),
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, p3.Variants, 1)
	v3 := p3.Variants[0]

	// Assign a valid non-placeholder barcode in DB
	_, err = pool.Exec(ctx, "UPDATE product_variants SET barcode = $1 WHERE id = $2", validBarcode, v3.ID)
	require.NoError(t, err)

	// Update the product -> existing valid barcode must be preserved exactly
	updatedTitleC := "Product C - Updated"
	p3Updated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p3.ID, products.UpdateProductRequest{
		Title: &updatedTitleC,
		Variants: []products.ProductVariantRequest{
			{
				ID:         &v3.ID,
				PriceCents: func(i int64) *int64 { return &i }(2700),
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, p3Updated.Variants, 1)
	require.NotNil(t, p3Updated.Variants[0].Barcode)
	assert.Equal(t, validBarcode, *p3Updated.Variants[0].Barcode, "existing valid barcode must be preserved on update")

	// D. Two generated variants -> cannot accidentally receive the same ZMK
	pMulti, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Product Multi Variants",
		CategoryID: &catID,
		Variants: []products.ProductVariantRequest{
			{
				PriceCents: func(i int64) *int64 { return &i }(1000),
			},
			{
				PriceCents: func(i int64) *int64 { return &i }(1200),
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, pMulti.Variants, 2)
	bar1 := pMulti.Variants[0].Barcode
	bar2 := pMulti.Variants[1].Barcode
	require.NotNil(t, bar1)
	require.NotNil(t, bar2)
	assert.True(t, strings.HasPrefix(*bar1, "ZMK-"))
	assert.True(t, strings.HasPrefix(*bar2, "ZMK-"))
	assert.NotEqual(t, *bar1, *bar2, "two generated variants must receive unique barcodes")
}

func TestProductBarcode_IsPlaceholderBarcode(t *testing.T) {
	// True cases: empty or only zeros
	assert.True(t, products.IsPlaceholderBarcode(""))
	assert.True(t, products.IsPlaceholderBarcode("   "))
	assert.True(t, products.IsPlaceholderBarcode("0"))
	assert.True(t, products.IsPlaceholderBarcode("000000000000"))
	assert.True(t, products.IsPlaceholderBarcode("000000000000000000"))

	// False cases: real codes
	assert.False(t, products.IsPlaceholderBarcode("ZMK-DEV-0001"))
	assert.False(t, products.IsPlaceholderBarcode("4607001234567"))
	assert.False(t, products.IsPlaceholderBarcode("DEV-SKU-0"))
	assert.False(t, products.IsPlaceholderBarcode("00001"))
	assert.False(t, products.IsPlaceholderBarcode("10000"))
}

func TestProductBarcode_Observability(t *testing.T) {
	dbClient, svc, sellerUserID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	svc.SetLogger(logger)

	catID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Obs Barcode Cat', $2, false)", catID, "cat-obs-"+uuid.New().String())
	require.NoError(t, err)

	// 1. Create a product with a variant
	p, err := svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Product Obs Placeholder",
		CategoryID: &catID,
		Variants: []products.ProductVariantRequest{
			{
				PriceCents: func(i int64) *int64 { return &i }(1500),
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, p.Variants, 1)
	vID := p.Variants[0].ID

	// Simulate existing legacy placeholder barcode "000000000000" in DB
	placeholder := "000000000000"
	_, err = pool.Exec(ctx, "UPDATE product_variants SET barcode = $1 WHERE id = $2", placeholder, vID)
	require.NoError(t, err)

	// Reset log buffer before update
	logBuf.Reset()

	// 2. Normalization on update -> triggers product.variant_barcodes_normalized
	updTitle := "Product Obs Placeholder - Normalized"
	pUpdated, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
		Title: &updTitle,
		Variants: []products.ProductVariantRequest{
			{
				ID:         &vID,
				PriceCents: func(i int64) *int64 { return &i }(1800),
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, pUpdated.Variants, 1)
	normalizedBarcode := pUpdated.Variants[0].Barcode
	require.NotNil(t, normalizedBarcode)
	assert.True(t, strings.HasPrefix(*normalizedBarcode, "ZMK-"))

	// Check logs
	logs := logBuf.String()
	require.NotEmpty(t, logs)

	// Find the business event
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	var matchedEvent map[string]interface{}
	for _, line := range lines {
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			if entry["event_name"] == "product.variant_barcodes_normalized" {
				matchedEvent = entry
				break
			}
		}
	}
	require.NotNil(t, matchedEvent, "expected product.variant_barcodes_normalized event in logs, got: %s", logs)
	assert.Equal(t, "product", matchedEvent["domain"])
	assert.Equal(t, "variant_barcodes_normalized", matchedEvent["action"])
	assert.Equal(t, "success", matchedEvent["result"])
	assert.Equal(t, sellerUserID.String(), matchedEvent["actor_id"])
	assert.Equal(t, "seller", matchedEvent["actor_role"])
	assert.Equal(t, p.ID.String(), matchedEvent["product_id"])
	assert.Equal(t, float64(1), matchedEvent["normalized_count"])
	assert.Equal(t, "placeholder_barcode", matchedEvent["reason"])

	// Privacy Guard: raw placeholder value must NEVER be emitted
	assert.NotContains(t, logs, placeholder, "raw placeholder barcode must not leak into logs")

	// 3. Idempotency safety: Repeated update on already canonical barcode emits 0 events
	logBuf.Reset()
	updTitle2 := "Product Obs Placeholder - Second Update"
	_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
		Title: &updTitle2,
		Variants: []products.ProductVariantRequest{
			{
				ID:         &vID,
				PriceCents: func(i int64) *int64 { return &i }(2000),
			},
		},
	})
	require.NoError(t, err)

	for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			assert.NotEqual(t, "product.variant_barcodes_normalized", entry["event_name"], "repeated update on canonical barcode must emit 0 events")
		}
	}

	// 4. Valid barcode preserved -> 0 events
	validBarcode := "4607009876543"
	_, err = pool.Exec(ctx, "UPDATE product_variants SET barcode = $1 WHERE id = $2", validBarcode, vID)
	require.NoError(t, err)

	logBuf.Reset()
	updTitle3 := "Product Obs Valid Barcode"
	pValid, err := svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
		Title: &updTitle3,
		Variants: []products.ProductVariantRequest{
			{
				ID:         &vID,
				PriceCents: func(i int64) *int64 { return &i }(2100),
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, validBarcode, *pValid.Variants[0].Barcode)

	for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			assert.NotEqual(t, "product.variant_barcodes_normalized", entry["event_name"], "valid barcode update must emit 0 events")
		}
	}

	// 5. Transaction rollback safety: If transaction rolls back, NO event is emitted
	// Set back to placeholder
	_, err = pool.Exec(ctx, "UPDATE product_variants SET barcode = $1 WHERE id = $2", placeholder, vID)
	require.NoError(t, err)

	// Create another product with SKU "SKU-COLLISION-1"
	skuCollision := "sku-collision-1"
	_, err = svc.CreateProductForSeller(ctx, sellerUserID, products.CreateProductRequest{
		Title:      "Other Product With Collision SKU",
		CategoryID: &catID,
		Variants: []products.ProductVariantRequest{
			{
				SellerSKU:  &skuCollision,
				PriceCents: func(i int64) *int64 { return &i }(1000),
			},
		},
	})
	require.NoError(t, err)

	logBuf.Reset()
	// Attempt update on p with conflicting SellerSKU -> fails with DuplicateSKUError in transaction
	_, err = svc.UpdateProductForSeller(ctx, sellerUserID, p.ID, products.UpdateProductRequest{
		Variants: []products.ProductVariantRequest{
			{
				ID:         &vID,
				SellerSKU:  &skuCollision,
				PriceCents: func(i int64) *int64 { return &i }(2200),
			},
		},
	})
	require.Error(t, err, "expected duplicate SKU error")

	// Verify NO event was emitted during the failed/rolled back transaction
	for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			assert.NotEqual(t, "product.variant_barcodes_normalized", entry["event_name"], "failed transaction must NOT emit normalization event")
		}
	}
}
