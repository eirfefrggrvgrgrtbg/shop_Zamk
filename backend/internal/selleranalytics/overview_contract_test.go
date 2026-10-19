package selleranalytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Case D: Multiple units within one order item
func TestSellerOverview_MultiUnitItem(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	// Order item has quantity = 3, gross = 450,000 kopecks (3 x 1500 RUB), commission = 36,000, earning = 414,000
	insertOrderWithLedger(t, db, sellerID, pID, vID, 450000, 36000, 414000, 3, true, now)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, int64(450000), resp.GrossSales.CurrentCents)
	assert.Equal(t, 3, resp.UnitsSold.Current, "UnitsSold must count all 3 units in the item")
	assert.Equal(t, 1, resp.Orders.Current, "Must count 1 order")
	assert.Equal(t, int64(450000), resp.AverageOrderValue.CurrentCents, "AOV = 450000 / 1")
	assert.Equal(t, int64(36000), resp.Commission.CurrentCents)
	assert.Equal(t, int64(414000), resp.SellerEarningBeforeReturns.CurrentCents)
	assert.Equal(t, int64(414000), resp.NetCommercialEarning.CurrentCents)
	assert.Equal(t, 0, resp.ReturnedUnits.Current)
}

// Case F: Historical commission correctness
func TestSellerOverview_HistoricalCommissionSnapshot(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	// Insert sale where recorded commission in ledger is 8000 kopecks
	insertOrderWithLedger(t, db, sellerID, pID, vID, 100000, 8000, 92000, 1, true, now)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, int64(100000), resp.GrossSales.CurrentCents)
	assert.Equal(t, int64(8000), resp.Commission.CurrentCents, "Commission must strictly match historical ledger snapshot, not current rate")
	assert.Equal(t, int64(92000), resp.SellerEarningBeforeReturns.CurrentCents)
}

// Case H: Multiple partial refunds without double deduction
func TestSellerOverview_MultiplePartialRefunds(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	// 1 delivered order of qty 3, gross 450,000, earning 414,000
	oID, oiID := insertOrderWithLedger(t, db, sellerID, pID, vID, 450000, 36000, 414000, 3, true, now)

	// First partial return: 1 unit returned, deduction 138,000 kopecks
	cID := insertCustomer(t, db)
	insertReturn(t, db, oID, oiID, cID, "completed", 1, 138000, now.Add(1*time.Hour))

	// Second partial return: 1 unit returned later, deduction 138,000 kopecks
	insertReturn(t, db, oID, oiID, cID, "completed", 1, 138000, now.Add(2*time.Hour))

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, int64(450000), resp.GrossSales.CurrentCents, "Gross sales must NOT be diminished by returns")
	assert.Equal(t, 3, resp.UnitsSold.Current, "Gross units sold remains 3")
	assert.Equal(t, 2, resp.ReturnedUnits.Current, "Returned units must be 2 (1 + 1)")
	assert.Equal(t, int64(-276000), resp.ReturnDeductions.CurrentCents, "Deductions sum to -276,000 without double counting")
	assert.Equal(t, int64(414000), resp.SellerEarningBeforeReturns.CurrentCents)
	assert.Equal(t, int64(138000), resp.NetCommercialEarning.CurrentCents, "Net commercial earning = 414,000 - 276,000 = 138,000")
	assert.InDelta(t, 66.6666, resp.ReturnRate.CurrentPercent, 0.01)
}

// Case I: Return after payout
func TestSellerOverview_ReturnAfterPayout(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	oID, oiID := insertOrderWithLedger(t, db, sellerID, pID, vID, 150000, 12000, 138000, 1, true, now)

	// Adjustment after payout has reason = 'return_post_payout'
	cID := insertCustomer(t, db)
	rID := uuid.New()
	var fID uuid.UUID
	err := db.QueryRow(ctx, "SELECT order_fulfillment_id FROM order_items WHERE id = $1", oiID).Scan(&fID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, completed_at) VALUES ($1, $2, $3, $4, 'completed', 'defective', $5)", rID, oID, fID, cID, now.Add(2*time.Hour))
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO return_items (id, return_id, order_item_id, quantity) VALUES ($1, $2, $3, 1)", uuid.New(), rID, oiID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, metadata, created_at) VALUES ($1, $2, $3, $4, 'adjustment', $5, $6, $7)", uuid.New(), sellerID, oID, oiID, -138000, `{"reason":"return_post_payout"}`, now.Add(2*time.Hour))
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, int64(-138000), resp.ReturnDeductions.CurrentCents, "Post-payout adjustment must be included in return deductions")
	assert.Equal(t, int64(0), resp.NetCommercialEarning.CurrentCents, "Net commercial earning becomes 0")
}

