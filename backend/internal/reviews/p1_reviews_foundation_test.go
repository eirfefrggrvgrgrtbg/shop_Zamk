package reviews_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/reviews"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

type TestFixtures struct {
	Customer1ID        uuid.UUID
	Customer2ID        uuid.UUID
	SellerUserID       uuid.UUID
	AdminUserID        uuid.UUID
	SellerID           uuid.UUID
	ProductID          uuid.UUID
	ProductSlug        string
	VariantID          uuid.UUID
	DraftProductID     uuid.UUID
	DraftProductSlug   string
	Order1DeliveredID  uuid.UUID
	Order2DeliveredID  uuid.UUID
	Order3PaidID       uuid.UUID
	Order4ReturnedID   uuid.UUID
	OrderItem1ID       uuid.UUID
	OrderItem2ID       uuid.UUID
	OrderItem3PaidID   uuid.UUID
	OrderItem4ReturnID uuid.UUID
	Fulfillment1ID     uuid.UUID
	Fulfillment2ID     uuid.UUID
	Fulfillment3ID     uuid.UUID
	Fulfillment4ID     uuid.UUID
	Return4ID          uuid.UUID
	ReturnItem4ID      uuid.UUID
	Refund4ID          uuid.UUID
}

func setupReviewsTestDB(t *testing.T) (*postgres.Client, *reviews.Service, *reviews.Repository, *orders.Repository) {
	t.Helper()
	dsn := testutil.GetTestDatabaseURL()
	ctx := context.Background()
	client, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)

	// Hard database safety guard
	testutil.AssertTestDatabase(t, client.Pool)

	ordersRepo := orders.NewRepository(client.Pool)
	reviewsRepo := reviews.NewRepository(client)
	sellerRepo := sellers.NewRepository(client.Pool)
	reviewsSvc := reviews.NewService(reviewsRepo, ordersRepo, sellerRepo, client, nil)

	return client, reviewsSvc, reviewsRepo, ordersRepo
}

