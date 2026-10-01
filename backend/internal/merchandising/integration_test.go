package merchandising_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/merchandising"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*postgres.Client, *merchandising.Repository, *merchandising.Service) {
	ctx := context.Background()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"
	}
	require.True(t, strings.Contains(dbURL, "zamk_test"), "MUST run only against zamk_test")

	client, err := postgres.NewClient(ctx, dbURL)
	require.NoError(t, err)

	var dbName string
	err = client.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "Destructive/integration tests MUST run only against zamk_test")

	repo := merchandising.NewRepository(client.Pool)
	svc := merchandising.NewService(repo, client.Pool)

	return client, repo, svc
}

// cleanupTest removes only fixture-owned rows.
// NOTE ON FK CASCADE:
// - collection_items has ON DELETE CASCADE from collections(id) and products(id).
// - product_variants has ON DELETE CASCADE from products(id).
// - inventory_items has ON DELETE CASCADE from products(id) and product_variants(id).
// Therefore, explicitly deleting collections, products, and sellers cleans up all child fixtures safely.
func cleanupTest(t *testing.T, client *postgres.Client, collectionIDs []uuid.UUID, productIDs []uuid.UUID, sellerIDs []uuid.UUID) {
	ctx := context.Background()
	for _, cid := range collectionIDs {
		client.Pool.Exec(ctx, "DELETE FROM collections WHERE id = $1", cid)
	}
	for _, pid := range productIDs {
		client.Pool.Exec(ctx, "DELETE FROM products WHERE id = $1", pid)
	}
	for _, sid := range sellerIDs {
		client.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sid)
	}
}

func createTestSeller(t *testing.T, client *postgres.Client, active bool) uuid.UUID {
	id := uuid.New()
	status := "active"
	if !active {
		status = "blocked"
	}
	_, err := client.Pool.Exec(context.Background(), `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, $2, $3, 't@t.com', $4, now(), now())
	`, id, "Brand "+id.String()[:8], id.String()[:8], status)
	require.NoError(t, err)
	return id
}

func createTestProduct(t *testing.T, client *postgres.Client, sellerID uuid.UUID, status string, stock int) uuid.UUID {
	id := uuid.New()
	_, err := client.Pool.Exec(context.Background(), "INSERT INTO products (id, seller_id, title, slug, status, source, currency, price_cents, published_at, created_at, updated_at) VALUES ($1, $2, 'Prod', $3, $4, 'draft', 'RUB', 1000, now(), now(), now())", id, sellerID, id.String(), status)
	require.NoError(t, err)

	if stock > 0 {
		varID := uuid.New()
		_, err = client.Pool.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, is_active) VALUES ($1, $2, true)", varID, id)
		require.NoError(t, err)

		_, err = client.Pool.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, $5, 0)", uuid.New(), id, sellerID, varID, stock)
		require.NoError(t, err)
	}
	return id
}

func TestMerchandisingMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	client, _, svc := setupTestDB(t)
	defer client.Close()
	ctx := context.Background()

	var colsToCleanup []uuid.UUID
	var prodsToCleanup []uuid.UUID
	var sellersToCleanup []uuid.UUID

	// Register cleanup BEFORE first mutation
	t.Cleanup(func() {
		cleanupTest(t, client, colsToCleanup, prodsToCleanup, sellersToCleanup)
	})

	// ==========================================
	// A. CREATE COLLECTION
	// ==========================================
	slugA := "col-" + uuid.NewString()[:8]
	colA, err := svc.CreateCollection(ctx, &merchandising.CreateCollectionRequest{
		Slug:     slugA,
		Title:    "Collection A",
		IsActive: true,
	})
	require.NoError(t, err, "A. create collection must succeed")
	colsToCleanup = append(colsToCleanup, colA.ID)
	require.Equal(t, slugA, colA.Slug)
	require.True(t, colA.IsActive)

	// ==========================================
	// B. DUPLICATE SLUG REJECTED
	// ==========================================
	_, err = svc.CreateCollection(ctx, &merchandising.CreateCollectionRequest{
		Slug:  slugA,
		Title: "Collection Duplicate Slug",
	})
	require.ErrorIs(t, err, merchandising.ErrDuplicateSlug, "B. duplicate slug must return ErrDuplicateSlug")

	// ==========================================
	// C. INVALID TIME RANGE (starts_at >= ends_at) REJECTED
	// ==========================================
	refTime := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	laterTime := refTime.Add(2 * time.Hour)
	_, err = svc.CreateCollection(ctx, &merchandising.CreateCollectionRequest{
		Slug:     "col-invalid-dates-" + uuid.NewString()[:8],
		Title:    "Invalid Dates",
		StartsAt: &laterTime,
		EndsAt:   &refTime,
	})
	require.ErrorIs(t, err, merchandising.ErrInvalidDates, "C. starts_at >= ends_at must return ErrInvalidDates")

	// ==========================================
	// D. UPDATE METADATA
	// ==========================================
	slugD := "col-updated-" + uuid.NewString()[:8]
	colD, err := svc.UpdateCollection(ctx, colA.ID, &merchandising.UpdateCollectionRequest{
		Slug:     slugD,
		Title:    "Updated Title",
		IsActive: true,
	})
	require.NoError(t, err, "D. update metadata must succeed")
	require.Equal(t, slugD, colD.Slug)
	require.Equal(t, "Updated Title", colD.Title)

	// ==========================================
	// E. DEACTIVATE COLLECTION
	// ==========================================
	colE, err := svc.UpdateCollection(ctx, colA.ID, &merchandising.UpdateCollectionRequest{
		Slug:     slugD,
		Title:    "Updated Title",
		IsActive: false,
	})
	require.NoError(t, err, "E. deactivation must succeed")
	require.False(t, colE.IsActive)

	// Reactivate for items testing
	_, err = svc.UpdateCollection(ctx, colA.ID, &merchandising.UpdateCollectionRequest{
		Slug:     slugD,
		Title:    "Updated Title",
		IsActive: true,
	})
	require.NoError(t, err)

	// Setup sellers and products
	sellerActive := createTestSeller(t, client, true)
	sellersToCleanup = append(sellersToCleanup, sellerActive)

	sellerBlocked := createTestSeller(t, client, false)
	sellersToCleanup = append(sellersToCleanup, sellerBlocked)

	pEligible1 := createTestProduct(t, client, sellerActive, "published", 10)
	pEligible2 := createTestProduct(t, client, sellerActive, "published", 10)
	pEligible3 := createTestProduct(t, client, sellerActive, "published", 10)
	pDraft := createTestProduct(t, client, sellerActive, "draft", 10)
	pHidden := createTestProduct(t, client, sellerActive, "hidden", 10)
	pBlocked := createTestProduct(t, client, sellerActive, "blocked", 10)
	pArchived := createTestProduct(t, client, sellerActive, "archived", 10)
	pInactiveSeller := createTestProduct(t, client, sellerBlocked, "published", 10)
	pLowStock := createTestProduct(t, client, sellerActive, "published", 1) // below MinStorefrontFreeSellableUnits (2)
	prodsToCleanup = append(prodsToCleanup,
		pEligible1, pEligible2, pEligible3,
		pDraft, pHidden, pBlocked, pArchived,
		pInactiveSeller, pLowStock,
	)

	// ==========================================
	// F. REPLACE ORDERED ITEM SET
	// K. INELIGIBLE PRODUCT CAN STILL BE ASSIGNED BY ADMIN
	// ==========================================
	initialItems := []uuid.UUID{pEligible1, pDraft, pHidden, pEligible2}
	err = svc.ReplaceCollectionItems(ctx, colA.ID, initialItems)
	require.NoError(t, err, "F & K. replace ordered item set with draft/hidden items must succeed")

	adminDetail, err := svc.GetAdminCollectionDetail(ctx, colA.ID)
	require.NoError(t, err)
	require.Len(t, adminDetail.Items, 4)
	require.Equal(t, pEligible1, adminDetail.Items[0].ProductID)
	require.Equal(t, pDraft, adminDetail.Items[1].ProductID)
	require.Equal(t, pHidden, adminDetail.Items[2].ProductID)
	require.Equal(t, pEligible2, adminDetail.Items[3].ProductID)

	// ==========================================
	// G. DUPLICATE PRODUCT ID REJECTED
	// ==========================================
	err = svc.ReplaceCollectionItems(ctx, colA.ID, []uuid.UUID{pEligible1, pEligible1})
	require.ErrorIs(t, err, merchandising.ErrDuplicateProductID, "G. duplicate product IDs must be rejected")

	// ==========================================
	// H. UNKNOWN PRODUCT ID ATOMIC REJECT
	// ==========================================
	nonExistentPID := uuid.New()
	err = svc.ReplaceCollectionItems(ctx, colA.ID, []uuid.UUID{pEligible1, nonExistentPID})
	require.ErrorIs(t, err, merchandising.ErrProductNotFound, "H. unknown product ID must reject whole replacement")
	// Verify old items unchanged
	adminDetailAfterH, err := svc.GetAdminCollectionDetail(ctx, colA.ID)
	require.NoError(t, err)
	require.Len(t, adminDetailAfterH.Items, 4, "Old items must remain untouched after failed replacement")

	// ==========================================
	// I. REORDER PRESERVES EXACT REQUESTED ORDER
	// ==========================================
	reorderedItems := []uuid.UUID{pEligible2, pEligible1, pHidden, pDraft}
	err = svc.ReplaceCollectionItems(ctx, colA.ID, reorderedItems)
	require.NoError(t, err)
	adminDetailI, err := svc.GetAdminCollectionDetail(ctx, colA.ID)
	require.NoError(t, err)
	require.Equal(t, pEligible2, adminDetailI.Items[0].ProductID)
	require.Equal(t, pEligible1, adminDetailI.Items[1].ProductID)
	require.Equal(t, pHidden, adminDetailI.Items[2].ProductID)
	require.Equal(t, pDraft, adminDetailI.Items[3].ProductID)

	// ==========================================
	// J. REMOVE ITEM VIA FULL-SET REPLACEMENT
	// ==========================================
	reducedItems := []uuid.UUID{pEligible1, pEligible2}
	err = svc.ReplaceCollectionItems(ctx, colA.ID, reducedItems)
	require.NoError(t, err)
	adminDetailJ, err := svc.GetAdminCollectionDetail(ctx, colA.ID)
	require.NoError(t, err)
	require.Len(t, adminDetailJ.Items, 2)
	require.Equal(t, pEligible1, adminDetailJ.Items[0].ProductID)
	require.Equal(t, pEligible2, adminDetailJ.Items[1].ProductID)

	// Setup full 9 items for comprehensive eligibility testing
	// Order: pEligible1, pHidden, pEligible2, pDraft, pBlocked, pArchived, pInactiveSeller, pLowStock, pEligible3
	fullSet := []uuid.UUID{
		pEligible1, pHidden, pEligible2, pDraft,
		pBlocked, pArchived, pInactiveSeller, pLowStock, pEligible3,
	}
	err = svc.ReplaceCollectionItems(ctx, colA.ID, fullSet)
	require.NoError(t, err)

	// ==========================================
	// L. ACTIVE COLLECTION APPEARS IN PUBLIC LIST
	// M. INACTIVE COLLECTION ABSENT
	// N. FUTURE COLLECTION ABSENT
	// O. EXPIRED COLLECTION ABSENT
	// ==========================================
	colInactive, err := svc.CreateCollection(ctx, &merchandising.CreateCollectionRequest{
		Slug:     "col-inactive-" + uuid.NewString()[:8],
		Title:    "Inactive Col",
		IsActive: false,
	})
	require.NoError(t, err)
	colsToCleanup = append(colsToCleanup, colInactive.ID)

	futureStart := refTime.Add(24 * time.Hour)
	colFuture, err := svc.CreateCollection(ctx, &merchandising.CreateCollectionRequest{
		Slug:     "col-future-" + uuid.NewString()[:8],
		Title:    "Future Col",
		IsActive: true,
		StartsAt: &futureStart,
	})
	require.NoError(t, err)
	colsToCleanup = append(colsToCleanup, colFuture.ID)

	pastStart := refTime.Add(-48 * time.Hour)
	pastEnd := refTime.Add(-24 * time.Hour)
	colExpired, err := svc.CreateCollection(ctx, &merchandising.CreateCollectionRequest{
		Slug:     "col-expired-" + uuid.NewString()[:8],
		Title:    "Expired Col",
		IsActive: true,
		StartsAt: &pastStart,
		EndsAt:   &pastEnd,
	})
	require.NoError(t, err)
	colsToCleanup = append(colsToCleanup, colExpired.ID)

	pubList, err := svc.ListPublicCollections(ctx, refTime)
	require.NoError(t, err)
	var foundActive, foundInactive, foundFuture, foundExpired bool
	for _, c := range pubList {
		if c.ID == colA.ID {
			foundActive = true
		}
		if c.ID == colInactive.ID {
			foundInactive = true
		}
		if c.ID == colFuture.ID {
			foundFuture = true
		}
		if c.ID == colExpired.ID {
			foundExpired = true
		}
	}
	require.True(t, foundActive, "L. active collection appears in public list")
	require.False(t, foundInactive, "M. inactive collection must be absent")
	require.False(t, foundFuture, "N. future collection must be absent")
	require.False(t, foundExpired, "O. expired collection must be absent")

	// ==========================================
	// EXACT TIME BOUNDARY ASSERTIONS
	// starts_at == asOf => ACTIVE
	// ends_at == asOf => INACTIVE
	// ==========================================
	exactStartCol, err := svc.CreateCollection(ctx, &merchandising.CreateCollectionRequest{
		Slug:     "col-exact-start-" + uuid.NewString()[:8],
		Title:    "Exact Start Col",
		IsActive: true,
		StartsAt: &refTime,
	})
	require.NoError(t, err)
	colsToCleanup = append(colsToCleanup, exactStartCol.ID)

	exactEndCol, err := svc.CreateCollection(ctx, &merchandising.CreateCollectionRequest{
		Slug:     "col-exact-end-" + uuid.NewString()[:8],
		Title:    "Exact End Col",
		IsActive: true,
		StartsAt: &pastStart,
		EndsAt:   &refTime,
	})
	require.NoError(t, err)
	colsToCleanup = append(colsToCleanup, exactEndCol.ID)

	boundaryList, err := svc.ListPublicCollections(ctx, refTime)
	require.NoError(t, err)
	var foundExactStart, foundExactEnd bool
	for _, c := range boundaryList {
		if c.ID == exactStartCol.ID {
			foundExactStart = true
		}
		if c.ID == exactEndCol.ID {
			foundExactEnd = true
		}
	}
	require.True(t, foundExactStart, "Boundary: starts_at == asOf MUST be active/visible")
	require.False(t, foundExactEnd, "Boundary: ends_at == asOf MUST be inactive/absent")

	exactStartDetail, err := svc.GetPublicCollectionDetail(ctx, exactStartCol.Slug, refTime)
	require.NoError(t, err)
	require.Equal(t, exactStartCol.ID, exactStartDetail.ID)

	_, err = svc.GetPublicCollectionDetail(ctx, exactEndCol.Slug, refTime)
	require.ErrorIs(t, err, merchandising.ErrCollectionNotFound, "Boundary: ends_at == asOf detail MUST be unavailable")

	// ==========================================
	// P. PUBLIC DETAIL RESOLVES BY SLUG
	// Q. INACTIVE/FUTURE/EXPIRED DETAIL UNAVAILABLE
	// ==========================================
	pubDetail, err := svc.GetPublicCollectionDetail(ctx, slugD, refTime)
	require.NoError(t, err, "P. active detail resolves by slug")
	require.Equal(t, colA.ID, pubDetail.ID)

	_, err = svc.GetPublicCollectionDetail(ctx, colInactive.Slug, refTime)
	require.ErrorIs(t, err, merchandising.ErrCollectionNotFound, "Q. inactive collection detail unavailable")
	_, err = svc.GetPublicCollectionDetail(ctx, colFuture.Slug, refTime)
	require.ErrorIs(t, err, merchandising.ErrCollectionNotFound, "Q. future collection detail unavailable")
	_, err = svc.GetPublicCollectionDetail(ctx, colExpired.Slug, refTime)
	require.ErrorIs(t, err, merchandising.ErrCollectionNotFound, "Q. expired collection detail unavailable")

	// ==========================================
	// R-X. STOREFRONT ELIGIBILITY FILTERING:
	// R: Published eligible products appear (pEligible1, pEligible2, pEligible3)
	// S: Draft product filtered out (pDraft)
	// T: Hidden product filtered out (pHidden)
	// U: Blocked product filtered out (pBlocked)
	// V: Archived product filtered out (pArchived)
	// W: Inactive seller product filtered out (pInactiveSeller)
	// X: Insufficient stock filtered out (pLowStock)
	// Z: Relative manual order preserved (pEligible1 -> pEligible2 -> pEligible3)
	// ==========================================
	require.Len(t, pubDetail.Products, 3, "Only the 3 eligible published products must appear")
	require.Equal(t, pEligible1, pubDetail.Products[0].ID, "R & Z. pEligible1 is first")
	require.Equal(t, pEligible2, pubDetail.Products[1].ID, "R & Z. pEligible2 is second")
	require.Equal(t, pEligible3, pubDetail.Products[2].ID, "R & Z. pEligible3 is third")

	// Verify filtered-out IDs are not present in public output
	for _, p := range pubDetail.Products {
		require.NotEqual(t, pDraft, p.ID, "S. draft product must be filtered out")
		require.NotEqual(t, pHidden, p.ID, "T. hidden product must be filtered out")
		require.NotEqual(t, pBlocked, p.ID, "U. blocked product must be filtered out")
		require.NotEqual(t, pArchived, p.ID, "V. archived product must be filtered out")
		require.NotEqual(t, pInactiveSeller, p.ID, "W. inactive seller product must be filtered out")
		require.NotEqual(t, pLowStock, p.ID, "X. insufficient stock product must be filtered out")
	}

	// ==========================================
	// ADMIN DETAIL: ALL ASSIGNED INELIGIBLE PRODUCTS REMAIN VISIBLE
	// ==========================================
	adminDetailFull, err := svc.GetAdminCollectionDetail(ctx, colA.ID)
	require.NoError(t, err)
	require.Len(t, adminDetailFull.Items, 9, "Admin detail MUST show all 9 assigned products including ineligible")

	// ==========================================
	// Y. PRODUCT BECOMING ELIGIBLE AGAIN REAPPEARS WITHOUT RELATION CHANGE
	// ==========================================
	// Make pHidden eligible again by transitioning status to 'published'
	_, err = client.Pool.Exec(ctx, "UPDATE products SET status = 'published' WHERE id = $1", pHidden)
	require.NoError(t, err)

	pubDetailReappear, err := svc.GetPublicCollectionDetail(ctx, slugD, refTime)
	require.NoError(t, err)
	require.Len(t, pubDetailReappear.Products, 4, "Y. 4 products must now be visible")

	// Check that pHidden reappears in its exact original relative position:
	// Full set was: pEligible1, pHidden, pEligible2, pDraft, pBlocked, pArchived, pInactiveSeller, pLowStock, pEligible3
	// Eligible are now: pEligible1, pHidden, pEligible2, pEligible3
	require.Equal(t, pEligible1, pubDetailReappear.Products[0].ID)
	require.Equal(t, pHidden, pubDetailReappear.Products[1].ID, "Y. pHidden must reappear in its original relative position")
	require.Equal(t, pEligible2, pubDetailReappear.Products[2].ID)
	require.Equal(t, pEligible3, pubDetailReappear.Products[3].ID)
}
