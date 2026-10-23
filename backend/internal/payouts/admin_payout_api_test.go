package payouts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type adminPayoutTestFixture struct {
	pool         *pgxpool.Pool
	client       *postgres.Client
	router       http.Handler
	tokenService *auth.TokenService
	adminToken        string
	updateOnlyToken   string
	markPaidOnlyToken string
	noPermToken       string
	custToken         string
	adminID           uuid.UUID
	updateOnlyAdminID uuid.UUID
	markPaidOnlyID    uuid.UUID
	noPermAdmin       uuid.UUID
	sellerAID         uuid.UUID
	sellerBID         uuid.UUID
	payoutSvc         *payouts.Service
}

func setupAdminPayoutFixture(t *testing.T) *adminPayoutTestFixture {
	dsn := testutil.DisposableDatabaseURL(t)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	testutil.AssertTestDatabase(t, pool)

	require.NoError(t, testutil.EnsureCanonicalStarterTaxonomy(ctx, pool))
	testutil.MustBeDisposableDatabase(t, pool)
	_, _ = pool.Exec(ctx, "TRUNCATE seller_ledger_entries CASCADE")
	_, _ = pool.Exec(ctx, "TRUNCATE payout_batches CASCADE")
	_, _ = pool.Exec(ctx, "TRUNCATE seller_commission_rules CASCADE")

	client := &postgres.Client{Pool: pool}

	cfg := &config.Config{}
	cfg.JWT.AccessTokenSecret = "test-secret-key-12345"
	cfg.JWT.AccessTokenTTLMinutes = 60
	cfg.RateLimit.Enabled = false

	dummyRedis := &redis.Client{Client: goredis.NewClient(&goredis.Options{})}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r, cancel := app.BuildRouter(ctx, cfg, client, dummyRedis, logger)
	t.Cleanup(cancel)
	t.Cleanup(func() { pool.Close() })

	tokenService := auth.NewTokenService(cfg.JWT.AccessTokenSecret, "refresh-secret", cfg.JWT.AccessTokenTTLMinutes)

	now := time.Now()
	adminID := uuid.New()
	updateOnlyAdminID := uuid.New()
	markPaidOnlyID := uuid.New()
	noPermAdminID := uuid.New()
	custID := uuid.New()

	// Users
	_, err = pool.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at) VALUES
		($1, 'Admin Perm', $2, 'hash', 'admin', $3, $3),
		($4, 'Admin UpdateOnly', $5, 'hash', 'admin', $3, $3),
		($6, 'Admin MarkPaidOnly', $7, 'hash', 'admin', $3, $3),
		($8, 'Admin NoPerm', $9, 'hash', 'admin', $3, $3),
		($10, 'Customer', $11, 'hash', 'customer', $3, $3)`,
		adminID, adminID.String()+"@admin.local", now,
		updateOnlyAdminID, updateOnlyAdminID.String()+"@admin.local",
		markPaidOnlyID, markPaidOnlyID.String()+"@admin.local",
		noPermAdminID, noPermAdminID.String()+"@admin.local",
		custID, custID.String()+"@cust.local",
	)
	require.NoError(t, err)

	// Roles & Staff Authorization
	roleWithPermID := uuid.New()
	roleUpdateOnlyID := uuid.New()
	roleMarkPaidOnlyID := uuid.New()
	roleNoPermID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name, description, is_system) VALUES
		($1, $2, 'Finance Master', '', false),
		($3, $4, 'Update Only Role', '', false),
		($5, $6, 'Mark Paid Only Role', '', false),
		($7, $8, 'Restricted', '', false)`,
		roleWithPermID, "perm_"+uuid.New().String()[:8],
		roleUpdateOnlyID, "upd_"+uuid.New().String()[:8],
		roleMarkPaidOnlyID, "mp_"+uuid.New().String()[:8],
		roleNoPermID, "noperm_"+uuid.New().String()[:8],
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at) VALUES
		($1, $2, 'active', $9, $9),
		($3, $4, 'active', $9, $9),
		($5, $6, 'active', $9, $9),
		($7, $8, 'active', $9, $9)`,
		adminID, roleWithPermID,
		updateOnlyAdminID, roleUpdateOnlyID,
		markPaidOnlyID, roleMarkPaidOnlyID,
		noPermAdminID, roleNoPermID,
		now,
	)
	require.NoError(t, err)

	require.NoError(t, testutil.GrantStaffAuthorizationState(ctx, pool, adminID, roleWithPermID,
		"payouts.read", "payouts.create", "payouts.mark_paid", "payouts.update", "payouts.approve",
	))
	require.NoError(t, testutil.GrantStaffAuthorizationState(ctx, pool, updateOnlyAdminID, roleUpdateOnlyID,
		"payouts.read", "payouts.update",
	))
	require.NoError(t, testutil.GrantStaffAuthorizationState(ctx, pool, markPaidOnlyID, roleMarkPaidOnlyID,
		"payouts.read", "payouts.mark_paid",
	))
	require.NoError(t, testutil.GrantStaffAuthorizationState(ctx, pool, noPermAdminID, roleNoPermID))

	// Sellers
	sellerAID := uuid.New()
	sellerBID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES
		($1, 'Brand Alpha', $2, 'alpha@sellers.local', 'active', $3, $3),
		($4, 'Brand Beta', $5, 'beta@sellers.local', 'active', $3, $3)`,
		sellerAID, "alpha-"+sellerAID.String()[:8], now,
		sellerBID, "beta-"+sellerBID.String()[:8],
	)
	require.NoError(t, err)

	adminToken, _ := tokenService.GenerateAccessToken(adminID, "admin@test.local", "admin")
	updateOnlyToken, _ := tokenService.GenerateAccessToken(updateOnlyAdminID, "updateonly@test.local", "admin")
	markPaidOnlyToken, _ := tokenService.GenerateAccessToken(markPaidOnlyID, "markpaidonly@test.local", "admin")
	noPermToken, _ := tokenService.GenerateAccessToken(noPermAdminID, "noperm@test.local", "admin")
	custToken, _ := tokenService.GenerateAccessToken(custID, "cust@test.local", "customer")

	payoutRepo := payouts.NewRepository(pool)
	ordersRepo := orders.NewRepository(pool)
	svc := payouts.NewService(payoutRepo, client, nil, ordersRepo, cfg, nil)

	return &adminPayoutTestFixture{
		pool:              pool,
		client:            client,
		router:            r,
		tokenService:      tokenService,
		adminToken:        adminToken,
		updateOnlyToken:   updateOnlyToken,
		markPaidOnlyToken: markPaidOnlyToken,
		noPermToken:       noPermToken,
		custToken:         custToken,
		adminID:           adminID,
		updateOnlyAdminID: updateOnlyAdminID,
		markPaidOnlyID:    markPaidOnlyID,
		noPermAdmin:       noPermAdminID,
		sellerAID:         sellerAID,
		sellerBID:         sellerBID,
		payoutSvc:         svc,
	}
}