// Case L: Moscow day boundaries
func TestSellerOverview_MoscowDayBoundaries(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"

	// Moscow is UTC+3.
	// 2026-10-15 20:59:00 UTC = 2026-10-15 23:59:00 MSK (Day 1)
	// 2026-10-15 21:01:00 UTC = 2026-10-16 00:01:00 MSK (Day 2)
	t1 := time.Date(2026, 10, 15, 20, 59, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 15, 21, 1, 0, 0, time.UTC)

	from := time.Date(2026, 10, 15, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	to := time.Date(2026, 10, 17, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600)) // 2 days: Oct 15 and Oct 16

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	insertOrderWithLedger(t, db, sellerID, pID, vID, 100000, 8000, 92000, 1, true, t1)
	insertOrderWithLedger(t, db, sellerID, pID, vID, 200000, 16000, 184000, 1, true, t2)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	require.Len(t, resp.Timeseries, 2, "Must produce exactly 2 day buckets")
	assert.Equal(t, "2026-10-15", resp.Timeseries[0].Date)
	assert.Equal(t, int64(100000), resp.Timeseries[0].GrossSalesCents, "t1 at 23:59 MSK belongs to Oct 15")
	assert.Equal(t, 1, resp.Timeseries[0].UnitsSold)

	assert.Equal(t, "2026-10-16", resp.Timeseries[1].Date)
	assert.Equal(t, int64(200000), resp.Timeseries[1].GrossSalesCents, "t2 at 00:01 MSK belongs to Oct 16")
	assert.Equal(t, 1, resp.Timeseries[1].UnitsSold)
}

// Case M: Leap-year / year-transition boundaries
func TestSellerOverview_LeapYearAndYearTransition(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	sellerID := insertSeller(t, db)

	// Leap year: 2024 is a leap year (Feb 29 exists)
	fromLeap := time.Date(2024, 2, 28, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	toLeap := time.Date(2024, 3, 1, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600)) // Feb 28 and Feb 29

	respLeap, err := svc.GetOverview(ctx, sellerID, fromLeap, toLeap, tz)
	require.NoError(t, err)
	require.Len(t, respLeap.Timeseries, 2, "Feb 28 to Mar 1 in 2024 must include Feb 28 and Feb 29")
	assert.Equal(t, "2024-02-28", respLeap.Timeseries[0].Date)
	assert.Equal(t, "2024-02-29", respLeap.Timeseries[1].Date)

	// Year transition: Dec 31 to Jan 2
	fromYear := time.Date(2025, 12, 31, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	toYear := time.Date(2026, 1, 2, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600))

	respYear, err := svc.GetOverview(ctx, sellerID, fromYear, toYear, tz)
	require.NoError(t, err)
	require.Len(t, respYear.Timeseries, 2)
	assert.Equal(t, "2025-12-31", respYear.Timeseries[0].Date)
	assert.Equal(t, "2026-01-01", respYear.Timeseries[1].Date)
}

// Timeseries mathematical sum invariant: Daily timeseries sums must strictly equal period totals
func TestSellerOverview_TimeseriesSumsMatchOverview(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"

	from := time.Date(2026, 10, 10, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	to := time.Date(2026, 10, 13, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600)) // 3 days: Oct 10, 11, 12

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	// Day 1: Oct 10 12:00 MSK
	tDay1 := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	insertOrderWithLedger(t, db, sellerID, pID, vID, 150000, 12000, 138000, 1, true, tDay1)

	// Day 2: Oct 11 15:00 MSK (2 items)
	tDay2 := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	oID2, oiID2 := insertOrderWithLedger(t, db, sellerID, pID, vID, 300000, 24000, 276000, 2, true, tDay2)

	// Day 3: Oct 12 18:00 MSK (return of 1 item from Day 2)
	tDay3 := time.Date(2026, 10, 12, 15, 0, 0, 0, time.UTC)
	cID := insertCustomer(t, db)
	insertReturn(t, db, oID2, oiID2, cID, "completed", 1, 138000, tDay3)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	var sumGross int64
	var sumOrders int
	var sumCommission int64
	var sumEarning int64
	var sumReturnDeductions int64
	var sumNet int64
	var sumUnits int
	var sumReturnedUnits int

	for _, ts := range resp.Timeseries {
		sumGross += ts.GrossSalesCents
		sumOrders += ts.OrdersCount
		sumCommission += ts.CommissionCents
		sumEarning += ts.SellerEarningCents
		sumReturnDeductions += ts.ReturnDeductionsCents
		sumNet += ts.NetCommercialEarningCents
		sumUnits += ts.UnitsSold
		sumReturnedUnits += ts.ReturnedUnits
	}

	assert.Equal(t, resp.GrossSales.CurrentCents, sumGross, "Gross sales sum must match")
	assert.Equal(t, resp.Orders.Current, sumOrders, "Orders count sum must match")
	assert.Equal(t, resp.Commission.CurrentCents, sumCommission, "Commission sum must match")
	assert.Equal(t, resp.SellerEarningBeforeReturns.CurrentCents, sumEarning, "Earning sum must match")
	assert.Equal(t, resp.ReturnDeductions.CurrentCents, sumReturnDeductions, "Return deductions sum must match")
	assert.Equal(t, resp.NetCommercialEarning.CurrentCents, sumNet, "Net commercial earning sum must match")
	assert.Equal(t, resp.UnitsSold.Current, sumUnits, "Units sold sum must match")
	assert.Equal(t, resp.ReturnedUnits.Current, sumReturnedUnits, "Returned units sum must match")
}

