package selleranalytics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
	}
}

func insertSeller(t *testing.T, db *pgxpool.Pool) uuid.UUID {
	id := uuid.New()
	userID := uuid.New()
	_, err := db.Exec(context.Background(), "INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, 'hash', 'seller')", userID, id.String()+"@test.com")
	require.NoError(t, err)
	_, err = db.Exec(context.Background(), "INSERT INTO sellers (id, user_id, status, company_name) VALUES ($1, $2, 'active', 'Test Seller')", id, userID)
	require.NoError(t, err)
	return id
}

func insertCustomer(t *testing.T, db *pgxpool.Pool) uuid.UUID {
	id := uuid.New()
	_, err := db.Exec(context.Background(), "INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, 'hash', 'customer')", id, id.String()+"@test.com")
	require.NoError(t, err)
	return id
}

func insertProduct(t *testing.T, db *pgxpool.Pool, sellerID uuid.UUID) (uuid.UUID, uuid.UUID) {
	pID := uuid.New()
	vID := uuid.New()
	catID := uuid.New()
	
	// Insert Category
	db.Exec(context.Background(), "INSERT INTO categories (id, name, slug) VALUES ($1, 'Test Cat', $2) ON CONFLICT DO NOTHING", catID, catID.String())

	_, err := db.Exec(context.Background(), "INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents) VALUES ($1, $2, $3, 'Test Product', $4, 'published', 150000)", pID, sellerID, catID, pID.String())
	require.NoError(t, err)
	_, err = db.Exec(context.Background(), "INSERT INTO product_variants (id, product_id, sku, price_cents, status) VALUES ($1, $2, $3, 150000, 'active')", vID, pID, vID.String())
	require.NoError(t, err)
	
	_, err = db.Exec(context.Background(), "INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 100, 0)", uuid.New(), pID, vID, sellerID)
	require.NoError(t, err)
	return pID, vID
}

func insertOrderWithLedger(t *testing.T, db *pgxpool.Pool, sellerID, pID, vID uuid.UUID, grossCents, commCents, earningCents int64, qty int, delivered bool, created time.Time) (uuid.UUID, uuid.UUID) {
	oID := uuid.New()
	oiID := uuid.New()
	cID := insertCustomer(t, db)

	status := "paid"
	if delivered {
		status = "delivered"
	}
	_, err := db.Exec(context.Background(), "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, $3, 'Name', '123', 'email@e.c', 'addr')", oID, cID, status)
	require.NoError(t, err)

	_, err = db.Exec(context.Background(), "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents) VALUES ($1, $2, $3, $4, $5, 'Test Product', 'slug', $6, $7, $8)", oiID, oID, pID, vID, sellerID, grossCents/int64(qty), qty, grossCents)
	require.NoError(t, err)

	_, err = db.Exec(context.Background(), "INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, updated_at) VALUES ($1, $2, $3, $4, $5, 800, $6, $7)", uuid.New(), oID, sellerID, status, grossCents, earningCents, created)
	require.NoError(t, err)

	if delivered {
		// Insert ledger entries explicitly anchoring to `created`
		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', $5, $6)", uuid.New(), sellerID, oID, oiID, grossCents, created)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'zamk_commission', $5, $6)", uuid.New(), sellerID, oID, oiID, -commCents, created)
		require.NoError(t, err)
		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'seller_earning', $5, $6)", uuid.New(), sellerID, oID, oiID, earningCents, created)
		require.NoError(t, err)
	}
	return oID, oiID
}

func insertReturn(t *testing.T, db *pgxpool.Pool, oID, oiID, cID uuid.UUID, status string, qty int, deduction int64, returnedAt time.Time) {
	rID := uuid.New()
	_, err := db.Exec(context.Background(), "INSERT INTO returns (id, order_id, user_id, status, reason, completed_at) VALUES ($1, $2, $3, $4, 'defective', $5)", rID, oID, cID, status, returnedAt)
	require.NoError(t, err)

	_, err = db.Exec(context.Background(), "INSERT INTO return_items (id, return_id, order_item_id, quantity) VALUES ($1, $2, $3, $4)", uuid.New(), rID, oiID, qty)
	require.NoError(t, err)

	if status == "completed" {
		// Deduction
		var sellerID uuid.UUID
		err = db.QueryRow(context.Background(), "SELECT seller_id FROM order_items WHERE id = $1", oiID).Scan(&sellerID)
		require.NoError(t, err)

		_, err = db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, metadata, created_at) VALUES ($1, $2, $3, $4, 'adjustment', $5, $6, $7)", uuid.New(), sellerID, oID, oiID, -deduction, `{"reason":"return_deduction"}`, returnedAt)
		require.NoError(t, err)
	}
}

