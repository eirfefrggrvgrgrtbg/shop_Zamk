package returns_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	goredis "github.com/redis/go-redis/v9"
)

func TestM54C_ProductionGuard(t *testing.T) {
	fix := setupM51Fixture(t)

	prodHandler := returns.NewHandler(fix.svc, "production")
	r := chi.NewRouter()
	r.Post("/admin/returns/{id}/simulate-refund-success", prodHandler.SimulateRefundSuccess)
	r.Post("/admin/returns/{id}/simulate-refund-failure", prodHandler.SimulateRefundFailure)

	retID := uuid.New().String()

	// 1. Success endpoint blocked in production
	req1 := httptest.NewRequest(http.MethodPost, "/admin/returns/"+retID+"/simulate-refund-success", nil)
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusForbidden, rec1.Code)
	assert.Contains(t, rec1.Body.String(), "dev_tool_disabled")

	// 2. Failure endpoint blocked in production
	req2 := httptest.NewRequest(http.MethodPost, "/admin/returns/"+retID+"/simulate-refund-failure", nil)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusForbidden, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "dev_tool_disabled")
}

func TestM54C_PendingResolutionAndErrors(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// 1. Zero pending refunds -> ErrNoPendingRefund
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	retID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.svc.SimulateRefundSuccess(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrNoPendingRefund)

	_, err = fix.svc.SimulateRefundFailure(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrNoPendingRefund)

	// 2. Multiple pending refunds (fail-closed invariant) -> ErrMultiplePendingRefunds
	payID := createSucceededPayment(t, fix, tOrd.orderID, 10000)

	ref1ID := uuid.New()
	ref2ID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO refunds (id, return_id, payment_id, order_id, status, amount_cents, currency, provider, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'pending', 1000, 'RUB', 'mock', now(), now()),
		       ($5, $2, $3, $4, 'pending', 1000, 'RUB', 'mock', now(), now())
	`, ref1ID, retID, payID, tOrd.orderID, ref2ID)
	require.NoError(t, err)

	_, err = fix.svc.SimulateRefundSuccess(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrMultiplePendingRefunds)

	_, err = fix.svc.SimulateRefundFailure(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrMultiplePendingRefunds)
}

func TestM54C_SimulateRefundSuccess_Lifecycle_SellerFinance_Timeline_Idempotency(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	createSucceededPayment(t, fix, tOrd.orderID, 10000)

	earningID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', 3000, 'RUB', now(), '{}', now())
	`, earningID, fix.sellerAID, tOrd.orderID, tOrd.orderItemID)
	require.NoError(t, err)

	retID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 3, 3, now())
	`, uuid.New(), retID, tOrd.orderItemID)
	require.NoError(t, err)

	// Create pending refund reservation
	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	require.NotNil(t, ref)
	assert.Equal(t, "pending", ref.Status)

	// Verify before quote
	quoteBefore, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, int64(3000), quoteBefore.TotalRefundCents)
	assert.Equal(t, int64(3000), quoteBefore.PendingRefundCents)
	assert.Equal(t, int64(0), quoteBefore.SucceededRefundedCents)
	assert.Equal(t, int64(0), quoteBefore.AlreadyRefundedCents)
	assert.Equal(t, int64(0), quoteBefore.RemainingRefundableCents)
	require.NotNil(t, quoteBefore.LatestRefundStatus)
	assert.Equal(t, "pending", *quoteBefore.LatestRefundStatus)

	// Verify before Return status
	retBefore, err := fix.svc.GetAdminReturn(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "item_received", retBefore.Status)

	// Execute SimulateRefundSuccess
	succRef, err := fix.svc.SimulateRefundSuccess(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, succRef)
	assert.Equal(t, ref.ID, succRef.ID)
	assert.Equal(t, "succeeded", succRef.Status)
	assert.NotNil(t, succRef.ProcessedAt)

	// Verify Return status transitioned to 'refunded'
	retAfter, err := fix.svc.GetAdminReturn(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "refunded", retAfter.Status)

	// Verify quote after success
	quoteAfter, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, int64(3000), quoteAfter.TotalRefundCents)
	assert.Equal(t, int64(0), quoteAfter.PendingRefundCents)
	assert.Equal(t, int64(3000), quoteAfter.SucceededRefundedCents)
	assert.Equal(t, int64(3000), quoteAfter.AlreadyRefundedCents)
	assert.Equal(t, int64(0), quoteAfter.RemainingRefundableCents)
	assert.False(t, quoteAfter.CanRefund)
	require.NotNil(t, quoteAfter.LatestRefundStatus)
	assert.Equal(t, "succeeded", *quoteAfter.LatestRefundStatus)

	// Verify seller finance deduction applied exactly once (-3000)
	var deductionCount int
	var deductedAmount int64
	err = fix.client.Pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount_cents), 0)
		FROM seller_ledger_entries
		WHERE order_id = $1 AND type = 'adjustment'
	`, tOrd.orderID).Scan(&deductionCount, &deductedAmount)
	require.NoError(t, err)
	assert.Equal(t, 1, deductionCount, "Must have exactly 1 seller deduction entry")
	assert.Equal(t, int64(-3000), deductedAmount, "Must deduct -3000 cents (1000 x 3 accepted items)")

	// Verify Return Timeline includes 'return.refunded'
	timeline, err := fix.svc.GetAdminTimeline(ctx, retID)
	require.NoError(t, err)
	var hasRefundedEvent bool
	for _, ev := range timeline.Events {
		if ev.Type == "return.refunded" {
			hasRefundedEvent = true
			assert.Equal(t, "Возврат средств выполнен", ev.Title)
		}
	}
	assert.True(t, hasRefundedEvent, "Return Timeline must include return.refunded event")

	// Idempotency: second simulation call must be a safe no-op on seller finance (no double debit)
	_, err = fix.svc.SimulateRefundSuccess(ctx, retID)
	assert.ErrorIs(t, err, returns.ErrNoPendingRefund, "Once succeeded, there are 0 pending refunds left to simulate")

	var deductionCount2 int
	err = fix.client.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM seller_ledger_entries
		WHERE order_id = $1 AND type = 'adjustment'
	`, tOrd.orderID).Scan(&deductionCount2)
	require.NoError(t, err)
	assert.Equal(t, 1, deductionCount2, "Duplicate operations must not duplicate seller deduction entries")
}

func TestM54C_SimulateRefundFailure_Lifecycle_RetryAllowed_NoTimeline_NoDeduction(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 3)
	createSucceededPayment(t, fix, tOrd.orderID, 10000)

	earningID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'seller_earning', 3000, 'RUB', now(), '{}', now())
	`, earningID, fix.sellerAID, tOrd.orderID, tOrd.orderItemID)
	require.NoError(t, err)

	retID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'item_received', 'defective', now(), now())
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, created_at)
		VALUES ($1, $2, $3, 3, 3, now())
	`, uuid.New(), retID, tOrd.orderItemID)
	require.NoError(t, err)

	// Create pending refund reservation
	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	assert.Equal(t, "pending", ref.Status)

	// Execute SimulateRefundFailure
	failRef, err := fix.svc.SimulateRefundFailure(ctx, retID)
	require.NoError(t, err)
	require.NotNil(t, failRef)
	assert.Equal(t, ref.ID, failRef.ID)
	assert.Equal(t, "failed", failRef.Status)

	// Verify Return remains in 'item_received'
	retAfter, err := fix.svc.GetAdminReturn(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, "item_received", retAfter.Status)

	// Verify quote allows retry and has restored remaining capacity
	quoteAfter, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, int64(3000), quoteAfter.TotalRefundCents)
	assert.Equal(t, int64(0), quoteAfter.PendingRefundCents)
	assert.Equal(t, int64(0), quoteAfter.SucceededRefundedCents)
	assert.Equal(t, int64(0), quoteAfter.AlreadyRefundedCents)
	assert.Equal(t, int64(3000), quoteAfter.RemainingRefundableCents)
	assert.True(t, quoteAfter.CanRefund, "Failed refund must allow retry")
	require.NotNil(t, quoteAfter.LatestRefundStatus)
	assert.Equal(t, "failed", *quoteAfter.LatestRefundStatus)

	// Verify 0 seller deductions
	var deductionCount int
	err = fix.client.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM seller_ledger_entries
		WHERE order_id = $1 AND type = 'adjustment'
	`, tOrd.orderID).Scan(&deductionCount)
	require.NoError(t, err)
	assert.Equal(t, 0, deductionCount, "Failed refund must NOT create seller deduction")

	// Verify timeline does NOT include 'return.refunded'
	timeline, err := fix.svc.GetAdminTimeline(ctx, retID)
	require.NoError(t, err)
	for _, ev := range timeline.Events {
		assert.NotEqual(t, "return.refunded", ev.Type, "Failed refund must not emit return.refunded event")
	}

	// Retry creating refund (now pending again)
	ref2, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	assert.NotEqual(t, ref.ID, ref2.ID)
	assert.Equal(t, "pending", ref2.Status)

	quoteRetry, err := fix.svc.CalculateRefundQuote(ctx, retID)
	require.NoError(t, err)
	assert.Equal(t, int64(3000), quoteRetry.PendingRefundCents)
	assert.Equal(t, int64(0), quoteRetry.RemainingRefundableCents)
	assert.False(t, quoteRetry.CanRefund)
}

