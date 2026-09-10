package payouts

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertReturnData(t *testing.T, env deductionTestEnv, retID uuid.UUID, retStatus string, retItemQty int, rraQty int, reasonCode string, status string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var userID uuid.UUID
	err := env.client.Pool.QueryRow(ctx, "SELECT user_id FROM orders WHERE id = $1", env.orderID).Scan(&userID)
	require.NoError(t, err)
	var fulfillmentID uuid.UUID
	err = env.client.Pool.QueryRow(ctx, "SELECT id FROM order_fulfillments WHERE order_id = $1", env.orderID).Scan(&fulfillmentID)
	require.NoError(t, err)

	_, err = env.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'defective', now(), now())
	`, retID, env.orderID, fulfillmentID, userID, retStatus)
	require.NoError(t, err)

	retItemID := uuid.New()
	damagedQty := 0
	acceptedQty := 0
	var disp *string
	if reasonCode == "zamk_warehouse_damage" || reasonCode == "carrier_damage" || reasonCode == "zamk_fulfillment_error" {
		damagedQty = rraQty
		d := "damaged"
		disp = &d
	} else if reasonCode == "customer_change_of_mind" {
		acceptedQty = rraQty
		a := "accepted"
		disp = &a
	}
	_, err = env.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 0)
	`, retItemID, retID, env.orderItemID, retItemQty, acceptedQty, damagedQty)
	require.NoError(t, err)

	rraID := uuid.New()
	var party *string
	if reasonCode == "zamk_warehouse_damage" || reasonCode == "zamk_fulfillment_error" {
		p := "zamk"
		party = &p
	} else if reasonCode == "carrier_damage" {
		p := "carrier"
		party = &p
	} else if reasonCode == "customer_change_of_mind" {
		party = nil
	} else {
		p := "customer"
		party = &p
	}
	_, err = env.client.Pool.Exec(ctx, `
		INSERT INTO return_responsibility_allocations (id, return_item_id, quantity, status, responsible_party, reason_code, decision_source, decided_at, legacy_disposition, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'system', now(), $7, now(), now())
	`, rraID, retItemID, rraQty, status, party, reasonCode, disp)
	require.NoError(t, err)

	return retItemID
}

func getCompensationForOrderItem(t *testing.T, env deductionTestEnv) []int64 {
	ctx := context.Background()
	rows, err := env.client.Pool.Query(ctx, `
		SELECT amount_cents
		FROM seller_ledger_entries
		WHERE order_item_id = $1
		  AND type = 'adjustment'
		  AND metadata->>'reason' IN ('return_compensation', 'return_compensation_correction')
		ORDER BY created_at ASC, id ASC
	`, env.orderItemID)
	require.NoError(t, err)
	defer rows.Close()

	var amts []int64
	for rows.Next() {
		var a int64
		require.NoError(t, rows.Scan(&a))
		amts = append(amts, a)
	}
	return amts
}

func getNetCompensation(t *testing.T, env deductionTestEnv) int64 {
	amts := getCompensationForOrderItem(t, env)
	var sum int64
	for _, a := range amts {
		sum += a
	}
	return sum
}

func TestCompensation_00_DatabaseGuard(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	var dbName string
	err := client.Pool.QueryRow(context.Background(), "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	assert.Equal(t, "zamk_test", dbName)
}

// 1. Counterexample Test (Section 3 & 11)
func TestCompensation_Counterexample_GlobalMinBug(t *testing.T) {
	env := setupDeductionTestEnv(t, 2, 350000, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	// Return A: q1 ordinary/restocked (customer_change_of_mind), Sale Reversal processed
	retA := uuid.New()
	insertReturnData(t, env, retA, "refunded", 1, 1, "customer_change_of_mind", "not_required")
	txA, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, txA, retA, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	require.NoError(t, txA.Commit(ctx))

	// Return B: q1 ZAMK physical loss, responsibility resolved ZAMK, NO Sale Reversal yet (still item_received)
	retB := uuid.New()
	insertReturnData(t, env, retB, "item_received", 1, 1, "zamk_warehouse_damage", "resolved")

	// Reconcile
	txReconcile, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, txReconcile, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, txReconcile.Commit(ctx))

	// Under correct per-return intersection: Return B is not reversed yet, Return A is not compensable. C=0.
	comps := getCompensationForOrderItem(t, env)
	assert.Len(t, comps, 0, "Return B is not reversed yet, so C must be 0 and zero compensation rows should exist")

	// Process Sale Reversal for Return B q1
	_, err = env.client.Pool.Exec(ctx, "UPDATE returns SET status = 'refunded' WHERE id = $1", retB)
	require.NoError(t, err)
	txB, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, txB, retB, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	require.NoError(t, txB.Commit(ctx))

	// Reconcile again
	txReconcile2, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, txReconcile2, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, txReconcile2.Commit(ctx))

	// Now Return B is reversed, C=1, target = floor(700000 * 1 / 2) = 350000
	compsAfter := getCompensationForOrderItem(t, env)
	require.Len(t, compsAfter, 1)
	assert.Equal(t, int64(350000), compsAfter[0])
}

