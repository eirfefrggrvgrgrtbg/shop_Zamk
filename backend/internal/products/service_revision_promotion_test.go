package products_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type revisionPromotionFixture struct {
	pool       *pgxpool.Pool
	svc        *products.Service
	prodRepo   *products.Repository
	sellerID   uuid.UUID
	sellerUser uuid.UUID
	adminUser  uuid.UUID
	categoryID uuid.UUID
	productID  uuid.UUID
}

func setupRevisionPromotionFixture(t *testing.T) revisionPromotionFixture {
	t.Helper()
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	testutil.AssertTestDatabase(t, pool)

	sellerID := uuid.New()
	brandName := fmt.Sprintf("Brand-%s", sellerID.String()[:8])
	sellerSlug := fmt.Sprintf("brand-slug-%s", sellerID.String()[:8])
	_, err = pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', now(), now())
	`, sellerID, brandName, sellerSlug, "test@test.com")
	require.NoError(t, err)

	sellerUser := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'seller', 'Seller User', now(), now())
	`, sellerUser, fmt.Sprintf("seller-%s@test.com", sellerUser.String()[:8]))
	require.NoError(t, err)

	adminUser := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, name, created_at, updated_at)
		VALUES ($1, $2, 'hash', 'admin', 'Admin User', now(), now())
	`, adminUser, fmt.Sprintf("admin-%s@test.com", adminUser.String()[:8]))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, 'owner', now())
	`, uuid.New(), sellerID, sellerUser)
	require.NoError(t, err)

	categoryID := uuid.New()
	catSlug := fmt.Sprintf("cat-%s", categoryID.String()[:8])
	_, err = pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Coats', $2, true, now(), now())
	`, categoryID, catSlug)
	require.NoError(t, err)

	productID := uuid.New()
	prodSlug := fmt.Sprintf("prod-%s", productID.String()[:8])
	_, err = pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, description, status, source, currency, price_cents, vision_content_version, created_at, updated_at)
		VALUES ($1, $2, $3, 'Wool Coat V1', $4, 'Warm wool coat', 'published', 'seller', 'RUB', 25000, 5, now(), now())
	`, productID, sellerID, categoryID, prodSlug)
	require.NoError(t, err)

	prodRepo := products.NewRepository(pool)
	sellerRepo := sellers.NewRepository(pool)
	dbClient := &postgres.Client{Pool: pool}
	svc := products.NewService(prodRepo, sellerRepo, dbClient, nil, nil)

	return revisionPromotionFixture{
		pool:       pool,
		svc:        svc,
		prodRepo:   prodRepo,
		sellerID:   sellerID,
		sellerUser: sellerUser,
		adminUser:  adminUser,
		categoryID: categoryID,
		productID:  productID,
	}
}

func TestProductsService_ApproveProduct_RevisionPromotion_Success(t *testing.T) {
	fix := setupRevisionPromotionFixture(t)
	defer fix.pool.Close()
	ctx := context.Background()

	// Initial version is 5
	var verBefore int64
	var titleBefore string
	err := fix.pool.QueryRow(ctx, "SELECT vision_content_version, title FROM products WHERE id = $1", fix.productID).Scan(&verBefore, &titleBefore)
	require.NoError(t, err)
	require.Equal(t, int64(5), verBefore)
	require.Equal(t, "Wool Coat V1", titleBefore)

	// Seller edits with ContinueSelling=true -> creates pending revision
	newTitle := "Wool Coat V2 - Luxury Edition"
	contSelling := true
	updReq := products.UpdateProductRequest{
		Title:           &newTitle,
		ContinueSelling: &contSelling,
	}
	updProd, err := fix.svc.UpdateProductForSeller(ctx, fix.sellerUser, fix.productID, updReq)
	require.NoError(t, err)
	require.NotNil(t, updProd.LiveRevisionID)

	// Live product unchanged before approval
	var verDuring int64
	var titleDuring string
	err = fix.pool.QueryRow(ctx, "SELECT vision_content_version, title FROM products WHERE id = $1", fix.productID).Scan(&verDuring, &titleDuring)
	require.NoError(t, err)
	require.Equal(t, int64(5), verDuring, "Pending revision must not bump version")
	require.Equal(t, "Wool Coat V1", titleDuring, "Live title must not change before approval")

	// Admin approves via canonical service method
	comment := "Approved promotion"
	err = fix.svc.ApproveProduct(ctx, fix.adminUser, fix.productID, &comment)
	require.NoError(t, err)

	// Live product must now be V2, version must be exactly 6 (N+1), revision approved
	var verAfter int64
	var titleAfter string
	var statusAfter string
	err = fix.pool.QueryRow(ctx, "SELECT vision_content_version, title, status FROM products WHERE id = $1", fix.productID).Scan(&verAfter, &titleAfter, &statusAfter)
	require.NoError(t, err)
	require.Equal(t, int64(6), verAfter, "Version must be exactly N+1 (6)")
	require.Equal(t, newTitle, titleAfter, "Live product title must be updated")
	require.Equal(t, products.StatusPublished, statusAfter)

	var revStatus string
	err = fix.pool.QueryRow(ctx, "SELECT status FROM product_revisions WHERE id = $1", *updProd.LiveRevisionID).Scan(&revStatus)
	require.NoError(t, err)
	require.Equal(t, "approved", revStatus)
}

func TestProductsService_ApproveProduct_PublishedWithoutRevision_Fails(t *testing.T) {
	fix := setupRevisionPromotionFixture(t)
	defer fix.pool.Close()
	ctx := context.Background()

	// Product is published with no pending revision
	err := fix.svc.ApproveProduct(ctx, fix.adminUser, fix.productID, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, products.ErrInvalidStatusTransition)
}

func TestProductsService_RejectProduct_PendingRevision_KeepsLiveProduct(t *testing.T) {
	fix := setupRevisionPromotionFixture(t)
	defer fix.pool.Close()
	ctx := context.Background()

	// Seller edits with ContinueSelling=true -> creates pending revision
	badTitle := "Misleading Wool Coat"
	contSelling := true
	updReq := products.UpdateProductRequest{
		Title:           &badTitle,
		ContinueSelling: &contSelling,
	}
	updProd, err := fix.svc.UpdateProductForSeller(ctx, fix.sellerUser, fix.productID, updReq)
	require.NoError(t, err)
	require.NotNil(t, updProd.LiveRevisionID)

	// Admin rejects via canonical service method
	rejectionReason := "Policy violation"
	err = fix.svc.RejectProduct(ctx, fix.adminUser, fix.productID, rejectionReason)
	require.NoError(t, err)

	// Live product must remain published with original title and unchanged version (5)
	var verAfter int64
	var titleAfter string
	var statusAfter string
	err = fix.pool.QueryRow(ctx, "SELECT vision_content_version, title, status FROM products WHERE id = $1", fix.productID).Scan(&verAfter, &titleAfter, &statusAfter)
	require.NoError(t, err)
	require.Equal(t, int64(5), verAfter, "Rejection must NOT bump vision_content_version")
	require.Equal(t, "Wool Coat V1", titleAfter, "Rejection must NOT alter live content")
	require.Equal(t, products.StatusPublished, statusAfter, "Live product remains published")

	var revStatus string
	err = fix.pool.QueryRow(ctx, "SELECT status FROM product_revisions WHERE id = $1", *updProd.LiveRevisionID).Scan(&revStatus)
	require.NoError(t, err)
	require.Equal(t, "rejected", revStatus, "Revision must be marked rejected")
}