func setupTestOrder(t *testing.T, pool *pgxpool.Pool, sellerID uuid.UUID, priceCents int64) (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	customerID := uuid.New()
	_, _ = pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role) VALUES ($1, 'Customer', $2, 'hash', 'customer')", customerID, customerID.String()+"@cust.local")
	catID := uuid.New()
	_, _ = pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', $2)", catID, catID.String())
	prodID := uuid.New()
	_, _ = pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, description, status, price_cents) VALUES ($1, $2, $3, 'Silk Dress', $4, 'desc', 'published', $5)", prodID, sellerID, catID, prodID.String(), priceCents)
	varID := uuid.New()
	_, _ = pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents) VALUES ($1, $2, 'SKU', $3)", varID, prodID, priceCents)
	orderID := uuid.New()
	_, _ = pool.Exec(ctx, "INSERT INTO orders (id, user_id, order_number, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, $3, 'awaiting_payment', $4, 'Cust', '123', 'c@c.com', 'addr')", orderID, customerID, "ZMK-100001", priceCents)
	fullID := uuid.New()
	_, _ = pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, commission_bps) VALUES ($1, $2, $3, 'paid', 1000)", fullID, orderID, sellerID)
	itemID := uuid.New()
	_, _ = pool.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Silk Dress', 'slug', $6, 1, $6, $7)", itemID, orderID, prodID, varID, sellerID, priceCents, fullID)
	return orderID, customerID, itemID, fullID
}