// 2. Required Rounding Tests (Section 8 & 17)
func TestCompensation_17_RoundingTests(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	// A. C=0, target=0, no row
	tx, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	comps := getCompensationForOrderItem(t, env)
	assert.Len(t, comps, 0)

	// B. C=1, target=233333
	ret1 := uuid.New()
	insertReturnData(t, env, ret1, "refunded", 1, 1, "zamk_warehouse_damage", "resolved")
	tx, err = env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx, ret1, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	comps = getCompensationForOrderItem(t, env)
	require.Len(t, comps, 1)
	assert.Equal(t, int64(233333), comps[0])

	// C. C=2, target=466666 (delta = 466666 - 233333 = 233333)
	ret2 := uuid.New()
	insertReturnData(t, env, ret2, "refunded", 1, 1, "carrier_damage", "resolved")
	tx, err = env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx, ret2, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	comps = getCompensationForOrderItem(t, env)
	require.Len(t, comps, 2)
	assert.Equal(t, int64(233333), comps[1])

	// D. C=3, target=700000 exact (delta = 700000 - 466666 = 233334)
	ret3 := uuid.New()
	insertReturnData(t, env, ret3, "refunded", 1, 1, "zamk_fulfillment_error", "resolved")
	tx, err = env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx, ret3, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	comps = getCompensationForOrderItem(t, env)
	require.Len(t, comps, 3)
	assert.Equal(t, int64(233334), comps[2])
	assert.Equal(t, int64(700000), comps[0]+comps[1]+comps[2])
}

// 3. Real Order Independence Test (Section 10)
func TestCompensation_18_OrderIndependence(t *testing.T) {
	type step struct {
		isZamk bool
	}

	runScenario := func(t *testing.T, steps []step) int64 {
		env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
		defer env.client.Close()
		ctx := context.Background()

		for _, s := range steps {
			retID := uuid.New()
			reason := "seller_product_defect"
			party := "seller"
			status := "resolved"
			if s.isZamk {
				reason = "zamk_warehouse_damage"
				party = "zamk"
			}
			var userID uuid.UUID
			err := env.client.Pool.QueryRow(ctx, "SELECT user_id FROM orders WHERE id = $1", env.orderID).Scan(&userID)
			require.NoError(t, err)
			var fulfillmentID uuid.UUID
			err = env.client.Pool.QueryRow(ctx, "SELECT id FROM order_fulfillments WHERE order_id = $1", env.orderID).Scan(&fulfillmentID)
			require.NoError(t, err)

			_, err = env.client.Pool.Exec(ctx, `
				INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
				VALUES ($1, $2, $3, $4, 'refunded', 'defective', now(), now())
			`, retID, env.orderID, fulfillmentID, userID)
			require.NoError(t, err)

			retItemID := uuid.New()
			damagedQty := 0
			var disp *string
			if s.isZamk {
				damagedQty = 1
				d := "damaged"
				disp = &d
			} else {
				d := "damaged"
				disp = &d
			}
			_, err = env.client.Pool.Exec(ctx, `
				INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity)
				VALUES ($1, $2, $3, 1, 0, $4, 0)
			`, retItemID, retID, env.orderItemID, damagedQty)
			require.NoError(t, err)

			rraID := uuid.New()
			_, err = env.client.Pool.Exec(ctx, `
				INSERT INTO return_responsibility_allocations (id, return_item_id, quantity, status, responsible_party, reason_code, decision_source, decided_at, legacy_disposition, created_at, updated_at)
				VALUES ($1, $2, 1, $3, $4, $5, 'system', now(), $6, now(), now())
			`, rraID, retItemID, status, party, reason, disp)
			require.NoError(t, err)

			tx, err := env.client.Pool.Begin(ctx)
			require.NoError(t, err)
			err = env.svc.ProcessReturnDeduction(ctx, tx, retID, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
			require.NoError(t, err)
			err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
			require.NoError(t, err)
			require.NoError(t, tx.Commit(ctx))
		}
		return getNetCompensation(t, env)
	}

	// Scenario A: seller -> zamk -> zamk
	resA := runScenario(t, []step{{isZamk: false}, {isZamk: true}, {isZamk: true}})
	assert.Equal(t, int64(466666), resA)

	// Scenario B: zamk -> seller -> zamk
	resB := runScenario(t, []step{{isZamk: true}, {isZamk: false}, {isZamk: true}})
	assert.Equal(t, int64(466666), resB)

	// Scenario C: zamk -> zamk -> seller
	resC := runScenario(t, []step{{isZamk: true}, {isZamk: true}, {isZamk: false}})
	assert.Equal(t, int64(466666), resC)
}

// 4. Cross-Return Test Proving No Leakage (Section 11 & 19)
func TestCompensation_19_CrossReturn(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	// Return A: ordinary / not compensable, reversed q1
	retA := uuid.New()
	insertReturnData(t, env, retA, "refunded", 1, 1, "customer_change_of_mind", "not_required")
	txA, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, txA, retA, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, txA, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, txA.Commit(ctx))
	assert.Equal(t, int64(0), getNetCompensation(t, env))

	// Return B: ZAMK loss q1, but NOT reversed yet
	retB := uuid.New()
	insertReturnData(t, env, retB, "item_received", 1, 1, "zamk_warehouse_damage", "resolved")
	txB1, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, txB1, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, txB1.Commit(ctx))
	// Must still be 0 (no leakage from Return A)
	assert.Equal(t, int64(0), getNetCompensation(t, env))

	// Now reverse Return B
	_, err = env.client.Pool.Exec(ctx, "UPDATE returns SET status = 'refunded' WHERE id = $1", retB)
	require.NoError(t, err)
	txB2, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, txB2, retB, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, txB2, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, txB2.Commit(ctx))
	assert.Equal(t, int64(233333), getNetCompensation(t, env))

	// Return C: ZAMK loss q1, reversed
	retC := uuid.New()
	insertReturnData(t, env, retC, "refunded", 1, 1, "zamk_warehouse_damage", "resolved")
	txC, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, txC, retC, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, txC, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, txC.Commit(ctx))

	// Total Sale Reversal: 700000 exact
	// Compensation net target: 466666 exact
	comps := getCompensationForOrderItem(t, env)
	require.Len(t, comps, 2)
	assert.Equal(t, int64(233333), comps[0])
	assert.Equal(t, int64(233333), comps[1])
	assert.Equal(t, int64(466666), getNetCompensation(t, env))
}

