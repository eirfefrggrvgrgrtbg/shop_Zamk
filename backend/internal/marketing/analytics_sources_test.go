package marketing_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func insertAnalyticsTestSession(ctx context.Context, db *pgxpool.Pool, id, visitorID uuid.UUID, source *string, campID *uuid.UUID, startedAt time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO analytics_sessions (id, visitor_id, source, campaign_id, started_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, id, visitorID, source, campID, startedAt)
	return err
}

func insertAnalyticsTestOrderAttribution(ctx context.Context, db *pgxpool.Pool, orderID uuid.UUID, sessionID *uuid.UUID, source *string, campID *uuid.UUID) error {
	_, err := db.Exec(ctx, `
		INSERT INTO order_attributions (order_id, session_id, source, campaign_id, attributed_at)
		VALUES ($1, $2, $3, $4, now())
	`, orderID, sessionID, source, campID)
	return err
}

func insertAnalyticsTestReturn(ctx context.Context, db *pgxpool.Pool, returnID, orderID, fulID, userID, orderItemID uuid.UUID, status string, completedAt *time.Time) error {
	t := time.Now()
	if completedAt != nil {
		t = *completedAt
	}
	_, err := db.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, completed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'Defective', $6, $7, $7)
	`, returnID, orderID, fulID, userID, status, completedAt, t)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, reason, created_at)
		VALUES ($1, $2, $3, 1, 'Defective', $4)
	`, uuid.New(), returnID, orderItemID, t)
	return err
}

func insertAnalyticsTestOrderItemPromotion(ctx context.Context, db *pgxpool.Pool, promoID, oiID, orderID, sellerID, campID, promoCodeID uuid.UUID, customerPaidCents int64) error {
	baseUnitCents := int64(5000)
	sellerDiscountUnitCents := baseUnitCents - customerPaidCents
	_, err := db.Exec(ctx, `
		INSERT INTO order_item_promotions (
			id, order_item_id, order_id, seller_id, campaign_id, promo_code_id,
			base_unit_price_cents, seller_discount_unit_cents, zamk_subsidy_unit_cents,
			customer_paid_unit_price_cents, commission_base_unit_cents, quantity,
			total_seller_discount_cents, total_zamk_subsidy_cents, total_customer_paid_cents,
			total_commission_base_cents, commission_rate_bps, total_commission_charged_cents, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, 0,
			$9, $9, 1,
			$8, 0, $9,
			$9, 1000, 400, now()
		)
	`, promoID, oiID, orderID, sellerID, campID, promoCodeID, baseUnitCents, sellerDiscountUnitCents, customerPaidCents)
	return err
}

// 1. DB Safety Guard: Prove SELECT current_database() equals strictly "zamk_test"
func TestSourcesAnalytics_DatabaseSafetyGuard(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()

	testutil.AssertTestDatabase(t, db)

	var currentDB string
	err := db.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB)
	require.NoError(t, err)
	assert.Equal(t, "zamk_test", currentDB, "Must execute exclusively against isolated zamk_test database")
}

