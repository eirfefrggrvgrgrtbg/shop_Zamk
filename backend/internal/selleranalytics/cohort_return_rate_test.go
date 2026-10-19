package selleranalytics

import (
	"context"
	"encoding/json"
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
)

func verifyTestDBSafety(t *testing.T, db *pgxpool.Pool) {
	var currentDB string
	err := db.QueryRow(context.Background(), "SELECT current_database()").Scan(&currentDB)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", currentDB, "CRITICAL: Tests MUST only run against isolated 'zamk_test'")
}

func setupCohortTestRouter(svc *Service, repo *Repository) *chi.Mux {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			userIDStr := req.Header.Get("X-Test-User-ID")
			if userIDStr != "" {
				if uid, err := uuid.Parse(userIDStr); err == nil {
					ctx := context.WithValue(req.Context(), "userID", uid)
					next.ServeHTTP(w, req.WithContext(ctx))
					return
				}
			}
			next.ServeHTTP(w, req)
		})
	})
	h := NewHandler(svc, repo)
	h.RegisterRoutes(r)
	return r
}

func insertSellerUser(t *testing.T, db *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	userID := uuid.New()
	sellerID := uuid.New()
	_, err := db.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role) VALUES ($1, $2, $3, 'hash', 'seller')`,
		userID, "Cohort Seller User", sellerID.String()+"@test.com")
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, status) VALUES ($1, 'Cohort Seller', $2, 'active')`,
		sellerID, "test-cohort-"+sellerID.String())
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')`,
		uuid.New(), sellerID, userID)
	require.NoError(t, err)

	return sellerID, userID
}

func insertProductWithVariant(t *testing.T, db *pgxpool.Pool, sellerID uuid.UUID, title, sku, size, color string, priceCents int64, stock int) (uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	var catID uuid.UUID
	err := db.QueryRow(ctx, "SELECT id FROM categories LIMIT 1").Scan(&catID)
	if err != nil {
		catID = uuid.New()
		_, err = db.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', $2)", catID, catID.String())
		require.NoError(t, err)
	}

	pID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents)
		VALUES ($1, $2, $3, $4, $5, 'published', $6)`,
		pID, sellerID, catID, title, pID.String(), priceCents)
	require.NoError(t, err)

	vID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, price_cents, is_active)
		VALUES ($1, $2, $3, $3, $4, $5, $6, true)`,
		vID, pID, sku, size, color, priceCents)
	require.NoError(t, err)

	if stock > 0 {
		_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
			VALUES ($1, $2, $3, $4, $5, 0)`,
			uuid.New(), pID, vID, sellerID, stock)
		require.NoError(t, err)
	}

	return pID, vID
}

func insertDeliveredOrderWithItem(t *testing.T, db *pgxpool.Pool, sellerID, pID, vID uuid.UUID, priceCents int64, qty int, deliveryTime time.Time) (uuid.UUID, uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	cID := insertCustomer(t, db)
	oID := uuid.New()
	_, err := db.Exec(ctx, `INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'delivered', 'Cust', '123', 'c@z.ru', 'Moscow')`, oID, cID)
	require.NoError(t, err)

	fID := uuid.New()
	subtotal := priceCents * int64(qty)
	comm := subtotal * 8 / 100
	earning := subtotal - comm
	_, err = db.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at)
		VALUES ($1, $2, $3, 'delivered', $4, 800, $5, $6)`, fID, oID, sellerID, subtotal, earning, deliveryTime)
	require.NoError(t, err)

	oiID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, created_at)
		VALUES ($1, $2, $3, $4, $5, 'Prod', 'slug', $6, $7, $8, $9, $10)`,
		oiID, oID, pID, vID, sellerID, priceCents, qty, subtotal, fID, deliveryTime)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO payments (id, order_id, provider, status, amount_cents, idempotency_key, created_at, updated_at, paid_at)
		VALUES ($1, $2, 'tbank', 'succeeded', $3, $4, $5, $5, $5)`,
		uuid.New(), oID, subtotal, uuid.New().String(), deliveryTime)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at)
		VALUES ($1, $2, $3, $4, 'sale_gross', $5, $6)`,
		uuid.New(), sellerID, oID, oiID, subtotal, deliveryTime)
	require.NoError(t, err)

	return oID, oiID, cID
}

func insertReturnWithItem(t *testing.T, db *pgxpool.Pool, oID, oiID, cID uuid.UUID, status string, returnQty int, completedAt time.Time) uuid.UUID {
	ctx := context.Background()
	rID := uuid.New()

	var fID uuid.UUID
	err := db.QueryRow(ctx, "SELECT order_fulfillment_id FROM order_items WHERE id = $1", oiID).Scan(&fID)
	require.NoError(t, err)

	var compPtr *time.Time
	if !completedAt.IsZero() {
		compPtr = &completedAt
	}

	_, err = db.Exec(ctx, `INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, completed_at, created_at)
		VALUES ($1, $2, $3, $4, $5, 'defective', $6, $7)`,
		rID, oID, fID, cID, status, compPtr, completedAt)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO return_items (id, return_id, order_item_id, quantity)
		VALUES ($1, $2, $3, $4)`, uuid.New(), rID, oiID, returnQty)
	require.NoError(t, err)

	return rID
}