// 5. Responsibility Correction Tests (Section 12 & 20)
func TestCompensation_20_Correction(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	ret1 := uuid.New()
	retItemID := insertReturnData(t, env, ret1, "refunded", 1, 1, "zamk_warehouse_damage", "resolved")
	tx, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx, ret1, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	comps := getCompensationForOrderItem(t, env)
	require.Len(t, comps, 1)
	assert.Equal(t, int64(233333), comps[0])

	// Case 1: ZAMK -> non-compensable (C: 1 -> 0)
	// (Direct SQL fixture testing reconciliation state consumption)
	_, err = env.client.Pool.Exec(ctx, "UPDATE return_responsibility_allocations SET reason_code = 'customer_change_of_mind', status = 'not_required', responsible_party = NULL, legacy_disposition = 'accepted' WHERE return_item_id = $1", retItemID)
	require.NoError(t, err)

	tx, err = env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	comps = getCompensationForOrderItem(t, env)
	require.Len(t, comps, 2)
	assert.Equal(t, int64(233333), comps[0])
	assert.Equal(t, int64(-233333), comps[1]) // negative append-only correction
	assert.Equal(t, int64(0), getNetCompensation(t, env))

	// Case 2: non-compensable -> ZAMK (C: 0 -> 1)
	_, err = env.client.Pool.Exec(ctx, "UPDATE return_responsibility_allocations SET reason_code = 'zamk_warehouse_damage', status = 'resolved', responsible_party = 'zamk', legacy_disposition = 'damaged' WHERE return_item_id = $1", retItemID)
	require.NoError(t, err)

	tx, err = env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	comps = getCompensationForOrderItem(t, env)
	require.Len(t, comps, 3)
	assert.Equal(t, int64(233333), comps[0])
	assert.Equal(t, int64(-233333), comps[1])
	assert.Equal(t, int64(233333), comps[2]) // positive compensation
	assert.Equal(t, int64(233333), getNetCompensation(t, env))
}

// 6. Refund Gating Test (Section 21)
func TestCompensation_21_RefundGating(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	ret1 := uuid.New()
	insertReturnData(t, env, ret1, "item_received", 1, 1, "zamk_warehouse_damage", "resolved")

	// BEFORE refund/reversal: R=0, C=0, delta=0 -> zero compensation
	tx, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	assert.Len(t, getCompensationForOrderItem(t, env), 0)

	// AFTER refund & reversal: R=1, C=1 -> positive compensation
	_, err = env.client.Pool.Exec(ctx, "UPDATE returns SET status = 'refunded' WHERE id = $1", ret1)
	require.NoError(t, err)
	tx, err = env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx, ret1, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	comps := getCompensationForOrderItem(t, env)
	require.Len(t, comps, 1)
	assert.Equal(t, int64(233333), comps[0])
}

// 7. Restocked ZAMK Error Test (Section 22)
func TestCompensation_22_RestockedZamkError(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	var userID uuid.UUID
	err := env.client.Pool.QueryRow(ctx, "SELECT user_id FROM orders WHERE id = $1", env.orderID).Scan(&userID)
	require.NoError(t, err)
	var fulfillmentID uuid.UUID
	err = env.client.Pool.QueryRow(ctx, "SELECT id FROM order_fulfillments WHERE order_id = $1", env.orderID).Scan(&fulfillmentID)
	require.NoError(t, err)

	ret1 := uuid.New()
	_, err = env.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'refunded', 'defective', now(), now())
	`, ret1, env.orderID, fulfillmentID, userID)
	require.NoError(t, err)

	retItemID := uuid.New()
	// accepted_quantity = 1 (restocked/saleable), damaged_quantity = 0
	_, err = env.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity)
		VALUES ($1, $2, $3, 1, 1, 0, 0)
	`, retItemID, ret1, env.orderItemID)
	require.NoError(t, err)

	rraID := uuid.New()
	disp := "accepted"
	_, err = env.client.Pool.Exec(ctx, `
		INSERT INTO return_responsibility_allocations (id, return_item_id, quantity, status, responsible_party, reason_code, decision_source, decided_at, legacy_disposition, created_at, updated_at)
		VALUES ($1, $2, 1, 'resolved', 'zamk', 'zamk_fulfillment_error', 'system', now(), $3, now(), now())
	`, rraID, retItemID, disp)
	require.NoError(t, err)

	tx, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx, ret1, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	comps := getCompensationForOrderItem(t, env)
	assert.Len(t, comps, 0, "No compensation when physical unit is restocked/saleable")
}

