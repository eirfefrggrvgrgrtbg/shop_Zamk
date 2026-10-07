package marketing_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupDesignerTestDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	ctx := context.Background()
	dbURL := testutil.GetTestDatabaseURL()
	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)

	testutil.AssertTestDatabase(t, db)
	return db, ctx
}

func TestDesignerAnalytics_Consistency(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()

	// Create test fixtures (sellers, products, orders, returns, behavioral)
	seller1ID := uuid.New()
	seller2ID := uuid.New()

	_, err := db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES
			($1, 'Designer One', $3, $4, 'active'),
			($2, 'Designer Two', $5, $6, 'active')
	`, seller1ID, seller2ID, "slug-"+seller1ID.String(), seller1ID.String()+"@test.com", "slug-"+seller2ID.String(), seller2ID.String()+"@test.com")
	require.NoError(t, err)

	catID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', $2)`, catID, "cat-"+catID.String())
	require.NoError(t, err)

	p1 := uuid.New()
	p2 := uuid.New()
	p3 := uuid.New()

	_, err = db.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status)
		VALUES
			($1, $4, $6, 'P1', $7, 1000, 'published'),
			($2, $4, $6, 'P2', $8, 2000, 'published'),
			($3, $5, $6, 'P3', $9, 3000, 'published')
	`, p1, p2, p3, seller1ID, seller2ID, catID, "p1-"+p1.String(), "p2-"+p2.String(), "p3-"+p3.String())
	require.NoError(t, err)

	v1 := uuid.New()
	v2 := uuid.New()
	v3 := uuid.New()

	_, err = db.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents, is_active)
		VALUES
			($1, $4, $7, 1000, true),
			($2, $5, $8, 2000, true),
			($3, $6, $9, 3000, true)
	`, v1, v2, v3, p1, p2, p3, "sku1-"+p1.String(), "sku2-"+p2.String(), "sku3-"+p3.String())
	require.NoError(t, err)

	now := time.Now().UTC()
	from := now.Add(-time.Hour * 24 * 7)
	to := now.Add(time.Hour)

	userID := uuid.New()

	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, status) VALUES ($1, 'Test User', $2, 'hash', 'active')`, userID, userID.String()+"@test.com")
	require.NoError(t, err)

	o1 := uuid.New()
	o2 := uuid.New()
	f1 := uuid.New()
	f2 := uuid.New()

	require.NoError(t, insertAnalyticsTestOrder(ctx, db, o1, userID, seller1ID, f1, "paid", 3000, now))
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, o2, userID, seller2ID, f2, "paid", 3000, now))

	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), o1, "succeeded", 3000, from.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), o2, "succeeded", 3000, from.Add(time.Hour)))

	// Seller 1 sells 2 different products in order 1: p1 (1 unit, 1000 cents) + p2 (1 unit, 2000 cents)
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), o1, f1, p1, v1, seller1ID, 1000, 1, now))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), o1, f1, p2, v2, seller1ID, 2000, 1, now))

	// Seller 2 sells p3 (1 unit, 3000 cents) in order 2
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), o2, f2, p3, v3, seller2ID, 3000, 1, now))

	repo := marketing.NewAnalyticsRepository(db)

	catIDStr := catID.String()
	reqProd := marketing.ProductAnalyticsRequest{
		From:       from,
		To:         to,
		CategoryID: &catIDStr,
	}
	resProd, err := repo.GetProductAnalytics(ctx, reqProd)
	require.NoError(t, err)

	reqDes := marketing.DesignerAnalyticsRequest{
		From:       from,
		To:         to,
		CategoryID: &catIDStr,
	}
	resDes, err := repo.GetDesignerAnalytics(ctx, reqDes)
	require.NoError(t, err)

	// 1. Consistency invariant: sum(Product revenue) == sum(Designer revenue)
	var sumProdRev int64
	for _, p := range resProd.Products {
		sumProdRev += p.RevenueCents
	}

	var sumDesRev int64
	for _, d := range resDes.Designers {
		sumDesRev += d.RevenueCents
	}

	assert.Equal(t, int64(6000), sumProdRev, "Expected total product revenue 6000 cents")
	assert.Equal(t, sumProdRev, sumDesRev, "Designer revenue must exactly equal sum of product analytics revenue")

	// 2. Multi-product same-designer aggregation check
	// Designer One owns p1 and p2, aggregating both into 1 row
	var designerOne, designerTwo *marketing.DesignerAnalyticsRow
	for i := range resDes.Designers {
		if resDes.Designers[i].DesignerName == "Designer One" {
			designerOne = &resDes.Designers[i]
		}
		if resDes.Designers[i].DesignerName == "Designer Two" {
			designerTwo = &resDes.Designers[i]
		}
	}

	require.NotNil(t, designerOne, "Designer One must be present")
	assert.Equal(t, 2, designerOne.ProductsCount, "Designer One has 2 published products")
	assert.Equal(t, int64(3000), designerOne.RevenueCents, "Designer One revenue must be 1000 + 2000 = 3000 cents")
	assert.Equal(t, 2, designerOne.SoldUnits, "Designer One sold 2 units across 2 products")
	assert.Equal(t, 1, designerOne.Purchases, "Designer One has 1 distinct order (no double count)")

	// 3. Multi-designer distinct check
	require.NotNil(t, designerTwo, "Designer Two must be present and distinct")
	assert.Equal(t, 1, designerTwo.ProductsCount, "Designer Two has 1 published product")
	assert.Equal(t, int64(3000), designerTwo.RevenueCents, "Designer Two revenue is 3000 cents")
	assert.Equal(t, 1, designerTwo.SoldUnits, "Designer Two sold 1 unit")
	assert.Equal(t, 1, designerTwo.Purchases, "Designer Two has 1 distinct order")
	assert.NotEqual(t, designerOne.DesignerID, designerTwo.DesignerID, "Designers must have distinct hashed IDs")
	assert.NotContains(t, designerOne.DesignerID, "-", "Hashed designer ID must not be raw UUID")

	// 4. UUID / empty fallback logic
	fallback1 := uuid.New()
	fallback2 := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES
			($1, '', $3, $4, 'active'),
			($2, '00000000-0000-0000-0000-000000000000', $5, $6, 'active')
	`, fallback1, fallback2, "fallback-"+fallback1.String(), fallback1.String()+"@test.com", "fallback-"+fallback2.String(), fallback2.String()+"@test.com")
	require.NoError(t, err)
}