func TestSellerAnalytics(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	tz := "Europe/Moscow"
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour) // current period

	t.Run("Scenario A - Basic Delivered Sale", func(t *testing.T) {
		sellerA := insertSeller(t, db)
		pID, vID := insertProduct(t, db, sellerA)
		insertOrderWithLedger(t, db, sellerA, pID, vID, 150000, 12000, 138000, 1, true, now)

		resp, err := svc.GetOverview(ctx, sellerA, from, to, tz)
		require.NoError(t, err)

		assert.Equal(t, int64(150000), resp.GrossSales.CurrentCents)
		assert.Equal(t, int64(12000), resp.Commission.CurrentCents)
		assert.Equal(t, int64(138000), resp.SellerEarningBeforeReturns.CurrentCents)
		assert.Equal(t, int64(138000), resp.NetCommercialEarning.CurrentCents)
		assert.Equal(t, 1, resp.Orders.Current)
		assert.Equal(t, 1, resp.UnitsSold.Current)
		assert.Equal(t, 0, resp.ReturnedUnits.Current)
	})

	t.Run("Scenario B - Paid But Not Delivered", func(t *testing.T) {
		sellerB := insertSeller(t, db)
		pID, vID := insertProduct(t, db, sellerB)
		insertOrderWithLedger(t, db, sellerB, pID, vID, 150000, 12000, 138000, 1, false, now)

		resp, err := svc.GetOverview(ctx, sellerB, from, to, tz)
		require.NoError(t, err)

		assert.Equal(t, int64(0), resp.GrossSales.CurrentCents)
		assert.Equal(t, 0, resp.Orders.Current)
		assert.Equal(t, 0, resp.UnitsSold.Current)
		assert.Equal(t, int64(0), resp.SellerEarningBeforeReturns.CurrentCents)
	})

	t.Run("Scenario C - Cancelled Before Delivery", func(t *testing.T) {
		sellerC := insertSeller(t, db)
		pID, vID := insertProduct(t, db, sellerC)
		insertOrderWithLedger(t, db, sellerC, pID, vID, 150000, 12000, 138000, 1, false, now)

		resp, err := svc.GetOverview(ctx, sellerC, from, to, tz)
		require.NoError(t, err)

		assert.Equal(t, int64(0), resp.GrossSales.CurrentCents)
		assert.Equal(t, 0, resp.Orders.Current)
		assert.Equal(t, 0, resp.UnitsSold.Current)
	})

	t.Run("Scenario D - Completed Return", func(t *testing.T) {
		sellerD := insertSeller(t, db)
		pID, vID := insertProduct(t, db, sellerD)
		oID, oiID := insertOrderWithLedger(t, db, sellerD, pID, vID, 150000, 12000, 138000, 1, true, now)
		
		cID := insertCustomer(t, db)
		insertReturn(t, db, oID, oiID, cID, "completed", 1, 138000, now.Add(1*time.Hour))

		resp, err := svc.GetOverview(ctx, sellerD, from, to, tz)
		require.NoError(t, err)

		assert.Equal(t, int64(150000), resp.GrossSales.CurrentCents) // Gross remains
		assert.Equal(t, int64(138000), resp.SellerEarningBeforeReturns.CurrentCents)
		assert.Equal(t, int64(-138000), resp.ReturnDeductions.CurrentCents)
		assert.Equal(t, int64(0), resp.NetCommercialEarning.CurrentCents)
		assert.Equal(t, 1, resp.ReturnedUnits.Current)
		assert.Equal(t, 100.0, resp.ReturnRate.CurrentPercent)
	})

	t.Run("Scenario E - Return In Progress", func(t *testing.T) {
		sellerE := insertSeller(t, db)
		pID, vID := insertProduct(t, db, sellerE)
		oID, oiID := insertOrderWithLedger(t, db, sellerE, pID, vID, 150000, 12000, 138000, 1, true, now)
		
		cID := insertCustomer(t, db)
		insertReturn(t, db, oID, oiID, cID, "item_received", 1, 0, now.Add(1*time.Hour))

		resp, err := svc.GetOverview(ctx, sellerE, from, to, tz)
		require.NoError(t, err)

		assert.Equal(t, 0, resp.ReturnedUnits.Current) // Not completed yet
	})

	t.Run("Scenario F - Manual Adjustment", func(t *testing.T) {
		sellerF := insertSeller(t, db)
		
		db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, type, amount_cents, created_at) VALUES ($1, $2, 'adjustment', 5000, $3)", uuid.New(), sellerF, now)

		resp, err := svc.GetOverview(ctx, sellerF, from, to, tz)
		require.NoError(t, err)

		assert.Equal(t, int64(5000), resp.OtherAdjustments.CurrentCents)
		assert.Equal(t, int64(5000), resp.NetCommercialEarning.CurrentCents) // Includes other adjustments
	})

	t.Run("Scenario G - Multi-Seller Order", func(t *testing.T) {
		sellerG1 := insertSeller(t, db)
		sellerG2 := insertSeller(t, db)
		p1, v1 := insertProduct(t, db, sellerG1)
		p2, v2 := insertProduct(t, db, sellerG2)

		oID := uuid.New()
		cID := insertCustomer(t, db)
		db.Exec(context.Background(), "INSERT INTO orders (id, user_id, status, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'delivered', 'Name', '123', 'email@e.c', 'addr')", oID, cID)
		
		oi1 := uuid.New()
		db.Exec(context.Background(), "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents) VALUES ($1, $2, $3, $4, $5, 'Test Product', 'slug', 1000, 1, 1000)", oi1, oID, p1, v1, sellerG1)
		db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 1000, $5)", uuid.New(), sellerG1, oID, oi1, now)

		oi2 := uuid.New()
		db.Exec(context.Background(), "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents) VALUES ($1, $2, $3, $4, $5, 'Test Product', 'slug', 2000, 1, 2000)", oi2, oID, p2, v2, sellerG2)
		db.Exec(context.Background(), "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, created_at) VALUES ($1, $2, $3, $4, 'sale_gross', 2000, $5)", uuid.New(), sellerG2, oID, oi2, now)

		res1, _ := svc.GetOverview(ctx, sellerG1, from, to, tz)
		res2, _ := svc.GetOverview(ctx, sellerG2, from, to, tz)

		assert.Equal(t, 1, res1.Orders.Current)
		assert.Equal(t, int64(1000), res1.GrossSales.CurrentCents)

		assert.Equal(t, 1, res2.Orders.Current)
		assert.Equal(t, int64(2000), res2.GrossSales.CurrentCents)
	})

	t.Run("Scenario H and I - Comparisons", func(t *testing.T) {
		sellerH := insertSeller(t, db)
		
		resp, err := svc.GetOverview(ctx, sellerH, from, to, tz)
		require.NoError(t, err)

		assert.Equal(t, "unchanged", resp.GrossSales.ComparisonState) // Both 0
		
		pID, vID := insertProduct(t, db, sellerH)
		insertOrderWithLedger(t, db, sellerH, pID, vID, 150000, 12000, 138000, 1, true, now)
		
		resp, err = svc.GetOverview(ctx, sellerH, from, to, tz)
		require.NoError(t, err)

		assert.Equal(t, "new", resp.GrossSales.ComparisonState) // Prev 0, Curr > 0
		assert.Nil(t, resp.GrossSales.ChangePercent)
	})

	t.Run("Scenario J - Variant Analytics", func(t *testing.T) {
		sellerJ := insertSeller(t, db)
		pID, vID := insertProduct(t, db, sellerJ)
		insertOrderWithLedger(t, db, sellerJ, pID, vID, 150000, 12000, 138000, 1, true, now)

		res, err := svc.GetProductDetail(ctx, sellerJ, pID, from, to, tz)
		require.NoError(t, err)

		assert.Len(t, res.Variants, 1)
		assert.Equal(t, int64(150000), res.Variants[0].GrossSalesCents)
		assert.Equal(t, 1, res.Variants[0].UnitsSold)
		assert.Equal(t, 100, res.Variants[0].AvailableStock) // From test setup
	})

	t.Run("Scenario K - Inventory", func(t *testing.T) {
		sellerK := insertSeller(t, db)
		insertProduct(t, db, sellerK)

		res, err := svc.GetInventory(ctx, sellerK, from, to)
		require.NoError(t, err)

		assert.Len(t, res.Items, 1)
		assert.Equal(t, 100, res.Items[0].OnHand)
		assert.Equal(t, 0, res.Items[0].Reserved)
		assert.Equal(t, 100, res.Items[0].Available)
		assert.Equal(t, 0, res.Items[0].Inbound)
	})
}