// 8. Concurrency Test without Error Swallowing (Section 9)
func TestCompensation_Concurrency_Safe(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	ret1 := uuid.New()
	insertReturnData(t, env, ret1, "refunded", 1, 1, "zamk_warehouse_damage", "resolved")
	tx1, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx1, ret1, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
	require.NoError(t, err)
	require.NoError(t, tx1.Commit(ctx))

	var wg sync.WaitGroup
	type res struct {
		beginErr     error
		reconcileErr error
		commitErr    error
	}
	results := make(chan res, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var r res
			tx, err := env.client.Pool.Begin(ctx)
			if err != nil {
				r.beginErr = err
				results <- r
				return
			}
			defer tx.Rollback(ctx)

			r.reconcileErr = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
			if r.reconcileErr != nil {
				results <- r
				return
			}
			r.commitErr = tx.Commit(ctx)
			results <- r
		}()
	}
	wg.Wait()
	close(results)

	successCount := 0
	for r := range results {
		assert.NoError(t, r.beginErr)
		assert.NoError(t, r.reconcileErr)
		assert.NoError(t, r.commitErr)
		if r.beginErr == nil && r.reconcileErr == nil && r.commitErr == nil {
			successCount++
		}
	}
	assert.Equal(t, 5, successCount, "SUCCESS COUNT must equal 5")

	comps := getCompensationForOrderItem(t, env)
	require.Len(t, comps, 1, "Exactly ONE compensation delta row must exist")
	assert.Equal(t, int64(233333), comps[0])
}