// 2. Direct != Unattributed != Named and NULL + "" Normalization
func TestSourcesAnalytics_DirectUnattributedNamedDistinctness(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	testutil.AssertTestDatabase(t, db)

	repo := marketing.NewAnalyticsRepository(db)

	baseTime := time.Date(2043, 1, 1, 0, 0, 0, 0, time.UTC)
	from := baseTime
	to := baseTime.Add(24 * time.Hour)
	now := baseTime.Add(12 * time.Hour)

	_, _ = db.Exec(ctx, `DELETE FROM analytics_sessions WHERE started_at >= $1 AND started_at < $2`, from, to)
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM analytics_sessions WHERE started_at >= $1 AND started_at < $2`, from, to)
	}()

	visitorID := uuid.New()
	directStr := "direct"
	emptyStr := ""
	vkStr := "vk"

	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &directStr, nil, now))
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, nil, nil, now))
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &emptyStr, nil, now))
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &vkStr, nil, now))

	res, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to})
	require.NoError(t, err)
	require.NotNil(t, res)

	var directRow, unattributedRow, vkRow *marketing.SourcePerformanceRow
	for i := range res.Sources {
		row := &res.Sources[i]
		if row.SourceKind == marketing.SourceKindDirect {
			directRow = row
		} else if row.SourceKind == marketing.SourceKindUnattributed {
			unattributedRow = row
		} else if row.SourceKind == marketing.SourceKindNamed && row.SourceKey != nil && *row.SourceKey == "vk" {
			vkRow = row
		}
	}

	require.NotNil(t, directRow, "Direct row must exist")
	require.NotNil(t, unattributedRow, "Unattributed row must exist")
	require.NotNil(t, vkRow, "VK row must exist")

	assert.Equal(t, 1, directRow.Visits, "Direct must only count explicit direct")
	assert.Equal(t, "direct", *directRow.SourceKey)

	// NULL and "" must normalize into ONE unattributed bucket with visits = 2
	assert.Equal(t, 2, unattributedRow.Visits, "NULL and empty string must merge into 1 unattributed row")
	assert.Nil(t, unattributedRow.SourceKey, "Unattributed row must have sourceKey: null")

	assert.Equal(t, 1, vkRow.Visits, "VK must have visits = 1")
	assert.Equal(t, "vk", *vkRow.SourceKey)

	// Prove they are separate isolated rows
	assert.NotEqual(t, directRow.SourceKind, unattributedRow.SourceKind)
	assert.NotEqual(t, directRow.SourceKind, vkRow.SourceKind)
	assert.NotEqual(t, unattributedRow.SourceKind, vkRow.SourceKind)
}

// 3. Populated Aggregation Across All Dimensions + Cross-Layer Revenue Invariant
func TestSourcesAnalytics_PopulatedAggregation_AllDimensions(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	testutil.AssertTestDatabase(t, db)

	repo := marketing.NewAnalyticsRepository(db)

	baseTime := time.Date(2043, 2, 1, 0, 0, 0, 0, time.UTC)
	from := baseTime
	to := baseTime.Add(24 * time.Hour)
	now := baseTime.Add(12 * time.Hour)

	// Create seller, category, products, variants
	sellerID := uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES ($1, 'Mega Brand', $2, $3, 'active')
	`, sellerID, "slug-"+sellerID.String(), sellerID.String()+"@test.com")
	require.NoError(t, err)

	catID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', $2)`, catID, "cat-"+catID.String())
	require.NoError(t, err)

	p1, p2 := uuid.New(), uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status)
		VALUES
			($1, $3, $4, 'P1', $5, 10000, 'published'),
			($2, $3, $4, 'P2', $6, 5000, 'published')
	`, p1, p2, sellerID, catID, "p1-"+p1.String(), "p2-"+p2.String())
	require.NoError(t, err)

	v1, v2 := uuid.New(), uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents, is_active)
		VALUES
			($1, $3, $5, 10000, true),
			($2, $4, $6, 5000, true)
	`, v1, v2, p1, p2, "v1-"+v1.String(), "v2-"+v2.String())
	require.NoError(t, err)

	campID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO marketing_campaigns (id, title, campaign_type, purpose, seller_id, starts_at, ends_at, funding_mode, status, discount_type, seller_discount_bps, planned_budget_cents)
		VALUES ($1, 'Winter Promo', 'discount', 'advertising', NULL, $2, $3, 'seller', 'active', 'percent', 2000, 100000)
	`, campID, from, to)
	require.NoError(t, err)

	promoID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO promo_codes (id, campaign_id, seller_id, code, discount_type, discount_value_bps, is_active, product_scope, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'percent', 2000, true, 'ENTIRE_STORE', now(), now())
	`, promoID, campID, sellerID, "PROMO-"+promoID.String()[:8])
	require.NoError(t, err)

	cleanFrom := from.Add(-48 * time.Hour)
	cleanTo := to.Add(48 * time.Hour)
	cleanPopulated := func() {
		_, _ = db.Exec(ctx, `DELETE FROM return_items WHERE return_id IN (SELECT id FROM returns WHERE COALESCE(completed_at, created_at) >= $1 AND COALESCE(completed_at, created_at) <= $2) OR (created_at >= $1 AND created_at <= $2)`, cleanFrom, cleanTo)
		_, _ = db.Exec(ctx, `DELETE FROM returns WHERE COALESCE(completed_at, created_at) >= $1 AND COALESCE(completed_at, created_at) <= $2`, cleanFrom, cleanTo)
		_, _ = db.Exec(ctx, `DELETE FROM order_item_promotions WHERE created_at >= $1 AND created_at <= $2`, cleanFrom, cleanTo)
		_, _ = db.Exec(ctx, `DELETE FROM order_items WHERE created_at >= $1 AND created_at <= $2`, cleanFrom, cleanTo)
		_, _ = db.Exec(ctx, `DELETE FROM payments WHERE created_at >= $1 AND created_at <= $2`, cleanFrom, cleanTo)
		_, _ = db.Exec(ctx, `DELETE FROM order_fulfillments WHERE created_at >= $1 AND created_at <= $2`, cleanFrom, cleanTo)
		_, _ = db.Exec(ctx, `DELETE FROM order_attributions WHERE attributed_at >= $1 AND attributed_at <= $2`, cleanFrom, cleanTo)
		_, _ = db.Exec(ctx, `DELETE FROM orders WHERE created_at >= $1 AND created_at <= $2`, cleanFrom, cleanTo)
		_, _ = db.Exec(ctx, `DELETE FROM analytics_sessions WHERE started_at >= $1 AND started_at <= $2`, cleanFrom, cleanTo)
	}
	cleanPopulated()

	// Clean up at exit
	defer func() {
		cleanPopulated()
		_, _ = db.Exec(ctx, `DELETE FROM promo_codes WHERE id = $1`, promoID)
		_, _ = db.Exec(ctx, `DELETE FROM marketing_campaigns WHERE id = $1`, campID)
	}()

	// Customers:
	// userNew: first order inside period => New Customer
	userNew := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, status, created_at) VALUES ($1, 'New User', $2, 'hash', 'active', $3)`, userNew, userNew.String()+"@test.com", now)
	require.NoError(t, err)

	// userRepeat: first order before period => Repeat Customer
	userRepeat := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, status, created_at) VALUES ($1, 'Old User', $2, 'hash', 'active', $3)`, userRepeat, userRepeat.String()+"@test.com", from.Add(-48*time.Hour))
	require.NoError(t, err)

	// Old historical order for userRepeat before period
	oldOrder := uuid.New()
	oldFul := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oldOrder, userRepeat, sellerID, oldFul, "paid", 1000, from.Add(-24*time.Hour)))

	visitorID := uuid.New()

	// A. DIRECT: 5 sessions, 1 paid order (new customer, p1: 1 unit, 10000 cents)
	directStr := "direct"
	for i := 0; i < 5; i++ {
		require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &directStr, nil, now))
	}
	oDirect := uuid.New()
	fulDirect := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oDirect, userNew, sellerID, fulDirect, "paid", 10000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oDirect, "succeeded", 10000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oDirect, fulDirect, p1, v1, sellerID, 10000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oDirect, nil, &directStr, nil))

	// B. UNATTRIBUTED: 3 sessions (1 null, 1 empty, 1 null with campaign), 1 paid order (repeat customer, p2: 1 unit, discounted to 4000 via promotions), 1 return
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, nil, nil, now))
	emptyStr := ""
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &emptyStr, nil, now))
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, nil, &campID, now))

	oUnattr := uuid.New()
	fulUnattr := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oUnattr, userRepeat, sellerID, fulUnattr, "paid", 4000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oUnattr, "succeeded", 4000, now))
	oiUnattr := uuid.New()
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, oiUnattr, oUnattr, fulUnattr, p2, v2, sellerID, 5000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderItemPromotion(ctx, db, uuid.New(), oiUnattr, oUnattr, sellerID, campID, promoID, 4000))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oUnattr, nil, nil, nil))

	// Return for Unattributed order completed inside period
	retCompletedAt := now.Add(time.Hour)
	require.NoError(t, insertAnalyticsTestReturn(ctx, db, uuid.New(), oUnattr, fulUnattr, userRepeat, oiUnattr, "completed", &retCompletedAt))

	// C. VK: 10 sessions, 2 paid orders (total revenue 30000, 3 units)
	vkStr := "vk"
	for i := 0; i < 10; i++ {
		require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &vkStr, nil, now))
	}
	oVk1, fulVk1 := uuid.New(), uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oVk1, userNew, sellerID, fulVk1, "paid", 20000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oVk1, "succeeded", 20000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oVk1, fulVk1, p1, v1, sellerID, 10000, 2, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oVk1, nil, &vkStr, nil))

	oVk2, fulVk2 := uuid.New(), uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oVk2, userNew, sellerID, fulVk2, "paid", 10000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oVk2, "succeeded", 10000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oVk2, fulVk2, p1, v1, sellerID, 10000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oVk2, nil, &vkStr, nil))

	// D. TELEGRAM: 8 sessions, 1 order with MULTIPLE items (item1: 1 unit 10000, item2: 2 units 5000 => 20000 cents, 3 sold units, 1 paid order)
	teleStr := "telegram"
	for i := 0; i < 8; i++ {
		require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &teleStr, nil, now))
	}
	oTele, fulTele := uuid.New(), uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oTele, userNew, sellerID, fulTele, "paid", 20000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oTele, "succeeded", 20000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oTele, fulTele, p1, v1, sellerID, 10000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oTele, fulTele, p2, v2, sellerID, 5000, 2, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oTele, nil, &teleStr, nil))

	res, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to})
	require.NoError(t, err)
	require.NotNil(t, res)

	// Exactly 4 sources, no Cartesian multiplication
	require.Len(t, res.Sources, 4)

	sourceMap := make(map[string]marketing.SourcePerformanceRow)
	var totalSourceRevenue int64
	for _, row := range res.Sources {
		key := "unattributed"
		if row.SourceKey != nil {
			key = *row.SourceKey
		}
		sourceMap[key] = row
		totalSourceRevenue += row.RevenueCents
	}

	// Direct Assertions
	dRow := sourceMap["direct"]
	assert.Equal(t, 5, dRow.Visits)
	assert.Equal(t, 1, dRow.PaidOrders)
	assert.Equal(t, 1, dRow.SoldUnits)
	assert.Equal(t, int64(10000), dRow.RevenueCents)
	assert.Equal(t, 1, dRow.NewCustomers)
	assert.Equal(t, 0, dRow.RepeatCustomers)

	// Unattributed Assertions
	uRow := sourceMap["unattributed"]
	assert.Equal(t, 3, uRow.Visits)
	assert.Equal(t, 1, uRow.PaidOrders)
	assert.Equal(t, 1, uRow.SoldUnits)
	assert.Equal(t, int64(4000), uRow.RevenueCents, "Revenue must reflect order_item_promotions customer paid snapshot")
	assert.Equal(t, 0, uRow.NewCustomers)
	assert.Equal(t, 1, uRow.RepeatCustomers, "userRepeat whose first order was before period must be counted as repeat customer")
	assert.Equal(t, 1, uRow.ReturnsCount, "Completed return in period must be counted")

	// VK Assertions
	vRow := sourceMap["vk"]
	assert.Equal(t, 10, vRow.Visits)
	assert.Equal(t, 2, vRow.PaidOrders)
	assert.Equal(t, 3, vRow.SoldUnits)
	assert.Equal(t, int64(30000), vRow.RevenueCents)

	// Telegram Assertions (order double-counting protection)
	tRow := sourceMap["telegram"]
	assert.Equal(t, 8, tRow.Visits)
	assert.Equal(t, 1, tRow.PaidOrders, "Order with 2 items must count as exactly 1 paid order")
	assert.Equal(t, 3, tRow.SoldUnits, "Sold units must sum quantities (1 + 2 = 3)")
	assert.Equal(t, int64(20000), tRow.RevenueCents)

	// Cross-layer invariant: sum(source revenue) == canonical customer paid order revenue
	expectedTotalRevenue := int64(10000 + 4000 + 30000 + 20000)
	assert.Equal(t, expectedTotalRevenue, totalSourceRevenue, "Cross-layer invariant: total source revenue must exactly equal sum of customer-paid order revenues")
}

