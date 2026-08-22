package catalog_test

import (
	"context"
	"testing"
	"os"
	"github.com/stretchr/testify/require"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/catalog"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
)

func TestRepository_ListCategories_ActiveOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	ctx := context.Background()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"
	}

	dbClient, err := postgres.NewClient(ctx, dbURL)
	require.NoError(t, err)
	defer dbClient.Close()

	repo := catalog.NewRepository(dbClient.Pool)

	catID1 := uuid.New()
	catID2 := uuid.New()
	_, err = dbClient.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, sort_order, is_active) VALUES ($1, 'Active Cat Test', $2, 1, true)", catID1, "active-" + catID1.String())
	require.NoError(t, err)
	_, err = dbClient.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug, sort_order, is_active) VALUES ($1, 'Inactive Cat Test', $2, 2, false)", catID2, "inactive-" + catID2.String())
	require.NoError(t, err)

	activeCats, err := repo.ListCategories(ctx, true)
	require.NoError(t, err)
	foundActive := false
	for _, c := range activeCats {
		if c.ID == catID2 {
			t.Fatalf("ListCategories(ctx, true) returned inactive category")
		}
		if c.ID == catID1 {
			foundActive = true
		}
	}
	require.True(t, foundActive, "Active category not found")

	allCats, err := repo.ListCategories(ctx, false)
	require.NoError(t, err)
	foundInactive := false
	for _, c := range allCats {
		if c.ID == catID2 {
			foundInactive = true
		}
	}
	require.True(t, foundInactive, "ListCategories(ctx, false) did not return inactive category")
}