func TestDesignerAnalytics_Sorting(t *testing.T) {
	db, ctx := setupDesignerTestDB(t)
	defer db.Close()

	// Create 3 designers with distinct previous and current revenues:
	// Alpha: prev = 10000, curr = 20000 (+100.0% growth, highest current revenue)
	// Beta:  prev = 10000, curr = 5000  (-50.0% drop, lowest current revenue)
	// Gamma: prev = 5000,  curr = 10000 (+100.0% growth, medium current revenue)
	sAlpha := uuid.New()
	sBeta := uuid.New()
	sGamma := uuid.New()

	_, err := db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES
			($1, 'Designer Alpha', $4, $5, 'active'),
			($2, 'Designer Beta',  $6, $7, 'active'),
			($3, 'Designer Gamma', $8, $9, 'active')
	`, sAlpha, sBeta, sGamma,
		"alpha-"+sAlpha.String(), sAlpha.String()+"@test.com",
		"beta-"+sBeta.String(), sBeta.String()+"@test.com",
		"gamma-"+sGamma.String(), sGamma.String()+"@test.com",
	)
	require.NoError(t, err)

	catID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'SortCat', $2)`, catID, "sortcat-"+catID.String())
	require.NoError(t, err)

	pAlpha := uuid.New()
	pBeta := uuid.New()
	pGamma := uuid.New()

	_, err = db.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status)
		VALUES
			($1, $4, $7, 'P Alpha', $8, 20000, 'published'),
			($2, $5, $7, 'P Beta',  $9, 5000,  'published'),
			($3, $6, $7, 'P Gamma', $10, 10000, 'published')
	`, pAlpha, pBeta, pGamma, sAlpha, sBeta, sGamma, catID,
		"palpha-"+pAlpha.String(), "pbeta-"+pBeta.String(), "pgamma-"+pGamma.String(),
	)
	require.NoError(t, err)

	vAlpha := uuid.New()
	vBeta := uuid.New()
	vGamma := uuid.New()

	_, err = db.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents, is_active)
		VALUES
			($1, $4, $7, 20000, true),
			($2, $5, $8, 5000,  true),
			($3, $6, $9, 10000, true)
	`, vAlpha, vBeta, vGamma, pAlpha, pBeta, pGamma,
		"skualpha-"+pAlpha.String(), "skubeta-"+pBeta.String(), "skugamma-"+pGamma.String(),
	)
	require.NoError(t, err)

	now := time.Now().UTC()
	from := now.Add(-time.Hour * 24 * 7)
	to := now.Add(time.Hour)
	duration := to.Sub(from)
	prevFrom := from.Add(-duration)

	userID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, status) VALUES ($1, 'Sort User', $2, 'hash', 'active')`, userID, userID.String()+"@test.com")
	require.NoError(t, err)

	// --- PREVIOUS PERIOD ORDERS ---
	// Alpha prev: 10000
	oPrevA, fPrevA := uuid.New(), uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oPrevA, userID, sAlpha, fPrevA, "paid", 10000, prevFrom.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oPrevA, "succeeded", 10000, prevFrom.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oPrevA, fPrevA, pAlpha, vAlpha, sAlpha, 10000, 1, prevFrom.Add(time.Hour)))

	// Beta prev: 10000
	oPrevB, fPrevB := uuid.New(), uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oPrevB, userID, sBeta, fPrevB, "paid", 10000, prevFrom.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oPrevB, "succeeded", 10000, prevFrom.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oPrevB, fPrevB, pBeta, vBeta, sBeta, 10000, 1, prevFrom.Add(time.Hour)))

	// Gamma prev: 5000
	oPrevG, fPrevG := uuid.New(), uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oPrevG, userID, sGamma, fPrevG, "paid", 5000, prevFrom.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oPrevG, "succeeded", 5000, prevFrom.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oPrevG, fPrevG, pGamma, vGamma, sGamma, 5000, 1, prevFrom.Add(time.Hour)))

	// --- CURRENT PERIOD ORDERS ---
	// Alpha curr: 20000 (+100%)
	oCurrA, fCurrA := uuid.New(), uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oCurrA, userID, sAlpha, fCurrA, "paid", 20000, from.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oCurrA, "succeeded", 20000, from.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oCurrA, fCurrA, pAlpha, vAlpha, sAlpha, 20000, 1, from.Add(time.Hour)))

	// Beta curr: 5000 (-50%)
	oCurrB, fCurrB := uuid.New(), uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oCurrB, userID, sBeta, fCurrB, "paid", 5000, from.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oCurrB, "succeeded", 5000, from.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oCurrB, fCurrB, pBeta, vBeta, sBeta, 5000, 1, from.Add(time.Hour)))

	// Gamma curr: 10000 (+100%)
	oCurrG, fCurrG := uuid.New(), uuid.New()
	require.NoError(t, insertAnalyticsTestOrder(ctx, db, oCurrG, userID, sGamma, fCurrG, "paid", 10000, from.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestPayment(ctx, db, uuid.New(), oCurrG, "succeeded", 10000, from.Add(time.Hour)))
	require.NoError(t, insertAnalyticsTestOrderItem(ctx, db, uuid.New(), oCurrG, fCurrG, pGamma, vGamma, sGamma, 10000, 1, from.Add(time.Hour)))

	repo := marketing.NewAnalyticsRepository(db)

	catIDStr := catID.String()

	// 1. REVENUE SORT
	sortRev := marketing.DesignerSortRevenue
	resRev, err := repo.GetDesignerAnalytics(ctx, marketing.DesignerAnalyticsRequest{
		From:       from,
		To:         to,
		Sort:       &sortRev,
		CategoryID: &catIDStr,
	})
	require.NoError(t, err)
	require.Len(t, resRev.Designers, 3)
	assert.Equal(t, "Designer Alpha", resRev.Designers[0].DesignerName, "Alpha highest current revenue (20,000)")
	assert.Equal(t, int64(20000), resRev.Designers[0].RevenueCents)
	assert.Equal(t, "Designer Gamma", resRev.Designers[1].DesignerName, "Gamma second current revenue (10,000)")
	assert.Equal(t, int64(10000), resRev.Designers[1].RevenueCents)
	assert.Equal(t, "Designer Beta", resRev.Designers[2].DesignerName, "Beta lowest current revenue (5,000)")
	assert.Equal(t, int64(5000), resRev.Designers[2].RevenueCents)

	// 2. REVENUE GROWTH SORT
	// Alpha and Gamma both grew +100.0%. Alpha has higher revenue (20,000 vs 10,000), tiebreaks first.
	// Beta declined -50.0%, ranks last.
	sortGrowth := marketing.DesignerSortRevenueGrowth
	resGrowth, err := repo.GetDesignerAnalytics(ctx, marketing.DesignerAnalyticsRequest{
		From:       from,
		To:         to,
		Sort:       &sortGrowth,
		CategoryID: &catIDStr,
	})
	require.NoError(t, err)
	require.Len(t, resGrowth.Designers, 3)
	assert.Equal(t, "Designer Alpha", resGrowth.Designers[0].DesignerName)
	require.NotNil(t, resGrowth.Designers[0].RevenueChangePct)
	assert.InDelta(t, 100.0, *resGrowth.Designers[0].RevenueChangePct, 0.01)

	assert.Equal(t, "Designer Gamma", resGrowth.Designers[1].DesignerName)
	require.NotNil(t, resGrowth.Designers[1].RevenueChangePct)
	assert.InDelta(t, 100.0, *resGrowth.Designers[1].RevenueChangePct, 0.01)

	assert.Equal(t, "Designer Beta", resGrowth.Designers[2].DesignerName)
	require.NotNil(t, resGrowth.Designers[2].RevenueChangePct)
	assert.InDelta(t, -50.0, *resGrowth.Designers[2].RevenueChangePct, 0.01)

	// 3. REVENUE DROP / DECLINE SORT
	// Beta declined -50.0%, must rank FIRST in decline/drop sort.
	// Gamma and Alpha (+100.0%) rank after.
	sortDrop := marketing.DesignerSortRevenueDrop
	resDrop, err := repo.GetDesignerAnalytics(ctx, marketing.DesignerAnalyticsRequest{
		From:       from,
		To:         to,
		Sort:       &sortDrop,
		CategoryID: &catIDStr,
	})
	require.NoError(t, err)
	require.Len(t, resDrop.Designers, 3)
	assert.Equal(t, "Designer Beta", resDrop.Designers[0].DesignerName, "Beta (-50% drop) must be first in decline sort")
	require.NotNil(t, resDrop.Designers[0].RevenueChangePct)
	assert.InDelta(t, -50.0, *resDrop.Designers[0].RevenueChangePct, 0.01)

	// Tiebreak between Gamma and Alpha in drop sort (revenue_cents DESC):
	assert.Equal(t, "Designer Alpha", resDrop.Designers[1].DesignerName)
	assert.Equal(t, "Designer Gamma", resDrop.Designers[2].DesignerName)
}