// 4. Campaign + Source Matrix and No Fuzzy Text Inference
func TestSourcesAnalytics_CampaignAttributionMatrix(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	testutil.AssertTestDatabase(t, db)

	repo := marketing.NewAnalyticsRepository(db)

	baseTime := time.Date(2043, 3, 1, 0, 0, 0, 0, time.UTC)
	from := baseTime
	to := baseTime.Add(24 * time.Hour)
	now := baseTime.Add(12 * time.Hour)

	// Create canonical campaigns
	campA, campB := uuid.New(), uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO marketing_campaigns (id, title, campaign_type, purpose, seller_id, starts_at, ends_at, funding_mode, status, discount_type, seller_discount_bps, planned_budget_cents)
		VALUES
			($1, 'Campaign VK Special', 'discount', 'advertising', NULL, $3, $4, 'seller', 'active', 'percent', 2000, 100000),
			($2, 'Campaign Unattributed Special', 'discount', 'advertising', NULL, $3, $4, 'seller', 'active', 'percent', 2000, 100000)
	`, campA, campB, from, to)
	require.NoError(t, err)
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM marketing_campaigns WHERE id IN ($1, $2)`, campA, campB)
	}()

	visitorID := uuid.New()
	vkStr := "vk"
	directStr := "direct"

	// Case A: source = "vk", campaign_id = campA
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &vkStr, &campA, now))

	// Case B: source = NULL, campaign_id = campB
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, nil, &campB, now))

	// Case C: source = "direct", campaign_id = NULL
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &directStr, nil, now))

	// Case D: text-only utm_campaign matches campaign title, but campaign_id IS NULL => MUST NOT infer campaign
	_, err = db.Exec(ctx, `
		INSERT INTO analytics_sessions (id, visitor_id, source, utm_campaign, campaign_id, started_at, last_seen_at)
		VALUES ($1, $2, 'vk', 'Campaign VK Special', NULL, $3, $3)
	`, uuid.New(), visitorID, now)
	require.NoError(t, err)

	// Check Sources List
	resList, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to})
	require.NoError(t, err)
	require.Len(t, resList.Sources, 3) // vk (2 visits), unattributed (1 visit), direct (1 visit)

	// Check Detail for VK: must list only explicit campaign campA
	detVk, err := repo.GetSourceDetail(ctx, "vk", marketing.SourceDetailRequest{From: from, To: to})
	require.NoError(t, err)
	require.Len(t, detVk.Campaigns, 1)
	assert.Equal(t, campA.String(), *detVk.Campaigns[0].CampaignID)
	assert.Equal(t, 1, detVk.Campaigns[0].Visits, "Only explicit campaign_id link counted for campaign; text-only utm session excluded")

	// Check Detail for _unattributed: must list explicit campB
	detUnattr, err := repo.GetSourceDetail(ctx, "_unattributed", marketing.SourceDetailRequest{From: from, To: to})
	require.NoError(t, err)
	require.Len(t, detUnattr.Campaigns, 1)
	assert.Equal(t, campB.String(), *detUnattr.Campaigns[0].CampaignID)

	// Check Detail for direct: no campaigns
	detDirect, err := repo.GetSourceDetail(ctx, "direct", marketing.SourceDetailRequest{From: from, To: to})
	require.NoError(t, err)
	assert.Empty(t, detDirect.Campaigns)
}

