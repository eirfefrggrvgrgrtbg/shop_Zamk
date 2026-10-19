package selleranalytics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Scenario 1: No returns, no adjustments — exact waterfall equality
func TestFinanceContract_Scenario1_NoReturnsNoAdjustments(t *testing.T) {
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
	insertOrderWithLedger(t, db, sellerID, pID, vID, 100000, 10000, 90000, 1, true, now)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, int64(100000), resp.GrossSales.CurrentCents)
	assert.Equal(t, int64(10000), resp.Commission.CurrentCents)
	assert.Equal(t, int64(90000), resp.SellerEarningBeforeReturns.CurrentCents)
	assert.Equal(t, int64(0), resp.ReturnDeductions.CurrentCents)
	assert.Equal(t, int64(0), resp.OtherAdjustments.CurrentCents)
	assert.Equal(t, int64(90000), resp.NetCommercialEarning.CurrentCents)
	assert.Equal(t, 0, resp.ReturnedUnits.Current)
	assert.Equal(t, 0.0, resp.ReturnRate.CurrentPercent)
}

// Scenario 2: Returns only (no other adjustments) — deductions correctly subtotaled
func TestFinanceContract_Scenario2_ReturnsOnly(t *testing.T) {
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
	oID, oiID := insertOrderWithLedger(t, db, sellerID, pID, vID, 100000, 10000, 90000, 1, true, now)

	cID := insertCustomer(t, db)
	insertReturn(t, db, oID, oiID, cID, "completed", 1, 90000, now.Add(1*time.Hour))

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, int64(100000), resp.GrossSales.CurrentCents)
	assert.Equal(t, int64(10000), resp.Commission.CurrentCents)
	assert.Equal(t, int64(90000), resp.SellerEarningBeforeReturns.CurrentCents)
	assert.Equal(t, int64(-90000), resp.ReturnDeductions.CurrentCents)
	assert.Equal(t, int64(0), resp.OtherAdjustments.CurrentCents)
	assert.Equal(t, int64(0), resp.NetCommercialEarning.CurrentCents)
	assert.Equal(t, 1, resp.ReturnedUnits.Current)
	assert.Equal(t, 100.0, resp.ReturnRate.CurrentPercent)
}

// Scenario 3: Returns + positive other adjustments (e.g. carrier damage compensation)
func TestFinanceContract_Scenario3_ReturnsWithPositiveOtherAdjustments(t *testing.T) {
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
	oID, oiID := insertOrderWithLedger(t, db, sellerID, pID, vID, 100000, 10000, 90000, 1, true, now)

	cID := insertCustomer(t, db)
	insertReturn(t, db, oID, oiID, cID, "completed", 1, 90000, now.Add(1*time.Hour))

	// Positive adjustment (e.g. return_compensation for item damaged in transit)
	_, err := db.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', 90000, 'RUB', '{"reason":"return_compensation"}', $5)
	`, uuid.New(), sellerID, oID, oiID, now.Add(2*time.Hour))
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, int64(90000), resp.SellerEarningBeforeReturns.CurrentCents)
	assert.Equal(t, int64(-90000), resp.ReturnDeductions.CurrentCents)
	assert.Equal(t, int64(90000), resp.OtherAdjustments.CurrentCents)
	// Commercial Result reflects direct product sales performance net of customer returns
	assert.Equal(t, int64(0), resp.NetCommercialEarning.CurrentCents)
	assert.Equal(t, 1, resp.ReturnedUnits.Current)
}

// Scenario 4: Returns + negative other adjustments (e.g. administrative penalty)
func TestFinanceContract_Scenario4_ReturnsWithNegativeOtherAdjustments(t *testing.T) {
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
	oID, oiID := insertOrderWithLedger(t, db, sellerID, pID, vID, 100000, 10000, 90000, 1, true, now)

	cID := insertCustomer(t, db)
	insertReturn(t, db, oID, oiID, cID, "completed", 1, 90000, now.Add(1*time.Hour))

	// Negative adjustment (e.g. administrative penalty / manual adjustment)
	_, err := db.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', -5000, 'RUB', '{"reason":"penalty"}', $5)
	`, uuid.New(), sellerID, oID, oiID, now.Add(2*time.Hour))
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	assert.Equal(t, int64(90000), resp.SellerEarningBeforeReturns.CurrentCents)
	assert.Equal(t, int64(-90000), resp.ReturnDeductions.CurrentCents)
	assert.Equal(t, int64(-5000), resp.OtherAdjustments.CurrentCents)
	assert.Equal(t, int64(0), resp.NetCommercialEarning.CurrentCents)
}

