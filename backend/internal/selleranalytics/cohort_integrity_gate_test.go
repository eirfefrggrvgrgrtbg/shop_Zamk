package selleranalytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func TestSellerAnalytics_CohortIntegrityGate(t *testing.T) {
	dbURL := testutil.RunnerDatabaseURL()
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration tests")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer db.Close()

	verifyTestDBSafety(t, db)

	repo := NewRepository(db)
	svc := NewService(repo)
	router := setupCohortTestRouter(svc, repo)

	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, loc).UTC()
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, loc).UTC()
	asOf := time.Date(2026, 10, 17, 12, 0, 0, 0, loc).UTC()

	// Scenario A: Valid single return against a single-unit order item
	t.Run("Scenario A - Valid single return against a single-unit order item", func(t *testing.T) {
		sellerA, userA := insertSellerUser(t, db)
		pA, vA := insertProductWithVariant(t, db, sellerA, "Prod A", "SKU-A", "M", "Black", 100000, 10)
		oA, oiA, cA := insertDeliveredOrderWithItem(t, db, sellerA, pA, vA, 100000, 1, time.Date(2026, 9, 5, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oA, oiA, cA, "completed", 1, time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC())

		req := httptest.NewRequest("GET", "/products/"+pA.String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+asOf.Format(time.RFC3339), nil)
		req.Header.Set("X-Test-User-ID", userA.String())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var detail ProductDetailResponse
		err := json.Unmarshal(rec.Body.Bytes(), &detail)
		require.NoError(t, err)

		assert.Equal(t, 1, detail.CohortDeliveredUnits)
		assert.Equal(t, 1, detail.CohortReturnedUnits)
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.InDelta(t, 100.0, *detail.ReturnRate.CurrentPercent, 0.001)
	})

	// Scenario B: Two valid partial returns against a multi-unit order item
	t.Run("Scenario B - Two valid partial returns against a multi-unit order item", func(t *testing.T) {
		sellerB, userB := insertSellerUser(t, db)
		pB, vB := insertProductWithVariant(t, db, sellerB, "Prod B", "SKU-B", "L", "White", 200000, 10)
		oB, oiB, cB := insertDeliveredOrderWithItem(t, db, sellerB, pB, vB, 200000, 5, time.Date(2026, 9, 5, 12, 0, 0, 0, loc).UTC())
		// Two partial returns: 2 units, then 3 units = 5 units total
		insertReturnWithItem(t, db, oB, oiB, cB, "completed", 2, time.Date(2026, 9, 12, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oB, oiB, cB, "completed", 3, time.Date(2026, 9, 18, 12, 0, 0, 0, loc).UTC())

		req := httptest.NewRequest("GET", "/products/"+pB.String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+asOf.Format(time.RFC3339), nil)
		req.Header.Set("X-Test-User-ID", userB.String())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var detail ProductDetailResponse
		err := json.Unmarshal(rec.Body.Bytes(), &detail)
		require.NoError(t, err)

		assert.Equal(t, 5, detail.CohortDeliveredUnits)
		assert.Equal(t, 5, detail.CohortReturnedUnits)
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.InDelta(t, 100.0, *detail.ReturnRate.CurrentPercent, 0.001)
	})

	// Scenario C: Over-return on one order item masked by other delivered items
	t.Run("Scenario C - Over-return on one order item masked by other delivered items", func(t *testing.T) {
		sellerC, userC := insertSellerUser(t, db)
		pC, vC := insertProductWithVariant(t, db, sellerC, "Prod C", "SKU-C", "XL", "Navy", 100000, 100)

		// Item 1: delivered qty = 1
		oC1, oiC1, cC1 := insertDeliveredOrderWithItem(t, db, sellerC, pC, vC, 100000, 1, time.Date(2026, 9, 5, 12, 0, 0, 0, loc).UTC())
		// Item 2: delivered qty = 99
		_, _, _ = insertDeliveredOrderWithItem(t, db, sellerC, pC, vC, 100000, 99, time.Date(2026, 9, 6, 12, 0, 0, 0, loc).UTC())

		// Total delivered for product = 100
		// Corrupted/anomalous return record: Item 1 has return qty = 2 (exceeding Item 1 delivered qty 1)
		// Total returned = 2 <= 100, which aggregate-only check would miss
		insertReturnWithItem(t, db, oC1, oiC1, cC1, "completed", 2, time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC())

		// HTTP GET /products -> must return 422 Unprocessable Entity
		reqList := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+asOf.Format(time.RFC3339), nil)
		reqList.Header.Set("X-Test-User-ID", userC.String())
		recList := httptest.NewRecorder()
		router.ServeHTTP(recList, reqList)

		assert.Equal(t, http.StatusUnprocessableEntity, recList.Code, "Over-return on single item must fail safely with 422 Unprocessable Entity")

		// HTTP GET /products/{id} -> must also return 422
		reqDetail := httptest.NewRequest("GET", "/products/"+pC.String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+asOf.Format(time.RFC3339), nil)
		reqDetail.Header.Set("X-Test-User-ID", userC.String())
		recDetail := httptest.NewRecorder()
		router.ServeHTTP(recDetail, reqDetail)

		assert.Equal(t, http.StatusUnprocessableEntity, recDetail.Code, "Product detail must fail safely with 422 Unprocessable Entity on item-level anomaly")
	})

	// Scenario D: Cohort rate and calendar returned units remain independently correct
	t.Run("Scenario D - Cohort rate and calendar returned units remain independently correct", func(t *testing.T) {
		sellerD, userD := insertSellerUser(t, db)
		pD, vD := insertProductWithVariant(t, db, sellerD, "Prod D", "SKU-D", "M", "Red", 100000, 10)

		// Delivered in August, returned in September
		oD1, oiD1, cD1 := insertDeliveredOrderWithItem(t, db, sellerD, pD, vD, 100000, 1, time.Date(2026, 8, 15, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oD1, oiD1, cD1, "completed", 1, time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC())

		// Delivered in September, returned in October
		oD2, oiD2, cD2 := insertDeliveredOrderWithItem(t, db, sellerD, pD, vD, 100000, 1, time.Date(2026, 9, 20, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oD2, oiD2, cD2, "completed", 1, time.Date(2026, 10, 5, 12, 0, 0, 0, loc).UTC())

		req := httptest.NewRequest("GET", "/products/"+pD.String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+asOf.Format(time.RFC3339), nil)
		req.Header.Set("X-Test-User-ID", userD.String())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var detail ProductDetailResponse
		err := json.Unmarshal(rec.Body.Bytes(), &detail)
		require.NoError(t, err)

		// September cohort deliveries = 1 (from Order 2). Returned as of Oct 17 = 1. Cohort rate = 100%
		assert.Equal(t, 1, detail.CohortDeliveredUnits)
		assert.Equal(t, 1, detail.CohortReturnedUnits)
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.InDelta(t, 100.0, *detail.ReturnRate.CurrentPercent, 0.001)

		// September calendar returns = 1 (from Order 1 returned on Sept 15)
		assert.Equal(t, 1, detail.ReturnedUnits.Current)
	})

	// Scenario E: Multiple equal-rate products across paginated results
	t.Run("Scenario E - Multiple equal-rate products across paginated results", func(t *testing.T) {
		sellerE, userE := insertSellerUser(t, db)

		// Create 3 products with identical rate (50%) and identical delivered units (10)
		pE1, vE1 := insertProductWithVariant(t, db, sellerE, "Prod E1", "SKU-E1", "M", "Red", 100000, 10)
		oE1, oiE1, cE1 := insertDeliveredOrderWithItem(t, db, sellerE, pE1, vE1, 100000, 10, time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oE1, oiE1, cE1, "completed", 5, time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC())

		pE2, vE2 := insertProductWithVariant(t, db, sellerE, "Prod E2", "SKU-E2", "L", "Blue", 100000, 10)
		oE2, oiE2, cE2 := insertDeliveredOrderWithItem(t, db, sellerE, pE2, vE2, 100000, 10, time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oE2, oiE2, cE2, "completed", 5, time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC())

		pE3, vE3 := insertProductWithVariant(t, db, sellerE, "Prod E3", "SKU-E3", "S", "Green", 100000, 10)
		oE3, oiE3, cE3 := insertDeliveredOrderWithItem(t, db, sellerE, pE3, vE3, 100000, 10, time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oE3, oiE3, cE3, "completed", 5, time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC())

		// Fetch page 1 (limit 1, offset 0)
		req1 := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&sort=returns&order=desc&limit=1&offset=0&asOf="+asOf.Format(time.RFC3339), nil)
		req1.Header.Set("X-Test-User-ID", userE.String())
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)
		var resp1 ProductsResponse
		require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &resp1))
		require.Len(t, resp1.Items, 1)

		// Fetch page 2 (limit 1, offset 1)
		req2 := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&sort=returns&order=desc&limit=1&offset=1&asOf="+asOf.Format(time.RFC3339), nil)
		req2.Header.Set("X-Test-User-ID", userE.String())
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)
		var resp2 ProductsResponse
		require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp2))
		require.Len(t, resp2.Items, 1)

		// Fetch page 3 (limit 1, offset 2)
		req3 := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&sort=returns&order=desc&limit=1&offset=2&asOf="+asOf.Format(time.RFC3339), nil)
		req3.Header.Set("X-Test-User-ID", userE.String())
		rec3 := httptest.NewRecorder()
		router.ServeHTTP(rec3, req3)
		var resp3 ProductsResponse
		require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &resp3))
		require.Len(t, resp3.Items, 1)

		seen := make(map[string]bool)
		seen[resp1.Items[0].ProductID] = true
		seen[resp2.Items[0].ProductID] = true
		seen[resp3.Items[0].ProductID] = true

		assert.Len(t, seen, 3, "Pagination across equal-rate products must not duplicate or skip items")
		assert.True(t, seen[pE1.String()])
		assert.True(t, seen[pE2.String()])
		assert.True(t, seen[pE3.String()])
	})

	// Scenario F: Ascending and descending sorting with null rates
	t.Run("Scenario F - Ascending and descending sorting with null rates", func(t *testing.T) {
		sellerF, userF := insertSellerUser(t, db)

		// Product F1: 10 sold, 5 returned -> 50%
		pF1, vF1 := insertProductWithVariant(t, db, sellerF, "Prod F1", "SKU-F1", "M", "Red", 100000, 10)
		oF1, oiF1, cF1 := insertDeliveredOrderWithItem(t, db, sellerF, pF1, vF1, 100000, 10, time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oF1, oiF1, cF1, "completed", 5, time.Date(2026, 9, 20, 12, 0, 0, 0, loc).UTC())

		// Product F2: 10 sold, 0 returned -> 0%
		pF2, vF2 := insertProductWithVariant(t, db, sellerF, "Prod F2", "SKU-F2", "L", "Blue", 100000, 10)
		insertDeliveredOrderWithItem(t, db, sellerF, pF2, vF2, 100000, 10, time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC())

		// Product F3: Delivered in August, returned in September -> 0 cohort deliveries, return rate is nil (unavailable)
		pF3, vF3 := insertProductWithVariant(t, db, sellerF, "Prod F3", "SKU-F3", "S", "Green", 100000, 10)
		oF3, oiF3, cF3 := insertDeliveredOrderWithItem(t, db, sellerF, pF3, vF3, 100000, 2, time.Date(2026, 8, 15, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oF3, oiF3, cF3, "completed", 1, time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC())

		// Desc: 50% (F1) -> 0% (F2) -> nil (F3 last)
		reqDesc := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&sort=returns&order=desc&asOf="+asOf.Format(time.RFC3339), nil)
		reqDesc.Header.Set("X-Test-User-ID", userF.String())
		recDesc := httptest.NewRecorder()
		router.ServeHTTP(recDesc, reqDesc)
		var respDesc ProductsResponse
		require.NoError(t, json.Unmarshal(recDesc.Body.Bytes(), &respDesc))
		require.Len(t, respDesc.Items, 3)
		assert.Equal(t, pF1.String(), respDesc.Items[0].ProductID)
		assert.Equal(t, pF2.String(), respDesc.Items[1].ProductID)
		assert.Equal(t, pF3.String(), respDesc.Items[2].ProductID)
		assert.Nil(t, respDesc.Items[2].ReturnRatePercent, "Unavailable rate must be last in desc")

		// Asc: 0% (F2) -> 50% (F1) -> nil (F3 last)
		reqAsc := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&sort=returns&order=asc&asOf="+asOf.Format(time.RFC3339), nil)
		reqAsc.Header.Set("X-Test-User-ID", userF.String())
		recAsc := httptest.NewRecorder()
		router.ServeHTTP(recAsc, reqAsc)
		var respAsc ProductsResponse
		require.NoError(t, json.Unmarshal(recAsc.Body.Bytes(), &respAsc))
		require.Len(t, respAsc.Items, 3)
		assert.Equal(t, pF2.String(), respAsc.Items[0].ProductID)
		assert.Equal(t, pF1.String(), respAsc.Items[1].ProductID)
		assert.Equal(t, pF3.String(), respAsc.Items[2].ProductID)
		assert.Nil(t, respAsc.Items[2].ReturnRatePercent, "Unavailable rate must be last in asc too")
	})

	// Scenario G: Malformed asOf
	t.Run("Scenario G - Malformed asOf", func(t *testing.T) {
		_, userG := insertSellerUser(t, db)

		reqList := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf=invalid-date", nil)
		reqList.Header.Set("X-Test-User-ID", userG.String())
		recList := httptest.NewRecorder()
		router.ServeHTTP(recList, reqList)
		assert.Equal(t, http.StatusBadRequest, recList.Code, "Malformed asOf must return 400 Bad Request on products list")

		reqDetail := httptest.NewRequest("GET", "/products/"+uuid.New().String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf=invalid-date", nil)
		reqDetail.Header.Set("X-Test-User-ID", userG.String())
		recDetail := httptest.NewRecorder()
		router.ServeHTTP(recDetail, reqDetail)
		assert.Equal(t, http.StatusBadRequest, recDetail.Code, "Malformed asOf must return 400 Bad Request on product detail")
	})

	// Scenario H: Future asOf
	t.Run("Scenario H - Future asOf", func(t *testing.T) {
		_, userH := insertSellerUser(t, db)
		futureAsOf := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)

		reqList := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+futureAsOf, nil)
		reqList.Header.Set("X-Test-User-ID", userH.String())
		recList := httptest.NewRecorder()
		router.ServeHTTP(recList, reqList)
		assert.Equal(t, http.StatusBadRequest, recList.Code, "Future asOf must return 400 Bad Request on products list")

		reqDetail := httptest.NewRequest("GET", "/products/"+uuid.New().String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+futureAsOf, nil)
		reqDetail.Header.Set("X-Test-User-ID", userH.String())
		recDetail := httptest.NewRecorder()
		router.ServeHTTP(recDetail, reqDetail)
		assert.Equal(t, http.StatusBadRequest, recDetail.Code, "Future asOf must return 400 Bad Request on product detail")
	})

	// Scenario I: Consistent snapshot used for current and previous periods
	t.Run("Scenario I - Consistent snapshot used for current and previous periods", func(t *testing.T) {
		sellerI, userI := insertSellerUser(t, db)
		pI, vI := insertProductWithVariant(t, db, sellerI, "Prod I", "SKU-I", "M", "Red", 100000, 10)

		// Delivered in current period (Sept 10), returned on Oct 10
		oCurr, oiCurr, cCurr := insertDeliveredOrderWithItem(t, db, sellerI, pI, vI, 100000, 1, time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oCurr, oiCurr, cCurr, "completed", 1, time.Date(2026, 10, 10, 12, 0, 0, 0, loc).UTC())

		// Delivered in previous period (Aug 10), returned on Oct 10
		oPrev, oiPrev, cPrev := insertDeliveredOrderWithItem(t, db, sellerI, pI, vI, 100000, 1, time.Date(2026, 8, 10, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oPrev, oiPrev, cPrev, "completed", 1, time.Date(2026, 10, 10, 12, 0, 0, 0, loc).UTC())

		// Query with snapshot before return (asOf = Oct 5)
		asOfBefore := time.Date(2026, 10, 5, 12, 0, 0, 0, loc).UTC()
		reqBefore := httptest.NewRequest("GET", "/products/"+pI.String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+asOfBefore.Format(time.RFC3339), nil)
		reqBefore.Header.Set("X-Test-User-ID", userI.String())
		recBefore := httptest.NewRecorder()
		router.ServeHTTP(recBefore, reqBefore)
		var detailBefore ProductDetailResponse
		require.NoError(t, json.Unmarshal(recBefore.Body.Bytes(), &detailBefore))
		require.NotNil(t, detailBefore.ReturnRate.CurrentPercent)
		assert.Equal(t, 0.0, *detailBefore.ReturnRate.CurrentPercent, "As of Oct 5, return on Oct 10 must not be included")
		if detailBefore.ReturnRate.PreviousPercent != nil {
			assert.Equal(t, 0.0, *detailBefore.ReturnRate.PreviousPercent, "As of Oct 5, previous period return on Oct 10 must not be included")
		}

		// Query with snapshot after return (asOf = Oct 15)
		asOfAfter := time.Date(2026, 10, 15, 12, 0, 0, 0, loc).UTC()
		reqAfter := httptest.NewRequest("GET", "/products/"+pI.String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+asOfAfter.Format(time.RFC3339), nil)
		reqAfter.Header.Set("X-Test-User-ID", userI.String())
		recAfter := httptest.NewRecorder()
		router.ServeHTTP(recAfter, reqAfter)
		var detailAfter ProductDetailResponse
		require.NoError(t, json.Unmarshal(recAfter.Body.Bytes(), &detailAfter))
		require.NotNil(t, detailAfter.ReturnRate.CurrentPercent)
		assert.Equal(t, 100.0, *detailAfter.ReturnRate.CurrentPercent, "As of Oct 15, return on Oct 10 must be included")
		if detailAfter.ReturnRate.PreviousPercent != nil {
			assert.Equal(t, 100.0, *detailAfter.ReturnRate.PreviousPercent, "As of Oct 15, previous period return on Oct 10 must be included")
		}
	})

	// Scenario J: Cross-seller isolation and existing product authorization
	t.Run("Scenario J - Cross-seller isolation and existing product authorization", func(t *testing.T) {
		sellerJ1, userJ1 := insertSellerUser(t, db)
		sellerJ2, userJ2 := insertSellerUser(t, db)

		pJ1, vJ1 := insertProductWithVariant(t, db, sellerJ1, "Prod J1", "SKU-J1", "M", "Red", 100000, 10)
		insertDeliveredOrderWithItem(t, db, sellerJ1, pJ1, vJ1, 100000, 2, time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC())

		pJ2, _ := insertProductWithVariant(t, db, sellerJ2, "Prod J2", "SKU-J2", "L", "Blue", 100000, 10)

		// User J1 requests Product J2 -> 404 Not Found
		reqDetail := httptest.NewRequest("GET", "/products/"+pJ2.String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339), nil)
		reqDetail.Header.Set("X-Test-User-ID", userJ1.String())
		recDetail := httptest.NewRecorder()
		router.ServeHTTP(recDetail, reqDetail)
		assert.Equal(t, http.StatusNotFound, recDetail.Code, "Access to other seller's product must return 404")

		// User J1 lists products -> only P J1, never P J2
		reqList := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339), nil)
		reqList.Header.Set("X-Test-User-ID", userJ1.String())
		recList := httptest.NewRecorder()
		router.ServeHTTP(recList, reqList)
		var respList ProductsResponse
		require.NoError(t, json.Unmarshal(recList.Body.Bytes(), &respList))
		for _, item := range respList.Items {
			assert.NotEqual(t, pJ2.String(), item.ProductID, "Seller J1 must never see Seller J2 products")
		}

		_ = userJ2
	})
}