// 5. Period Boundaries: Exact [from, to) semantics
func TestSourcesAnalytics_PeriodBoundarySemantics(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	testutil.AssertTestDatabase(t, db)

	repo := marketing.NewAnalyticsRepository(db)

	baseTime := time.Date(2043, 4, 1, 10, 0, 0, 0, time.UTC)
	from := baseTime
	to := baseTime.Add(24 * time.Hour)

	cleanFrom := from.Add(-48 * time.Hour)
	cleanTo := to.Add(48 * time.Hour)
	_, err := db.Exec(ctx, `DELETE FROM analytics_sessions WHERE started_at >= $1 AND started_at <= $2`, cleanFrom, cleanTo)
	require.NoError(t, err)
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM analytics_sessions WHERE started_at >= $1 AND started_at <= $2`, cleanFrom, cleanTo)
	}()

	visitorID := uuid.New()
	src := "vk"

	// 1. Before period
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &src, nil, from.Add(-time.Second)))
	// 2. Exact inclusive start (from)
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &src, nil, from))
	// 3. Inside period
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &src, nil, from.Add(12*time.Hour)))
	// 4. Exact exclusive upper bound (to)
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &src, nil, to))
	// 5. After period
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &src, nil, to.Add(time.Second)))

	res, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to})
	require.NoError(t, err)
	require.Len(t, res.Sources, 1)
	assert.Equal(t, 2, res.Sources[0].Visits, "Exact [from, to) window must include exact start and inside, excluding before, exact to, and after")
}

// 6. Revenue Truth: Exclude Unpaid, Non-succeeded, and Cancelled
func TestSourcesAnalytics_RevenueTruthAndExclusions(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	testutil.AssertTestDatabase(t, db)

	repo := marketing.NewAnalyticsRepository(db)

	baseTime := time.Date(2043, 5, 1, 0, 0, 0, 0, time.UTC)
	from := baseTime
	to := baseTime.Add(24 * time.Hour)
	now := baseTime.Add(12 * time.Hour)

	sellerID := uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES ($1, 'Rev Brand', $2, $3, 'active')
	`, sellerID, "slug-"+sellerID.String(), sellerID.String()+"@test.com")
	require.NoError(t, err)

	catID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', $2)`, catID, "cat-"+catID.String())
	require.NoError(t, err)

	pID, vID := uuid.New(), uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status) VALUES ($1, $2, $3, 'P', $4, 5000, 'published')`, pID, sellerID, catID, "p-"+pID.String())
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, $3, 5000, true)`, vID, pID, "v-"+vID.String())
	require.NoError(t, err)

	userID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, status) VALUES ($1, 'U', $2, 'hash', 'active')`, userID, userID.String()+"@test.com")
	require.NoError(t, err)

	src := "vk"

	oSuc, oPend, oFail, oCanc := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM order_items WHERE order_id IN ($1, $2, $3, $4)`, oSuc, oPend, oFail, oCanc)
		_, _ = db.Exec(ctx, `DELETE FROM payments WHERE order_id IN ($1, $2, $3, $4)`, oSuc, oPend, oFail, oCanc)
		_, _ = db.Exec(ctx, `DELETE FROM order_fulfillments WHERE order_id IN ($1, $2, $3, $4)`, oSuc, oPend, oFail, oCanc)
		_, _ = db.Exec(ctx, `DELETE FROM order_attributions WHERE order_id IN ($1, $2, $3, $4)`, oSuc, oPend, oFail, oCanc)
		_, _ = db.Exec(ctx, `DELETE FROM orders WHERE id IN ($1, $2, $3, $4)`, oSuc, oPend, oFail, oCanc)
	}()

	// 1. Succeeded payment: included (5000 cents)
	fulSuc := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oSuc, userID, sellerID, fulSuc, "paid", 5000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oSuc, "succeeded", 5000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oSuc, fulSuc, pID, vID, sellerID, 5000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oSuc, nil, &src, nil))

	// 2. Pending payment: excluded
	fulPend := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oPend, userID, sellerID, fulPend, "awaiting_payment", 5000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oPend, "pending", 5000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oPend, fulPend, pID, vID, sellerID, 5000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oPend, nil, &src, nil))

	// 3. Failed payment: excluded
	fulFail := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oFail, userID, sellerID, fulFail, "awaiting_payment", 5000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oFail, "failed", 5000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oFail, fulFail, pID, vID, sellerID, 5000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oFail, nil, &src, nil))

	// 4. Cancelled order with succeeded payment: excluded
	fulCanc := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oCanc, userID, sellerID, fulCanc, "cancelled", 5000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oCanc, "succeeded", 5000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oCanc, fulCanc, pID, vID, sellerID, 5000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oCanc, nil, &src, nil))

	res, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to})
	require.NoError(t, err)
	require.Len(t, res.Sources, 1)
	assert.Equal(t, 1, res.Sources[0].PaidOrders, "Only succeeded payment on non-cancelled order is counted")
	assert.Equal(t, int64(5000), res.Sources[0].RevenueCents, "Only succeeded payment revenue is counted")
}

