package payouts

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
)

// MANDATORY TEST A: Available Earning -> available = earning
func TestBalance_CaseA_AvailableEarning(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()

	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()

	adminID, sellerID := setupUserAndSeller(t, client)
	require.NoError(t, svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID))

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	require.NoError(t, svc.CreatePendingSalesForOrder(ctx, orderID))

	tx, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour)))
	require.NoError(t, tx.Commit(ctx))

	_, err = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)

	bal, err := svc.GetSellerBalance(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), bal.AvailableCents, "Available balance must equal earning")
	assert.Equal(t, int64(0), bal.FrozenCents, "Frozen balance must be 0")
	assert.Equal(t, int64(0), bal.PaidCents, "Paid cents must be 0")
}

// MANDATORY TEST B: Paid Payout -> available becomes 0
func TestBalance_CaseB_PaidPayout(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()

	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()

	adminID, sellerID := setupUserAndSeller(t, client)
	require.NoError(t, svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID))

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	require.NoError(t, svc.CreatePendingSalesForOrder(ctx, orderID))

	tx, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour)))
	require.NoError(t, tx.Commit(ctx))

	_, err = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)

	batch, err := svc.CreatePayoutBatchForSeller(ctx, sellerID)
	require.NoError(t, err)
	require.NoError(t, svc.ProcessPayoutBatch(ctx, batch.ID))

	bal, err := svc.GetSellerBalance(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal.AvailableCents, "Available balance must be 0 after payout")
	assert.Equal(t, int64(0), bal.FrozenCents, "Frozen balance must be 0")
	assert.Equal(t, int64(10000), bal.PaidCents, "Paid cents must reflect disbursed payout")
}

// MANDATORY TEST C: Paid payout history remains persisted/immutable
func TestBalance_CaseC_PaidPayoutHistoryPersisted(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()

	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()

	adminID, sellerID := setupUserAndSeller(t, client)
	require.NoError(t, svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID))

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	require.NoError(t, svc.CreatePendingSalesForOrder(ctx, orderID))

	tx, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour)))
	require.NoError(t, tx.Commit(ctx))

	_, err = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)

	batch, err := svc.CreatePayoutBatchForSeller(ctx, sellerID)
	require.NoError(t, err)
	require.NoError(t, svc.ProcessPayoutBatch(ctx, batch.ID))

	// Verify batch is immutable and status is paid
	persistedBatch, err := repo.GetPayoutBatch(ctx, batch.ID)
	require.NoError(t, err)
	require.NotNil(t, persistedBatch)
	assert.Equal(t, "paid", persistedBatch.Status)
	assert.Equal(t, int64(10000), persistedBatch.AmountCents)
	assert.NotNil(t, persistedBatch.ProcessedAt)

	// Verify ledger entries
	entries, count, err := repo.ListSellerLedger(ctx, sellerID, 100, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 2)

	var foundEarning, foundPayout bool
	for _, e := range entries {
		if e.Type == "seller_earning" {
			foundEarning = true
			assert.Equal(t, int64(10000), e.AmountCents)
			require.NotNil(t, e.PayoutBatchID)
			assert.Equal(t, batch.ID, *e.PayoutBatchID)
		}
		if e.Type == "payout" {
			foundPayout = true
			assert.Equal(t, int64(-10000), e.AmountCents)
			require.NotNil(t, e.PayoutBatchID)
			assert.Equal(t, batch.ID, *e.PayoutBatchID)
		}
	}
	assert.True(t, foundEarning, "seller_earning entry must persist")
	assert.True(t, foundPayout, "payout entry must persist")
}