// 9. Physical Loss Matrix Tests (Section 13)
func TestCompensation_13_PhysicalLossMatrix(t *testing.T) {
	ctx := context.Background()

	// Helper for legacy physical loss tests
	testLegacyCase := func(t *testing.T, name string, accepted, damaged, rejected int, reason string, party *string, reversed bool, expectedCompensableQty int64) {
		t.Run("Legacy_"+name, func(t *testing.T) {
			env := setupDeductionTestEnv(t, 1, 100000, 100000, false)
			defer env.client.Close()

			var userID, fulfillmentID uuid.UUID
			_ = env.client.Pool.QueryRow(ctx, "SELECT user_id FROM orders WHERE id = $1", env.orderID).Scan(&userID)
			_ = env.client.Pool.QueryRow(ctx, "SELECT id FROM order_fulfillments WHERE order_id = $1", env.orderID).Scan(&fulfillmentID)

			retID := uuid.New()
			retStatus := "item_received"
			if reversed {
				retStatus = "refunded"
			}
			_, err := env.client.Pool.Exec(ctx, `
				INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, 'defective', now(), now())
			`, retID, env.orderID, fulfillmentID, userID, retStatus)
			require.NoError(t, err)

			retItemID := uuid.New()
			_, err = env.client.Pool.Exec(ctx, `
				INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity)
				VALUES ($1, $2, $3, 1, $4, $5, $6)
			`, retItemID, retID, env.orderItemID, accepted, damaged, rejected)
			require.NoError(t, err)

			rraID := uuid.New()
			var disp *string
			if damaged > 0 {
				d := "damaged"
				disp = &d
			} else if accepted > 0 {
				a := "accepted"
				disp = &a
			} else if rejected > 0 {
				r := "rejected"
				disp = &r
			} else {
				u := "unreceived"
				disp = &u
			}
			_, err = env.client.Pool.Exec(ctx, `
				INSERT INTO return_responsibility_allocations (id, return_item_id, quantity, status, responsible_party, reason_code, decision_source, decided_at, legacy_disposition, created_at, updated_at)
				VALUES ($1, $2, 1, 'resolved', $3, $4, 'system', now(), $5, now(), now())
			`, rraID, retItemID, party, reason, disp)
			require.NoError(t, err)

			tx, err := env.client.Pool.Begin(ctx)
			require.NoError(t, err)
			defer tx.Rollback(ctx)

			if reversed {
				err = env.svc.ProcessReturnDeduction(ctx, tx, retID, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
				require.NoError(t, err)
			}

			c, err := env.repo.GetCompensableQuantityTx(ctx, tx, env.orderItemID)
			require.NoError(t, err)
			assert.Equal(t, expectedCompensableQty, c)
		})
	}

	zamkParty := "zamk"
	// LEGACY:
	// accepted q1 + ZAMK responsibility + reversed q1 => C=0
	testLegacyCase(t, "accepted_q1_reversed", 1, 0, 0, "zamk_warehouse_damage", &zamkParty, true, 0)
	// damaged q1 + ZAMK responsibility + reversed q1 => C=1
	testLegacyCase(t, "damaged_q1_reversed", 0, 1, 0, "zamk_warehouse_damage", &zamkParty, true, 1)
	// rejected q1 + ZAMK responsibility + no reversal => C=0
	testLegacyCase(t, "rejected_q1_no_reversal", 0, 0, 1, "zamk_warehouse_damage", &zamkParty, false, 0)
	// unreceived q1 + ZAMK responsibility + no reversal => C=0
	testLegacyCase(t, "unreceived_q1_no_reversal", 0, 0, 0, "zamk_warehouse_damage", &zamkParty, false, 0)

	// Helper for serialized unit tests
	testSerializedCase := func(t *testing.T, name string, disposition *string, reversed bool, expectedCompensableQty int64) {
		t.Run("Serialized_"+name, func(t *testing.T) {
			env := setupDeductionTestEnv(t, 1, 100000, 100000, false)
			defer env.client.Close()

			var userID, fulfillmentID, variantID uuid.UUID
			_ = env.client.Pool.QueryRow(ctx, "SELECT user_id FROM orders WHERE id = $1", env.orderID).Scan(&userID)
			_ = env.client.Pool.QueryRow(ctx, "SELECT id FROM order_fulfillments WHERE order_id = $1", env.orderID).Scan(&fulfillmentID)
			_ = env.client.Pool.QueryRow(ctx, "SELECT product_variant_id FROM order_items WHERE id = $1", env.orderItemID).Scan(&variantID)

			// Setup supply, supply item, inventory unit, and allocation
			supplyID := uuid.New()
			_, err := env.client.Pool.Exec(ctx, `
				INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
				VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
			`, supplyID, env.sellerID, "SUP-"+uuid.New().String()[:8])
			require.NoError(t, err)

			supplyItemID := uuid.New()
			_, err = env.client.Pool.Exec(ctx, `
				INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
				VALUES ($1, $2, $3, 1, now(), now())
			`, supplyItemID, supplyID, variantID)
			require.NoError(t, err)

			invUnitID := uuid.New()
			unitCode := "ZMU-" + uuid.New().String()[:8]
			_, err = env.client.Pool.Exec(ctx, `
				INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, 1, 'shipped', now(), now())
			`, invUnitID, unitCode, variantID, supplyID, supplyItemID)
			require.NoError(t, err)

			allocID := uuid.New()
			_, err = env.client.Pool.Exec(ctx, `
				INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at, created_at)
				VALUES ($1, $2, $3, now(), now())
			`, allocID, env.orderItemID, invUnitID)
			require.NoError(t, err)

			retID := uuid.New()
			retStatus := "item_received"
			if reversed {
				retStatus = "refunded"
			}
			_, err = env.client.Pool.Exec(ctx, `
				INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, 'defective', now(), now())
			`, retID, env.orderID, fulfillmentID, userID, retStatus)
			require.NoError(t, err)

			retItemID := uuid.New()
			_, err = env.client.Pool.Exec(ctx, `
				INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity)
				VALUES ($1, $2, $3, 1, 0, 0, 0)
			`, retItemID, retID, env.orderItemID)
			require.NoError(t, err)

			// Insert serialized unit if disposition is provided
			if disposition != nil {
				unitID := uuid.New()
				_, err = env.client.Pool.Exec(ctx, `
					INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, disposition, created_at, updated_at)
					VALUES ($1, $2, $3, $4, now(), now())
				`, unitID, retItemID, allocID, *disposition)
				require.NoError(t, err)
			}

			rraID := uuid.New()
			_, err = env.client.Pool.Exec(ctx, `
				INSERT INTO return_responsibility_allocations (id, return_item_id, order_item_allocation_id, quantity, status, responsible_party, reason_code, decision_source, decided_at, created_at, updated_at)
				VALUES ($1, $2, $3, 1, 'resolved', 'zamk', 'zamk_warehouse_damage', 'system', now(), now(), now())
			`, rraID, retItemID, allocID)
			require.NoError(t, err)

			tx, err := env.client.Pool.Begin(ctx)
			require.NoError(t, err)
			defer tx.Rollback(ctx)

			if reversed {
				err = env.svc.ProcessReturnDeduction(ctx, tx, retID, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
				require.NoError(t, err)
			}

			c, err := env.repo.GetCompensableQuantityTx(ctx, tx, env.orderItemID)
			require.NoError(t, err)
			assert.Equal(t, expectedCompensableQty, c)
		})
	}

	restockDisp := "restock"
	damagedDisp := "damaged"
	rejectDisp := "reject"

	// SERIALIZED:
	// restock unit => C=0
	testSerializedCase(t, "restock_reversed", &restockDisp, true, 0)
	// damaged unit + reversed => C=1
	testSerializedCase(t, "damaged_reversed", &damagedDisp, true, 1)
	// reject unit without reversal => C=0
	testSerializedCase(t, "reject_without_reversal", &rejectDisp, false, 0)
	// missing/no return_item_unit without explicit reversal => C=0
	testSerializedCase(t, "missing_unit_without_reversal", nil, false, 0)
}

// 10. Comprehensive Balance Tests & Available_At (Section 14 & 16)
func TestCompensation_24_Balance(t *testing.T) {
	ctx := context.Background()

	t.Run("PostPayout_ReversalPlusEqualCompensation", func(t *testing.T) {
		env := setupDeductionTestEnv(t, 1, 700000, 700000, true) // Post-payout
		defer env.client.Close()

		summaryBefore, err := env.repo.GetSellerBalanceSummary(ctx, env.sellerID)
		require.NoError(t, err)

		ret1 := uuid.New()
		insertReturnData(t, env, ret1, "refunded", 1, 1, "zamk_warehouse_damage", "resolved")
		tx, err := env.client.Pool.Begin(ctx)
		require.NoError(t, err)
		err = env.svc.ProcessReturnDeduction(ctx, tx, ret1, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
		require.NoError(t, err)
		err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))

		summaryAfter, err := env.repo.GetSellerBalanceSummary(ctx, env.sellerID)
		require.NoError(t, err)

		// Gross, Commission, Paid remain UNCHANGED
		assert.Equal(t, summaryBefore.GrossSalesCents, summaryAfter.GrossSalesCents, "Gross sales unchanged")
		assert.Equal(t, summaryBefore.CommissionCents, summaryAfter.CommissionCents, "Commission unchanged")
		assert.Equal(t, summaryBefore.PaidCents, summaryAfter.PaidCents, "Paid cents unchanged")

		// Adjustments net unchanged: deduction -700000 + compensation +700000 = 0
		assert.Equal(t, summaryBefore.AdjustmentsCents, summaryAfter.AdjustmentsCents, "Adjustments net unchanged")
		assert.Equal(t, summaryBefore.AvailableCents, summaryAfter.AvailableCents, "Available balance unchanged")
		assert.Equal(t, summaryBefore.FrozenCents, summaryAfter.FrozenCents, "Frozen balance unchanged")
	})

	t.Run("A_EarningStillFrozen_InheritsAvailableAt", func(t *testing.T) {
		env := setupDeductionTestEnv(t, 1, 700000, 700000, false)
		defer env.client.Close()

		futureTime := time.Now().Add(14 * 24 * time.Hour).UTC().Truncate(time.Second)
		_, err := env.client.Pool.Exec(ctx, "UPDATE seller_ledger_entries SET available_at = $1 WHERE order_item_id = $2 AND type = 'seller_earning'", futureTime, env.orderItemID)
		require.NoError(t, err)

		ret1 := uuid.New()
		insertReturnData(t, env, ret1, "refunded", 1, 1, "zamk_warehouse_damage", "resolved")
		tx, err := env.client.Pool.Begin(ctx)
		require.NoError(t, err)
		err = env.svc.ProcessReturnDeduction(ctx, tx, ret1, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
		require.NoError(t, err)
		err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))

		var compAvailAt *time.Time
		err = env.client.Pool.QueryRow(ctx, "SELECT available_at FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_compensation'", env.orderItemID).Scan(&compAvailAt)
		require.NoError(t, err)
		require.NotNil(t, compAvailAt)
		assert.Equal(t, futureTime, compAvailAt.UTC().Truncate(time.Second), "Frozen earning compensation must inherit future available_at")

		summary, err := env.repo.GetSellerBalanceSummary(ctx, env.sellerID)
		require.NoError(t, err)
		// Since deduction (-700000) and compensation (+700000) both share future available_at, they both reside in frozen pool
		assert.Equal(t, int64(0), summary.AvailableCents)
		assert.Equal(t, int64(700000), summary.FrozenCents) // 700000 earning - 700000 deduction + 700000 comp = 700000 frozen
	})

	t.Run("B_EarningAlreadyAvailable_AvailableAtNull", func(t *testing.T) {
		env := setupDeductionTestEnv(t, 1, 700000, 700000, false)
		defer env.client.Close()

		pastTime := time.Now().Add(-2 * time.Hour).UTC()
		_, err := env.client.Pool.Exec(ctx, "UPDATE seller_ledger_entries SET available_at = $1 WHERE order_item_id = $2 AND type = 'seller_earning'", pastTime, env.orderItemID)
		require.NoError(t, err)

		ret1 := uuid.New()
		insertReturnData(t, env, ret1, "refunded", 1, 1, "zamk_warehouse_damage", "resolved")
		tx, err := env.client.Pool.Begin(ctx)
		require.NoError(t, err)
		err = env.svc.ProcessReturnDeduction(ctx, tx, ret1, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
		require.NoError(t, err)
		err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))

		var compAvailAt *time.Time
		err = env.client.Pool.QueryRow(ctx, "SELECT available_at FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_compensation'", env.orderItemID).Scan(&compAvailAt)
		require.NoError(t, err)
		assert.Nil(t, compAvailAt, "Available earning compensation must have available_at NULL")

		summary, err := env.repo.GetSellerBalanceSummary(ctx, env.sellerID)
		require.NoError(t, err)
		assert.Equal(t, int64(700000), summary.AvailableCents) // Available immediately
	})

	t.Run("C_EarningAlreadyPaid_AvailableAtNull", func(t *testing.T) {
		env := setupDeductionTestEnv(t, 1, 700000, 700000, true) // isPaidOut = true
		defer env.client.Close()

		ret1 := uuid.New()
		insertReturnData(t, env, ret1, "refunded", 1, 1, "zamk_warehouse_damage", "resolved")
		tx, err := env.client.Pool.Begin(ctx)
		require.NoError(t, err)
		err = env.svc.ProcessReturnDeduction(ctx, tx, ret1, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
		require.NoError(t, err)
		err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))

		var compAvailAt *time.Time
		err = env.client.Pool.QueryRow(ctx, "SELECT available_at FROM seller_ledger_entries WHERE order_item_id = $1 AND type = 'adjustment' AND metadata->>'reason' = 'return_compensation'", env.orderItemID).Scan(&compAvailAt)
		require.NoError(t, err)
		assert.Nil(t, compAvailAt, "Paid earning compensation must have available_at NULL")
	})
}