// 7. Conversion Coverage & Math
func TestSourcesAnalytics_ConversionTrustworthiness(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	testutil.AssertTestDatabase(t, db)

	repo := marketing.NewAnalyticsRepository(db)

	baseTime := time.Date(2043, 6, 1, 0, 0, 0, 0, time.UTC)
	from := baseTime
	to := baseTime.Add(24 * time.Hour)
	now := baseTime.Add(12 * time.Hour)

	sellerID := uuid.New()
	_, err := db.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Conv Brand', $2, $3, 'active')`, sellerID, "slug-"+sellerID.String(), sellerID.String()+"@test.com")
	require.NoError(t, err)
	catID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', $2)`, catID, "cat-"+catID.String())
	require.NoError(t, err)
	pID, vID := uuid.New(), uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status) VALUES ($1, $2, $3, 'P', $4, 1000, 'published')`, pID, sellerID, catID, "p-"+pID.String())
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, $3, 1000, true)`, vID, pID, "v-"+vID.String())
	require.NoError(t, err)
	userID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, status) VALUES ($1, 'U', $2, 'hash', 'active')`, userID, userID.String()+"@test.com")
	require.NoError(t, err)

	visitorID := uuid.New()
	src := "vk"

	_, _ = db.Exec(ctx, `DELETE FROM analytics_sessions WHERE started_at >= $1 AND started_at < $2`, from, to)

	var orderIDs []uuid.UUID
	defer func() {
		if len(orderIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM order_items WHERE order_id = ANY($1)`, orderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM payments WHERE order_id = ANY($1)`, orderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM order_fulfillments WHERE order_id = ANY($1)`, orderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM order_attributions WHERE order_id = ANY($1)`, orderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM orders WHERE id = ANY($1)`, orderIDs)
		}
		_, _ = db.Exec(ctx, `DELETE FROM analytics_sessions WHERE started_at >= $1 AND started_at < $2`, from, to)
	}()

	// 10 sessions, 2 paid orders => 20% conversion (0.20)
	for i := 0; i < 10; i++ {
		require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &src, nil, now))
	}
	for i := 0; i < 2; i++ {
		oID := uuid.New()
		orderIDs = append(orderIDs, oID)
		fulID := uuid.New()
		require.NoError(t, insertAnalyticsTestOrder(ctx, db, oID, userID, sellerID, fulID, "paid", 1000, now))
		require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oID, "succeeded", 1000, now))
		require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oID, fulID, pID, vID, sellerID, 1000, 1, now))
		require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oID, nil, &src, nil))
	}

	// Telegram: 5 sessions, 0 paid orders => 0% conversion
	srcTele := "telegram"
	for i := 0; i < 5; i++ {
		require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &srcTele, nil, now))
	}

	res, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to})
	require.NoError(t, err)
	require.Len(t, res.Sources, 2)

	var vkRow, teleRow *marketing.SourcePerformanceRow
	for i := range res.Sources {
		if res.Sources[i].SourceKey != nil && *res.Sources[i].SourceKey == "vk" {
			vkRow = &res.Sources[i]
		} else if res.Sources[i].SourceKey != nil && *res.Sources[i].SourceKey == "telegram" {
			teleRow = &res.Sources[i]
		}
	}
	require.NotNil(t, vkRow)
	require.NotNil(t, teleRow)

	assert.InDelta(t, 0.20, vkRow.ConversionRate, 0.0001, "10 visits and 2 orders must yield 0.20 conversion")
	assert.Equal(t, 0.0, teleRow.ConversionRate, "5 visits and 0 orders must yield exactly 0.0 conversion")

	// Incomplete coverage check: sorting by conversion when views tracking is incomplete returns ErrConversionCoverageIncomplete
	sortConv := marketing.SourceSortConversion
	_, err = repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to, Sort: &sortConv})
	assert.ErrorIs(t, err, marketing.ErrConversionCoverageIncomplete, "Sorting by conversion when coverage incomplete must fail")
}

// 8. Search: Case-insensitive and Raw Source Based
func TestSourcesAnalytics_Search(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	testutil.AssertTestDatabase(t, db)

	repo := marketing.NewAnalyticsRepository(db)

	baseTime := time.Date(2043, 7, 1, 0, 0, 0, 0, time.UTC)
	from := baseTime
	to := baseTime.Add(24 * time.Hour)
	now := baseTime.Add(12 * time.Hour)

	visitorID := uuid.New()
	sVK := "vk"
	sTele := "telegram"
	sYan := "yandex"

	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &sVK, nil, now))
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &sTele, nil, now))
	require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &sYan, nil, now))

	// Search "vk" case-insensitively
	qUpper := "VK"
	resUpper, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to, Search: &qUpper})
	require.NoError(t, err)
	require.Len(t, resUpper.Sources, 1)
	assert.Equal(t, "vk", *resUpper.Sources[0].SourceKey)

	// Partial search "tele"
	qPart := "tele"
	resPart, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to, Search: &qPart})
	require.NoError(t, err)
	require.Len(t, resPart.Sources, 1)
	assert.Equal(t, "telegram", *resPart.Sources[0].SourceKey)
}

