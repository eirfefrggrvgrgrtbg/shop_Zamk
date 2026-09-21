package products_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestUpdateProductForSeller_BrandContract(t *testing.T) {
	dbClient, svc, userID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()
	
	// Get sellerID for the user
	var sellerID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT seller_id FROM seller_users WHERE user_id = $1", userID).Scan(&sellerID)
	require.NoError(t, err)

	// Brand A is the current primary brand created by setupBlockATestDB
	var brandA uuid.UUID
	err = pool.QueryRow(ctx, "SELECT brand_id FROM seller_brands WHERE seller_id = $1 AND is_primary = true", sellerID).Scan(&brandA)
	require.NoError(t, err)

	// Create Brand B and make it an allowed brand for the seller, but NOT primary
	brandB := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, 'Brand B', 'brand-b')", brandB)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO seller_brands (id, seller_id, brand_id, is_primary, relationship_type, status) VALUES ($1, $2, $3, false, 'owner', 'active')", uuid.New(), sellerID, brandB)
	require.NoError(t, err)

	// Brand C is NOT allowed
	brandC := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, 'Brand C', 'brand-c')", brandC)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM seller_brands WHERE brand_id IN ($1, $2)", brandB, brandC)
		pool.Exec(context.Background(), "DELETE FROM brands WHERE id IN ($1, $2)", brandB, brandC)
	})

	// Create a draft product with Brand A (implicitly, via CreateProductForSeller)
	title := "Test Brand Product"
	reqCreate := products.CreateProductRequest{Title: title}
	prod, err := svc.CreateProductForSeller(ctx, userID, reqCreate)
	require.NoError(t, err)
	require.Equal(t, &brandA, prod.BrandID)

	// 1. PATCH without brandId -> remains Brand A (even if we swap primary brand to B)
	_, err = pool.Exec(ctx, "UPDATE seller_brands SET is_primary = false WHERE seller_id = $1 AND brand_id = $2", sellerID, brandA)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE seller_brands SET is_primary = true WHERE seller_id = $1 AND brand_id = $2", sellerID, brandB)
	require.NoError(t, err)

	reqUpdateNoBrand := products.UpdateProductRequest{
		Description: func(s string) *string { return &s }("updated desc"),
	}
	updatedProd1, err := svc.UpdateProductForSeller(ctx, userID, prod.ID, reqUpdateNoBrand)
	require.NoError(t, err)
	require.Equal(t, &brandA, updatedProd1.BrandID, "Brand should remain Brand A when omitted from request")

	// Restore primary brand A for cleanup consistency
	_, err = pool.Exec(ctx, "UPDATE seller_brands SET is_primary = false WHERE seller_id = $1 AND brand_id = $2", sellerID, brandB)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE seller_brands SET is_primary = true WHERE seller_id = $1 AND brand_id = $2", sellerID, brandA)
	require.NoError(t, err)

	// 2. PATCH with valid explicit brandId B -> becomes Brand B
	reqUpdateBrandB := products.UpdateProductRequest{
		BrandID: &brandB,
	}
	updatedProd2, err := svc.UpdateProductForSeller(ctx, userID, prod.ID, reqUpdateBrandB)
	require.NoError(t, err)
	require.Equal(t, &brandB, updatedProd2.BrandID, "Brand should become Brand B")

	// 3. PATCH with unauthorized/invalid brandId C -> explicit error
	reqUpdateBrandC := products.UpdateProductRequest{
		BrandID: &brandC,
	}
	_, err = svc.UpdateProductForSeller(ctx, userID, prod.ID, reqUpdateBrandC)
	require.Error(t, err)
	require.Contains(t, err.Error(), "brand not authorized")

	// 4. Brand lifecycle tests:
	// 4a. Active seller_brand + INACTIVE brand (is_active = false) -> rejected
	brandD := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO brands (id, name, slug, is_active) VALUES ($1, 'Brand D Inactive', 'brand-d', false)", brandD)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO seller_brands (id, seller_id, brand_id, is_primary, relationship_type, status) VALUES ($1, $2, $3, false, 'owner', 'active')", uuid.New(), sellerID, brandD)
	require.NoError(t, err)

	reqUpdateBrandD := products.UpdateProductRequest{
		BrandID: &brandD,
	}
	_, err = svc.UpdateProductForSeller(ctx, userID, prod.ID, reqUpdateBrandD)
	require.Error(t, err, "active seller_brand + inactive brand must be rejected")
	require.Contains(t, err.Error(), "brand not authorized")

	// 4b. INACTIVE seller_brand (status = 'inactive') + ACTIVE brand (is_active = true) -> rejected
	brandE := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO brands (id, name, slug, is_active) VALUES ($1, 'Brand E Active', 'brand-e', true)", brandE)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO seller_brands (id, seller_id, brand_id, is_primary, relationship_type, status) VALUES ($1, $2, $3, false, 'owner', 'inactive')", uuid.New(), sellerID, brandE)
	require.NoError(t, err)

	reqUpdateBrandE := products.UpdateProductRequest{
		BrandID: &brandE,
	}
	_, err = svc.UpdateProductForSeller(ctx, userID, prod.ID, reqUpdateBrandE)
	require.Error(t, err, "inactive seller_brand + active brand must be rejected")
	require.Contains(t, err.Error(), "brand not authorized")

	// Clean up additional test brands
	defer func() {
		pool.Exec(context.Background(), "DELETE FROM seller_brands WHERE brand_id IN ($1, $2)", brandD, brandE)
		pool.Exec(context.Background(), "DELETE FROM brands WHERE id IN ($1, $2)", brandD, brandE)
	}()

	// Verify brand unchanged (remains Brand B)
	finalProd, _ := products.NewRepository(pool).GetProductByIDForSeller(ctx, prod.ID, sellerID)
	require.Equal(t, &brandB, finalProd.BrandID)
}