func insertCustomReturnData(t *testing.T, env deductionTestEnv, retID uuid.UUID, retStatus string, retItemQty int, rraQty int, party *string, reasonCode string, legacyDisp *string, status string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var userID uuid.UUID
	err := env.client.Pool.QueryRow(ctx, "SELECT user_id FROM orders WHERE id = $1", env.orderID).Scan(&userID)
	require.NoError(t, err)
	var fulfillmentID uuid.UUID
	err = env.client.Pool.QueryRow(ctx, "SELECT id FROM order_fulfillments WHERE order_id = $1", env.orderID).Scan(&fulfillmentID)
	require.NoError(t, err)

	_, err = env.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'defective', now(), now())
	`, retID, env.orderID, fulfillmentID, userID, retStatus)
	require.NoError(t, err)

	retItemID := uuid.New()
	damagedQty := 0
	if legacyDisp != nil && *legacyDisp == "damaged" {
		damagedQty = rraQty
	}
	acceptedQty := 0
	if legacyDisp != nil && *legacyDisp == "accepted" {
		acceptedQty = rraQty
	}

	_, err = env.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 0)
	`, retItemID, retID, env.orderItemID, retItemQty, acceptedQty, damagedQty)
	require.NoError(t, err)

	rraID := uuid.New()
	_, err = env.client.Pool.Exec(ctx, `
		INSERT INTO return_responsibility_allocations (id, return_item_id, quantity, status, responsible_party, reason_code, decision_source, decided_at, legacy_disposition, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'system', now(), $7, now(), now())
	`, rraID, retItemID, rraQty, status, party, reasonCode, legacyDisp)
	require.NoError(t, err)

	return retItemID
}