// Case N: Invalid and reversed date ranges validation at Handler level
func TestSellerOverview_HandlerValidation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)
	handler := NewHandler(svc, repo)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	sellerID := insertSeller(t, db)
	// Find userID
	var userID uuid.UUID
	err := db.QueryRow(context.Background(), "SELECT user_id FROM seller_users WHERE seller_id = $1", sellerID).Scan(&userID)
	require.NoError(t, err)

	execReq := func(url string, uid *uuid.UUID) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", url, nil)
		if uid != nil {
			req = req.WithContext(context.WithValue(req.Context(), "userID", *uid))
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// Subtest 1: Missing from or to -> 400
	rec := execReq("/overview", &userID)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "parameters are required")

	// Subtest 2: Invalid RFC3339 -> 400
	rec = execReq("/overview?from=2026-10-01&to=invalid", &userID)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid 'from' timestamp")

	// Subtest 3: to < from -> 400
	rec = execReq("/overview?from=2026-10-10T00:00:00Z&to=2026-10-01T00:00:00Z", &userID)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "'to' cannot be before 'from'")

	// Subtest 4: Exceeds 366 days -> 400
	rec = execReq("/overview?from=2024-01-01T00:00:00Z&to=2025-02-01T00:00:00Z", &userID)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "exceeds maximum allowed limit of 366 days")

	// Subtest 5: Unauthenticated -> 401
	rec = execReq("/overview?from=2026-10-01T00:00:00Z&to=2026-10-05T00:00:00Z", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// Case O: Cross-seller API access isolation
func TestSellerOverview_CrossSellerIsolation_HTTP(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)
	handler := NewHandler(svc, repo)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	seller1 := insertSeller(t, db)
	seller2 := insertSeller(t, db)

	var user1, user2 uuid.UUID
	err := db.QueryRow(context.Background(), "SELECT user_id FROM seller_users WHERE seller_id = $1", seller1).Scan(&user1)
	require.NoError(t, err)
	err = db.QueryRow(context.Background(), "SELECT user_id FROM seller_users WHERE seller_id = $1", seller2).Scan(&user2)
	require.NoError(t, err)

	now := time.Now().UTC()
	fromStr := now.Add(-24 * time.Hour).Format(time.RFC3339)
	toStr := now.Add(24 * time.Hour).Format(time.RFC3339)

	p1, v1 := insertProduct(t, db, seller1)
	p2, v2 := insertProduct(t, db, seller2)

	// Seller 1 sells 1 item for 1500 RUB (150,000 cents)
	insertOrderWithLedger(t, db, seller1, p1, v1, 150000, 12000, 138000, 1, true, now)
	// Seller 2 sells 1 item for 5000 RUB (500,000 cents)
	insertOrderWithLedger(t, db, seller2, p2, v2, 500000, 40000, 460000, 1, true, now)

	// User 1 requests overview
	req1 := httptest.NewRequest("GET", "/overview?from="+fromStr+"&to="+toStr, nil)
	req1 = req1.WithContext(context.WithValue(req1.Context(), "userID", user1))
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	var resp1 OverviewResponse
	err = json.Unmarshal(rec1.Body.Bytes(), &resp1)
	require.NoError(t, err)
	assert.Equal(t, int64(150000), resp1.GrossSales.CurrentCents, "User 1 must see exactly Seller 1 sales")
	assert.Equal(t, 1, resp1.Orders.Current)

	// User 2 requests overview
	req2 := httptest.NewRequest("GET", "/overview?from="+fromStr+"&to="+toStr, nil)
	req2 = req2.WithContext(context.WithValue(req2.Context(), "userID", user2))
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var resp2 OverviewResponse
	err = json.Unmarshal(rec2.Body.Bytes(), &resp2)
	require.NoError(t, err)
	assert.Equal(t, int64(500000), resp2.GrossSales.CurrentCents, "User 2 must see exactly Seller 2 sales")
	assert.Equal(t, 1, resp2.Orders.Current)

	// Even if User 1 attempts to pass ?seller_id=<seller2>, backend resolves seller from JWT userID and returns Seller 1
	reqMalicious := httptest.NewRequest("GET", "/overview?from="+fromStr+"&to="+toStr+"&seller_id="+seller2.String(), nil)
	reqMalicious = reqMalicious.WithContext(context.WithValue(reqMalicious.Context(), "userID", user1))
	recMalicious := httptest.NewRecorder()
	r.ServeHTTP(recMalicious, reqMalicious)
	require.Equal(t, http.StatusOK, recMalicious.Code)

	var respMalicious OverviewResponse
	err = json.Unmarshal(recMalicious.Body.Bytes(), &respMalicious)
	require.NoError(t, err)
	assert.Equal(t, int64(150000), respMalicious.GrossSales.CurrentCents, "Injected seller_id query param must be completely ignored")
}