// Scenario 5: Post-payout adjustment scenario (reason = 'return_post_payout')
func TestFinanceContract_Scenario5_PostPayoutAdjustment(t *testing.T) {
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
	oID, oiID := insertOrderWithLedger(t, db, sellerID, pID, vID, 100000, 10000, 90000, 1, true, now)

	// Adjustment after payout has reason = 'return_post_payout'
	_, err := db.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, metadata, created_at)
		VALUES ($1, $2, $3, $4, 'adjustment', $5, $6, $7)
	`, uuid.New(), sellerID, oID, oiID, -90000, `{"reason":"return_post_payout"}`, now.Add(2*time.Hour))
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	// Must be categorized under return deductions, NOT other adjustments!
	assert.Equal(t, int64(-90000), resp.ReturnDeductions.CurrentCents)
	assert.Equal(t, int64(0), resp.OtherAdjustments.CurrentCents)
	assert.Equal(t, int64(0), resp.NetCommercialEarning.CurrentCents)
}

// Scenario 6: Commercial Result formula invariant
func TestFinanceContract_Scenario6_CommercialResultFormulaInvariant(t *testing.T) {
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
	oID, oiID := insertOrderWithLedger(t, db, sellerID, pID, vID, 250000, 25000, 225000, 2, true, now)

	cID := insertCustomer(t, db)
	insertReturn(t, db, oID, oiID, cID, "completed", 1, 112500, now.Add(1*time.Hour))

	_, err := db.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, type, amount_cents, metadata, created_at)
		VALUES ($1, $2, 'adjustment', 15000, '{"reason":"bonus"}', $3)
	`, uuid.New(), sellerID, now.Add(2*time.Hour))
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	// Invariant: NetCommercialEarning == SellerEarningBeforeReturns + ReturnDeductions
	expectedNet := resp.SellerEarningBeforeReturns.CurrentCents + resp.ReturnDeductions.CurrentCents
	assert.Equal(t, expectedNet, resp.NetCommercialEarning.CurrentCents)
	assert.Equal(t, int64(112500), resp.NetCommercialEarning.CurrentCents)
	assert.Equal(t, int64(15000), resp.OtherAdjustments.CurrentCents)
}

// Scenario 7: Return rate decoupled from monetary deductions
func TestFinanceContract_Scenario7_ReturnRateDecoupledFromMonetaryDeductions(t *testing.T) {
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
	oID, oiID := insertOrderWithLedger(t, db, sellerID, pID, vID, 200000, 20000, 180000, 2, true, now)

	// Physical return completed for 1 unit, but deduction is waived or adjusted separately
	rID := uuid.New()
	cID := insertCustomer(t, db)
	var fID uuid.UUID
	err := db.QueryRow(ctx, "SELECT order_fulfillment_id FROM order_items WHERE id = $1", oiID).Scan(&fID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, "INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, completed_at) VALUES ($1, $2, $3, $4, 'completed', 'defective', $5)", rID, oID, fID, cID, now.Add(1*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(ctx, "INSERT INTO return_items (id, return_id, order_item_id, quantity) VALUES ($1, $2, $3, 1)", uuid.New(), rID, oiID)
	require.NoError(t, err)

	// Zero monetary deduction row inserted for this return
	_, err = db.Exec(ctx, "INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, metadata, created_at) VALUES ($1, $2, $3, $4, 'adjustment', 0, '{\"reason\":\"return_deduction\"}', $5)", uuid.New(), sellerID, oID, oiID, now.Add(1*time.Hour))
	require.NoError(t, err)

	// Independent non-return adjustment inserted
	_, err = db.Exec(ctx, "INSERT INTO seller_ledger_entries (id, seller_id, type, amount_cents, metadata, created_at) VALUES ($1, $2, 'adjustment', -10000, '{\"reason\":\"penalty\"}', $3)", uuid.New(), sellerID, now.Add(2*time.Hour))
	require.NoError(t, err)

	resp, err := svc.GetOverview(ctx, sellerID, from, to, tz)
	require.NoError(t, err)

	// Physical returned units strictly matches the completed return item quantity (1), not monetary counts
	assert.Equal(t, 1, resp.ReturnedUnits.Current)
	assert.Equal(t, 2, resp.UnitsSold.Current)
	// Return rate is 1 / 2 = 50.0%
	assert.Equal(t, 50.0, resp.ReturnRate.CurrentPercent)
	assert.Equal(t, int64(0), resp.ReturnDeductions.CurrentCents)
	assert.Equal(t, int64(-10000), resp.OtherAdjustments.CurrentCents)
	assert.Equal(t, int64(180000), resp.NetCommercialEarning.CurrentCents)
}