func createTestFixtures(t *testing.T, db *postgres.Client) TestFixtures {
	t.Helper()
	ctx := context.Background()

	f := TestFixtures{
		Customer1ID:        uuid.New(),
		Customer2ID:        uuid.New(),
		SellerUserID:       uuid.New(),
		AdminUserID:        uuid.New(),
		SellerID:           uuid.New(),
		ProductID:          uuid.New(),
		ProductSlug:        "test-prod-slug-" + uuid.New().String()[:8],
		VariantID:          uuid.New(),
		DraftProductID:     uuid.New(),
		DraftProductSlug:   "draft-prod-slug-" + uuid.New().String()[:8],
		Order1DeliveredID:  uuid.New(),
		Order2DeliveredID:  uuid.New(),
		Order3PaidID:       uuid.New(),
		Order4ReturnedID:   uuid.New(),
		OrderItem1ID:       uuid.New(),
		OrderItem2ID:       uuid.New(),
		OrderItem3PaidID:   uuid.New(),
		OrderItem4ReturnID: uuid.New(),
		Fulfillment1ID:     uuid.New(),
		Fulfillment2ID:     uuid.New(),
		Fulfillment3ID:     uuid.New(),
		Fulfillment4ID:     uuid.New(),
		Return4ID:          uuid.New(),
		ReturnItem4ID:      uuid.New(),
		Refund4ID:          uuid.New(),
	}

	// Register deterministic Cleanup in reverse FK order
	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM product_review_moderation_logs WHERE review_id IN (SELECT id FROM product_reviews WHERE product_id IN ($1, $2))`, f.ProductID, f.DraftProductID)
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM product_reviews WHERE product_id IN ($1, $2) OR user_id IN ($3, $4)`, f.ProductID, f.DraftProductID, f.Customer1ID, f.Customer2ID)
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM refunds WHERE id = $1`, f.Refund4ID)
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM return_items WHERE return_id = $1`, f.Return4ID)
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM returns WHERE id = $1`, f.Return4ID)
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM order_items WHERE id IN ($1, $2, $3, $4)`, f.OrderItem1ID, f.OrderItem2ID, f.OrderItem3PaidID, f.OrderItem4ReturnID)
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM order_fulfillments WHERE id IN ($1, $2, $3, $4)`, f.Fulfillment1ID, f.Fulfillment2ID, f.Fulfillment3ID, f.Fulfillment4ID)
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM orders WHERE id IN ($1, $2, $3, $4)`, f.Order1DeliveredID, f.Order2DeliveredID, f.Order3PaidID, f.Order4ReturnedID)
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM product_variants WHERE id = $1`, f.VariantID)
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM products WHERE id IN ($1, $2)`, f.ProductID, f.DraftProductID)
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM sellers WHERE id = $1`, f.SellerID)
		_, _ = db.Pool.Exec(cleanCtx, `DELETE FROM users WHERE id IN ($1, $2, $3, $4)`, f.Customer1ID, f.Customer2ID, f.SellerUserID, f.AdminUserID)

		// Verify zero leftover rows
		var count int
		_ = db.Pool.QueryRow(cleanCtx, `SELECT COUNT(*) FROM product_reviews WHERE product_id IN ($1, $2)`, f.ProductID, f.DraftProductID).Scan(&count)
		require.Equal(t, 0, count, "leaked product_reviews detected")
		_ = db.Pool.QueryRow(cleanCtx, `SELECT COUNT(*) FROM refunds WHERE id = $1`, f.Refund4ID).Scan(&count)
		require.Equal(t, 0, count, "leaked refunds detected")
		_ = db.Pool.QueryRow(cleanCtx, `SELECT COUNT(*) FROM returns WHERE id = $1`, f.Return4ID).Scan(&count)
		require.Equal(t, 0, count, "leaked returns detected")
		_ = db.Pool.QueryRow(cleanCtx, `SELECT COUNT(*) FROM order_items WHERE id IN ($1, $2, $3, $4)`, f.OrderItem1ID, f.OrderItem2ID, f.OrderItem3PaidID, f.OrderItem4ReturnID).Scan(&count)
		require.Equal(t, 0, count, "leaked order_items detected")
		_ = db.Pool.QueryRow(cleanCtx, `SELECT COUNT(*) FROM orders WHERE id IN ($1, $2, $3, $4)`, f.Order1DeliveredID, f.Order2DeliveredID, f.Order3PaidID, f.Order4ReturnedID).Scan(&count)
		require.Equal(t, 0, count, "leaked orders detected")
	})

	// Insert Users
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, status, first_name, name)
		VALUES ($1, $2, 'hash', 'customer', 'active', 'Alice', 'Alice Customer')
	`, f.Customer1ID, "c1-"+f.Customer1ID.String()+"@test.com")
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, status, first_name, name)
		VALUES ($1, $2, 'hash', 'customer', 'active', 'Bob', 'Bob Customer')
	`, f.Customer2ID, "c2-"+f.Customer2ID.String()+"@test.com")
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, status, name)
		VALUES ($1, $2, 'hash', 'seller', 'active', 'Seller User')
	`, f.SellerUserID, "s-"+f.SellerUserID.String()+"@test.com")
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, status, name)
		VALUES ($1, $2, 'hash', 'admin', 'active', 'Admin User')
	`, f.AdminUserID, "a-"+f.AdminUserID.String()+"@test.com")
	require.NoError(t, err)

	// Insert Seller
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES ($1, 'Acme Brand', $2, 'acme@test.com', 'active')
	`, f.SellerID, "slug-"+f.SellerID.String())
	require.NoError(t, err)

	// Insert Published Product & Variant
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, status, slug, description, price_cents)
		VALUES ($1, $2, 'Durable Running Shoes', 'published', $3, 'High quality shoes', 15000)
	`, f.ProductID, f.SellerID, f.ProductSlug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents, is_active)
		VALUES ($1, $2, 'SKU-RUN-42', 15000, true)
	`, f.VariantID, f.ProductID)
	require.NoError(t, err)

	// Insert Draft Product (Non-public)
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, status, slug, description, price_cents)
		VALUES ($1, $2, 'Draft Sneakers', 'draft', $3, 'Draft sneaker description', 20000)
	`, f.DraftProductID, f.SellerID, f.DraftProductSlug)
	require.NoError(t, err)

	// Order 1: Delivered (Customer 1)
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'delivered', 15000, 'RUB', 'Address 1', 'Courier', 0, 'Alice', 'alice@test.com', '+79001111111')
	`, f.Order1DeliveredID, f.Customer1ID, "ORD-1-"+f.Order1DeliveredID.String())
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, f.Fulfillment1ID, f.Order1DeliveredID, f.SellerID)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Durable Running Shoes (Black 42)', $7, '42', 'Black', 15000, 1, 15000)
	`, f.OrderItem1ID, f.Order1DeliveredID, f.Fulfillment1ID, f.ProductID, f.VariantID, f.SellerID, f.ProductSlug)
	require.NoError(t, err)

	// Order 2: Delivered (Customer 1, Second Purchase of same product)
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'delivered', 15000, 'RUB', 'Address 2', 'Courier', 0, 'Alice', 'alice@test.com', '+79001111111')
	`, f.Order2DeliveredID, f.Customer1ID, "ORD-2-"+f.Order2DeliveredID.String())
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, f.Fulfillment2ID, f.Order2DeliveredID, f.SellerID)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Durable Running Shoes (Black 42)', $7, '42', 'Black', 15000, 1, 15000)
	`, f.OrderItem2ID, f.Order2DeliveredID, f.Fulfillment2ID, f.ProductID, f.VariantID, f.SellerID, f.ProductSlug)
	require.NoError(t, err)

	// Order 3: Paid but NOT delivered (Customer 1)
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'paid', 15000, 'RUB', 'Address 3', 'Courier', 0, 'Alice', 'alice@test.com', '+79001111111')
	`, f.Order3PaidID, f.Customer1ID, "ORD-3-"+f.Order3PaidID.String())
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'paid')
	`, f.Fulfillment3ID, f.Order3PaidID, f.SellerID)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Durable Running Shoes (Black 42)', $7, '42', 'Black', 15000, 1, 15000)
	`, f.OrderItem3PaidID, f.Order3PaidID, f.Fulfillment3ID, f.ProductID, f.VariantID, f.SellerID, f.ProductSlug)
	require.NoError(t, err)

	// Order 4: Delivered, returned and refunded with real durable return & refund records
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'delivered', 15000, 'RUB', 'Address 4', 'Courier', 0, 'Alice', 'alice@test.com', '+79001111111')
	`, f.Order4ReturnedID, f.Customer1ID, "ORD-4-"+f.Order4ReturnedID.String())
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, f.Fulfillment4ID, f.Order4ReturnedID, f.SellerID)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, variant_size, variant_color, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Durable Running Shoes (Black 42)', $7, '42', 'Black', 15000, 1, 15000)
	`, f.OrderItem4ReturnID, f.Order4ReturnedID, f.Fulfillment4ID, f.ProductID, f.VariantID, f.SellerID, f.ProductSlug)
	require.NoError(t, err)

	// Real Return record
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at, completed_at)
		VALUES ($1, $2, $3, $4, 'refunded', 'defective', NOW(), NOW(), NOW())
	`, f.Return4ID, f.Order4ReturnedID, f.Fulfillment4ID, f.Customer1ID)
	require.NoError(t, err)

	// Real Return Item record
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, reason)
		VALUES ($1, $2, $3, 1, 1, 'defective')
	`, f.ReturnItem4ID, f.Return4ID, f.OrderItem4ReturnID)
	require.NoError(t, err)

	// Real Refund record
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO refunds (id, return_id, order_id, status, amount_cents, currency, processed_at)
		VALUES ($1, $2, $3, 'succeeded', 15000, 'RUB', NOW())
	`, f.Refund4ID, f.Return4ID, f.Order4ReturnedID)
	require.NoError(t, err)

	return f
}

func TestReviewsFoundation_DoubleRun_Suite(t *testing.T) {
	for run := 1; run <= 2; run++ {
		t.Run(fmt.Sprintf("Run_%d", run), func(t *testing.T) {
			db, svc, repo, _ := setupReviewsTestDB(t)
			defer db.Close()

			f := createTestFixtures(t, db)

			// 1. Delivered owner can review (status -> pending_moderation)
			reviewText := "Excellent shoes, very comfortable!"
			rev1, err := svc.CreateReview(context.Background(), f.Customer1ID, f.OrderItem1ID, nil, reviews.CreateReviewRequest{
				Rating: 5,
				Text:   &reviewText,
			})
			require.NoError(t, err)
			require.NotNil(t, rev1)
			require.Equal(t, "pending_moderation", rev1.Status)
			require.Equal(t, 5, rev1.Rating)
			require.Equal(t, &reviewText, rev1.Comment)
			require.Equal(t, f.ProductID, rev1.ProductID)
			require.Equal(t, &f.VariantID, rev1.ProductVariantID)
			require.Equal(t, f.SellerID, rev1.SellerID)
			require.Equal(t, "Durable Running Shoes (Black 42)", *rev1.ProductTitle)

			// 2. Duplicate review for the same order_item is rejected (ErrReviewAlreadyExists via constraint name check)
			_, err = svc.CreateReview(context.Background(), f.Customer1ID, f.OrderItem1ID, nil, reviews.CreateReviewRequest{
				Rating: 4,
			})
			require.ErrorIs(t, err, reviews.ErrReviewAlreadyExists)

			// 3. Same customer purchasing same product in a second delivered order CAN review that second order item
			rev2, err := svc.CreateReview(context.Background(), f.Customer1ID, f.OrderItem2ID, nil, reviews.CreateReviewRequest{
				Rating: 4,
			})
			require.NoError(t, err)
			require.NotNil(t, rev2)
			require.Equal(t, "pending_moderation", rev2.Status)

			// 4. Rating bounds checking
			_, err = svc.CreateReview(context.Background(), f.Customer1ID, f.OrderItem4ReturnID, nil, reviews.CreateReviewRequest{Rating: 0})
			require.ErrorIs(t, err, reviews.ErrInvalidRating)
			_, err = svc.CreateReview(context.Background(), f.Customer1ID, f.OrderItem4ReturnID, nil, reviews.CreateReviewRequest{Rating: 6})
			require.ErrorIs(t, err, reviews.ErrInvalidRating)

			// 5. Review text too long (> 1000 chars) rejected
			longText := strings.Repeat("a", 1001)
			_, err = svc.CreateReview(context.Background(), f.Customer1ID, f.OrderItem4ReturnID, nil, reviews.CreateReviewRequest{
				Rating: 5,
				Text:   &longText,
			})
			require.ErrorIs(t, err, reviews.ErrReviewTextTooLong)

			// 6. Foreign customer cannot review order item
			_, err = svc.CreateReview(context.Background(), f.Customer2ID, f.OrderItem4ReturnID, nil, reviews.CreateReviewRequest{Rating: 5})
			require.ErrorIs(t, err, reviews.ErrItemNotPurchased)

			// 7. Paid / not delivered cannot review
			_, err = svc.CreateReview(context.Background(), f.Customer1ID, f.OrderItem3PaidID, nil, reviews.CreateReviewRequest{Rating: 5})
			require.ErrorIs(t, err, reviews.ErrOrderNotDelivered)

			// 8. Real returned/refunded delivered purchase IS eligible
			rev4, err := svc.CreateReview(context.Background(), f.Customer1ID, f.OrderItem4ReturnID, nil, reviews.CreateReviewRequest{Rating: 5})
			require.NoError(t, err)
			require.NotNil(t, rev4)

			// 9. Legacy orderId verification in service: mismatched path orderId is rejected
			wrongOrderID := f.Order1DeliveredID
			_, err = svc.CreateReview(context.Background(), f.Customer1ID, f.OrderItem4ReturnID, &wrongOrderID, reviews.CreateReviewRequest{Rating: 5})
			require.ErrorIs(t, err, reviews.ErrItemNotPurchased)

			// 10. Moderation & Public View
			pub0, err := svc.GetPublicProductReviews(context.Background(), f.ProductID, 10, 0)
			require.NoError(t, err)
			require.Len(t, pub0, 0)

			// Moderate to published
			now := time.Now()
			err = repo.UpdateReviewStatus(context.Background(), nil, rev1.ID, "published", &now, nil, nil)
			require.NoError(t, err)
			err = repo.UpdateReviewStatus(context.Background(), nil, rev2.ID, "published", &now, nil, nil)
			require.NoError(t, err)

			// Public query returns both published reviews with snapshot fields
			pub, err := svc.GetPublicProductReviews(context.Background(), f.ProductID, 10, 0)
			require.NoError(t, err)
			require.Len(t, pub, 2)
			require.Equal(t, "Alice", pub[0].ReviewerFirstName)
			require.Equal(t, "Durable Running Shoes (Black 42)", pub[0].OrderItemTitle)
			require.Equal(t, "42", *pub[0].OrderItemSize)
			require.Equal(t, "Black", *pub[0].OrderItemColor)

			// Check Rating Summary
			summary, err := svc.GetRatingSummary(context.Background(), f.ProductID)
			require.NoError(t, err)
			require.Equal(t, 2, summary.Count)
			require.Equal(t, 4.5, summary.Average)
		})
	}
}

func TestReviewsFoundation_RealRouterSecurityAndContract(t *testing.T) {
	db, _, repo, _ := setupReviewsTestDB(t)
	defer db.Close()

	ctx := context.Background()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 60,
			RefreshTokenTTLDays:   7,
		},
		Auth:   config.AuthConfig{},
		App:    config.AppConfig{Env: "test"},
		Worker: config.WorkerConfig{MarketplaceCommissionBPS: 1500},
	}

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	router, cancel := app.BuildRouter(ctx, cfg, db, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)
	makeToken := func(userID uuid.UUID, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		require.NoError(t, err)
		return tok
	}

	f := createTestFixtures(t, db)

	// 1. Unauthenticated -> 401
	bodyJSON, _ := json.Marshal(map[string]any{
		"orderItemId": f.OrderItem1ID.String(),
		"rating":      5,
		"text":        "Great shoes!",
	})
	req := httptest.NewRequest("POST", "/api/customer/reviews", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// 2. Seller role attempting customer review -> 403
	sellerToken := makeToken(f.SellerUserID, string(users.RoleSeller))
	req = httptest.NewRequest("POST", "/api/customer/reviews", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+sellerToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	// 3. Admin role attempting customer review -> 403
	adminToken := makeToken(f.AdminUserID, string(users.RoleAdmin))
	req = httptest.NewRequest("POST", "/api/customer/reviews", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	// 4. Foreign customer attempting review -> 400 Bad Request
	cust2Token := makeToken(f.Customer2ID, string(users.RoleCustomer))
	req = httptest.NewRequest("POST", "/api/customer/reviews", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cust2Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// 5. Customer owner + delivered order item -> 201 Created (via new canonical route)
	cust1Token := makeToken(f.Customer1ID, string(users.RoleCustomer))
	req = httptest.NewRequest("POST", "/api/customer/reviews", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cust1Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var createdReview reviews.ReviewResponse
	err = json.Unmarshal(rec.Body.Bytes(), &createdReview)
	require.NoError(t, err)
	require.Equal(t, 5, createdReview.Rating)
	require.Equal(t, "pending_moderation", createdReview.Status)
	require.NotNil(t, createdReview.ProductTitle)
	require.Equal(t, "Durable Running Shoes (Black 42)", *createdReview.ProductTitle)

	// 6. Rating bounds: 0 -> 400, 6 -> 400, text > 1000 -> 400
	badRating0, _ := json.Marshal(map[string]any{"orderItemId": f.OrderItem2ID.String(), "rating": 0})
	req = httptest.NewRequest("POST", "/api/customer/reviews", bytes.NewReader(badRating0))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cust1Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	badRating6, _ := json.Marshal(map[string]any{"orderItemId": f.OrderItem2ID.String(), "rating": 6})
	req = httptest.NewRequest("POST", "/api/customer/reviews", bytes.NewReader(badRating6))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cust1Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	badTextLong, _ := json.Marshal(map[string]any{"orderItemId": f.OrderItem2ID.String(), "rating": 5, "text": strings.Repeat("x", 1005)})
	req = httptest.NewRequest("POST", "/api/customer/reviews", bytes.NewReader(badTextLong))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cust1Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// 7. Paid / not delivered order -> 400 Bad Request
	paidItemJSON, _ := json.Marshal(map[string]any{"orderItemId": f.OrderItem3PaidID.String(), "rating": 5})
	req = httptest.NewRequest("POST", "/api/customer/reviews", bytes.NewReader(paidItemJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cust1Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// 8. Real returned/refunded delivered purchase -> 201 Created
	returnedItemJSON, _ := json.Marshal(map[string]any{"orderItemId": f.OrderItem4ReturnID.String(), "rating": 5, "comment": "Refunded item review"})
	req = httptest.NewRequest("POST", "/api/customer/reviews", bytes.NewReader(returnedItemJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cust1Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// 9. Legacy route consistency:
	// A. Mismatched orderId from Order 1 with orderItemId from Order 2 -> 400 Bad Request
	bodyJSON2, _ := json.Marshal(map[string]any{
		"rating":  4,
		"comment": "Second order review via legacy route",
	})
	mismatchedURL := fmt.Sprintf("/api/customer/orders/%s/items/%s/review", f.Order1DeliveredID, f.OrderItem2ID)
	req = httptest.NewRequest("POST", mismatchedURL, bytes.NewReader(bodyJSON2))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cust1Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// B. Matching orderId + orderItemId -> 201 Created
	matchedURL := fmt.Sprintf("/api/customer/orders/%s/items/%s/review", f.Order2DeliveredID, f.OrderItem2ID)
	req = httptest.NewRequest("POST", matchedURL, bytes.NewReader(bodyJSON2))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cust1Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var createdReview2 reviews.ReviewResponse
	err = json.Unmarshal(rec.Body.Bytes(), &createdReview2)
	require.NoError(t, err)
	require.Equal(t, 4, createdReview2.Rating)

	// 10. Duplicate review on same orderItem -> 409 Conflict
	req = httptest.NewRequest("POST", "/api/customer/reviews", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cust1Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code)

	// 11. Customer reviews list: GET /api/me/reviews and GET /api/customer/reviews
	req = httptest.NewRequest("GET", "/api/me/reviews", nil)
	req.Header.Set("Authorization", "Bearer "+cust1Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var customerReviewsList reviews.ReviewListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &customerReviewsList)
	require.NoError(t, err)
	require.NotEmpty(t, customerReviewsList.Items)
	require.NotNil(t, customerReviewsList.Items[0].ProductTitle)
	require.Equal(t, "Durable Running Shoes (Black 42)", *customerReviewsList.Items[0].ProductTitle)
	require.NotNil(t, customerReviewsList.Items[0].Comment)

	// 12. Moderate reviews to published and set exact same created_at to prove deterministic id DESC tie-breaker
	sameCreatedAt := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	err = repo.UpdateReviewStatus(ctx, nil, createdReview.ID, "published", &sameCreatedAt, nil, nil)
	require.NoError(t, err)
	err = repo.UpdateReviewStatus(ctx, nil, createdReview2.ID, "published", &sameCreatedAt, nil, nil)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, "UPDATE product_reviews SET created_at = $1 WHERE id IN ($2, $3)", sameCreatedAt, createdReview.ID, createdReview2.ID)
	require.NoError(t, err)

	// 13. Public Product Visibility:
	// A. Published product by UUID -> 200 OK
	reqUUID := httptest.NewRequest("GET", fmt.Sprintf("/api/public/products/%s/reviews", f.ProductID), nil)
	recUUID := httptest.NewRecorder()
	router.ServeHTTP(recUUID, reqUUID)
	require.Equal(t, http.StatusOK, recUUID.Code)

	// B. Published product by Slug -> 200 OK
	reqSlug := httptest.NewRequest("GET", fmt.Sprintf("/api/public/products/%s/reviews", f.ProductSlug), nil)
	recSlug := httptest.NewRecorder()
	router.ServeHTTP(recSlug, reqSlug)
	require.Equal(t, http.StatusOK, recSlug.Code)

	// C. Non-published / Draft product by UUID -> 404 Not Found
	reqDraftUUID := httptest.NewRequest("GET", fmt.Sprintf("/api/public/products/%s/reviews", f.DraftProductID), nil)
	recDraftUUID := httptest.NewRecorder()
	router.ServeHTTP(recDraftUUID, reqDraftUUID)
	require.Equal(t, http.StatusNotFound, recDraftUUID.Code)

	// D. Non-published / Draft product by Slug -> 404 Not Found
	reqDraftSlug := httptest.NewRequest("GET", fmt.Sprintf("/api/public/products/%s/reviews", f.DraftProductSlug), nil)
	recDraftSlug := httptest.NewRecorder()
	router.ServeHTTP(recDraftSlug, reqDraftSlug)
	require.Equal(t, http.StatusNotFound, recDraftSlug.Code)

	// 14. Public Contract Assertions
	var pubList reviews.PublicReviewListResponse
	err = json.Unmarshal(recSlug.Body.Bytes(), &pubList)
	require.NoError(t, err)
	require.Equal(t, 2, pubList.ReviewCount)
	require.Equal(t, 4.5, pubList.AverageRating)
	require.Len(t, pubList.Items, 2)

	// Deterministic ordering assertion: created_at DESC, id DESC
	// When created_at timestamps are identical, the review with greater UUID (DB id DESC) MUST appear first
	var expectedFirstID uuid.UUID
	if strings.Compare(createdReview.ID.String(), createdReview2.ID.String()) > 0 {
		expectedFirstID = createdReview.ID
	} else {
		expectedFirstID = createdReview2.ID
	}
	require.Equal(t, expectedFirstID, pubList.Items[0].ID)
	require.True(t, pubList.Items[0].CreatedAt.Equal(sameCreatedAt))
	require.True(t, pubList.Items[1].CreatedAt.Equal(sameCreatedAt))

	// Also verify that when created_at differs, the newer timestamp appears first
	newerTime := sameCreatedAt.Add(1 * time.Hour)
	olderReviewID := pubList.Items[1].ID
	_, err = db.Pool.Exec(ctx, "UPDATE product_reviews SET created_at = $1 WHERE id = $2", newerTime, olderReviewID)
	require.NoError(t, err)

	recNewer := httptest.NewRecorder()
	router.ServeHTTP(recNewer, reqSlug)
	require.Equal(t, http.StatusOK, recNewer.Code)
	var pubListNewer reviews.PublicReviewListResponse
	err = json.Unmarshal(recNewer.Body.Bytes(), &pubListNewer)
	require.NoError(t, err)
	require.Equal(t, olderReviewID, pubListNewer.Items[0].ID)
	require.True(t, pubListNewer.Items[0].CreatedAt.After(pubListNewer.Items[1].CreatedAt))

	// Historical snapshot fields
	require.Equal(t, "Alice", pubList.Items[0].ReviewerDisplayName)
	require.Equal(t, "Durable Running Shoes (Black 42)", pubList.Items[0].ProductTitle)
	require.Equal(t, "42", *pubList.Items[0].VariantSize)
	require.Equal(t, "Black", *pubList.Items[0].VariantColor)

	// Automated JSON Security Contract Assertion: Ensure sensitive fields are absent
	var rawJSONMap map[string]any
	err = json.Unmarshal(recSlug.Body.Bytes(), &rawJSONMap)
	require.NoError(t, err)

	itemsRaw, ok := rawJSONMap["items"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, itemsRaw)

	forbiddenKeys := []string{
		"userId", "user_id", "email", "customerEmail", "customer_email",
		"phone", "customerPhone", "customer_phone",
		"orderId", "order_id", "orderNumber", "order_number",
		"sellerId", "seller_id",
	}

	for _, itemObj := range itemsRaw {
		itemMap, ok := itemObj.(map[string]any)
		require.True(t, ok)
		for _, forbiddenKey := range forbiddenKeys {
			_, found := itemMap[forbiddenKey]
			require.False(t, found, "Security violation: sensitive key %q exposed in public review item", forbiddenKey)
		}
	}
}
