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
	dbClient, svc, sellerUserID := setupTestDB(t)
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
