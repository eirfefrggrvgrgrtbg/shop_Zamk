package products_test

import (
	"context"
	"fmt"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDraftAndModeration(t *testing.T) {
	dbClient, svc, sellerUserID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// 1. Create a strict category with required attributes, variants, etc.
	catID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Strict Category', $2, true)", catID, "strict-"+uuid.New().String())
	require.NoError(t, err)

	// Fetch color attribute definition
	var colorID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM attribute_definitions WHERE code = 'COLOR'").Scan(&colorID)
	require.NoError(t, err)

	// Add color to category schema as REQUIRED
	_, err = pool.Exec(ctx, "INSERT INTO category_attribute_definitions (category_id, attribute_definition_id, required, filterable, variant_axis) VALUES ($1, $2, true, true, true)", catID, colorID)
	require.NoError(t, err)

	t.Run("Save Incomplete Draft Successfully", func(t *testing.T) {
		req := products.CreateProductRequest{
			Title:      "Incomplete Product",
			CategoryID: &catID,
			// No sizes, no color, no size chart, no variants
		}

		p, err := svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.NoError(t, err)
		require.Equal(t, products.StatusDraft, p.Status)

		// Submit should fail strict validation
		submitReq := products.SubmitProductModerationRequest{}
		err = svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, submitReq)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "moderation validation failed")
	})
}

func TestCreateProduct_RejectsNonLeafCategory(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Create parent
	parentID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, sort_order) VALUES ($1, 'Parent123', $2, 1)", parentID, parentID.String())
	require.NoError(t, err)

	// Create child
	childID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO categories (id, parent_id, name, slug, sort_order) VALUES ($1, $2, 'Child123', $3, 2)", childID, parentID, childID.String())
	require.NoError(t, err)

	// Try creating with parent
	_, err = svc.CreateProductForSeller(ctx, sellerID, products.CreateProductRequest{
		Title:       "Test",
		Description: nil,
		CategoryID:  &parentID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot use non-leaf category")
}

func TestCreateProduct_RejectsInactiveCategory(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Create inactive leaf
	inactiveID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, sort_order, is_active) VALUES ($1, 'Inactive Cat', $2, 1, false)", inactiveID, inactiveID.String())
	require.NoError(t, err)

	// Try creating with inactive category
	_, err = svc.CreateProductForSeller(ctx, sellerID, products.CreateProductRequest{
		Title:      "Test",
		CategoryID: &inactiveID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot use inactive category")
}

func TestCompositionValidation(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Get a valid material
	var materialID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT id FROM materials WHERE is_active = true LIMIT 1").Scan(&materialID)
	require.NoError(t, err)

	// Inactive material
	inactiveMatID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO materials (id, code, name_ru, sort_order, is_active) VALUES ($1, $2, 'INACT', 99, false)", inactiveMatID, inactiveMatID.String())
	require.NoError(t, err)

	// Helper for creating req
	makeReq := func(comps []products.ProductMaterialCompositionRequest) products.CreateProductRequest {
		return products.CreateProductRequest{
			Title:               "Test " + uuid.New().String(),
			MaterialComposition: comps,
		}
	}

	t.Run("percentage <= 0 rejected", func(t *testing.T) {
		_, err := svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: materialID, Percentage: 0},
		}))
		require.ErrorContains(t, err, "percentage must be > 0 and <= 100")
	})

	t.Run("percentage > 100 rejected", func(t *testing.T) {
		_, err := svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: materialID, Percentage: 101},
		}))
		require.ErrorContains(t, err, "percentage must be > 0 and <= 100")
	})

	t.Run("duplicate material rejected", func(t *testing.T) {
		_, err := svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: materialID, Percentage: 40},
			{MaterialID: materialID, Percentage: 60},
		}))
		require.ErrorContains(t, err, "duplicate material entry")
	})

	t.Run("draft total > 100 rejected", func(t *testing.T) {
		var matID2 uuid.UUID
		err := pool.QueryRow(ctx, "SELECT id FROM materials WHERE is_active = true AND id != $1 LIMIT 1", materialID).Scan(&matID2)
		require.NoError(t, err)
		_, err = svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: materialID, Percentage: 60},
			{MaterialID: matID2, Percentage: 50},
		}))
		require.ErrorContains(t, err, "total material composition exceeds 100")
	})

	t.Run("unknown/inactive material rejected", func(t *testing.T) {
		_, err := svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: uuid.New(), Percentage: 100},
		}))
		require.ErrorContains(t, err, "unknown material")

		_, err = svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: inactiveMatID, Percentage: 100},
		}))
		require.ErrorContains(t, err, "inactive material")
	})
}

func TestDraftSaveSemantics(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	t.Run("incomplete draft can save before Photo upload", func(t *testing.T) {
		req := products.CreateProductRequest{
			Title:    "Minimal Draft " + uuid.New().String(),
			Currency: "RUB", // As now required by frontend
		}
		p, err := svc.CreateProductForSeller(ctx, sellerID, req)
		require.NoError(t, err)
		require.NotEmpty(t, p.ID)

		// Assert no inventory is created from draft save
		var invCount int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM inventory_items WHERE product_id = $1", p.ID).Scan(&invCount)
		require.NoError(t, err)
		require.Equal(t, 0, invCount, "No inventory should be created for draft save")
	})

	t.Run("draft with composition <100 can save", func(t *testing.T) {
		var materialID uuid.UUID
		err := pool.QueryRow(ctx, "SELECT id FROM materials WHERE is_active = true LIMIT 1").Scan(&materialID)
		require.NoError(t, err)

		req := products.CreateProductRequest{
			Title:    "Draft Comp " + uuid.New().String(),
			Currency: "RUB",
			MaterialComposition: []products.ProductMaterialCompositionRequest{
				{MaterialID: materialID, Percentage: 50},
			},
		}
		p, err := svc.CreateProductForSeller(ctx, sellerID, req)
		require.NoError(t, err)
		require.NotEmpty(t, p.ID)
	})
}