// 9. Sort Enum & Deterministic Ordering
func TestSourcesAnalytics_SortEnum(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	testutil.AssertTestDatabase(t, db)

	repo := marketing.NewAnalyticsRepository(db)

	baseTime := time.Date(2043, 8, 1, 0, 0, 0, 0, time.UTC)
	from := baseTime
	to := baseTime.Add(24 * time.Hour)
	now := baseTime.Add(12 * time.Hour)

	sellerID := uuid.New()
	_, err := db.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Sort Brand', $2, $3, 'active')`, sellerID, "slug-"+sellerID.String(), sellerID.String()+"@test.com")
	require.NoError(t, err)
	catID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', $2)`, catID, "cat-"+catID.String())
	require.NoError(t, err)
	pID, vID := uuid.New(), uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status) VALUES ($1, $2, $3, 'P', $4, 1000, 'published')`, pID, sellerID, catID, "p-"+pID.String())
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, $3, 1000, true)`, vID, pID, "v-"+vID.String())
	require.NoError(t, err)
	userID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, status) VALUES ($1, 'U', $2, 'hash', 'active')`, userID, userID.String()+"@test.com")
	require.NoError(t, err)

	visitorID := uuid.New()

	var sortOrderIDs []uuid.UUID
	defer func() {
		if len(sortOrderIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM order_items WHERE order_id = ANY($1)`, sortOrderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM payments WHERE order_id = ANY($1)`, sortOrderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM order_fulfillments WHERE order_id = ANY($1)`, sortOrderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM order_attributions WHERE order_id = ANY($1)`, sortOrderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM orders WHERE id = ANY($1)`, sortOrderIDs)
		}
		_, _ = db.Exec(ctx, `DELETE FROM analytics_sessions WHERE started_at >= $1 AND started_at < $2`, from, to)
	}()

	// Source A (vk): visits 100, rev 500, orders 1
	srcA := "vk"
	for i := 0; i < 100; i++ {
		require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &srcA, nil, now))
	}
	oA := uuid.New()
	sortOrderIDs = append(sortOrderIDs, oA)
	fulA := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oA, userID, sellerID, fulA, "paid", 500, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oA, "succeeded", 500, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oA, fulA, pID, vID, sellerID, 500, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oA, nil, &srcA, nil))

	// Source B (telegram): visits 50, rev 2000, orders 2
	srcB := "telegram"
	for i := 0; i < 50; i++ {
		require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &srcB, nil, now))
	}
	for i := 0; i < 2; i++ {
		oB := uuid.New()
		sortOrderIDs = append(sortOrderIDs, oB)
		fulB := uuid.New()
		require.NoError(t, insertAnalyticsTestOrder(ctx, db, oB, userID, sellerID, fulB, "paid", 1000, now))
		require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oB, "succeeded", 1000, now))
		require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oB, fulB, pID, vID, sellerID, 1000, 1, now))
		require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oB, nil, &srcB, nil))
	}

	// Source C (yandex): visits 10, rev 3000, orders 3
	srcC := "yandex"
	for i := 0; i < 10; i++ {
		require.NoError(t, insertAnalyticsTestSession(ctx, db, uuid.New(), visitorID, &srcC, nil, now))
	}
	for i := 0; i < 3; i++ {
		oC := uuid.New()
		sortOrderIDs = append(sortOrderIDs, oC)
		fulC := uuid.New()
		require.NoError(t, insertAnalyticsTestOrder(ctx, db, oC, userID, sellerID, fulC, "paid", 1000, now))
		require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oC, "succeeded", 1000, now))
		require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oC, fulC, pID, vID, sellerID, 1000, 1, now))
		require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oC, nil, &srcC, nil))
	}

	// 1. Sort by visits: vk (100) -> telegram (50) -> yandex (10)
	sortVisits := marketing.SourceSortVisits
	resVisits, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to, Sort: &sortVisits})
	require.NoError(t, err)
	require.Len(t, resVisits.Sources, 3)
	assert.Equal(t, "vk", *resVisits.Sources[0].SourceKey)
	assert.Equal(t, "telegram", *resVisits.Sources[1].SourceKey)
	assert.Equal(t, "yandex", *resVisits.Sources[2].SourceKey)

	// 2. Sort by revenue: yandex (3000) -> telegram (2000) -> vk (500)
	sortRev := marketing.SourceSortRevenue
	resRev, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to, Sort: &sortRev})
	require.NoError(t, err)
	assert.Equal(t, "yandex", *resRev.Sources[0].SourceKey)
	assert.Equal(t, "telegram", *resRev.Sources[1].SourceKey)
	assert.Equal(t, "vk", *resRev.Sources[2].SourceKey)

	// 3. Sort by orders: yandex (3) -> telegram (2) -> vk (1)
	sortOrders := marketing.SourceSortOrders
	resOrders, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to, Sort: &sortOrders})
	require.NoError(t, err)
	assert.Equal(t, "yandex", *resOrders.Sources[0].SourceKey)
	assert.Equal(t, "telegram", *resOrders.Sources[1].SourceKey)
	assert.Equal(t, "vk", *resOrders.Sources[2].SourceKey)

	// 4. Invalid sort parameter returns ErrInvalidSort
	sortInvalid := "random_injected_sort"
	_, err = repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to, Sort: &sortInvalid})
	assert.ErrorIs(t, err, marketing.ErrInvalidSort)
}

// 10. Growth & Decline Calculations (Zero-baseline math)
func TestSourcesAnalytics_GrowthDeclineMath(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	testutil.AssertTestDatabase(t, db)

	repo := marketing.NewAnalyticsRepository(db)

	baseTime := time.Date(2043, 9, 1, 0, 0, 0, 0, time.UTC)
	from := baseTime
	to := baseTime.Add(24 * time.Hour)
	now := baseTime.Add(12 * time.Hour)
	prevNow := from.Add(-12 * time.Hour)

	sellerID := uuid.New()
	_, err := db.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Growth Brand', $2, $3, 'active')`, sellerID, "slug-"+sellerID.String(), sellerID.String()+"@test.com")
	require.NoError(t, err)
	catID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', $2)`, catID, "cat-"+catID.String())
	require.NoError(t, err)
	pID, vID := uuid.New(), uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status) VALUES ($1, $2, $3, 'P', $4, 1000, 'published')`, pID, sellerID, catID, "p-"+pID.String())
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, $3, 1000, true)`, vID, pID, "v-"+vID.String())
	require.NoError(t, err)
	userID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, status) VALUES ($1, 'U', $2, 'hash', 'active')`, userID, userID.String()+"@test.com")
	require.NoError(t, err)

	srcVk := "vk"
	srcTele := "telegram"
	srcDirect := "direct"

	var growthOrderIDs []uuid.UUID
	defer func() {
		if len(growthOrderIDs) > 0 {
			_, _ = db.Exec(ctx, `DELETE FROM order_items WHERE order_id = ANY($1)`, growthOrderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM payments WHERE order_id = ANY($1)`, growthOrderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM order_fulfillments WHERE order_id = ANY($1)`, growthOrderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM order_attributions WHERE order_id = ANY($1)`, growthOrderIDs)
			_, _ = db.Exec(ctx, `DELETE FROM orders WHERE id = ANY($1)`, growthOrderIDs)
		}
	}()

	// 1. VK: Previous 10000 cents, Current 20000 cents => +100%
	oVkPrev := uuid.New()
	growthOrderIDs = append(growthOrderIDs, oVkPrev)
	fulVkPrev := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oVkPrev, userID, sellerID, fulVkPrev, "paid", 10000, prevNow))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oVkPrev, "succeeded", 10000, prevNow))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oVkPrev, fulVkPrev, pID, vID, sellerID, 10000, 1, prevNow))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oVkPrev, nil, &srcVk, nil))

	oVkCurr := uuid.New()
	growthOrderIDs = append(growthOrderIDs, oVkCurr)
	fulVkCurr := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oVkCurr, userID, sellerID, fulVkCurr, "paid", 20000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oVkCurr, "succeeded", 20000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oVkCurr, fulVkCurr, pID, vID, sellerID, 20000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oVkCurr, nil, &srcVk, nil))

	// 2. Telegram: Previous 10000 cents, Current 5000 cents => -50%
	oTelePrev := uuid.New()
	growthOrderIDs = append(growthOrderIDs, oTelePrev)
	fulTelePrev := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oTelePrev, userID, sellerID, fulTelePrev, "paid", 10000, prevNow))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oTelePrev, "succeeded", 10000, prevNow))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oTelePrev, fulTelePrev, pID, vID, sellerID, 10000, 1, prevNow))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oTelePrev, nil, &srcTele, nil))

	oTeleCurr := uuid.New()
	growthOrderIDs = append(growthOrderIDs, oTeleCurr)
	fulTeleCurr := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oTeleCurr, userID, sellerID, fulTeleCurr, "paid", 5000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oTeleCurr, "succeeded", 5000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oTeleCurr, fulTeleCurr, pID, vID, sellerID, 5000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oTeleCurr, nil, &srcTele, nil))

	// 3. Direct: Previous 0 cents, Current 5000 cents => nil (zero baseline, no infinity)
	oDirCurr := uuid.New()
	growthOrderIDs = append(growthOrderIDs, oDirCurr)
	fulDirCurr := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oDirCurr, userID, sellerID, fulDirCurr, "paid", 5000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oDirCurr, "succeeded", 5000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oDirCurr, fulDirCurr, pID, vID, sellerID, 5000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, oDirCurr, nil, &srcDirect, nil))

	// Sort Revenue Growth: vk (+100%) -> telegram (-50%) -> direct (nil / last)
	sortGrowth := marketing.SourceSortRevenueGrowth
	resGrowth, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to, Sort: &sortGrowth})
	require.NoError(t, err)
	require.Len(t, resGrowth.Sources, 3)

	assert.Equal(t, "vk", *resGrowth.Sources[0].SourceKey)
	require.NotNil(t, resGrowth.Sources[0].RevenueChangePct)
	assert.InDelta(t, 100.0, *resGrowth.Sources[0].RevenueChangePct, 0.01)

	assert.Equal(t, "telegram", *resGrowth.Sources[1].SourceKey)
	require.NotNil(t, resGrowth.Sources[1].RevenueChangePct)
	assert.InDelta(t, -50.0, *resGrowth.Sources[1].RevenueChangePct, 0.01)

	assert.Equal(t, "direct", *resGrowth.Sources[2].SourceKey)
	assert.Nil(t, resGrowth.Sources[2].RevenueChangePct, "Zero-baseline revenue change must be nil, never infinity")

	// Sort Revenue Drop: telegram (-50%) must rank FIRST
	sortDrop := marketing.SourceSortRevenueDrop
	resDrop, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to, Sort: &sortDrop})
	require.NoError(t, err)
	assert.Equal(t, "telegram", *resDrop.Sources[0].SourceKey, "Negative change (-50%) must rank first in drop sort")
}