// Case P: Payment date and delivery date separation across different days
func TestSellerOverview_PaymentAndDeliveryDateSeparation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"

	// Interval: Oct 10 to Oct 16 (6 days)
	from := time.Date(2026, 10, 10, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	to := time.Date(2026, 10, 16, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600))

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	// Payment happens on Oct 10 at 12:00 MSK (09:00 UTC)
	tPayment := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	// Delivery occurs on Oct 14 at 15:00 MSK (12:00 UTC)
	tDelivery := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	oID := uuid.New()
	oiID := uuid.New()
	fID := uuid.New()
	cID := insertCustomer(t, db)

	_, err := db.Exec(ctx, "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'delivered', 'Name', '123', 'email@e.c', 'addr')", oID, cID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'delivered', 150000, 800, 138000, $4)", fID, oID, sellerID, tDelivery)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Test Product', 'slug', 150000, 1, 150000, $6)", oiID, oID, pID, vID, sellerID, fID)
	require.NoError(t, err)

	// Payment recorded on Oct 10
	_, err = db.Exec(ctx,
		`INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at)
		 VALUES ($1, $2, 'tbank', 'succeeded', 150000, $3, $4, $4, $4)`,
		uuid.New(), oID, uuid.New().String(), tPayment,
	)
	require.NoError(t, err)

	// Ledger entries created on Oct 14 upon delivery
	_, err = db.Exec(ctx, "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 150000, $5)", uuid.New(), sellerID, oID, oiID, tDelivery)
	require.NoError(t, err)
	_, err = db.Exec(ctx, "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'zamk_commission', -12000, $5)", uuid.New(), sellerID, oID, oiID, tDelivery)
	require.NoError(t, err)
	_, err = db.Exec(ctx, "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'seller_earning', 138000, $5)", uuid.New(), sellerID, oID, oiID, tDelivery)
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	// Period summary: 1 paid order, 150000 gross, AOV = 150000
	assert.Equal(t, 1, resp.Orders.Current)
	assert.Equal(t, int64(150000), resp.GrossSales.CurrentCents)
	assert.Equal(t, int64(150000), resp.AverageOrderValue.CurrentCents)
	require.NotNil(t, resp.DeliveredOrders)
	assert.Equal(t, 1, resp.DeliveredOrders.Current)

	// Timeseries verification:
	var oct10Found, oct14Found bool
	for _, bucket := range resp.Timeseries {
		if bucket.Date == "2026-10-10" {
			oct10Found = true
			assert.Equal(t, 1, bucket.OrdersCount, "Oct 10 must record the paid order")
			assert.Equal(t, int64(0), bucket.GrossSalesCents, "Oct 10 has no deliveries yet, gross must be 0")
		}
		if bucket.Date == "2026-10-14" {
			oct14Found = true
			assert.Equal(t, 0, bucket.OrdersCount, "Oct 14 had no payment, ordersCount must be 0")
			assert.Equal(t, int64(150000), bucket.GrossSalesCents, "Oct 14 had delivery, gross must be 150000")
		}
	}
	assert.True(t, oct10Found, "Oct 10 bucket must exist")
	assert.True(t, oct14Found, "Oct 14 bucket must exist")
}