func TestUpdateProductForSeller_ModerationSafety(t *testing.T) {
	dbClient, svc, userID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Get sellerID
	var sellerID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT seller_id FROM seller_users WHERE user_id = $1", userID).Scan(&sellerID)
	require.NoError(t, err)

	// Create a valid admin user for in_review tests
	adminID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'admin', 'Test Admin')", adminID, fmt.Sprintf("test-admin-%s@test.com", adminID))
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", adminID)
	})

	// Helper to create and set product status
	createProdWithStatus := func(status string) products.Product {
		req := products.CreateProductRequest{Title: "Test Mod Product " + uuid.New().String()[:8]}
		prod, err := svc.CreateProductForSeller(ctx, userID, req)
		require.NoError(t, err)
		
		prod.Status = status
		if status == products.StatusPendingModeration || status == products.StatusInReview {
			now := time.Now()
			prod.SubmittedAt = &now
		}
		if status == products.StatusInReview {
			now := time.Now()
			prod.AssignedAdminUserID = &adminID
			prod.ReviewStartedAt = &now
		}
		
		query := `UPDATE products SET status = $1, submitted_at = $2, assigned_admin_user_id = $3, review_started_at = $4 WHERE id = $5`
		_, err = pool.Exec(ctx, query, prod.Status, prod.SubmittedAt, prod.AssignedAdminUserID, prod.ReviewStartedAt, prod.ID)
		require.NoError(t, err)
		
		p, _ := products.NewRepository(pool).GetProductByIDForSeller(ctx, prod.ID, sellerID)
		return *p
	}

	// 5. draft + PATCH -> remains draft
	prodDraft := createProdWithStatus(products.StatusDraft)
	updatedDraft, err := svc.UpdateProductForSeller(ctx, userID, prodDraft.ID, products.UpdateProductRequest{Title: func(s string) *string { return &s }("Updated Draft")})
	require.NoError(t, err)
	require.Equal(t, products.StatusDraft, updatedDraft.Status)

	// 6. rejected + PATCH -> remains rejected
	prodRejected := createProdWithStatus(products.StatusRejected)
	updatedRejected, err := svc.UpdateProductForSeller(ctx, userID, prodRejected.ID, products.UpdateProductRequest{Title: func(s string) *string { return &s }("Updated Rejected")})
	require.NoError(t, err)
	require.Equal(t, products.StatusRejected, updatedRejected.Status)

	// 7. pending_moderation + actual edit -> transitions to draft
	prodPending := createProdWithStatus(products.StatusPendingModeration)
	updatedPending, err := svc.UpdateProductForSeller(ctx, userID, prodPending.ID, products.UpdateProductRequest{Title: func(s string) *string { return &s }("Updated Pending")})
	require.NoError(t, err)
	require.Equal(t, products.StatusDraft, updatedPending.Status)
	require.Nil(t, updatedPending.SubmittedAt)

	// 8. in_review + actual edit -> transitions to draft, invalidates review
	prodReview := createProdWithStatus(products.StatusInReview)
	updatedReview, err := svc.UpdateProductForSeller(ctx, userID, prodReview.ID, products.UpdateProductRequest{Title: func(s string) *string { return &s }("Updated In Review")})
	require.NoError(t, err)
	require.Equal(t, products.StatusDraft, updatedReview.Status)
	require.Nil(t, updatedReview.SubmittedAt)
	require.Nil(t, updatedReview.AssignedAdminUserID)
	require.Nil(t, updatedReview.ReviewStartedAt)
	
	// Verify moderation log exists for in_review -> draft
	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM product_moderation_logs WHERE product_id = $1 AND to_status = 'draft'", prodReview.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// 9. blocked + PATCH -> still rejected as not editable
	prodBlocked := createProdWithStatus(products.StatusBlocked)
	_, err = svc.UpdateProductForSeller(ctx, userID, prodBlocked.ID, products.UpdateProductRequest{Title: func(s string) *string { return &s }("Updated Blocked")})
	require.ErrorIs(t, err, products.ErrProductNotEditable)

	// 10. published behavior regression (ensure it creates a revision, doesn't just mutate)
	prodPublished := createProdWithStatus(products.StatusPublished)
	updatedPublished, err := svc.UpdateProductForSeller(ctx, userID, prodPublished.ID, products.UpdateProductRequest{Title: func(s string) *string { return &s }("Updated Published")})
	require.NoError(t, err)
	// It should go to pending_moderation by default if continueSelling is not provided
	require.Equal(t, products.StatusPendingModeration, updatedPublished.Status)
	require.NotNil(t, updatedPublished.LiveRevisionID)
}