// MANDATORY TEST D: Post-payout return -> available = -return adjustment magnitude (NOT -(payout + adjustment))
func TestBalance_CaseD_PostPayoutReturn(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()

	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()

	adminID, sellerID := setupUserAndSeller(t, client)
	require.NoError(t, svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID))

	orderID, customerID := setupTestOrderWithItem(t, client, sellerID, 10000)
	require.NoError(t, svc.CreatePendingSalesForOrder(ctx, orderID))

	tx, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour)))
	require.NoError(t, tx.Commit(ctx))

	_, err = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)

	batch, err := svc.CreatePayoutBatchForSeller(ctx, sellerID)
	require.NoError(t, err)
	require.NoError(t, svc.ProcessPayoutBatch(ctx, batch.ID))

	// Get order items for return deduction
	orderItems, err := ordersRepo.GetOrderItems(ctx, orderID)
	require.NoError(t, err)
	require.Len(t, orderItems, 1)

	// Customer returns the item post-payout
	beforeReturn := time.Now()
	returnID := uuid.New()

	var fulfillmentID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM order_fulfillments WHERE order_id = $1", orderID).Scan(&fulfillmentID)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'refunded', 'size_fit', now(), now())
	`, returnID, orderID, fulfillmentID, customerID)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, reason, condition, restock, created_at)
		VALUES ($1, $2, $3, $4, 'size_fit', 'new', true, now())
	`, uuid.New(), returnID, orderItems[0].ID, orderItems[0].Quantity)
	require.NoError(t, err)

	deductionTx, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = svc.ProcessReturnDeduction(ctx, deductionTx, returnID, orderID, []ReturnItemDeduction{
		{OrderItemID: orderItems[0].ID, Quantity: orderItems[0].Quantity},
	})
	require.NoError(t, err)
	require.NoError(t, deductionTx.Commit(ctx))
	afterReturn := time.Now()

	// Verify the adjustment ledger entry directly:
	var adjAmount int64
	var adjAvailableAt *time.Time
	var adjCreatedAt time.Time
	var adjMeta []byte
	err = client.Pool.QueryRow(ctx, `
		SELECT amount_cents, available_at, created_at, metadata
		FROM seller_ledger_entries
		WHERE seller_id = $1 AND type = 'adjustment'
	`, sellerID).Scan(&adjAmount, &adjAvailableAt, &adjCreatedAt, &adjMeta)
	require.NoError(t, err)

	// A. post-payout adjustment has truthful created_at
	// B. no arbitrary -1 second shift
	assert.Nil(t, adjAvailableAt, "available_at MUST be NULL for post-payout adjustment (no hold applies)")
	assert.True(t, !adjCreatedAt.Before(beforeReturn.Add(-100*time.Millisecond)) && !adjCreatedAt.After(afterReturn.Add(100*time.Millisecond)),
		"created_at must be truthful event time, not arbitrarily shifted into past")

	// C. immediate post-payout adjustment contributes to available balance immediately
	// F. PaidCents aggregate has positive documented magnitude
	// H. after post-payout return: available = -return adjustment, paid remains historical payout magnitude
	bal, err := svc.GetSellerBalance(ctx, sellerID)
	require.NoError(t, err)

	assert.Equal(t, int64(-10000), bal.AvailableCents, "Available balance must equal -return adjustment magnitude, NOT -(payout + adjustment)")
	assert.Equal(t, int64(10000), bal.PaidCents, "Paid cents must remain positive historical payout magnitude (10000)")
	assert.Equal(t, int64(-10000), bal.AdjustmentsCents, "Adjustments cents must be -10000")
	assert.Equal(t, int64(0), bal.FrozenCents, "Frozen balance must be 0")

	// E. payout ledger remains negative in database
	var payoutEntryAmount int64
	err = client.Pool.QueryRow(ctx, `SELECT amount_cents FROM seller_ledger_entries WHERE seller_id = $1 AND type = 'payout'`, sellerID).Scan(&payoutEntryAmount)
	require.NoError(t, err)
	assert.Equal(t, int64(-10000), payoutEntryAmount, "payout entry amount_cents must remain negative in the ledger table")

	// D. post-payout context remains post_payout in return read model
	var evalContext string
	err = client.Pool.QueryRow(ctx, `
		SELECT
			CASE
				WHEN adj.amount_cents IS NULL THEN NULL
				WHEN adj.reason = 'return_post_payout' THEN 'post_payout'
				WHEN COALESCE(adj.adjustment_available_at, se.available_at) IS NOT NULL
				     AND adj.adjusted_at < COALESCE(adj.adjustment_available_at, se.available_at) THEN 'hold'
				ELSE 'available'
			END
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		JOIN order_items oi ON oi.id = ri.order_item_id
		LEFT JOIN LATERAL (
			SELECT available_at, payout_batch_id
			FROM seller_ledger_entries
			WHERE order_item_id = oi.id AND type = 'seller_earning'
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) se ON true
		LEFT JOIN LATERAL (
			SELECT
				sle.amount_cents,
				sle.created_at AS adjusted_at,
				sle.metadata->>'reason' AS reason,
				sle.available_at AS adjustment_available_at
			FROM seller_ledger_entries sle
			WHERE sle.order_item_id = oi.id
			  AND sle.type = 'adjustment'
			  AND sle.metadata->>'return_id' = r.id::text
			  AND sle.metadata->>'reason' IN ('return_deduction', 'return_post_payout')
			  AND sle.amount_cents < 0
			ORDER BY sle.created_at DESC, sle.id DESC
			LIMIT 1
		) adj ON true
		WHERE r.id = $1
	`, returnID).Scan(&evalContext)
	require.NoError(t, err)
	assert.Equal(t, "post_payout", evalContext, "Return read model MUST evaluate context as post_payout")
}