// Case Q: Multiple payment attempts without double counting
func TestSellerOverview_PaymentRetries(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	oID := uuid.New()
	oiID := uuid.New()
	fID := uuid.New()
	cID := insertCustomer(t, db)

	_, err := db.Exec(ctx, "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'paid', 'Name', '123', 'email@e.c', 'addr')", oID, cID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'paid', 150000, 800, 138000, $4)", fID, oID, sellerID, now)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Test Product', 'slug', 150000, 1, 150000, $6)", oiID, oID, pID, vID, sellerID, fID)
	require.NoError(t, err)

	// Attempt 1: failed
	_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, failed_at) VALUES ($1, $2, 'tbank', 'failed', 150000, $3, $4, $4)", uuid.New(), oID, uuid.New().String(), now.Add(-10*time.Minute))
	require.NoError(t, err)

	// Attempt 2: succeeded
	_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at) VALUES ($1, $2, 'tbank', 'succeeded', 150000, $3, $4, $4, $4)", uuid.New(), oID, uuid.New().String(), now)
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Orders.Current, "Multiple payment attempts on same order must count as exactly 1 paid order")
}

// Case R: Multiple order items for the same seller count as 1 order
func TestSellerOverview_MultipleItemsSameSeller(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	sellerID := insertSeller(t, db)
	pID1, vID1 := insertProduct(t, db, sellerID)
	pID2, vID2 := insertProduct(t, db, sellerID)

	oID := uuid.New()
	fID := uuid.New()
	cID := insertCustomer(t, db)

	_, err := db.Exec(ctx, "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'delivered', 'Name', '123', 'email@e.c', 'addr')", oID, cID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'delivered', 450000, 800, 414000, $4)", fID, oID, sellerID, now)
	require.NoError(t, err)

	oiID1 := uuid.New()
	_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Test Product 1', 'slug1', 150000, 1, 150000, $6)", oiID1, oID, pID1, vID1, sellerID, fID)
	require.NoError(t, err)

	oiID2 := uuid.New()
	_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Test Product 2', 'slug2', 150000, 2, 300000, $6)", oiID2, oID, pID2, vID2, sellerID, fID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at) VALUES ($1, $2, 'tbank', 'succeeded', 450000, $3, $4, $4, $4)", uuid.New(), oID, uuid.New().String(), now)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 150000, $5)", uuid.New(), sellerID, oID, oiID1, now)
	require.NoError(t, err)
	_, err = db.Exec(ctx, "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 300000, $5)", uuid.New(), sellerID, oID, oiID2, now)
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Orders.Current, "2 items in same order from same seller must count as exactly 1 order")
	assert.Equal(t, 3, resp.UnitsSold.Current, "UnitsSold must be 1 + 2 = 3")
	assert.Equal(t, int64(450000), resp.GrossSales.CurrentCents)
	assert.Equal(t, int64(450000), resp.AverageOrderValue.CurrentCents)
}

// Case S: Test C - Cancelled fulfillment later updated for an unrelated reason
func TestSellerOverview_CancelledFulfillmentLaterUpdated(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	oID := uuid.New()
	fID := uuid.New()
	oiID := uuid.New()
	cID := insertCustomer(t, db)

	tCancelled := now.Add(-2 * time.Hour)
	tPayment := now.Add(-1 * time.Hour)
	tUnrelatedUpdate := now

	// Order cancelled before payment
	_, err := db.Exec(ctx, "INSERT INTO orders (id, user_id, status, cancelled_at, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'cancelled', $3, 'Name', '123', 'email@e.c', 'addr')", oID, cID, tCancelled)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'cancelled', 150000, 800, 138000, $4)", fID, oID, sellerID, tCancelled)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Test Product', 'slug', 150000, 1, 150000, $6)", oiID, oID, pID, vID, sellerID, fID)
	require.NoError(t, err)

	// Payment attempt recorded after cancellation
	_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at) VALUES ($1, $2, 'tbank', 'succeeded', 150000, $3, $4, $4, $4)", uuid.New(), oID, uuid.New().String(), tPayment)
	require.NoError(t, err)

	// Unrelated subsequent update to fulfillment row (e.g. barcode / scan / note)
	_, err = db.Exec(ctx, "UPDATE order_fulfillments SET receiving_code = $1, updated_at = $2 WHERE id = $3", "REC-"+uuid.New().String(), tUnrelatedUpdate, fID)
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Orders.Current, "Fulfillment cancelled before payment must NOT count, even if updated_at was modified later")
	assert.Equal(t, int64(0), resp.GrossSales.CurrentCents)
}

