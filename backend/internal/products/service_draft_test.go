package products_test

import (
	"context"
	"fmt"
	"testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestDraftAndModeration(t *testing.T) {
	dbClient, svc, sellerUserID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// 1. Create a strict category with required attributes, variants, etc.
	catID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Strict Category', $2, true)", catID, "strict-" + uuid.New().String())
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
			Title: "Incomplete Product",
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
		Title: "Test",
		Description: nil,
		CategoryID: &parentID,
		
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
		Title: "Test",
		CategoryID: &inactiveID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot use inactive category")
}