// MANDATORY TEST E: Available-return case: earning + adjustment = 0
func TestBalance_CaseE_AvailableReturn(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()

	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()

	adminID, sellerID := setupUserAndSeller(t, client)
	require.NoError(t, svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID))

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	require.NoError(t, svc.CreatePendingSalesForOrder(ctx, orderID))

	tx, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour)))
	require.NoError(t, tx.Commit(ctx))

	_, err = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)

	orderItems, err := ordersRepo.GetOrderItems(ctx, orderID)
	require.NoError(t, err)
	require.Len(t, orderItems, 1)

	// Return before payout
	returnID := uuid.New()
	deductionTx, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = svc.ProcessReturnDeduction(ctx, deductionTx, returnID, orderID, []ReturnItemDeduction{
		{OrderItemID: orderItems[0].ID, Quantity: orderItems[0].Quantity},
	})
	require.NoError(t, err)
	require.NoError(t, deductionTx.Commit(ctx))

	bal, err := svc.GetSellerBalance(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal.AvailableCents, "Earning + Adjustment must cancel out to 0")
	assert.Equal(t, int64(0), bal.FrozenCents, "Frozen must be 0")
	assert.Equal(t, int64(0), bal.PaidCents, "Paid must be 0")
}

// MANDATORY TEST F: Frozen earning + frozen adjustment: frozen net = 0
func TestBalance_CaseF_FrozenEarningAndFrozenAdjustment(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()

	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()

	adminID, sellerID := setupUserAndSeller(t, client)
	require.NoError(t, svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID))

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	require.NoError(t, svc.CreatePendingSalesForOrder(ctx, orderID))

	// Delivered today: 14-day hold active
	tx, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now()))
	require.NoError(t, tx.Commit(ctx))

	orderItems, err := ordersRepo.GetOrderItems(ctx, orderID)
	require.NoError(t, err)
	require.Len(t, orderItems, 1)

	// Return during hold
	returnID := uuid.New()
	deductionTx, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = svc.ProcessReturnDeduction(ctx, deductionTx, returnID, orderID, []ReturnItemDeduction{
		{OrderItemID: orderItems[0].ID, Quantity: orderItems[0].Quantity},
	})
	require.NoError(t, err)
	require.NoError(t, deductionTx.Commit(ctx))

	bal, err := svc.GetSellerBalance(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal.AvailableCents, "Available must be 0")
	assert.Equal(t, int64(0), bal.FrozenCents, "Frozen net must be 0 (earning 10000 + adjustment -10000)")
}

// MANDATORY TEST G: Scheduled/processing payout: correct "in transit" semantics preserved
func TestBalance_CaseG_ScheduledPayoutInTransit(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()

	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()

	adminID, sellerID := setupUserAndSeller(t, client)
	require.NoError(t, svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID))

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	require.NoError(t, svc.CreatePendingSalesForOrder(ctx, orderID))

	tx, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour)))
	require.NoError(t, tx.Commit(ctx))

	_, err = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)

	// Payout is created (scheduled)
	batch, err := svc.CreatePayoutBatchForSeller(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, "scheduled", batch.Status)

	// Available balance must drop to 0 because funds have moved to scheduled batch ("in transit")
	bal, err := svc.GetSellerBalance(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal.AvailableCents, "Available must be 0 while batch is in transit")
	assert.Equal(t, int64(0), bal.PaidCents, "PaidCents must still be 0 before execution")

	// Payout list must show scheduled batch for in-transit calculation
	batches, totalCount, err := svc.ListSellerPayoutBatches(ctx, sellerID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, totalCount)
	assert.Equal(t, int64(10000), batches[0].AmountCents)
	assert.Equal(t, "scheduled", batches[0].Status)
}