// Case T: Test D - Multiple successful payment records on different dates deduplicated
func TestSellerOverview_MultipleSuccessfulPaymentsDeduplicated(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"

	from := time.Date(2026, 10, 10, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	to := time.Date(2026, 10, 15, 0, 0, 0, 0, time.FixedZone("MSK", 3*3600))

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	tPay1 := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)  // 12:00 MSK Oct 10
	tPay2 := time.Date(2026, 10, 12, 11, 0, 0, 0, time.UTC) // 14:00 MSK Oct 12

	oID := uuid.New()
	fID := uuid.New()
	oiID := uuid.New()
	cID := insertCustomer(t, db)

	_, err := db.Exec(ctx, "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'paid', 'Name', '123', 'email@e.c', 'addr')", oID, cID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'paid', 150000, 800, 138000, $4)", fID, oID, sellerID, tPay1)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Test Product', 'slug', 150000, 1, 150000, $6)", oiID, oID, pID, vID, sellerID, fID)
	require.NoError(t, err)

	// First successful payment on Oct 10
	_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at) VALUES ($1, $2, 'tbank', 'succeeded', 150000, $3, $4, $4, $4)", uuid.New(), oID, uuid.New().String(), tPay1)
	require.NoError(t, err)

	// Second successful payment on Oct 12 (anomaly/reconciliation replay)
	_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at) VALUES ($1, $2, 'tbank', 'succeeded', 150000, $3, $4, $4, $4)", uuid.New(), oID, uuid.New().String(), tPay2)
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Orders.Current, "Order must be counted exactly once across the period despite multiple succeeded payment rows")

	var oct10Orders, oct12Orders, sumDailyOrders int
	for _, b := range resp.Timeseries {
		sumDailyOrders += b.OrdersCount
		if b.Date == "2026-10-10" {
			oct10Orders = b.OrdersCount
		}
		if b.Date == "2026-10-12" {
			oct12Orders = b.OrdersCount
		}
	}
	assert.Equal(t, 1, oct10Orders, "Order belongs strictly to the first canonical payment date (Oct 10)")
	assert.Equal(t, 0, oct12Orders, "Second payment date must NOT create a duplicate order count")
	assert.Equal(t, resp.Orders.Current, sumDailyOrders, "Sum of daily timeseries paid orders must strictly equal overview total")
}

// Case U: Test F - Multi-seller order where only one seller remains eligible at payment time
func TestSellerOverview_MultiSellerOrder_OnlyOneEligibleAtPayment(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	seller1 := insertSeller(t, db)
	seller2 := insertSeller(t, db)
	pID1, vID1 := insertProduct(t, db, seller1)
	pID2, vID2 := insertProduct(t, db, seller2)

	oID := uuid.New()
	fID1 := uuid.New()
	fID2 := uuid.New()
	cID := insertCustomer(t, db)

	tCancelled := now.Add(-1 * time.Hour)
	tPayment := now

	// Customer order is active and paid (not cancelled at order level)
	_, err := db.Exec(ctx, "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'paid', 'Name', '123', 'email@e.c', 'addr')", oID, cID)
	require.NoError(t, err)

	// Seller 1 fulfillment cancelled before payment (e.g. out of stock / seller reject)
	_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'cancelled', 150000, 800, 138000, $4)", fID1, oID, seller1, tCancelled)
	require.NoError(t, err)
	oiID1 := uuid.New()
	_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Product 1', 'slug1', 150000, 1, 150000, $6)", oiID1, oID, pID1, vID1, seller1, fID1)
	require.NoError(t, err)

	// Seller 2 fulfillment is active and paid
	_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'paid', 250000, 800, 230000, $4)", fID2, oID, seller2, tPayment)
	require.NoError(t, err)
	oiID2 := uuid.New()
	_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Product 2', 'slug2', 250000, 1, 250000, $6)", oiID2, oID, pID2, vID2, seller2, fID2)
	require.NoError(t, err)

	// Order paid
	_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at) VALUES ($1, $2, 'tbank', 'succeeded', 250000, $3, $4, $4, $4)", uuid.New(), oID, uuid.New().String(), tPayment)
	require.NoError(t, err)

	resp1, err := svc.GetOverview(ctx, seller1, from, to, tz)
	require.NoError(t, err)
	assert.Equal(t, 0, resp1.Orders.Current, "Seller 1 was cancelled before payment and must NOT count as a paid order")

	resp2, err := svc.GetOverview(ctx, seller2, from, to, tz)
	require.NoError(t, err)
	assert.Equal(t, 1, resp2.Orders.Current, "Seller 2 remained eligible and paid, must count as 1 paid order")
}