func TestSellerAnalytics_ReturnOnlyProducts(t *testing.T) {
	dbURL := testutil.RunnerDatabaseURL()
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration tests")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer db.Close()

	verifyTestDBSafety(t, db)

	repo := NewRepository(db)
	svc := NewService(repo)
	router := setupCohortTestRouter(svc, repo)

	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, loc).UTC()
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, loc).UTC()
	asOf := time.Date(2026, 10, 17, 12, 0, 0, 0, loc).UTC()

	sellerA, userA := insertSellerUser(t, db)
	sellerB, _ := insertSellerUser(t, db)

	// Product A (Seller A): Sale delivered BEFORE period (Aug 15), return completed INSIDE period (Sept 15)
	// Zero delivered sales inside September period
	pA, vA := insertProductWithVariant(t, db, sellerA, "Return-Only Product A", "SKU-RO-A", "M", "Black", 150000, 10)
	oA, oiA, cA := insertDeliveredOrderWithItem(t, db, sellerA, pA, vA, 150000, 1, time.Date(2026, 8, 15, 12, 0, 0, 0, loc).UTC())
	insertReturnWithItem(t, db, oA, oiA, cA, "completed", 1, time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC())

	// Product B (Seller A): Ordinary sales INSIDE period (Sept 10), 0 returns
	pB, vB := insertProductWithVariant(t, db, sellerA, "Active Product B", "SKU-NORM-B", "L", "White", 250000, 20)
	insertDeliveredOrderWithItem(t, db, sellerA, pB, vB, 250000, 5, time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC())

	// Product C (Seller A): Zero sales, zero returns in period (inactive/no-movement)
	pC, _ := insertProductWithVariant(t, db, sellerA, "Inactive Product C", "SKU-ZERO-C", "S", "Grey", 100000, 5)

	// Product D (Seller B): Return-only product belonging to foreign Seller B
	pD, vD := insertProductWithVariant(t, db, sellerB, "Foreign Product D", "SKU-FOREIGN-D", "XL", "Navy", 300000, 10)
	oD, oiD, cD := insertDeliveredOrderWithItem(t, db, sellerB, pD, vD, 300000, 1, time.Date(2026, 8, 20, 12, 0, 0, 0, loc).UTC())
	insertReturnWithItem(t, db, oD, oiD, cD, "completed", 1, time.Date(2026, 9, 20, 12, 0, 0, 0, loc).UTC())

	// 1. Seller A lists products for September
	reqList := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+asOf.Format(time.RFC3339), nil)
	reqList.Header.Set("X-Test-User-ID", userA.String())
	recList := httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)

	require.Equal(t, http.StatusOK, recList.Code)
	var respList ProductsResponse
	err = json.Unmarshal(recList.Body.Bytes(), &respList)
	require.NoError(t, err)

	// Inactive Product C (0 sales, 0 returns) must NOT be present
	// Foreign Product D must NOT be present
	// Exactly Product A and Product B should be present!
	require.Len(t, respList.Items, 2, "Products list must contain exactly active and return-only products, without zero-movement products")

	var rowA, rowB *ProductRow
	for i := range respList.Items {
		item := &respList.Items[i]
		if item.ProductID == pA.String() {
			rowA = item
		} else if item.ProductID == pB.String() {
			rowB = item
		}
		assert.NotEqual(t, pC.String(), item.ProductID, "Inactive Product C must not flood the products list")
		assert.NotEqual(t, pD.String(), item.ProductID, "Foreign Product D must not be visible to Seller A")
	}

	require.NotNil(t, rowA, "Product A with calendar returns must be present in Products table")
	assert.Equal(t, int64(0), rowA.GrossSalesCents, "Return-only product gross sales must be 0")
	assert.Equal(t, 0, rowA.UnitsSold, "Return-only product delivered units sold must be 0")
	assert.Equal(t, 0, rowA.OrdersCount, "Return-only product delivered orders count must be 0")
	assert.Equal(t, 1, rowA.ReturnedUnits, "Return-only product calendar returned units must be 1")
	assert.Nil(t, rowA.ReturnRatePercent, "Return-only product cohort return rate must be nil (unavailable)")
	assert.Equal(t, 0, rowA.CohortDeliveredUnits)
	assert.Equal(t, 0, rowA.CohortReturnedUnits)

	require.NotNil(t, rowB, "Normal product B must be present")
	assert.Equal(t, int64(1250000), rowB.GrossSalesCents)
	assert.Equal(t, 5, rowB.UnitsSold)
	assert.Equal(t, 1, rowB.OrdersCount)
	assert.Equal(t, 0, rowB.ReturnedUnits)
	require.NotNil(t, rowB.ReturnRatePercent)
	assert.Equal(t, 0.0, *rowB.ReturnRatePercent)

	// 2. Opening Product Detail for Return-Only Product A
	reqDetailA := httptest.NewRequest("GET", "/products/"+pA.String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+asOf.Format(time.RFC3339), nil)
	reqDetailA.Header.Set("X-Test-User-ID", userA.String())
	recDetailA := httptest.NewRecorder()
	router.ServeHTTP(recDetailA, reqDetailA)

	require.Equal(t, http.StatusOK, recDetailA.Code, "Opening Product Detail for return-only product must return 200 OK")
	var detailA ProductDetailResponse
	require.NoError(t, json.Unmarshal(recDetailA.Body.Bytes(), &detailA))

	assert.Equal(t, pA.String(), detailA.ProductID)
	assert.Equal(t, "Return-Only Product A", detailA.Title)
	assert.Equal(t, int64(0), detailA.GrossSales.CurrentCents)
	assert.Equal(t, 0, detailA.UnitsSold.Current)
	assert.Equal(t, 1, detailA.ReturnedUnits.Current, "Product detail must display calendar return quantity = 1")
	assert.Nil(t, detailA.ReturnRate.CurrentPercent, "Product detail cohort return rate must be nil (unavailable)")

	// 3. Opening Product Detail for Foreign Product D as Seller A -> 404
	reqDetailD := httptest.NewRequest("GET", "/products/"+pD.String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+asOf.Format(time.RFC3339), nil)
	reqDetailD.Header.Set("X-Test-User-ID", userA.String())
	recDetailD := httptest.NewRecorder()
	router.ServeHTTP(recDetailD, reqDetailD)

	assert.Equal(t, http.StatusNotFound, recDetailD.Code, "Access to foreign product D must return 404 Not Found")
}