// 11. Source Detail: Identity, Trend, Top Products, Top Designers, Campaigns, Returns
func TestSourcesAnalytics_SourceDetail_Comprehensive(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	testutil.AssertTestDatabase(t, db)

	repo := marketing.NewAnalyticsRepository(db)

	baseTime := time.Date(2043, 10, 1, 0, 0, 0, 0, time.UTC)
	from := baseTime
	to := baseTime.Add(24 * time.Hour)
	now := baseTime.Add(12 * time.Hour)

	// Designers A and B
	desA, desB := uuid.New(), uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES
			($1, 'Designer Alpha', $3, $4, 'active'),
			($2, 'Designer Beta', $5, $6, 'active')
	`, desA, desB, "desA-"+desA.String(), desA.String()+"@test.com", "desB-"+desB.String(), desB.String()+"@test.com")
	require.NoError(t, err)

	catID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', $2)`, catID, "cat-"+catID.String())
	require.NoError(t, err)

	// Designer Alpha has 2 products (pA1, pA2), Designer Beta has 1 product (pB)
	pA1, pA2, pB := uuid.New(), uuid.New(), uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status)
		VALUES
			($1, $4, $6, 'Product Alpha 1', $7, 10000, 'published'),
			($2, $4, $6, 'Product Alpha 2', $8, 5000, 'published'),
			($3, $5, $6, 'Product Beta', $9, 8000, 'published')
	`, pA1, pA2, pB, desA, desB, catID, "pA1-"+pA1.String(), "pA2-"+pA2.String(), "pB-"+pB.String())
	require.NoError(t, err)

	vA1, vA2, vB := uuid.New(), uuid.New(), uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents, is_active)
		VALUES
			($1, $4, $7, 10000, true),
			($2, $5, $8, 5000, true),
			($3, $6, $9, 8000, true)
	`, vA1, vA2, vB, pA1, pA2, pB, "vA1-"+vA1.String(), "vA2-"+vA2.String(), "vB-"+vB.String())
	require.NoError(t, err)

	campID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO marketing_campaigns (id, title, campaign_type, purpose, seller_id, starts_at, ends_at, funding_mode, status, discount_type, seller_discount_bps, planned_budget_cents)
		VALUES ($1, 'VK Campaign', 'discount', 'advertising', NULL, $2, $3, 'seller', 'active', 'percent', 2000, 100000)
	`, campID, from, to)
	require.NoError(t, err)
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM marketing_campaigns WHERE id = $1`, campID)
	}()

	userID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, status) VALUES ($1, 'U', $2, 'hash', 'active')`, userID, userID.String()+"@test.com")
	require.NoError(t, err)

	srcVk := "vk"

	// Paid order for VK with pA1 (1 unit, 10000) + pA2 (1 unit, 5000) => Designer Alpha total 15000
	o1 := uuid.New()
	ful1 := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, o1, userID, desA, ful1, "paid", 15000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), o1, "succeeded", 15000, now))
	oi1 := uuid.New()
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, oi1, o1, ful1, pA1, vA1, desA, 10000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), o1, ful1, pA2, vA2, desA, 5000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, o1, nil, &srcVk, &campID))

	// Paid order for VK with pB (1 unit, 8000) => Designer Beta total 8000
	o2 := uuid.New()
	ful2 := uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, o2, userID, desB, ful2, "paid", 8000, now))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), o2, "succeeded", 8000, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), o2, ful2, pB, vB, desB, 8000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderAttribution(ctx, db, o2, nil, &srcVk, nil))

	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM order_items WHERE order_id IN ($1, $2)`, o1, o2)
		_, _ = db.Exec(ctx, `DELETE FROM payments WHERE order_id IN ($1, $2)`, o1, o2)
		_, _ = db.Exec(ctx, `DELETE FROM order_fulfillments WHERE order_id IN ($1, $2)`, o1, o2)
		_, _ = db.Exec(ctx, `DELETE FROM order_attributions WHERE order_id IN ($1, $2)`, o1, o2)
		_, _ = db.Exec(ctx, `DELETE FROM orders WHERE id IN ($1, $2)`, o1, o2)
	}()

	// Query Source Detail for "vk"
	detVk, err := repo.GetSourceDetail(ctx, "vk", marketing.SourceDetailRequest{From: from, To: to})
	require.NoError(t, err)
	require.NotNil(t, detVk)

	// 1. Identity
	assert.Equal(t, marketing.SourceKindNamed, detVk.Source.SourceKind)
	require.NotNil(t, detVk.Source.SourceKey)
	assert.Equal(t, "vk", *detVk.Source.SourceKey)
	assert.Equal(t, 2, detVk.Source.PaidOrders)
	assert.Equal(t, int64(23000), detVk.Source.RevenueCents)

	// 2. Trend: Continuous daily buckets
	require.NotEmpty(t, detVk.Trend)
	var trendRev int64
	for _, tp := range detVk.Trend {
		assert.NotEmpty(t, tp.Date)
		trendRev += tp.RevenueCents
	}
	assert.Equal(t, int64(23000), trendRev, "Sum of trend daily revenue must match source total revenue")

	// 3. Top Products: Ranked by revenue DESC, formatted ID, no raw UUID
	require.Len(t, detVk.TopProducts, 3)
	assert.Equal(t, "Product Alpha 1", detVk.TopProducts[0].ProductName)
	assert.Equal(t, int64(10000), detVk.TopProducts[0].RevenueCents)
	assert.Regexp(t, `^prod_[0-9a-f]{16}$`, detVk.TopProducts[0].ProductID, "ProductID must follow formatted prefix prod_... without raw UUID")

	assert.Equal(t, "Product Beta", detVk.TopProducts[1].ProductName)
	assert.Equal(t, int64(8000), detVk.TopProducts[1].RevenueCents)

	assert.Equal(t, "Product Alpha 2", detVk.TopProducts[2].ProductName)
	assert.Equal(t, int64(5000), detVk.TopProducts[2].RevenueCents)

	// 4. Top Designers: Designer Alpha aggregates both products (15000), ranks first
	require.Len(t, detVk.TopDesigners, 2)
	assert.Equal(t, "Designer Alpha", detVk.TopDesigners[0].DesignerName)
	assert.Equal(t, int64(15000), detVk.TopDesigners[0].RevenueCents)
	assert.Regexp(t, `^dsgn_[0-9a-f]{16}$`, detVk.TopDesigners[0].DesignerID, "DesignerID must follow formatted prefix dsgn_... without raw UUID")

	assert.Equal(t, "Designer Beta", detVk.TopDesigners[1].DesignerName)
	assert.Equal(t, int64(8000), detVk.TopDesigners[1].RevenueCents)

	// 5. Explicit Campaigns
	require.Len(t, detVk.Campaigns, 1)
	assert.Equal(t, "VK Campaign", detVk.Campaigns[0].Name)
	assert.Equal(t, int64(15000), detVk.Campaigns[0].RevenueCents)

	// 6. Source Detail: Direct
	detDirect, err := repo.GetSourceDetail(ctx, "direct", marketing.SourceDetailRequest{From: from, To: to})
	require.NoError(t, err)
	assert.Equal(t, marketing.SourceKindDirect, detDirect.Source.SourceKind)
	require.NotNil(t, detDirect.Source.SourceKey)
	assert.Equal(t, "direct", *detDirect.Source.SourceKey)

	// 7. Source Detail: _unattributed sentinel
	detUnattr, err := repo.GetSourceDetail(ctx, "_unattributed", marketing.SourceDetailRequest{From: from, To: to})
	require.NoError(t, err)
	assert.Equal(t, marketing.SourceKindUnattributed, detUnattr.Source.SourceKind)
	assert.Nil(t, detUnattr.Source.SourceKey)

	// 8. Literal "unattributed" named source collision protection
	detLiteral, err := repo.GetSourceDetail(ctx, "unattributed", marketing.SourceDetailRequest{From: from, To: to})
	require.NoError(t, err)
	assert.Equal(t, marketing.SourceKindNamed, detLiteral.Source.SourceKind, "Literal 'unattributed' without underscore must resolve as named source")
	require.NotNil(t, detLiteral.Source.SourceKey)
	assert.Equal(t, "unattributed", *detLiteral.Source.SourceKey)

	// 9. Unknown source returns canonical empty detail
	detUnknown, err := repo.GetSourceDetail(ctx, "nonexistent_source_123", marketing.SourceDetailRequest{From: from, To: to})
	require.NoError(t, err)
	assert.Equal(t, marketing.SourceKindNamed, detUnknown.Source.SourceKind)
	assert.Equal(t, "nonexistent_source_123", *detUnknown.Source.SourceKey)
	assert.Equal(t, 0, detUnknown.Source.Visits)
	assert.Equal(t, 0, detUnknown.Source.PaidOrders)
	assert.Empty(t, detUnknown.TopProducts)
	assert.Empty(t, detUnknown.TopDesigners)
	assert.Empty(t, detUnknown.Campaigns)
}

// 12. Canonical Truth No Cyrillic / Russian localized text in backend DTOs
func TestSourcesAnalytics_CanonicalTruthNoCyrillic(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()
	testutil.AssertTestDatabase(t, db)

	repo := marketing.NewAnalyticsRepository(db)

	baseTime := time.Date(2043, 11, 1, 0, 0, 0, 0, time.UTC)
	from := baseTime
	to := baseTime.Add(24 * time.Hour)

	res, err := repo.GetSourcesAnalytics(ctx, marketing.SourceAnalyticsRequest{From: from, To: to})
	require.NoError(t, err)

	cyrillicRegex := regexp.MustCompile(`[\p{Cyrillic}]`)
	for _, row := range res.Sources {
		if row.SourceKey != nil {
			assert.False(t, cyrillicRegex.MatchString(*row.SourceKey), "sourceKey must never contain Cyrillic display labels")
		}
		assert.False(t, cyrillicRegex.MatchString(string(row.SourceKind)), "sourceKind must never contain Cyrillic display labels")
	}
}