// Phase 3 Test 3: Cancellation before payment where order_status_history has from_status not in ('created', 'awaiting_payment')
// Verifies that unconstrained from_status predicate does NOT qualify a cancellation that occurred before payment.
func TestSellerOverview_CancellationBeforePayment_WithPreviousStatusNotAwaitingPayment(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	oID := uuid.New()
	fID := uuid.New()
	oiID := uuid.New()
	cID := insertCustomer(t, db)

	tCancelled := now.Add(-2 * time.Hour)
	tPayment := now.Add(-1 * time.Hour)

	// Order was cancelled before payment
	_, err := db.Exec(ctx, "INSERT INTO orders (id, user_id, status, cancelled_at, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'cancelled', $3, 'Name', '123', 'email@e.c', 'addr')", oID, cID, tCancelled)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'cancelled', 150000, 800, 138000, $4)", fID, oID, sellerID, tCancelled)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Test Product', 'slug', 150000, 1, 150000, $6)", oiID, oID, pID, vID, sellerID, fID)
	require.NoError(t, err)

	// History record with from_status = 'assembling' (not awaiting_payment, not created)
	// BUT created_at is tCancelled (-2h), which is BEFORE payment (-1h)
	_, err = db.Exec(ctx, "INSERT INTO order_status_history (id, order_id, from_status, to_status, created_at) VALUES ($1, $2, 'assembling', 'cancelled', $3)", uuid.New(), oID, tCancelled)
	require.NoError(t, err)

	// Payment arrives later at tPayment (-1h)
	_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at) VALUES ($1, $2, 'tbank', 'succeeded', 150000, $3, $4, $4, $4)", uuid.New(), oID, uuid.New().String(), tPayment)
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Orders.Current, "Cancellation before payment must NOT count, even if from_status was not awaiting_payment/created")
	assert.Equal(t, int64(0), resp.GrossSales.CurrentCents)
}