func TestSellerAnalytics_SKUVelocityPrecisionContract(t *testing.T) {
	dbURL := testutil.RunnerDatabaseURL()
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration tests")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer db.Close()

	verifyTestDBSafety(t, db)

	repo := NewRepository(db)
	svc := NewService(repo)
	router := setupCohortTestRouter(svc, repo)

	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	// Exactly 30 days reporting window (720 hours)
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, loc).UTC()
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, loc).UTC()
	asOf := time.Date(2026, 10, 17, 12, 0, 0, 0, loc).UTC()

	sellerID, userID := insertSellerUser(t, db)

	// Product with 3 variants:
	// Variant 1: 1 delivered unit in 30 days, availableStock = 21 -> velocity = 1/30 ≈ 0.03333333333333333
	// Variant 2: 0 delivered units in 30 days, availableStock = 5 -> velocity = 0, coverage = no_sales
	// Variant 3: 0 delivered units in 30 days, availableStock = 0 -> velocity = 0, coverage = out_of_stock
	pID, vID1 := insertProductWithVariant(t, db, sellerID, "Velocity Test Product", "SKU-VEL-1", "M", "Black", 122200, 21)

	vID2 := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, price_cents, is_active)
		VALUES ($1, $2, $3, $3, $4, $5, $6, true)`,
		vID2, pID, "SKU-VEL-2", "L", "White", 122200)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, $5, 0)`,
		uuid.New(), pID, vID2, sellerID, 5)
	require.NoError(t, err)

	vID3 := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, price_cents, is_active)
		VALUES ($1, $2, $3, $3, $4, $5, $6, true)`,
		vID3, pID, "SKU-VEL-3", "XL", "Navy", 122200)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
		VALUES ($1, $2, $3, $4, $5, 0)`,
		uuid.New(), pID, vID3, sellerID, 0)
	require.NoError(t, err)

	// Insert exactly 1 delivered order item for Variant 1 within the 30-day window
	insertDeliveredOrderWithItem(t, db, sellerID, pID, vID1, 122200, 1, time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC())

	req := httptest.NewRequest("GET", "/products/"+pID.String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&asOf="+asOf.Format(time.RFC3339), nil)
	req.Header.Set("X-Test-User-ID", userID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	rawJSON := rec.Body.String()
	// Assert raw JSON serialization transmits unrounded float64 precision and does not clamp/round to 0 or 0.0
	assert.Contains(t, rawJSON, `"salesVelocity":0.03333333333333333`, "Raw JSON must preserve full float64 precision for 1 unit over 30 days")
	assert.NotContains(t, rawJSON, `"salesVelocity":0.0,`, "Raw JSON must not prematurely round positive velocity to 0.0")

	var detail ProductDetailResponse
	err = json.Unmarshal(rec.Body.Bytes(), &detail)
	require.NoError(t, err)
	require.Len(t, detail.Variants, 3)

	var var1, var2, var3 *VariantRow
	for i := range detail.Variants {
		v := &detail.Variants[i]
		if v.VariantID == vID1.String() {
			var1 = v
		} else if v.VariantID == vID2.String() {
			var2 = v
		} else if v.VariantID == vID3.String() {
			var3 = v
		}
	}

	require.NotNil(t, var1, "Variant 1 must be present")
	assert.Equal(t, 1, var1.UnitsSold)
	assert.Equal(t, 21, var1.AvailableStock)
	// Mathematical verification: 1 unit / 30 days = 0.03333333333333333
	expectedVelocity := 1.0 / 30.0
	assert.InDelta(t, expectedVelocity, var1.SalesVelocity, 0.0000001, "SalesVelocity in DTO must match 1/30")
	assert.Greater(t, var1.SalesVelocity, 0.03, "SalesVelocity must be strictly > 0.03")
	assert.Less(t, var1.SalesVelocity, 0.04, "SalesVelocity must be strictly < 0.04")
	require.NotNil(t, var1.DaysOfStock)
	// Days to depletion: 21 / (1/30) = 630.0 days
	assert.InDelta(t, 630.0, *var1.DaysOfStock, 0.001)
	assert.Equal(t, "", var1.StockCoverageState)

	require.NotNil(t, var2, "Variant 2 must be present")
	assert.Equal(t, 0, var2.UnitsSold)
	assert.Equal(t, 5, var2.AvailableStock)
	assert.Equal(t, 0.0, var2.SalesVelocity)
	assert.Nil(t, var2.DaysOfStock)
	assert.Equal(t, "no_sales", var2.StockCoverageState)

	require.NotNil(t, var3, "Variant 3 must be present")
	assert.Equal(t, 0, var3.UnitsSold)
	assert.Equal(t, 0, var3.AvailableStock)
	assert.Equal(t, 0.0, var3.SalesVelocity)
	require.NotNil(t, var3.DaysOfStock)
	assert.Equal(t, 0.0, *var3.DaysOfStock)
	assert.Equal(t, "out_of_stock", var3.StockCoverageState)
}