func doJSONReq(handler http.Handler, method, target, token string, body any) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, bodyReader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// 1. Empty list must return 200 with items: [] and totalCount: 0
func TestAdminPayouts_EmptyList(t *testing.T) {
	f := setupAdminPayoutFixture(t)

	rr := doJSONReq(f.router, "GET", "/api/admin/payouts", f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Items      []payouts.AdminPayoutListItem `json:"items"`
		TotalCount int                           `json:"totalCount"`
	}
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, 0, resp.TotalCount)
	require.NotNil(t, resp.Items)
	require.Empty(t, resp.Items)
}

// 2. Multiple batches, pagination and filters
func TestAdminPayouts_ListFiltersAndPagination(t *testing.T) {
	f := setupAdminPayoutFixture(t)
	ctx := context.Background()

	now := time.Now()
	batch1 := uuid.New()
	batch2 := uuid.New()
	batch3 := uuid.New()
	batch4 := uuid.New()
	batch5 := uuid.New()

	_, err := f.pool.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, created_at, updated_at) VALUES
		($1, $2, 10000, 'scheduled', $3, $3, $3),
		($4, $2, 20000, 'paid', $3, $3 - interval '1 hour', $3),
		($5, $2, 30000, 'held', $3, $3 - interval '2 hour', $3),
		($6, $7, 40000, 'scheduled', $3, $3 - interval '3 hour', $3),
		($8, $7, 50000, 'failed', $3, $3 - interval '4 hour', $3)`,
		batch1, f.sellerAID, now,
		batch2,
		batch3,
		batch4, f.sellerBID,
		batch5,
	)
	require.NoError(t, err)

	// A. List all (5 total)
	rr := doJSONReq(f.router, "GET", "/api/admin/payouts", f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Items      []payouts.AdminPayoutListItem `json:"items"`
		TotalCount int                           `json:"totalCount"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, 5, resp.TotalCount)
	require.Len(t, resp.Items, 5)

	// B. Filter by status: scheduled (2 total: batch1, batch4)
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts?status=scheduled", f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.TotalCount)
	for _, it := range resp.Items {
		require.Equal(t, "scheduled", it.Status)
	}

	// C. Filter by sellerId: seller A (3 total)
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts?sellerId="+f.sellerAID.String(), f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, 3, resp.TotalCount)
	for _, it := range resp.Items {
		require.Equal(t, f.sellerAID, it.SellerID)
		require.Equal(t, "Brand Alpha", *it.SellerName)
	}

	// D. Search query q by seller brand name: "Beta"
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts?q=Beta", f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.TotalCount)
	for _, it := range resp.Items {
		require.Equal(t, f.sellerBID, it.SellerID)
	}

	// E. Search query q by batch ID prefix
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts?q="+batch1.String()[:8], f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.TotalCount)
	require.Equal(t, batch1, resp.Items[0].ID)

	// F. Pagination: limit=2 offset=0
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts?limit=2&offset=0", f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, 5, resp.TotalCount)
	require.Len(t, resp.Items, 2)

	// G. Invalid status filter -> 400
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts?status=rejected", f.adminToken, nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)

	// H. Invalid sellerId filter -> 400
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts?sellerId=not-a-uuid", f.adminToken, nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// 3. Payout Batch Details
func TestAdminPayoutDetail_SuccessAndNotFound(t *testing.T) {
	f := setupAdminPayoutFixture(t)
	ctx := context.Background()

	now := time.Now()
	batchID := uuid.New()
	orderID, _, itemID, _ := setupTestOrder(t, f.pool, f.sellerAID, 10000)

	// Insert batch
	_, err := f.pool.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, created_at, updated_at) VALUES
		($1, $2, 9000, 'scheduled', $3, $3, $3)`, batchID, f.sellerAID, now)
	require.NoError(t, err)

	// Insert ledger entry linked to batch
	_, err = f.pool.Exec(ctx, `INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, payout_batch_id, type, amount_cents, currency, available_at, created_at)
		VALUES ($1, $2, $3, $4, $5, 'seller_earning', 9000, 'RUB', $6, $6)`,
		uuid.New(), f.sellerAID, orderID, itemID, batchID, now)
	require.NoError(t, err)

	// A. Query detail
	rr := doJSONReq(f.router, "GET", "/api/admin/payouts/"+batchID.String(), f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var detail payouts.AdminPayoutDetail
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &detail))
	require.Equal(t, batchID, detail.Payout.ID)
	require.Equal(t, int64(9000), detail.Payout.AmountCents)
	require.Equal(t, "Brand Alpha", *detail.Payout.SellerName)
	require.True(t, detail.CanConfirmPaid)
	require.True(t, detail.CanHold)
	require.Equal(t, int64(9000), detail.Totals.EarningsCents)
	require.Len(t, detail.Entries, 1)
	require.Equal(t, "ZMK-100001", *detail.Entries[0].OrderNumber)
	require.Equal(t, "Silk Dress", *detail.Entries[0].ProductTitle)

	// B. Unknown batch ID -> 404
	unknownID := uuid.New()
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts/"+unknownID.String(), f.adminToken, nil)
	require.Equal(t, http.StatusNotFound, rr.Code)

	// C. Malformed batch ID -> 400
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts/not-a-uuid", f.adminToken, nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// 4. RBAC: permissions enforcement (Strict A, B, C, D, E requirements)
func TestAdminPayouts_RBAC(t *testing.T) {
	f := setupAdminPayoutFixture(t)
	ctx := context.Background()

	// -------------------------------------------------------------
	// Requirement A:
	// Сотрудник ТОЛЬКО с payouts.update вызывает PATCH status=paid -> 403,
	// batch остаётся scheduled, запись payout не создаётся.
	// Также POST /process -> 403.
	// -------------------------------------------------------------
	t.Run("A_UpdateOnly_Cannot_Mark_Paid", func(t *testing.T) {
		batchA := uuid.New()
		now := time.Now()
		_, err := f.pool.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, created_at, updated_at) VALUES
			($1, $2, 9000, 'scheduled', $3, $3, $3)`, batchA, f.sellerAID, now)
		require.NoError(t, err)

		// 1. PATCH status=paid with only payouts.update -> 403 Forbidden
		rr := doJSONReq(f.router, "PATCH", "/api/admin/payouts/"+batchA.String()+"/status", f.updateOnlyToken, map[string]string{
			"status": "paid",
		})
		require.Equal(t, http.StatusForbidden, rr.Code, "PATCH status=paid must be forbidden without payouts.mark_paid")

		// Verify batch remains 'scheduled' in DB
		var status string
		var processedAt *time.Time
		err = f.pool.QueryRow(ctx, `SELECT status, processed_at FROM payout_batches WHERE id = $1`, batchA).Scan(&status, &processedAt)
		require.NoError(t, err)
		require.Equal(t, "scheduled", status, "batch must remain scheduled after 403")
		require.Nil(t, processedAt, "processed_at must remain nil")

		// Verify 0 payout ledger entries created
		var payoutCount int
		err = f.pool.QueryRow(ctx, `SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, batchA).Scan(&payoutCount)
		require.NoError(t, err)
		require.Equal(t, 0, payoutCount, "no payout ledger entry must be created on 403")

		// 2. Direct POST /process with only payouts.update -> 403 Forbidden
		rr = doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchA.String()+"/process", f.updateOnlyToken, nil)
		require.Equal(t, http.StatusForbidden, rr.Code, "POST /process must be forbidden without payouts.mark_paid")

		// Still scheduled
		err = f.pool.QueryRow(ctx, `SELECT status FROM payout_batches WHERE id = $1`, batchA).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "scheduled", status)
	})

	// -------------------------------------------------------------
	// Requirement B:
	// Сотрудник ТОЛЬКО с payouts.mark_paid вызывает разрешённую операцию Process -> штатное поведение
	// -------------------------------------------------------------
	t.Run("B_MarkPaidOnly_Can_Process", func(t *testing.T) {
		batchB := uuid.New()
		now := time.Now()
		_, err := f.pool.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, created_at, updated_at) VALUES
			($1, $2, 12000, 'scheduled', $3, $3, $3)`, batchB, f.sellerAID, now)
		require.NoError(t, err)

		rr := doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchB.String()+"/process", f.markPaidOnlyToken, nil)
		require.Equal(t, http.StatusOK, rr.Code, "staff with payouts.mark_paid can process payout")

		var status string
		var processedAt *time.Time
		err = f.pool.QueryRow(ctx, `SELECT status, processed_at FROM payout_batches WHERE id = $1`, batchB).Scan(&status, &processedAt)
		require.NoError(t, err)
		require.Equal(t, "paid", status)
		require.NotNil(t, processedAt)

		var payoutCount int
		err = f.pool.QueryRow(ctx, `SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, batchB).Scan(&payoutCount)
		require.NoError(t, err)
		require.Equal(t, 1, payoutCount)
	})

	// -------------------------------------------------------------
	// Requirement C:
	// Сотрудник с payouts.update вызывает Hold -> разрешённое поведение
	// -------------------------------------------------------------
	t.Run("C_UpdateOnly_Can_Hold", func(t *testing.T) {
		batchC1 := uuid.New()
		batchC2 := uuid.New()
		now := time.Now()
		_, err := f.pool.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, created_at, updated_at) VALUES
			($1, $2, 5000, 'scheduled', $3, $3, $3),
			($4, $2, 6000, 'scheduled', $3, $3, $3)`,
			batchC1, f.sellerAID, now,
			batchC2,
		)
		require.NoError(t, err)

		// 1. Hold via dedicated POST /{id}/hold
		rr := doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchC1.String()+"/hold", f.updateOnlyToken, nil)
		require.Equal(t, http.StatusOK, rr.Code)
		var status1 string
		err = f.pool.QueryRow(ctx, `SELECT status FROM payout_batches WHERE id = $1`, batchC1).Scan(&status1)
		require.NoError(t, err)
		require.Equal(t, "held", status1)

		// 2. Hold via PATCH status=held
		rr = doJSONReq(f.router, "PATCH", "/api/admin/payouts/"+batchC2.String()+"/status", f.updateOnlyToken, map[string]string{
			"status": "held",
		})
		require.Equal(t, http.StatusOK, rr.Code)
		var status2 string
		err = f.pool.QueryRow(ctx, `SELECT status FROM payout_batches WHERE id = $1`, batchC2).Scan(&status2)
		require.NoError(t, err)
		require.Equal(t, "held", status2)
	})

	// -------------------------------------------------------------
	// Requirement D:
	// Пользователь без финансовых прав -> 403
	// -------------------------------------------------------------
	t.Run("D_NoFinancialPermissions_Forbidden", func(t *testing.T) {
		batchD := uuid.New()
		now := time.Now()
		_, err := f.pool.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, created_at, updated_at) VALUES
			($1, $2, 8000, 'scheduled', $3, $3, $3)`, batchD, f.sellerAID, now)
		require.NoError(t, err)

		// GET list -> 403
		rr := doJSONReq(f.router, "GET", "/api/admin/payouts", f.noPermToken, nil)
		require.Equal(t, http.StatusForbidden, rr.Code)

		// GET summary -> 403
		rr = doJSONReq(f.router, "GET", "/api/admin/payouts/summary", f.noPermToken, nil)
		require.Equal(t, http.StatusForbidden, rr.Code)

		// GET detail -> 403
		rr = doJSONReq(f.router, "GET", "/api/admin/payouts/"+batchD.String(), f.noPermToken, nil)
		require.Equal(t, http.StatusForbidden, rr.Code)

		// POST process -> 403
		rr = doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchD.String()+"/process", f.noPermToken, nil)
		require.Equal(t, http.StatusForbidden, rr.Code)

		// POST hold -> 403
		rr = doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchD.String()+"/hold", f.noPermToken, nil)
		require.Equal(t, http.StatusForbidden, rr.Code)

		// PATCH status -> 403
		rr = doJSONReq(f.router, "PATCH", "/api/admin/payouts/"+batchD.String()+"/status", f.noPermToken, map[string]string{"status": "held"})
		require.Equal(t, http.StatusForbidden, rr.Code)

		// Customer token -> 403
		rr = doJSONReq(f.router, "GET", "/api/admin/payouts", f.custToken, nil)
		require.Equal(t, http.StatusForbidden, rr.Code)

		// No token -> 401
		rr = doJSONReq(f.router, "GET", "/api/admin/payouts", "", nil)
		require.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	// -------------------------------------------------------------
	// Requirement E:
	// Повтор и два конкурентных подтверждения не создают дублей
	// -------------------------------------------------------------
	t.Run("E_Replay_And_Concurrent_NoDuplicates", func(t *testing.T) {
		batchE := uuid.New()
		now := time.Now()
		_, err := f.pool.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, created_at, updated_at) VALUES
			($1, $2, 11000, 'scheduled', $3, $3, $3)`, batchE, f.sellerAID, now)
		require.NoError(t, err)

		// First process call -> 200 OK
		rr1 := doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchE.String()+"/process", f.markPaidOnlyToken, nil)
		require.Equal(t, http.StatusOK, rr1.Code)

		// Replay call -> 409 Conflict (already paid)
		rr2 := doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchE.String()+"/process", f.markPaidOnlyToken, nil)
		require.Equal(t, http.StatusConflict, rr2.Code)

		// Verify exactly 1 ledger payout entry
		var payoutCount int
		err = f.pool.QueryRow(ctx, `SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, batchE).Scan(&payoutCount)
		require.NoError(t, err)
		require.Equal(t, 1, payoutCount, "replay must not create duplicate ledger entries")
	})
}

// 5. Process payout batch: manual confirmation
func TestAdminPayoutProcess_ManualConfirmation(t *testing.T) {
	f := setupAdminPayoutFixture(t)
	ctx := context.Background()

	now := time.Now()
	batchID := uuid.New()
	_, err := f.pool.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, created_at, updated_at) VALUES
		($1, $2, 15000, 'scheduled', $3, $3, $3)`, batchID, f.sellerAID, now)
	require.NoError(t, err)

	// Process
	rr := doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchID.String()+"/process", f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)

	// Verify DB state
	var status string
	var processedAt *time.Time
	err = f.pool.QueryRow(ctx, `SELECT status, processed_at FROM payout_batches WHERE id = $1`, batchID).Scan(&status, &processedAt)
	require.NoError(t, err)
	require.Equal(t, "paid", status)
	require.NotNil(t, processedAt)

	// Verify ledger entry
	var payoutCount int
	var payoutAmount int64
	err = f.pool.QueryRow(ctx, `SELECT count(*), COALESCE(SUM(amount_cents), 0) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, batchID).Scan(&payoutCount, &payoutAmount)
	require.NoError(t, err)
	require.Equal(t, 1, payoutCount)
	require.Equal(t, int64(-15000), payoutAmount)

	// Second call -> 409 already paid
	rr2 := doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchID.String()+"/process", f.adminToken, nil)
	require.Equal(t, http.StatusConflict, rr2.Code)
}

// 6. Concurrency: multiple admins confirming at the same time
func TestAdminPayoutProcess_ConcurrentAdmins(t *testing.T) {
	f := setupAdminPayoutFixture(t)
	ctx := context.Background()

	now := time.Now()
	batchID := uuid.New()
	_, err := f.pool.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, created_at, updated_at) VALUES
		($1, $2, 25000, 'scheduled', $3, $3, $3)`, batchID, f.sellerAID, now)
	require.NoError(t, err)

	const concurrency = 6
	var wg sync.WaitGroup
	var successCount int32
	var conflictCount int32

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			rr := doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchID.String()+"/process", f.adminToken, nil)
			if rr.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else if rr.Code == http.StatusConflict {
				atomic.AddInt32(&conflictCount, 1)
			}
		}()
	}
	wg.Wait()

	require.Equal(t, int32(1), successCount, "exactly one process must succeed")
	require.Equal(t, int32(concurrency-1), conflictCount, "all concurrent attempts must be safely rejected with 409")

	// Ensure exactly 1 payout ledger entry was inserted
	var payoutEntries int
	err = f.pool.QueryRow(ctx, `SELECT count(*) FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'payout'`, batchID).Scan(&payoutEntries)
	require.NoError(t, err)
	require.Equal(t, 1, payoutEntries)
}