// MANDATORY TEST H: Multiple orders aggregate correctly
func TestBalance_CaseH_MultipleOrdersAggregateCorrectly(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()

	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()

	adminID, sellerID := setupUserAndSeller(t, client)
	require.NoError(t, svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID))

	// Step 1: Order 1 (10 000) delivered 15d ago, unfrozen, paid out via batch
	order1, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	require.NoError(t, svc.CreatePendingSalesForOrder(ctx, order1))
	tx1, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.MarkOrderDeliveredTx(ctx, tx1, order1, time.Now().Add(-15*24*time.Hour)))
	require.NoError(t, tx1.Commit(ctx))
	_, err = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)
	batch1, err := svc.CreatePayoutBatchForSeller(ctx, sellerID)
	require.NoError(t, err)
	require.NoError(t, svc.ProcessPayoutBatch(ctx, batch1.ID))

	// Step 2: Order 2 (20 000) delivered 15d ago, unfrozen, customer returned before payout -> net 0
	order2, _ := setupTestOrderWithItem(t, client, sellerID, 20000)
	require.NoError(t, svc.CreatePendingSalesForOrder(ctx, order2))
	tx2, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.MarkOrderDeliveredTx(ctx, tx2, order2, time.Now().Add(-15*24*time.Hour)))
	require.NoError(t, tx2.Commit(ctx))
	_, err = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)
	items2, err := ordersRepo.GetOrderItems(ctx, order2)
	require.NoError(t, err)
	deductionTx2, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = svc.ProcessReturnDeduction(ctx, deductionTx2, uuid.New(), order2, []ReturnItemDeduction{
		{OrderItemID: items2[0].ID, Quantity: items2[0].Quantity},
	})
	require.NoError(t, err)
	require.NoError(t, deductionTx2.Commit(ctx))

	// Step 3: Order 3 (15 000) delivered 15d ago, unfrozen -> available 15 000
	order3, _ := setupTestOrderWithItem(t, client, sellerID, 15000)
	require.NoError(t, svc.CreatePendingSalesForOrder(ctx, order3))
	tx3, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.MarkOrderDeliveredTx(ctx, tx3, order3, time.Now().Add(-15*24*time.Hour)))
	require.NoError(t, tx3.Commit(ctx))
	_, err = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	require.NoError(t, err)

	// Step 4: Order 4 (8 000) delivered today -> frozen 8 000
	order4, _ := setupTestOrderWithItem(t, client, sellerID, 8000)
	require.NoError(t, svc.CreatePendingSalesForOrder(ctx, order4))
	tx4, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.MarkOrderDeliveredTx(ctx, tx4, order4, time.Now()))
	require.NoError(t, tx4.Commit(ctx))

	// Step 5: Post-payout return on Order 1 (-10 000)
	items1, err := ordersRepo.GetOrderItems(ctx, order1)
	require.NoError(t, err)
	deductionTx1, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = svc.ProcessReturnDeduction(ctx, deductionTx1, uuid.New(), order1, []ReturnItemDeduction{
		{OrderItemID: items1[0].ID, Quantity: items1[0].Quantity},
	})
	require.NoError(t, err)
	require.NoError(t, deductionTx1.Commit(ctx))

	// Step 6: Verify Aggregate State
	bal, err := svc.GetSellerBalance(ctx, sellerID)
	require.NoError(t, err)

	// Expected:
	// Available: Order 3 (15000) + Order 1 Return (-10000) = 5000
	// Frozen: Order 4 (8000)
	// Paid: Order 1 (10000)
	assert.Equal(t, int64(5000), bal.AvailableCents, "Available must correctly aggregate (15000 - 10000 = 5000)")
	assert.Equal(t, int64(8000), bal.FrozenCents, "Frozen must be 8000")
	assert.Equal(t, int64(10000), bal.PaidCents, "Paid must be 10000")
}