// 11. Fraud and Unknown Responsibility Auto-Compensation Tests (Section 10)
func TestCompensation_25_FraudAndUnknownNoAutoCompensation(t *testing.T) {
	ctx := context.Background()
	zamkParty := "zamk"
	carrierParty := "carrier"
	damagedDisp := "damaged"

	cases := []struct {
		name       string
		party      *string
		reasonCode string
	}{
		{name: "FraudWithZamkParty", party: &zamkParty, reasonCode: "fraud_or_substitution"},
		{name: "FraudWithCarrierParty", party: &carrierParty, reasonCode: "fraud_or_substitution"},
		{name: "UnknownWithZamkParty", party: &zamkParty, reasonCode: "unknown"},
		{name: "UnknownWithCarrierParty", party: &carrierParty, reasonCode: "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupDeductionTestEnv(t, 1, 700000, 700000, false)
			defer env.client.Close()

			retID := uuid.New()
			insertCustomReturnData(t, env, retID, "refunded", 1, 1, tc.party, tc.reasonCode, &damagedDisp, "resolved")

			tx, err := env.client.Pool.Begin(ctx)
			require.NoError(t, err)
			err = env.svc.ProcessReturnDeduction(ctx, tx, retID, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
			require.NoError(t, err)

			c, err := env.repo.GetCompensableQuantityTx(ctx, tx, env.orderItemID)
			require.NoError(t, err)
			assert.Equal(t, int64(0), c, "Reason %s must not be auto-compensated", tc.reasonCode)

			err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
			require.NoError(t, err)
			require.NoError(t, tx.Commit(ctx))

			comps := getCompensationForOrderItem(t, env)
			assert.Empty(t, comps, "No compensation should be created for %s", tc.reasonCode)
			assert.Equal(t, int64(0), getNetCompensation(t, env))
		})
	}
}

