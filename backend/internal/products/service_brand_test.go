package products_test

import (
	"context"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSellerBrandInvariants(t *testing.T) {
	dbClient, svc, sellerUserID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Get the seller ID for this user
	var sellerID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT seller_id FROM seller_users WHERE user_id = $1", sellerUserID).Scan(&sellerID)
	require.NoError(t, err)

	// Clear existing brand mapping for the test
	_, err = pool.Exec(ctx, "DELETE FROM seller_brands WHERE seller_id = $1", sellerID)
	require.NoError(t, err)

	makeReq := func(brandID *uuid.UUID) products.CreateProductRequest {
		return products.CreateProductRequest{
			Title:    "Brand Test",
			Currency: "RUB",
			BrandID:  brandID,
		}
	}

	t.Run("0 active primary Brands: Product creation rejected", func(t *testing.T) {
		_, err := svc.CreateProductForSeller(ctx, sellerUserID, makeReq(nil))
		require.ErrorIs(t, err, products.ErrSellerHasNoPrimaryBrand)
	})

	t.Run("1 active primary: Product creation succeeds, brand is canonical", func(t *testing.T) {
		brandID := uuid.New()
		_, err := pool.Exec(ctx, "INSERT INTO brands (id, name, slug, is_active) VALUES ($1, 'B1', $2, true)", brandID, brandID.String())
		require.NoError(t, err)
		_, err = pool.Exec(ctx, "INSERT INTO seller_brands (id, seller_id, brand_id, status, is_primary) VALUES ($1, $2, $3, 'active', true)", uuid.New(), sellerID, brandID)
		require.NoError(t, err)

		foreignBrandID := uuid.New()
		_, err = pool.Exec(ctx, "INSERT INTO brands (id, name, slug, is_active) VALUES ($1, 'BForeign', $2, true)", foreignBrandID, foreignBrandID.String())
		require.NoError(t, err)

		// Client sends foreign/arbitrary brandId: cannot override canonical Seller Brand
		p, err := svc.CreateProductForSeller(ctx, sellerUserID, makeReq(&foreignBrandID))
		require.NoError(t, err)
		require.Equal(t, brandID, *p.BrandID)
	})

	t.Run("2 active primary: Product creation rejected", func(t *testing.T) {
		_, err := pool.Exec(ctx, "DROP INDEX IF EXISTS seller_brands_primary_brand_idx")
		require.NoError(t, err)

		brandID2 := uuid.New()
		_, err = pool.Exec(ctx, "INSERT INTO brands (id, name, slug, is_active) VALUES ($1, 'B2', $2, true)", brandID2, brandID2.String())
		require.NoError(t, err)
		_, err = pool.Exec(ctx, "INSERT INTO seller_brands (id, seller_id, brand_id, status, is_primary) VALUES ($1, $2, $3, 'active', true)", uuid.New(), sellerID, brandID2)
		require.NoError(t, err)

		_, err = svc.CreateProductForSeller(ctx, sellerUserID, makeReq(nil))
		require.ErrorIs(t, err, products.ErrSellerHasMultiplePrimaryBrands)

		// Restore index for subsequent tests
		// Note: since we inserted duplicates, we must delete them before recreating index!
		_, err = pool.Exec(ctx, "DELETE FROM seller_brands WHERE brand_id = $1", brandID2)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, "CREATE UNIQUE INDEX seller_brands_primary_brand_idx ON seller_brands(seller_id) WHERE is_primary = true")
		require.NoError(t, err)
	})
}