// 7. Hold vs Process
func TestAdminPayoutHold_AndHoldVsProcess(t *testing.T) {
	f := setupAdminPayoutFixture(t)
	ctx := context.Background()

	now := time.Now()
	batchID := uuid.New()
	_, err := f.pool.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, created_at, updated_at) VALUES
		($1, $2, 18000, 'scheduled', $3, $3, $3)`, batchID, f.sellerAID, now)
	require.NoError(t, err)

	// A. Hold the batch
	rr := doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchID.String()+"/hold", f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var status string
	err = f.pool.QueryRow(ctx, `SELECT status FROM payout_batches WHERE id = $1`, batchID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "held", status)

	// B. Trying to process a held batch must fail with 400 not_processable
	rr = doJSONReq(f.router, "POST", "/api/admin/payouts/"+batchID.String()+"/process", f.adminToken, nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)

	// Detail must indicate held and blocked
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts/"+batchID.String(), f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var detail payouts.AdminPayoutDetail
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &detail))
	require.False(t, detail.CanConfirmPaid)
	require.Equal(t, "held", detail.ConfirmBlockedReason)
}

// 8. Safe PATCH status handler
func TestAdminPayoutPatchStatus_SafeTransitionOnly(t *testing.T) {
	f := setupAdminPayoutFixture(t)
	ctx := context.Background()

	now := time.Now()
	batchID := uuid.New()
	_, err := f.pool.Exec(ctx, `INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, created_at, updated_at) VALUES
		($1, $2, 12000, 'scheduled', $3, $3, $3)`, batchID, f.sellerAID, now)
	require.NoError(t, err)

	// A. Forbidden status (rejected) -> 400
	rr := doJSONReq(f.router, "PATCH", "/api/admin/payouts/"+batchID.String()+"/status", f.adminToken, map[string]string{
		"status": "rejected",
	})
	require.Equal(t, http.StatusBadRequest, rr.Code)

	// B. Forbidden status (approved) -> 400
	rr = doJSONReq(f.router, "PATCH", "/api/admin/payouts/"+batchID.String()+"/status", f.adminToken, map[string]string{
		"status": "approved",
	})
	require.Equal(t, http.StatusBadRequest, rr.Code)

	// C. Permitted action via patch: held -> 200
	rr = doJSONReq(f.router, "PATCH", "/api/admin/payouts/"+batchID.String()+"/status", f.adminToken, map[string]string{
		"status": "held",
	})
	require.Equal(t, http.StatusOK, rr.Code)

	var status string
	err = f.pool.QueryRow(ctx, `SELECT status FROM payout_batches WHERE id = $1`, batchID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "held", status)
}

// 9. Admin Payout Summary
func TestAdminPayoutSummary_Endpoint(t *testing.T) {
	f := setupAdminPayoutFixture(t)
	ctx := context.Background()

	// Configure commission rule
	require.NoError(t, f.payoutSvc.SetCommissionRate(ctx, f.sellerAID, payouts.AdminSellerCommissionRequest{RateBPS: 1000, Reason: "test"}, f.adminID))

	// Setup order with earning
	orderID, _, _, _ := setupTestOrder(t, f.pool, f.sellerAID, 10000)
	require.NoError(t, f.payoutSvc.CreatePendingSalesForOrder(ctx, orderID))
	tx, _ := f.pool.Begin(ctx)
	require.NoError(t, f.payoutSvc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour)))
	require.NoError(t, tx.Commit(ctx))

	_, err := f.payoutSvc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)

	// Check summary via endpoint
	rr := doJSONReq(f.router, "GET", "/api/admin/payouts/summary", f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var sum payouts.AdminPayoutSummary
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &sum))
	require.Equal(t, int64(9000), sum.TotalAvailableCents)
	require.Equal(t, int64(1000), sum.TotalCommissionCents)
	require.Equal(t, int64(0), sum.TotalPendingCents)
	require.Equal(t, int64(0), sum.TotalReturnHoldCents)
	require.Equal(t, "RUB", sum.Currency)

	// Create a payout batch
	batch, err := f.payoutSvc.CreatePayoutBatchForSeller(ctx, f.sellerAID)
	require.NoError(t, err)
	require.Equal(t, int64(9000), batch.AmountCents)

	// Available balance is now in pending batch
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts/summary", f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &sum))
	require.Equal(t, int64(0), sum.TotalAvailableCents, "funds in batch are not double available")
	require.Equal(t, int64(9000), sum.TotalPendingCents)

	// Confirm batch
	require.NoError(t, f.payoutSvc.ProcessPayoutBatch(ctx, batch.ID))

	// Now pending is 0, paid is 9000
	rr = doJSONReq(f.router, "GET", "/api/admin/payouts/summary", f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &sum))
	require.Equal(t, int64(0), sum.TotalPendingCents)
	require.Equal(t, int64(9000), sum.TotalPaidCents)
}

// 10. Process blocked if open return appears after batch formation
func TestAdminPayoutProcess_BlockedByOpenReturn(t *testing.T) {
	f := setupAdminPayoutFixture(t)
	ctx := context.Background()

	require.NoError(t, f.payoutSvc.SetCommissionRate(ctx, f.sellerAID, payouts.AdminSellerCommissionRequest{RateBPS: 0, Reason: "zero"}, f.adminID))

	orderID, customerID, itemID, fullID := setupTestOrder(t, f.pool, f.sellerAID, 10000)
	require.NoError(t, f.payoutSvc.CreatePendingSalesForOrder(ctx, orderID))
	tx, _ := f.pool.Begin(ctx)
	require.NoError(t, f.payoutSvc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour)))
	require.NoError(t, tx.Commit(ctx))

	_, err := f.payoutSvc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)

	// Create batch
	batch, err := f.payoutSvc.CreatePayoutBatchForSeller(ctx, f.sellerAID)
	require.NoError(t, err)

	// Customer opens a return for this item AFTER the batch is formed!
	retID := uuid.New()
	_, err = f.pool.Exec(ctx, `INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at) VALUES ($1, $2, $3, $4, 'requested', 'defective', now(), now())`, retID, orderID, fullID, customerID)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `INSERT INTO return_items (id, return_id, order_item_id, quantity, reason, created_at) VALUES ($1, $2, $3, 1, 'defective', now())`, uuid.New(), retID, itemID)
	require.NoError(t, err)

	// Detail must show uncovered dispute and block confirmation
	rr := doJSONReq(f.router, "GET", "/api/admin/payouts/"+batch.ID.String(), f.adminToken, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var detail payouts.AdminPayoutDetail
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &detail))
	require.False(t, detail.CanConfirmPaid)
	require.Equal(t, int64(10000), detail.UncoveredDisputeCents)
	require.Equal(t, "open_return_not_covered", detail.ConfirmBlockedReason)

	// Process must return 400 open_return_not_covered
	rr = doJSONReq(f.router, "POST", "/api/admin/payouts/"+batch.ID.String()+"/process", f.adminToken, nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	require.Equal(t, "open_return_not_covered", errResp["code"])
}