// 12. Serialized Return Event Disposition vs Mutable Inventory Status Test (Section 11)
func TestCompensation_26_SerializedHistoricalEventVsInventoryStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("HistoricalRestock_LaterInventoryDamaged_NoCompensation", func(t *testing.T) {
		env := setupDeductionTestEnv(t, 1, 700000, 700000, false)
		defer env.client.Close()

		var userID uuid.UUID
		err := env.client.Pool.QueryRow(ctx, "SELECT user_id FROM orders WHERE id = $1", env.orderID).Scan(&userID)
		require.NoError(t, err)
		var fulfillmentID uuid.UUID
		err = env.client.Pool.QueryRow(ctx, "SELECT id FROM order_fulfillments WHERE order_id = $1", env.orderID).Scan(&fulfillmentID)
		require.NoError(t, err)
		var variantID uuid.UUID
		err = env.client.Pool.QueryRow(ctx, "SELECT product_variant_id FROM order_items WHERE id = $1", env.orderItemID).Scan(&variantID)
		require.NoError(t, err)

		supplyID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
			VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
		`, supplyID, env.sellerID, "SUP-"+uuid.New().String()[:8])
		require.NoError(t, err)

		supplyItemID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
			VALUES ($1, $2, $3, 1, now(), now())
		`, supplyItemID, supplyID, variantID)
		require.NoError(t, err)

		invUnitID := uuid.New()
		unitCode := "ZMU-" + uuid.New().String()[:8]
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 1, 'shipped', now(), now())
		`, invUnitID, unitCode, variantID, supplyID, supplyItemID)
		require.NoError(t, err)

		allocID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at, created_at)
			VALUES ($1, $2, $3, now(), now())
		`, allocID, env.orderItemID, invUnitID)
		require.NoError(t, err)

		retID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'refunded', 'defective', now(), now())
		`, retID, env.orderID, fulfillmentID, userID)
		require.NoError(t, err)

		retItemID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity)
			VALUES ($1, $2, $3, 1, 1, 0, 0)
		`, retItemID, retID, env.orderItemID)
		require.NoError(t, err)

		// Historical return event disposition was restock
		unitID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, disposition, created_at, updated_at)
			VALUES ($1, $2, $3, 'restock', now(), now())
		`, unitID, retItemID, allocID)
		require.NoError(t, err)

		// Responsibility was resolved ZAMK / zamk_fulfillment_error
		rraID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO return_responsibility_allocations (id, return_item_id, order_item_allocation_id, quantity, status, responsible_party, reason_code, decision_source, decided_at, created_at, updated_at)
			VALUES ($1, $2, $3, 1, 'resolved', 'zamk', 'zamk_fulfillment_error', 'system', now(), now(), now())
		`, rraID, retItemID, allocID)
		require.NoError(t, err)

		// Sale Reversal processed
		tx, err := env.client.Pool.Begin(ctx)
		require.NoError(t, err)
		err = env.svc.ProcessReturnDeduction(ctx, tx, retID, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))

		// Later unrelated damage to inventory unit in warehouse (mutable inventory status change)
		_, err = env.client.Pool.Exec(ctx, "UPDATE inventory_units SET status = 'damaged' WHERE id = $1", invUnitID)
		require.NoError(t, err)

		// Reconcile
		tx2, err := env.client.Pool.Begin(ctx)
		require.NoError(t, err)
		c, err := env.repo.GetCompensableQuantityTx(ctx, tx2, env.orderItemID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), c, "Historical disposition was restock, so C must be 0 despite current inventory_units.status = damaged")

		err = env.svc.ReconcileReturnCompensationTx(ctx, tx2, env.orderItemID)
		require.NoError(t, err)
		require.NoError(t, tx2.Commit(ctx))

		comps := getCompensationForOrderItem(t, env)
		assert.Empty(t, comps, "No Seller Compensation when historical return was restocked")
		assert.Equal(t, int64(0), getNetCompensation(t, env))
	})

	t.Run("HistoricalDamaged_AllowedReason_Reversed_GivesCompensation", func(t *testing.T) {
		env := setupDeductionTestEnv(t, 1, 700000, 700000, false)
		defer env.client.Close()

		var userID uuid.UUID
		err := env.client.Pool.QueryRow(ctx, "SELECT user_id FROM orders WHERE id = $1", env.orderID).Scan(&userID)
		require.NoError(t, err)
		var fulfillmentID uuid.UUID
		err = env.client.Pool.QueryRow(ctx, "SELECT id FROM order_fulfillments WHERE order_id = $1", env.orderID).Scan(&fulfillmentID)
		require.NoError(t, err)
		var variantID uuid.UUID
		err = env.client.Pool.QueryRow(ctx, "SELECT product_variant_id FROM order_items WHERE id = $1", env.orderItemID).Scan(&variantID)
		require.NoError(t, err)

		supplyID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
			VALUES ($1, $2, 'completed', $3, 'pickup', now(), now())
		`, supplyID, env.sellerID, "SUP-"+uuid.New().String()[:8])
		require.NoError(t, err)

		supplyItemID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at)
			VALUES ($1, $2, $3, 1, now(), now())
		`, supplyItemID, supplyID, variantID)
		require.NoError(t, err)

		invUnitID := uuid.New()
		unitCode := "ZMU-" + uuid.New().String()[:8]
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 1, 'shipped', now(), now())
		`, invUnitID, unitCode, variantID, supplyID, supplyItemID)
		require.NoError(t, err)

		allocID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, picked_at, created_at)
			VALUES ($1, $2, $3, now(), now())
		`, allocID, env.orderItemID, invUnitID)
		require.NoError(t, err)

		retID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'refunded', 'defective', now(), now())
		`, retID, env.orderID, fulfillmentID, userID)
		require.NoError(t, err)

		retItemID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, accepted_quantity, damaged_quantity, rejected_quantity)
			VALUES ($1, $2, $3, 1, 0, 1, 0)
		`, retItemID, retID, env.orderItemID)
		require.NoError(t, err)

		// Historical return event disposition was damaged
		unitID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, disposition, created_at, updated_at)
			VALUES ($1, $2, $3, 'damaged', now(), now())
		`, unitID, retItemID, allocID)
		require.NoError(t, err)

		// Responsibility was resolved ZAMK / zamk_warehouse_damage
		rraID := uuid.New()
		_, err = env.client.Pool.Exec(ctx, `
			INSERT INTO return_responsibility_allocations (id, return_item_id, order_item_allocation_id, quantity, status, responsible_party, reason_code, decision_source, decided_at, created_at, updated_at)
			VALUES ($1, $2, $3, 1, 'resolved', 'zamk', 'zamk_warehouse_damage', 'system', now(), now(), now())
		`, rraID, retItemID, allocID)
		require.NoError(t, err)

		// Sale Reversal processed
		tx, err := env.client.Pool.Begin(ctx)
		require.NoError(t, err)
		err = env.svc.ProcessReturnDeduction(ctx, tx, retID, env.orderID, []ReturnItemDeduction{{OrderItemID: env.orderItemID, Quantity: 1}})
		require.NoError(t, err)

		c, err := env.repo.GetCompensableQuantityTx(ctx, tx, env.orderItemID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), c)

		err = env.svc.ReconcileReturnCompensationTx(ctx, tx, env.orderItemID)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))

		comps := getCompensationForOrderItem(t, env)
		require.Len(t, comps, 1)
		assert.Equal(t, int64(700000), comps[0])
		assert.Equal(t, int64(700000), getNetCompensation(t, env))
	})
}