func TestSellerAnalytics_CohortReturnRate(t *testing.T) {
	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"
	}
	db, err := pgxpool.New(context.Background(), connStr)
	require.NoError(t, err)
	defer db.Close()

	verifyTestDBSafety(t, db)

	repo := NewRepository(db)
	svc := NewService(repo)
	router := setupCohortTestRouter(svc, repo)
	ctx := context.Background()

	// Canonical MSK base time: September 2026 delivery window
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, loc).UTC()
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, loc).UTC()
	asOf := time.Date(2026, 10, 17, 12, 0, 0, 0, loc).UTC()

	// Scenario A: Delivered 100 units in the cohort, 8 completed returns later -> 8%
	t.Run("Scenario A - Delivered 100 units in cohort, 8 completed returns later -> 8%", func(t *testing.T) {
		sellerA, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerA, "Dress A", "SKU-A", "M", "Black", 100000, 50)
		tSale := time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC()
		tReturn := time.Date(2026, 10, 5, 12, 0, 0, 0, loc).UTC() // returned in October, after September delivery period

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerA, pID, vID, 100000, 100, tSale)
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 8, tReturn)

		detail, err := svc.GetProductDetail(ctx, sellerA, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		assert.Equal(t, 100, detail.UnitsSold.Current)
		assert.Equal(t, 100, detail.CohortDeliveredUnits)
		assert.Equal(t, 8, detail.CohortReturnedUnits)
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.InDelta(t, 8.0, *detail.ReturnRate.CurrentPercent, 0.001)
		// Calendar returns in September [from, to) is 0 because return completed in October
		assert.Equal(t, 0, detail.ReturnedUnits.Current)

		require.Len(t, detail.Variants, 1)
		assert.Equal(t, 100, detail.Variants[0].CohortDeliveredUnits)
		assert.Equal(t, 8, detail.Variants[0].CohortReturnedUnits)
		require.NotNil(t, detail.Variants[0].ReturnRatePercent)
		assert.InDelta(t, 8.0, *detail.Variants[0].ReturnRatePercent, 0.001)
	})

	// Scenario B: Delivered cohort with no completed returns -> 0%
	t.Run("Scenario B - Delivered cohort with no completed returns -> 0%", func(t *testing.T) {
		sellerB, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerB, "Shirt B", "SKU-B", "L", "White", 50000, 30)
		tSale := time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC()

		insertDeliveredOrderWithItem(t, db, sellerB, pID, vID, 50000, 20, tSale)

		detail, err := svc.GetProductDetail(ctx, sellerB, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		assert.Equal(t, 20, detail.UnitsSold.Current)
		assert.Equal(t, 20, detail.CohortDeliveredUnits)
		assert.Equal(t, 0, detail.CohortReturnedUnits)
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.Equal(t, 0.0, *detail.ReturnRate.CurrentPercent)
		assert.Equal(t, 0, detail.ReturnedUnits.Current)
	})

	// Scenario C: Zero cohort deliveries, no returns -> rate unavailable, not 0%
	t.Run("Scenario C - Zero cohort deliveries, no returns -> rate unavailable (nil)", func(t *testing.T) {
		sellerC, _ := insertSellerUser(t, db)
		pID, _ := insertProductWithVariant(t, db, sellerC, "Coat C", "SKU-C", "XL", "Beige", 300000, 10)

		detail, err := svc.GetProductDetail(ctx, sellerC, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		assert.Equal(t, 0, detail.UnitsSold.Current)
		assert.Equal(t, 0, detail.CohortDeliveredUnits)
		assert.Equal(t, 0, detail.CohortReturnedUnits)
		assert.Nil(t, detail.ReturnRate.CurrentPercent, "Zero deliveries must produce nil return rate, not 0%")
		assert.Equal(t, 0, detail.ReturnedUnits.Current)
	})

	// Scenario D: Zero cohort deliveries, calendar returns from older sales -> unavailable cohort rate AND positive calendar returned units
	t.Run("Scenario D - Zero cohort deliveries, calendar returns from older sales", func(t *testing.T) {
		sellerD, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerD, "Jacket D", "SKU-D", "S", "Grey", 150000, 5)
		tOlderSale := time.Date(2026, 8, 15, 12, 0, 0, 0, loc).UTC() // delivered in August
		tReturnInSept := time.Date(2026, 9, 12, 12, 0, 0, 0, loc).UTC() // returned in September

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerD, pID, vID, 150000, 3, tOlderSale)
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 3, tReturnInSept)

		detail, err := svc.GetProductDetail(ctx, sellerD, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		// In September: 0 units delivered
		assert.Equal(t, 0, detail.UnitsSold.Current)
		assert.Equal(t, 0, detail.CohortDeliveredUnits)
		assert.Equal(t, 0, detail.CohortReturnedUnits)
		assert.Nil(t, detail.ReturnRate.CurrentPercent, "Cohort return rate must be nil when 0 units delivered in cohort")
		// But calendar returned units in September is 3!
		assert.Equal(t, 3, detail.ReturnedUnits.Current)
	})

	// Scenario E: September sale, October completed return -> counted for September cohort, calendar count belongs to October
	t.Run("Scenario E - September sale, October return -> counted for Sept cohort, calendar belongs to Oct", func(t *testing.T) {
		sellerE, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerE, "Skirt E", "SKU-E", "M", "Navy", 80000, 15)
		tSaleSept := time.Date(2026, 9, 20, 10, 0, 0, 0, loc).UTC()
		tReturnOct := time.Date(2026, 10, 3, 15, 0, 0, 0, loc).UTC()

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerE, pID, vID, 80000, 5, tSaleSept)
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 2, tReturnOct)

		// View September:
		septDetail, err := svc.GetProductDetail(ctx, sellerE, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)
		assert.Equal(t, 5, septDetail.CohortDeliveredUnits)
		assert.Equal(t, 2, septDetail.CohortReturnedUnits)
		require.NotNil(t, septDetail.ReturnRate.CurrentPercent)
		assert.InDelta(t, 40.0, *septDetail.ReturnRate.CurrentPercent, 0.001)
		assert.Equal(t, 0, septDetail.ReturnedUnits.Current, "Sept calendar returns is 0")

		// View October:
		fromOct := time.Date(2026, 10, 1, 0, 0, 0, 0, loc).UTC()
		toOct := time.Date(2026, 11, 1, 0, 0, 0, 0, loc).UTC()
		octDetail, err := svc.GetProductDetail(ctx, sellerE, pID, fromOct, toOct, "Europe/Moscow", asOf)
		require.NoError(t, err)
		assert.Equal(t, 0, octDetail.CohortDeliveredUnits, "No sales in October")
		assert.Equal(t, 0, octDetail.CohortReturnedUnits)
		assert.Nil(t, octDetail.ReturnRate.CurrentPercent)
		assert.Equal(t, 2, octDetail.ReturnedUnits.Current, "October calendar returns is 2")
	})

	// Scenario F: Returns from sales BEFORE the selected cohort -> excluded from cohort numerator
	t.Run("Scenario F - Returns from sales BEFORE cohort excluded from cohort numerator", func(t *testing.T) {
		sellerF, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerF, "Pants F", "SKU-F", "32", "Black", 120000, 20)
		tJuly := time.Date(2026, 7, 10, 10, 0, 0, 0, loc).UTC()
		tSept := time.Date(2026, 9, 10, 10, 0, 0, 0, loc).UTC()

		// Older order in July
		oOld, oiOld, cOld := insertDeliveredOrderWithItem(t, db, sellerF, pID, vID, 120000, 10, tJuly)
		// Return completed in September for July sale
		insertReturnWithItem(t, db, oOld, oiOld, cOld, "completed", 5, tSept)

		// New order in September (cohort) with 0 returns
		insertDeliveredOrderWithItem(t, db, sellerF, pID, vID, 120000, 10, tSept)

		detail, err := svc.GetProductDetail(ctx, sellerF, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		assert.Equal(t, 10, detail.CohortDeliveredUnits)
		assert.Equal(t, 0, detail.CohortReturnedUnits, "July order return must NOT enter September cohort numerator")
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.Equal(t, 0.0, *detail.ReturnRate.CurrentPercent)
		assert.Equal(t, 5, detail.ReturnedUnits.Current, "September calendar return includes the 5 units")
	})

	// Scenario G: Multiple partial completed returns for the same order item -> quantities sum exactly once
	t.Run("Scenario G - Multiple partial completed returns for same order item sum once", func(t *testing.T) {
		sellerG, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerG, "Blouse G", "SKU-G", "40", "Red", 90000, 10)
		tSale := time.Date(2026, 9, 5, 12, 0, 0, 0, loc).UTC()

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerG, pID, vID, 90000, 4, tSale)
		// Partial return 1: 1 unit on Sept 20
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 1, time.Date(2026, 9, 20, 12, 0, 0, 0, loc).UTC())
		// Partial return 2: 2 units on Oct 2
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 2, time.Date(2026, 10, 2, 12, 0, 0, 0, loc).UTC())

		detail, err := svc.GetProductDetail(ctx, sellerG, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		assert.Equal(t, 4, detail.CohortDeliveredUnits)
		assert.Equal(t, 3, detail.CohortReturnedUnits, "1 + 2 = 3 returned units")
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.InDelta(t, 75.0, *detail.ReturnRate.CurrentPercent, 0.001)
		// Calendar returns in September was 1 unit
		assert.Equal(t, 1, detail.ReturnedUnits.Current)
	})

	// Scenario H: Pending return request -> excluded from completed physical return counts
	t.Run("Scenario H - Pending return excluded from completed physical returns", func(t *testing.T) {
		sellerH, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerH, "Top H", "SKU-H", "S", "Pink", 70000, 10)
		tSale := time.Date(2026, 9, 8, 12, 0, 0, 0, loc).UTC()

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerH, pID, vID, 70000, 5, tSale)
		insertReturnWithItem(t, db, oID, oiID, cID, "requested", 2, time.Time{})

		detail, err := svc.GetProductDetail(ctx, sellerH, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		assert.Equal(t, 5, detail.CohortDeliveredUnits)
		assert.Equal(t, 0, detail.CohortReturnedUnits)
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.Equal(t, 0.0, *detail.ReturnRate.CurrentPercent)
		assert.Equal(t, 0, detail.ReturnedUnits.Current)
	})

	// Scenario I: Rejected or cancelled return -> excluded
	t.Run("Scenario I - Rejected or cancelled return excluded", func(t *testing.T) {
		sellerI, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerI, "Jeans I", "SKU-I", "30", "Blue", 110000, 10)
		tSale := time.Date(2026, 9, 8, 12, 0, 0, 0, loc).UTC()

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerI, pID, vID, 110000, 6, tSale)
		insertReturnWithItem(t, db, oID, oiID, cID, "rejected", 3, time.Date(2026, 9, 25, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oID, oiID, cID, "cancelled", 1, time.Date(2026, 9, 26, 12, 0, 0, 0, loc).UTC())

		detail, err := svc.GetProductDetail(ctx, sellerI, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		assert.Equal(t, 6, detail.CohortDeliveredUnits)
		assert.Equal(t, 0, detail.CohortReturnedUnits)
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.Equal(t, 0.0, *detail.ReturnRate.CurrentPercent)
		assert.Equal(t, 0, detail.ReturnedUnits.Current)
	})

	// Scenario J: Completed return after asOf -> excluded until later reporting snapshot
	t.Run("Scenario J - Return completed after asOf excluded until later snapshot", func(t *testing.T) {
		sellerJ, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerJ, "Hat J", "SKU-J", "One", "Black", 30000, 10)
		tSale := time.Date(2026, 9, 8, 12, 0, 0, 0, loc).UTC()
		tReturn := time.Date(2026, 10, 20, 12, 0, 0, 0, loc).UTC() // after asOf (Oct 17)

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerJ, pID, vID, 30000, 10, tSale)
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 4, tReturn)

		// asOf snapshot on Oct 17:
		detail17, err := svc.GetProductDetail(ctx, sellerJ, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)
		assert.Equal(t, 10, detail17.CohortDeliveredUnits)
		assert.Equal(t, 0, detail17.CohortReturnedUnits, "Return completed after Oct 17 must be excluded as of Oct 17")
		require.NotNil(t, detail17.ReturnRate.CurrentPercent)
		assert.Equal(t, 0.0, *detail17.ReturnRate.CurrentPercent)

		// Later snapshot on Oct 25:
		asOfLater := time.Date(2026, 10, 25, 12, 0, 0, 0, loc).UTC()
		detail25, err := svc.GetProductDetail(ctx, sellerJ, pID, from, to, "Europe/Moscow", asOfLater)
		require.NoError(t, err)
		assert.Equal(t, 10, detail25.CohortDeliveredUnits)
		assert.Equal(t, 4, detail25.CohortReturnedUnits, "Return included in later snapshot")
		require.NotNil(t, detail25.ReturnRate.CurrentPercent)
		assert.InDelta(t, 40.0, *detail25.ReturnRate.CurrentPercent, 0.001)
	})

	// Scenario K: Return completed before asOf -> included even when after delivery-period end
	t.Run("Scenario K - Return completed before asOf included even after delivery-period end", func(t *testing.T) {
		sellerK, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerK, "Scarf K", "SKU-K", "One", "Red", 40000, 10)
		tSale := time.Date(2026, 9, 28, 12, 0, 0, 0, loc).UTC()
		tReturn := time.Date(2026, 10, 10, 12, 0, 0, 0, loc).UTC() // after Sept 30, before Oct 17

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerK, pID, vID, 40000, 10, tSale)
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 3, tReturn)

		detail, err := svc.GetProductDetail(ctx, sellerK, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)
		assert.Equal(t, 10, detail.CohortDeliveredUnits)
		assert.Equal(t, 3, detail.CohortReturnedUnits)
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.InDelta(t, 30.0, *detail.ReturnRate.CurrentPercent, 0.001)
	})

	// Scenario L: One product with multiple variants -> product rate is quantity-weighted
	t.Run("Scenario L - Product rate is quantity-weighted across variants", func(t *testing.T) {
		sellerL, _ := insertSellerUser(t, db)
		pID, v1 := insertProductWithVariant(t, db, sellerL, "Hoodie L", "SKU-L1", "M", "Black", 200000, 50)
		// Add variant 2 to the same product
		v2 := uuid.New()
		_, err := db.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, size, color, price_cents, is_active)
			VALUES ($1, $2, 'SKU-L2', 'SKU-L2', 'L', 'Black', 200000, true)`, v2, pID)
		require.NoError(t, err)
		_, err = db.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock)
			VALUES ($1, $2, $3, $4, 50, 0)`, uuid.New(), pID, v2, sellerL)
		require.NoError(t, err)

		tSale := time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC()
		// Variant 1: 10 units delivered, 5 returned -> 50%
		o1, oi1, c1 := insertDeliveredOrderWithItem(t, db, sellerL, pID, v1, 200000, 10, tSale)
		insertReturnWithItem(t, db, o1, oi1, c1, "completed", 5, time.Date(2026, 9, 20, 12, 0, 0, 0, loc).UTC())

		// Variant 2: 90 units delivered, 0 returned -> 0%
		insertDeliveredOrderWithItem(t, db, sellerL, pID, v2, 200000, 90, tSale)

		detail, err := svc.GetProductDetail(ctx, sellerL, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		// Unweighted avg would be (50% + 0%) / 2 = 25%
		// Quantity-weighted MUST be (5 / 100) * 100 = 5.0%!
		assert.Equal(t, 100, detail.CohortDeliveredUnits)
		assert.Equal(t, 5, detail.CohortReturnedUnits)
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.InDelta(t, 5.0, *detail.ReturnRate.CurrentPercent, 0.001, "Product cohort rate must be quantity-weighted 5%, not 25%")

		// Check variants
		require.Len(t, detail.Variants, 2)
		var v1Row, v2Row VariantRow
		for _, v := range detail.Variants {
			if v.VariantID == v1.String() {
				v1Row = v
			} else if v.VariantID == v2.String() {
				v2Row = v
			}
		}
		assert.Equal(t, 10, v1Row.CohortDeliveredUnits)
		assert.Equal(t, 5, v1Row.CohortReturnedUnits)
		require.NotNil(t, v1Row.ReturnRatePercent)
		assert.InDelta(t, 50.0, *v1Row.ReturnRatePercent, 0.001)

		assert.Equal(t, 90, v2Row.CohortDeliveredUnits)
		assert.Equal(t, 0, v2Row.CohortReturnedUnits)
		require.NotNil(t, v2Row.ReturnRatePercent)
		assert.Equal(t, 0.0, *v2Row.ReturnRatePercent)
	})

	// Scenario M: Variant without deliveries but with calendar returns from older sales -> unavailable cohort rate, positive calendar returned units
	t.Run("Scenario M - Variant without deliveries but with calendar returns", func(t *testing.T) {
		sellerM, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerM, "Gloves M", "SKU-M", "L", "Leather", 60000, 10)
		tOlder := time.Date(2026, 8, 1, 12, 0, 0, 0, loc).UTC()
		tReturnInSept := time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC()

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerM, pID, vID, 60000, 2, tOlder)
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 2, tReturnInSept)

		detail, err := svc.GetProductDetail(ctx, sellerM, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		require.Len(t, detail.Variants, 1)
		v := detail.Variants[0]
		assert.Equal(t, 0, v.CohortDeliveredUnits)
		assert.Equal(t, 0, v.CohortReturnedUnits)
		assert.Nil(t, v.ReturnRatePercent, "Variant cohort return rate must be nil when 0 units delivered")
		assert.Equal(t, 2, v.ReturnedUnits, "Calendar returned units must be 2")
	})

	// Scenario N: Archived variant -> historical cohort attribution remains correct
	t.Run("Scenario N - Archived variant historical cohort attribution remains correct", func(t *testing.T) {
		sellerN, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerN, "Shoes N", "SKU-N", "42", "Brown", 250000, 0)
		tSale := time.Date(2026, 9, 12, 12, 0, 0, 0, loc).UTC()

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerN, pID, vID, 250000, 2, tSale)
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 1, time.Date(2026, 10, 1, 12, 0, 0, 0, loc).UTC())

		// Now deactivate variant
		_, err := db.Exec(ctx, "UPDATE product_variants SET is_active = false WHERE id = $1", vID)
		require.NoError(t, err)

		detail, err := svc.GetProductDetail(ctx, sellerN, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		require.Len(t, detail.Variants, 1)
		assert.Equal(t, 2, detail.Variants[0].CohortDeliveredUnits)
		assert.Equal(t, 1, detail.Variants[0].CohortReturnedUnits)
		require.NotNil(t, detail.Variants[0].ReturnRatePercent)
		assert.InDelta(t, 50.0, *detail.Variants[0].ReturnRatePercent, 0.001)
	})

	// Scenario O: Multi-seller order -> strict seller isolation
	t.Run("Scenario O - Multi-seller order strict isolation", func(t *testing.T) {
		sellerO1, _ := insertSellerUser(t, db)
		sellerO2, _ := insertSellerUser(t, db)

		p1, v1 := insertProductWithVariant(t, db, sellerO1, "Prod O1", "SKU-O1", "M", "Black", 100000, 10)
		p2, v2 := insertProductWithVariant(t, db, sellerO2, "Prod O2", "SKU-O2", "L", "White", 200000, 10)

		tSale := time.Date(2026, 9, 14, 12, 0, 0, 0, loc).UTC()
		cID := insertCustomer(t, db)
		oID := uuid.New()
		_, err := db.Exec(ctx, `INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address)
			VALUES ($1, $2, 'delivered', 'Cust', '123', 'c@z.ru', 'Moscow')`, oID, cID)
		require.NoError(t, err)

		// Seller 1 items
		f1 := uuid.New()
		_, err = db.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at)
			VALUES ($1, $2, $3, 'delivered', 100000, 800, 92000, $4)`, f1, oID, sellerO1, tSale)
		require.NoError(t, err)
		oi1 := uuid.New()
		_, err = db.Exec(ctx, `INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, created_at)
			VALUES ($1, $2, $3, $4, $5, 'Prod O1', 'slug1', 100000, 1, 100000, $6, $7)`,
			oi1, oID, p1, v1, sellerO1, f1, tSale)
		require.NoError(t, err)
		_, err = db.Exec(ctx, `INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at)
			VALUES ($1, $2, $3, $4, 'sale_gross', 100000, $5)`, uuid.New(), sellerO1, oID, oi1, tSale)
		require.NoError(t, err)

		// Seller 2 items
		f2 := uuid.New()
		_, err = db.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at)
			VALUES ($1, $2, $3, 'delivered', 200000, 800, 184000, $4)`, f2, oID, sellerO2, tSale)
		require.NoError(t, err)
		oi2 := uuid.New()
		_, err = db.Exec(ctx, `INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, created_at)
			VALUES ($1, $2, $3, $4, $5, 'Prod O2', 'slug2', 200000, 2, 200000, $6, $7)`,
			oi2, oID, p2, v2, sellerO2, f2, tSale)
		require.NoError(t, err)
		_, err = db.Exec(ctx, `INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at)
			VALUES ($1, $2, $3, $4, 'sale_gross', 200000, $5)`, uuid.New(), sellerO2, oID, oi2, tSale)
		require.NoError(t, err)

		// Return only for Seller 2's item
		insertReturnWithItem(t, db, oID, oi2, cID, "completed", 2, time.Date(2026, 9, 25, 12, 0, 0, 0, loc).UTC())

		// Query Seller 1:
		detail1, err := svc.GetProductDetail(ctx, sellerO1, p1, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)
		assert.Equal(t, 1, detail1.CohortDeliveredUnits)
		assert.Equal(t, 0, detail1.CohortReturnedUnits, "Seller 1 must NOT see Seller 2's return")
		require.NotNil(t, detail1.ReturnRate.CurrentPercent)
		assert.Equal(t, 0.0, *detail1.ReturnRate.CurrentPercent)

		// Query Seller 2:
		detail2, err := svc.GetProductDetail(ctx, sellerO2, p2, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)
		assert.Equal(t, 2, detail2.CohortDeliveredUnits)
		assert.Equal(t, 2, detail2.CohortReturnedUnits)
		require.NotNil(t, detail2.ReturnRate.CurrentPercent)
		assert.InDelta(t, 100.0, *detail2.ReturnRate.CurrentPercent, 0.001)
	})

	// Scenario P: Return quantity exceeding originally delivered quantity -> anomaly detected, not silently clamped
	t.Run("Scenario P - Return quantity exceeding delivered quantity triggers ErrCohortDataAnomaly", func(t *testing.T) {
		sellerP, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerP, "Glitch P", "SKU-P", "M", "Red", 50000, 10)
		tSale := time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC()

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerP, pID, vID, 50000, 1, tSale)
		// Corrupted record: returned 3 units from a 1-unit order item
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 3, time.Date(2026, 9, 20, 12, 0, 0, 0, loc).UTC())

		_, err := svc.GetProductDetail(ctx, sellerP, pID, from, to, "Europe/Moscow", asOf)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCohortDataAnomaly, "Anomaly must be detected, not silently clamped to 100%")

		_, err = svc.GetProducts(ctx, sellerP, from, to, 50, 0, "gross_sales", "desc", asOf)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCohortDataAnomaly)
	})

	// Scenario Q: One order item with multiple return records -> no SQL join multiplication
	t.Run("Scenario Q - Multiple return records for same order item produce no multiplication", func(t *testing.T) {
		sellerQ, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerQ, "Sweater Q", "SKU-Q", "L", "Wool", 150000, 10)
		tSale := time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC()

		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerQ, pID, vID, 150000, 2, tSale)
		// Return 1: 1 unit
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 1, time.Date(2026, 9, 18, 12, 0, 0, 0, loc).UTC())
		// Return 2: 1 unit
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 1, time.Date(2026, 9, 22, 12, 0, 0, 0, loc).UTC())

		detail, err := svc.GetProductDetail(ctx, sellerQ, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		// Gross sales must NOT be doubled (remains 150000 * 2 = 300000 cents)
		assert.Equal(t, int64(300000), detail.GrossSales.CurrentCents)
		assert.Equal(t, 2, detail.UnitsSold.Current, "Units sold must NOT be multiplied by return joins")
		assert.Equal(t, 2, detail.CohortDeliveredUnits)
		assert.Equal(t, 2, detail.CohortReturnedUnits)
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.InDelta(t, 100.0, *detail.ReturnRate.CurrentPercent, 0.001)
	})

	// Scenario R: Original sale price change -> Gross Sales remains historically correct
	t.Run("Scenario R - Original sale price change preserves historical gross sales", func(t *testing.T) {
		sellerR, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerR, "Belt R", "SKU-R", "One", "Black", 10000, 10)
		tSale := time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC()

		insertDeliveredOrderWithItem(t, db, sellerR, pID, vID, 10000, 1, tSale)

		// Later catalog price is updated to 50,000 cents
		_, err := db.Exec(ctx, "UPDATE product_variants SET price_cents = 50000 WHERE id = $1", vID)
		require.NoError(t, err)

		detail, err := svc.GetProductDetail(ctx, sellerR, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		// Historical gross sales in ledger was 10,000 cents
		assert.Equal(t, int64(10000), detail.GrossSales.CurrentCents)
	})

	// Scenario S: Current vs previous cohort rates calculated using same asOf timestamp
	t.Run("Scenario S - Current and previous cohort rates share exact same asOf timestamp", func(t *testing.T) {
		sellerS, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerS, "Dress S", "SKU-S", "M", "Red", 100000, 20)

		// August sale (previous period)
		tSalePrev := time.Date(2026, 8, 15, 12, 0, 0, 0, loc).UTC()
		oPrev, oiPrev, cPrev := insertDeliveredOrderWithItem(t, db, sellerS, pID, vID, 100000, 10, tSalePrev)
		// August sale return completed in October (before asOf Oct 17)
		insertReturnWithItem(t, db, oPrev, oiPrev, cPrev, "completed", 2, time.Date(2026, 10, 5, 12, 0, 0, 0, loc).UTC())

		// September sale (current period)
		tSaleCurr := time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC()
		oCurr, oiCurr, cCurr := insertDeliveredOrderWithItem(t, db, sellerS, pID, vID, 100000, 10, tSaleCurr)
		// September sale return completed in October (before asOf Oct 17)
		insertReturnWithItem(t, db, oCurr, oiCurr, cCurr, "completed", 1, time.Date(2026, 10, 6, 12, 0, 0, 0, loc).UTC())

		detail, err := svc.GetProductDetail(ctx, sellerS, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		// Current period: 10 delivered, 1 returned -> 10%
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.InDelta(t, 10.0, *detail.ReturnRate.CurrentPercent, 0.001)

		// Previous period: 10 delivered, 2 returned -> 20% (counted up to same asOf in October!)
		require.NotNil(t, detail.ReturnRate.PreviousPercent)
		assert.InDelta(t, 20.0, *detail.ReturnRate.PreviousPercent, 0.001)
	})

	// Scenario T: Moscow delivery-period boundaries and return completion timestamps
	t.Run("Scenario T - Moscow timezone boundary (21:30 UTC -> 00:30 MSK next day)", func(t *testing.T) {
		sellerT, _ := insertSellerUser(t, db)
		pID, vID := insertProductWithVariant(t, db, sellerT, "Watch T", "SKU-T", "One", "Gold", 500000, 10)

		// Delivery at August 31 21:30 UTC -> September 1 00:30 MSK (INSIDE September)
		tSaleMSK := time.Date(2026, 8, 31, 21, 30, 0, 0, time.UTC)
		oID, oiID, cID := insertDeliveredOrderWithItem(t, db, sellerT, pID, vID, 500000, 1, tSaleMSK)

		// Return at September 30 21:30 UTC -> October 1 00:30 MSK (completed in October)
		tReturnMSK := time.Date(2026, 9, 30, 21, 30, 0, 0, time.UTC)
		insertReturnWithItem(t, db, oID, oiID, cID, "completed", 1, tReturnMSK)

		detail, err := svc.GetProductDetail(ctx, sellerT, pID, from, to, "Europe/Moscow", asOf)
		require.NoError(t, err)

		assert.Equal(t, 1, detail.CohortDeliveredUnits, "Sale at 00:30 MSK on Sept 1 belongs to September cohort")
		assert.Equal(t, 1, detail.CohortReturnedUnits, "Return belongs to cohort as of Oct 17")
		require.NotNil(t, detail.ReturnRate.CurrentPercent)
		assert.Equal(t, 100.0, *detail.ReturnRate.CurrentPercent)
		assert.Equal(t, 0, detail.ReturnedUnits.Current, "Return occurred at 00:30 MSK on Oct 1, outside Sept calendar period")
	})

	// Scenario U: Seller Products sorting and pagination with unavailable return rates
	t.Run("Scenario U - Products sorting with unavailable return rates sorted last", func(t *testing.T) {
		sellerU, userU := insertSellerUser(t, db)

		// Product U1: 10 sold, 5 returned -> 50%
		pU1, vU1 := insertProductWithVariant(t, db, sellerU, "Prod U1", "SKU-U1", "M", "Red", 100000, 10)
		oU1, oiU1, cU1 := insertDeliveredOrderWithItem(t, db, sellerU, pU1, vU1, 100000, 10, time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oU1, oiU1, cU1, "completed", 5, time.Date(2026, 9, 20, 12, 0, 0, 0, loc).UTC())

		// Product U2: 10 sold, 0 returned -> 0%
		pU2, vU2 := insertProductWithVariant(t, db, sellerU, "Prod U2", "SKU-U2", "L", "Blue", 100000, 10)
		insertDeliveredOrderWithItem(t, db, sellerU, pU2, vU2, 100000, 10, time.Date(2026, 9, 10, 12, 0, 0, 0, loc).UTC())

		// Product U3: Delivered in August, returned in September -> 0 cohort deliveries, return rate is nil (unavailable)
		pU3, vU3 := insertProductWithVariant(t, db, sellerU, "Prod U3", "SKU-U3", "S", "Green", 100000, 10)
		oU3, oiU3, cU3 := insertDeliveredOrderWithItem(t, db, sellerU, pU3, vU3, 100000, 2, time.Date(2026, 8, 15, 12, 0, 0, 0, loc).UTC())
		insertReturnWithItem(t, db, oU3, oiU3, cU3, "completed", 1, time.Date(2026, 9, 15, 12, 0, 0, 0, loc).UTC())

		// HTTP GET /products?sort=returns&order=desc
		reqDesc := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&sort=returns&order=desc&asOf="+asOf.Format(time.RFC3339), nil)
		reqDesc.Header.Set("X-Test-User-ID", userU.String())
		recDesc := httptest.NewRecorder()
		router.ServeHTTP(recDesc, reqDesc)

		require.Equal(t, http.StatusOK, recDesc.Code)
		var respDesc ProductsResponse
		err := json.Unmarshal(recDesc.Body.Bytes(), &respDesc)
		require.NoError(t, err)

		require.Len(t, respDesc.Items, 3)
		// Desc: 50% (U1) -> 0% (U2) -> nil (U3 sorted last)
		assert.Equal(t, pU1.String(), respDesc.Items[0].ProductID)
		require.NotNil(t, respDesc.Items[0].ReturnRatePercent)
		assert.InDelta(t, 50.0, *respDesc.Items[0].ReturnRatePercent, 0.001)

		assert.Equal(t, pU2.String(), respDesc.Items[1].ProductID)
		require.NotNil(t, respDesc.Items[1].ReturnRatePercent)
		assert.Equal(t, 0.0, *respDesc.Items[1].ReturnRatePercent)

		assert.Equal(t, pU3.String(), respDesc.Items[2].ProductID)
		assert.Nil(t, respDesc.Items[2].ReturnRatePercent, "Product with 0 cohort deliveries must have nil return rate")

		// HTTP GET /products?sort=returns&order=asc
		reqAsc := httptest.NewRequest("GET", "/products?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&sort=returns&order=asc&asOf="+asOf.Format(time.RFC3339), nil)
		reqAsc.Header.Set("X-Test-User-ID", userU.String())
		recAsc := httptest.NewRecorder()
		router.ServeHTTP(recAsc, reqAsc)

		require.Equal(t, http.StatusOK, recAsc.Code)
		var respAsc ProductsResponse
		err = json.Unmarshal(recAsc.Body.Bytes(), &respAsc)
		require.NoError(t, err)

		require.Len(t, respAsc.Items, 3)
		// Asc: 0% (U2) -> 50% (U1) -> nil (U3 sorted last in asc too)
		assert.Equal(t, pU2.String(), respAsc.Items[0].ProductID)
		require.NotNil(t, respAsc.Items[0].ReturnRatePercent)
		assert.Equal(t, 0.0, *respAsc.Items[0].ReturnRatePercent)

		assert.Equal(t, pU1.String(), respAsc.Items[1].ProductID)
		require.NotNil(t, respAsc.Items[1].ReturnRatePercent)
		assert.InDelta(t, 50.0, *respAsc.Items[1].ReturnRatePercent, 0.001)

		assert.Equal(t, pU3.String(), respAsc.Items[2].ProductID)
		assert.Nil(t, respAsc.Items[2].ReturnRatePercent, "Product with 0 cohort deliveries must have nil return rate sorted last in asc too")
	})

	// Scenario V: Product Detail timeseries and cross-seller security tests remain PASS
	t.Run("Scenario V - Cross-seller security returns 404", func(t *testing.T) {
		_, userV1 := insertSellerUser(t, db)
		sellerV2, _ := insertSellerUser(t, db)
		pV2, _ := insertProductWithVariant(t, db, sellerV2, "Secret V2", "SKU-V2", "M", "Black", 500000, 10)

		req := httptest.NewRequest("GET", "/products/"+pV2.String()+"?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339), nil)
		req.Header.Set("X-Test-User-ID", userV1.String())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code, "Unauthorized seller must receive 404")
	})
}
