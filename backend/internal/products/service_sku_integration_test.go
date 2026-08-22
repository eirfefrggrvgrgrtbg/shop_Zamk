package products_test

import (
	"context"
	"testing"
	"strings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSKUsIntegration(t *testing.T) {
	db, svc, sellerUserID := setupBlockATestDB(t)
	defer db.Close()
	ctx := context.Background()

	skus, err := svc.GenerateSKUs(ctx, sellerUserID, 5)
	require.NoError(t, err)
	require.Len(t, skus, 5)

	for _, sku := range skus {
		assert.True(t, strings.HasPrefix(sku, "SKU-"))
	}
}