// Phase 3 Test 4: Cancellation history exists but orders.cancelled_at is null
func TestSellerOverview_CancellationHistory_NullCancelledAt(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)
	cID := insertCustomer(t, db)

	// Subcase 4A: orders.cancelled_at IS NULL, but order_status_history has cancellation AFTER payment
	{
		oID := uuid.New()
		fID := uuid.New()
		oiID := uuid.New()
		tPay := now.Add(-2 * time.Hour)
		tCancel := now.Add(-1 * time.Hour) // after payment

		// orders.cancelled_at is NULL
		_, err := db.Exec(ctx, "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'cancelled', 'Name', '123', 'email@e.c', 'addr')", oID, cID)
		require.NoError(t, err)

		_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'cancelled', 150000, 800, 138000, $4)", fID, oID, sellerID, tCancel)
		require.NoError(t, err)

		_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Product A', 'slugA', 150000, 1, 150000, $6)", oiID, oID, pID, vID, sellerID, fID)
		require.NoError(t, err)

		_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at) VALUES ($1, $2, 'tbank', 'succeeded', 150000, $3, $4, $4, $4)", uuid.New(), oID, uuid.New().String(), tPay)
		require.NoError(t, err)

		// order_status_history records cancellation at tCancel (>= tPay)
		_, err = db.Exec(ctx, "INSERT INTO order_status_history (id, order_id, from_status, to_status, created_at) VALUES ($1, $2, 'paid', 'cancelled', $3)", uuid.New(), oID, tCancel)
		require.NoError(t, err)

		resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Orders.Current, "Subcase 4A: Post-payment cancellation with null orders.cancelled_at but valid history must count")
	}

	// Subcase 4B: orders.cancelled_at IS NULL, order_status_history has cancellation BEFORE payment
	{
		sellerB := insertSeller(t, db)
		pIDB, vIDB := insertProduct(t, db, sellerB)
		oID := uuid.New()
		fID := uuid.New()
		oiID := uuid.New()
		tCancel := now.Add(-2 * time.Hour)
		tPay := now.Add(-1 * time.Hour) // after cancel

		_, err := db.Exec(ctx, "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'cancelled', 'Name', '123', 'email@e.c', 'addr')", oID, cID)
		require.NoError(t, err)

		_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'cancelled', 150000, 800, 138000, $4)", fID, oID, sellerB, tCancel)
		require.NoError(t, err)

		_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Product B', 'slugB', 150000, 1, 150000, $6)", oiID, oID, pIDB, vIDB, sellerB, fID)
		require.NoError(t, err)

		_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at) VALUES ($1, $2, 'tbank', 'succeeded', 150000, $3, $4, $4, $4)", uuid.New(), oID, uuid.New().String(), tPay)
		require.NoError(t, err)

		// order_status_history records cancellation at tCancel (< tPay)
		_, err = db.Exec(ctx, "INSERT INTO order_status_history (id, order_id, from_status, to_status, created_at) VALUES ($1, $2, 'processing', 'cancelled', $3)", uuid.New(), oID, tCancel)
		require.NoError(t, err)

		resp, err := svc.GetOverview(ctx, sellerB, from, to, tz)
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Orders.Current, "Subcase 4B: Pre-payment cancellation in history must NOT count")
	}

	// Subcase 4C: orders.cancelled_at IS NULL, NO cancellation history exists
	{
		sellerC := insertSeller(t, db)
		pIDC, vIDC := insertProduct(t, db, sellerC)
		oID := uuid.New()
		fID := uuid.New()
		oiID := uuid.New()
		tPay := now.Add(-1 * time.Hour)

		_, err := db.Exec(ctx, "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'cancelled', 'Name', '123', 'email@e.c', 'addr')", oID, cID)
		require.NoError(t, err)

		_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'cancelled', 150000, 800, 138000, $4)", fID, oID, sellerC, tPay)
		require.NoError(t, err)

		_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Product C', 'slugC', 150000, 1, 150000, $6)", oiID, oID, pIDC, vIDC, sellerC, fID)
		require.NoError(t, err)

		_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at) VALUES ($1, $2, 'tbank', 'succeeded', 150000, $3, $4, $4, $4)", uuid.New(), oID, uuid.New().String(), tPay)
		require.NoError(t, err)

		// No order_status_history

		resp, err := svc.GetOverview(ctx, sellerC, from, to, tz)
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Orders.Current, "Subcase 4C: Cancelled order lacking reliable cancellation timing must not count")
	}
}

// Phase 3 Test 8: Delivered AOV remains based exclusively on delivered ledger sales
// Verifies that paid but undelivered orders increase Orders.Current without diluting delivered AOV.
func TestSellerOverview_DeliveredAOV_SeparationFromPaidUndelivered(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	sellerID := insertSeller(t, db)
	pID, vID := insertProduct(t, db, sellerID)

	// Order 1: Delivered sale with ledger entries (Gross = 150,000 kopecks)
	insertOrderWithLedger(t, db, sellerID, pID, vID, 150000, 12000, 138000, 1, true, now.Add(-2*time.Hour))

	// Order 2: Paid but undelivered order (Status = 'paid', NO ledger entry yet)
	cID := insertCustomer(t, db)
	oID2 := uuid.New()
	fID2 := uuid.New()
	oiID2 := uuid.New()
	_, err := db.Exec(ctx, "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'paid', 'Name', '123', 'email@e.c', 'addr')", oID2, cID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, 'paid', 250000, 800, 230000, $4)", fID2, oID2, sellerID, now.Add(-1*time.Hour))
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Test Product', 'slug', 250000, 1, 250000, $6)", oiID2, oID2, pID, vID, sellerID, fID2)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at) VALUES ($1, $2, 'tbank', 'succeeded', 250000, $3, $4, $4, $4)", uuid.New(), oID2, uuid.New().String(), now.Add(-1*time.Hour))
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, 2, resp.Orders.Current, "Total paid orders must be 2 (1 delivered + 1 undelivered)")
	assert.Equal(t, 1, resp.DeliveredOrders.Current, "Delivered orders must be 1")
	assert.Equal(t, int64(150000), resp.GrossSales.CurrentCents, "Gross sales must be strictly 150,000 (from delivered sale)")
	// Under Option A: AOV = delivered gross (150,000) / delivered orders (1) = 150,000 kopecks (NOT 150,000 / 2 = 75,000!)
	assert.Equal(t, int64(150000), resp.AverageOrderValue.CurrentCents, "Delivered AOV must not be diluted by undelivered orders")
}