// -----------------------------------------------------------------------------
// REAL ROUTER RBAC TESTS (Production router & middleware stack)
// -----------------------------------------------------------------------------

func TestM54C_RealRouterRBAC_SimulateRefundSuccess(t *testing.T) {
	data := SetupRealRouterAuthFixture(t)
	ctx := context.Background()

	generateToken := func(userID uuid.UUID, role string) string {
		token, err := data.TokenService.GenerateAccessToken(userID, "test@test.com", role)
		require.NoError(t, err)
		return token
	}

	makeRequest := func(token string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest(http.MethodPost, "/api/admin/returns/"+data.ReturnID.String()+"/simulate-refund-success", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)
		return rr
	}

	t.Run("unauthenticated_401", func(t *testing.T) {
		rr := makeRequest("")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("customer_token_403", func(t *testing.T) {
		token := generateToken(data.CustomerID, "customer")
		rr := makeRequest(token)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("seller_token_403", func(t *testing.T) {
		token := generateToken(data.SellerID, "seller")
		rr := makeRequest(token)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("admin_without_refunds_create_permission_403", func(t *testing.T) {
		token := generateToken(data.AdminNoPermID, "admin")
		rr := makeRequest(token)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("admin_with_refunds_create_permission_executes_success", func(t *testing.T) {
		defer data.Pool.Exec(ctx, "DELETE FROM refunds WHERE return_id = $1", data.ReturnID)

		// Setup seller earning
		earningID := uuid.New()
		_, err := data.Pool.Exec(ctx, `
			INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, metadata, created_at)
			VALUES ($1, $2, $3, $4, 'seller_earning', 100000, 'RUB', now(), '{}', now())
		`, earningID, data.SellerProfileID, data.OrderID, data.OrderItemID)
		require.NoError(t, err)

		// Setup pending refund
		refID := uuid.New()
		_, err = data.Pool.Exec(ctx, `
			INSERT INTO refunds (id, return_id, payment_id, order_id, status, amount_cents, currency, provider, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'pending', 100000, 'RUB', 'tbank', now(), now())
		`, refID, data.ReturnID, data.PaymentID, data.OrderID)
		require.NoError(t, err)

		token := generateToken(data.AdminWithPermID, "admin")
		rr := makeRequest(token)
		require.Equal(t, http.StatusOK, rr.Code)

		var resp map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "succeeded", resp["status"])
		assert.NotNil(t, resp["processedAt"])

		// Verify DB status
		var dbRefStatus string
		var dbRetStatus string
		err = data.Pool.QueryRow(ctx, "SELECT status FROM refunds WHERE id = $1", refID).Scan(&dbRefStatus)
		require.NoError(t, err)
		assert.Equal(t, "succeeded", dbRefStatus)

		err = data.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", data.ReturnID).Scan(&dbRetStatus)
		require.NoError(t, err)
		assert.Equal(t, "refunded", dbRetStatus)

		// Verify seller deduction created in DB
		var deductionCount int
		var deductedAmount int64
		err = data.Pool.QueryRow(ctx, `
			SELECT COUNT(*), COALESCE(SUM(amount_cents), 0)
			FROM seller_ledger_entries
			WHERE order_id = $1 AND type = 'adjustment'
		`, data.OrderID).Scan(&deductionCount, &deductedAmount)
		require.NoError(t, err)
		assert.Equal(t, 1, deductionCount)
		assert.Equal(t, int64(-100000), deductedAmount)
	})
}

func TestM54C_RealRouterRBAC_SimulateRefundFailure(t *testing.T) {
	data := SetupRealRouterAuthFixture(t)
	ctx := context.Background()

	generateToken := func(userID uuid.UUID, role string) string {
		token, err := data.TokenService.GenerateAccessToken(userID, "test@test.com", role)
		require.NoError(t, err)
		return token
	}

	makeRequest := func(token string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest(http.MethodPost, "/api/admin/returns/"+data.ReturnID.String()+"/simulate-refund-failure", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		data.Router.ServeHTTP(rr, req)
		return rr
	}

	t.Run("unauthenticated_401", func(t *testing.T) {
		rr := makeRequest("")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("customer_token_403", func(t *testing.T) {
		token := generateToken(data.CustomerID, "customer")
		rr := makeRequest(token)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("seller_token_403", func(t *testing.T) {
		token := generateToken(data.SellerID, "seller")
		rr := makeRequest(token)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("admin_without_refunds_create_permission_403", func(t *testing.T) {
		token := generateToken(data.AdminNoPermID, "admin")
		rr := makeRequest(token)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("admin_with_refunds_create_permission_executes_failure", func(t *testing.T) {
		defer data.Pool.Exec(ctx, "DELETE FROM refunds WHERE return_id = $1", data.ReturnID)

		// Setup pending refund
		refID := uuid.New()
		_, err := data.Pool.Exec(ctx, `
			INSERT INTO refunds (id, return_id, payment_id, order_id, status, amount_cents, currency, provider, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'pending', 100000, 'RUB', 'tbank', now(), now())
		`, refID, data.ReturnID, data.PaymentID, data.OrderID)
		require.NoError(t, err)

		token := generateToken(data.AdminWithPermID, "admin")
		rr := makeRequest(token)
		require.Equal(t, http.StatusOK, rr.Code)

		var resp map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "failed", resp["status"])

		// Verify DB status: refund failed, return remains item_received
		var dbRefStatus string
		var dbRetStatus string
		err = data.Pool.QueryRow(ctx, "SELECT status FROM refunds WHERE id = $1", refID).Scan(&dbRefStatus)
		require.NoError(t, err)
		assert.Equal(t, "failed", dbRefStatus)

		err = data.Pool.QueryRow(ctx, "SELECT status FROM returns WHERE id = $1", data.ReturnID).Scan(&dbRetStatus)
		require.NoError(t, err)
		assert.Equal(t, "item_received", dbRetStatus)

		// Verify 0 seller deductions created
		var deductionCount int
		err = data.Pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM seller_ledger_entries
			WHERE order_id = $1 AND type = 'adjustment'
		`, data.OrderID).Scan(&deductionCount)
		require.NoError(t, err)
		assert.Equal(t, 0, deductionCount)
	})
}

func TestM54C_RealRouterRBAC_ProductionGuardComposition(t *testing.T) {
	dbURL := testutil.GetTestDatabaseURL()
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	testutil.AssertTestDatabase(t, pool)
	defer pool.Close()

	// Build router with appEnv = "production"
	cfg := &config.Config{}
	cfg.App.Env = "production"
	cfg.JWT.AccessTokenSecret = "test-secret"
	cfg.JWT.AccessTokenTTLMinutes = 60
	cfg.RateLimit.Enabled = false
	cfg.Worker.ReturnWindowDays = 14

	pgClient := &postgres.Client{Pool: pool}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dummyRedis := &redis.Client{Client: goredis.NewClient(&goredis.Options{})}

	prodRouter, cancel := app.BuildRouter(context.Background(), cfg, pgClient, dummyRedis, logger)
	defer cancel()

	tokenService := auth.NewTokenService(cfg.JWT.AccessTokenSecret, cfg.JWT.RefreshTokenSecret, cfg.JWT.AccessTokenTTLMinutes)

	// Create admin with refunds.create permission
	ctx := context.Background()
	adminID := uuid.New()
	roleID := uuid.New()
	now := time.Now()

	_, err = pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at) VALUES ($1, 'Prod Admin', $2, 'hash', 'admin', $3, $3)", adminID, adminID.String()+"@test.com", now)
	require.NoError(t, err)
	defer pool.Exec(ctx, "DELETE FROM users WHERE id = $1", adminID)

	_, err = pool.Exec(ctx, "INSERT INTO staff_roles (id, code, name, description, is_system) VALUES ($1, $2, 'Admin Role', '', false)", roleID, "role_"+roleID.String()[:8])
	require.NoError(t, err)
	defer pool.Exec(ctx, "DELETE FROM staff_roles WHERE id = $1", roleID)

	_, err = pool.Exec(ctx, "INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, 'refunds.create')", roleID)
	require.NoError(t, err)
	defer pool.Exec(ctx, "DELETE FROM staff_role_permissions WHERE role_id = $1", roleID)

	_, err = pool.Exec(ctx, "INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at) VALUES ($1, $2, 'active', $3, $3)", adminID, roleID, now)
	require.NoError(t, err)
	defer pool.Exec(ctx, "DELETE FROM staff_members WHERE user_id = $1", adminID)

	// Create test Return and pending refund
	orderID := uuid.New()
	sellerID := uuid.New()
	fulID := uuid.New()
	payID := uuid.New()
	retID := uuid.New()
	refID := uuid.New()

	_, err = pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, 'Test Seller', $2, 'test@test.com', 'active', $3, $3)", sellerID, "slug-"+sellerID.String(), now)
	require.NoError(t, err)
	defer pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)

	_, err = pool.Exec(ctx, "INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at) VALUES ($1, $2, $3, 'awaiting_payment', 100000, 'RUB', 'test', 'Delivery', 0, 'test', 'test', 'test', $4, $4)", orderID, adminID, "ORD-"+orderID.String()[:8], now)
	require.NoError(t, err)
	defer pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", orderID)

	_, err = pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at) VALUES ($1, $2, $3, 'delivered', $4, $4)", fulID, orderID, sellerID, now)
	require.NoError(t, err)
	defer pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE id = $1", fulID)

	_, err = pool.Exec(ctx, "INSERT INTO payments (id, order_id, amount_cents, currency, status, provider, idempotency_key, created_at, updated_at) VALUES ($1, $2, 100000, 'RUB', 'succeeded', 'tbank', $3, $4, $4)", payID, orderID, uuid.New().String(), now)
	require.NoError(t, err)
	defer pool.Exec(ctx, "DELETE FROM payments WHERE id = $1", payID)

	_, err = pool.Exec(ctx, "INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at) VALUES ($1, $2, $3, $4, 'item_received', 'defective', $5, $5)", retID, orderID, fulID, adminID, now)
	require.NoError(t, err)
	defer pool.Exec(ctx, "DELETE FROM returns WHERE id = $1", retID)

	_, err = pool.Exec(ctx, `
		INSERT INTO refunds (id, return_id, payment_id, order_id, status, amount_cents, currency, provider, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'pending', 100000, 'RUB', 'tbank', now(), now())
	`, refID, retID, payID, orderID)
	require.NoError(t, err)
	defer pool.Exec(ctx, "DELETE FROM refunds WHERE id = $1", refID)

	adminToken, err := tokenService.GenerateAccessToken(adminID, "admin@test.com", "admin")
	require.NoError(t, err)

	// 1. Success endpoint on production router with authorized token -> 403 dev_tool_disabled
	req1, _ := http.NewRequest(http.MethodPost, "/api/admin/returns/"+retID.String()+"/simulate-refund-success", nil)
	req1.Header.Set("Authorization", "Bearer "+adminToken)
	rr1 := httptest.NewRecorder()
	prodRouter.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusForbidden, rr1.Code)
	assert.Contains(t, rr1.Body.String(), "dev_tool_disabled")

	// 2. Failure endpoint on production router with authorized token -> 403 dev_tool_disabled
	req2, _ := http.NewRequest(http.MethodPost, "/api/admin/returns/"+retID.String()+"/simulate-refund-failure", nil)
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	rr2 := httptest.NewRecorder()
	prodRouter.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusForbidden, rr2.Code)
	assert.Contains(t, rr2.Body.String(), "dev_tool_disabled")

	// Verify refund row remained 'pending' (composition prevented execution)
	var dbStatus string
	err = pool.QueryRow(ctx, "SELECT status FROM refunds WHERE id = $1", refID).Scan(&dbStatus)
	require.NoError(t, err)
	assert.Equal(t, "pending", dbStatus)
}
