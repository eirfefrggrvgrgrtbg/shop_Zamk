package payouts

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type deductionTestEnv struct {
	client      *postgres.Client
	repo        *Repository
	svc         *Service
	sellerID    uuid.UUID
	orderID     uuid.UUID
	orderItemID uuid.UUID
}

func setupDeductionTestEnv(t *testing.T, qty int, priceCents int64, earningCents int64, isPaidOut bool) deductionTestEnv {
	t.Helper()
	client := setupTestDB(t)
	ctx := context.Background()

	suffix := uuid.NewString()[:8]
	sellerID := uuid.New()
	userID := uuid.New()

	_, err := client.Pool.Exec(ctx, "INSERT INTO users (id, name, phone, email, password_hash, role, created_at) VALUES ($1, 'Test', '+123', $2, 'hash', 'seller', now())", userID, fmt.Sprintf("test-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at) VALUES ($1, 'store', $2, $3, 'active', now())", sellerID, fmt.Sprintf("store-%s", suffix), fmt.Sprintf("test-%s@store.com", suffix))
	require.NoError(t, err)

	orderID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at) VALUES ($1, $2, 'delivered', $3, 'RUB', 'Test', '+1', 'a@b.c', 'Addr', now())", orderID, userID, priceCents*int64(qty))
	require.NoError(t, err)

	categoryID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'cat', $2)", categoryID, fmt.Sprintf("cat-%s", suffix))
	require.NoError(t, err)

	productID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, description, status, price_cents) VALUES ($1, $2, $3, 'P', $4, 'desc', 'published', $5)", productID, sellerID, categoryID, fmt.Sprintf("p-%s", suffix), priceCents)
	require.NoError(t, err)

	variantID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents) VALUES ($1, $2, $3, $4)", variantID, productID, fmt.Sprintf("sku-%s", suffix), priceCents)
	require.NoError(t, err)

	fulfillmentID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'delivered')", fulfillmentID, orderID, sellerID)
	require.NoError(t, err)

	orderItemID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, quantity, price_cents, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Title', 'slug', $7, $8, $9)
	`, orderItemID, orderID, fulfillmentID, productID, variantID, sellerID, qty, priceCents, priceCents*int64(qty))
	require.NoError(t, err)

	repo := NewRepository(client.Pool)
	mockOrders := &mockOrdersRepo{
		items: []orders.OrderItem{
			{
				ID:       orderItemID,
				OrderID:  orderID,
				SellerID: sellerID,
				Quantity: qty,
			},
		},
	}
	svc := NewService(repo, client, nil, mockOrders, nil, nil)

	var payoutBatchID *uuid.UUID
	if isPaidOut {
		pbID := uuid.New()
		_, err = client.Pool.Exec(ctx, `
			INSERT INTO payout_batches (id, seller_id, amount_cents, status, scheduled_for, processed_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'paid', now(), now(), now(), now())
		`, pbID, sellerID, earningCents)
		require.NoError(t, err)
		payoutBatchID = &pbID
	}

	availableAt := time.Now().AddDate(0, 0, 14).UTC()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, payout_batch_id, type, amount_cents, currency, available_at, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, 'seller_earning', $6, 'RUB', $7, '{}'::jsonb, now())
	`, uuid.New(), sellerID, orderID, orderItemID, payoutBatchID, earningCents, availableAt)
	require.NoError(t, err)

	return deductionTestEnv{
		client:      client,
		repo:        repo,
		svc:         svc,
		sellerID:    sellerID,
		orderID:     orderID,
		orderItemID: orderItemID,
	}
}

func getDeductionsForOrderItem(t *testing.T, client *postgres.Client, orderItemID uuid.UUID) []int64 {
	t.Helper()
	ctx := context.Background()
	rows, err := client.Pool.Query(ctx, `
		SELECT amount_cents
		FROM seller_ledger_entries
		WHERE order_item_id = $1 AND type = 'adjustment'
		ORDER BY created_at ASC, id ASC
	`, orderItemID)
	require.NoError(t, err)
	defer rows.Close()

	var deductions []int64
	for rows.Next() {
		var amt int64
		err := rows.Scan(&amt)
		require.NoError(t, err)
		deductions = append(deductions, -amt) // positive magnitude
	}
	return deductions
}

// Case A: E=700000, Q=3, single full return q=3 -> total=700000
func TestCumulativeRounding_CaseA_SingleFullReturn(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	tx, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	retID := uuid.New()
	err = env.svc.ProcessReturnDeduction(ctx, tx, retID, env.orderID, []ReturnItemDeduction{
		{OrderItemID: env.orderItemID, Quantity: 3},
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	deductions := getDeductionsForOrderItem(t, env.client, env.orderItemID)
	require.Len(t, deductions, 1)
	assert.Equal(t, int64(700000), deductions[0])
}

// Case B: E=700000, Q=3, returns 1 + 1 + 1 -> cumulative total = 700000
// Expected slices: 233333, 233333, 233334
func TestCumulativeRounding_CaseB_ThreeSequentialReturns_1_1_1(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		tx, err := env.client.Pool.Begin(ctx)
		require.NoError(t, err)

		retID := uuid.New()
		err = env.svc.ProcessReturnDeduction(ctx, tx, retID, env.orderID, []ReturnItemDeduction{
			{OrderItemID: env.orderItemID, Quantity: 1},
		})
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
	}

	deductions := getDeductionsForOrderItem(t, env.client, env.orderItemID)
	require.Len(t, deductions, 3)
	assert.Equal(t, int64(233333), deductions[0])
	assert.Equal(t, int64(233333), deductions[1])
	assert.Equal(t, int64(233334), deductions[2])

	var total int64
	for _, d := range deductions {
		total += d
	}
	assert.Equal(t, int64(700000), total, "Cumulative deduction across 1+1+1 must equal 700000 exactly")
}

// Case C: E=700000, Q=3, returns 2 + 1 -> cumulative total = 700000
// Expected slices: 466666, 233334
func TestCumulativeRounding_CaseC_Returns_2_plus_1(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	// Return 1: qty 2
	tx1, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx1, uuid.New(), env.orderID, []ReturnItemDeduction{
		{OrderItemID: env.orderItemID, Quantity: 2},
	})
	require.NoError(t, err)
	require.NoError(t, tx1.Commit(ctx))

	// Return 2: qty 1
	tx2, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx2, uuid.New(), env.orderID, []ReturnItemDeduction{
		{OrderItemID: env.orderItemID, Quantity: 1},
	})
	require.NoError(t, err)
	require.NoError(t, tx2.Commit(ctx))

	deductions := getDeductionsForOrderItem(t, env.client, env.orderItemID)
	require.Len(t, deductions, 2)
	assert.Equal(t, int64(466666), deductions[0])
	assert.Equal(t, int64(233334), deductions[1])
	assert.Equal(t, int64(700000), deductions[0]+deductions[1])
}

// Case D: E=700000, Q=3, returns 1 + 2 -> cumulative total = 700000
// Expected slices: 233333, 466667
func TestCumulativeRounding_CaseD_Returns_1_plus_2(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	// Return 1: qty 1
	tx1, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx1, uuid.New(), env.orderID, []ReturnItemDeduction{
		{OrderItemID: env.orderItemID, Quantity: 1},
	})
	require.NoError(t, err)
	require.NoError(t, tx1.Commit(ctx))

	// Return 2: qty 2
	tx2, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx2, uuid.New(), env.orderID, []ReturnItemDeduction{
		{OrderItemID: env.orderItemID, Quantity: 2},
	})
	require.NoError(t, err)
	require.NoError(t, tx2.Commit(ctx))

	deductions := getDeductionsForOrderItem(t, env.client, env.orderItemID)
	require.Len(t, deductions, 2)
	assert.Equal(t, int64(233333), deductions[0])
	assert.Equal(t, int64(466667), deductions[1])
	assert.Equal(t, int64(700000), deductions[0]+deductions[1])
}

// Case E: E=10000, Q=3, returns 1 + 1 + 1 -> cumulative total = 10000
// Expected slices: 3333, 3333, 3334
func TestCumulativeRounding_CaseE_SmallEarning_10000_ThreeReturns(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 5000, 10000, false)
	defer env.client.Close()
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		tx, err := env.client.Pool.Begin(ctx)
		require.NoError(t, err)

		retID := uuid.New()
		err = env.svc.ProcessReturnDeduction(ctx, tx, retID, env.orderID, []ReturnItemDeduction{
			{OrderItemID: env.orderItemID, Quantity: 1},
		})
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
	}

	deductions := getDeductionsForOrderItem(t, env.client, env.orderItemID)
	require.Len(t, deductions, 3)
	assert.Equal(t, int64(3333), deductions[0])
	assert.Equal(t, int64(3333), deductions[1])
	assert.Equal(t, int64(3334), deductions[2])

	var total int64
	for _, d := range deductions {
		total += d
	}
	assert.Equal(t, int64(10000), total)
}

// Case F: Partial only: first 1 of 3 -> deduction <= proportional target, no over-deduction
func TestCumulativeRounding_CaseF_PartialOnlyNoOverDeduction(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	tx, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx, uuid.New(), env.orderID, []ReturnItemDeduction{
		{OrderItemID: env.orderItemID, Quantity: 1},
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	deductions := getDeductionsForOrderItem(t, env.client, env.orderItemID)
	require.Len(t, deductions, 1)
	assert.Equal(t, int64(233333), deductions[0])
	// 233333 is <= floor(700000 / 3) and strictly less than 700000 / 3 (233333.33)
	assert.LessOrEqual(t, deductions[0], int64(700000/3))
}

// Case G: Two concurrent returns -> PostgreSQL row-level lock serializes them; final cumulative total is exact
func TestCumulativeRounding_CaseG_TwoConcurrentReturns(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Return 1: qty 1, Return 2: qty 2 (sum to 3)
	qtys := []int{1, 2}

	for _, q := range qtys {
		wg.Add(1)
		go func(qty int) {
			defer wg.Done()
			tx, err := env.client.Pool.Begin(ctx)
			if err != nil {
				errChan <- err
				return
			}
			defer tx.Rollback(ctx)

			retID := uuid.New()
			err = env.svc.ProcessReturnDeduction(ctx, tx, retID, env.orderID, []ReturnItemDeduction{
				{OrderItemID: env.orderItemID, Quantity: qty},
			})
			if err != nil {
				errChan <- err
				return
			}

			if err := tx.Commit(ctx); err != nil {
				errChan <- err
				return
			}
		}(q)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		require.NoError(t, err)
	}

	deductions := getDeductionsForOrderItem(t, env.client, env.orderItemID)
	require.Len(t, deductions, 2)
	assert.Equal(t, int64(700000), deductions[0]+deductions[1], "Concurrent partial returns must cumulatively deduct exactly 700000")
}

// Case H: Retry same return -> idempotent, no duplicate adjustment or extra deduction
func TestCumulativeRounding_CaseH_RetrySameReturn_Idempotent(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	retID := uuid.New()

	// Call 1
	tx1, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx1, retID, env.orderID, []ReturnItemDeduction{
		{OrderItemID: env.orderItemID, Quantity: 1},
	})
	require.NoError(t, err)
	require.NoError(t, tx1.Commit(ctx))

	// Call 2 with identical returnID
	tx2, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx2, retID, env.orderID, []ReturnItemDeduction{
		{OrderItemID: env.orderItemID, Quantity: 1},
	})
	require.NoError(t, err)
	require.NoError(t, tx2.Commit(ctx))

	deductions := getDeductionsForOrderItem(t, env.client, env.orderItemID)
	require.Len(t, deductions, 1, "Must contain exactly 1 adjustment entry after retry")
	assert.Equal(t, int64(233333), deductions[0])
}

// Case I: Cancelled/rejected return does not consume earning allocation
func TestCumulativeRounding_CaseI_CancelledOrRejectedReturnNoDeduction(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, false)
	defer env.client.Close()
	ctx := context.Background()

	// A return that is cancelled or rejected never calls ProcessReturnDeduction.
	// Therefore no ledger entry is created.
	// When an accepted return is later processed for qty 3, it receives full earning 700000.
	tx, err := env.client.Pool.Begin(ctx)
	require.NoError(t, err)
	err = env.svc.ProcessReturnDeduction(ctx, tx, uuid.New(), env.orderID, []ReturnItemDeduction{
		{OrderItemID: env.orderItemID, Quantity: 3},
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	deductions := getDeductionsForOrderItem(t, env.client, env.orderItemID)
	require.Len(t, deductions, 1)
	assert.Equal(t, int64(700000), deductions[0])
}

// Case J: Post_payout path uses SAME exact allocation semantics
// E=700000, Q=3, returns 1 + 1 + 1 under payoutBatchID != nil
// All adjustments have reason 'return_post_payout', available_at = nil, sum = 700000
func TestCumulativeRounding_CaseJ_PostPayoutCumulativeAllocation(t *testing.T) {
	env := setupDeductionTestEnv(t, 3, 333333, 700000, true)
	defer env.client.Close()
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		tx, err := env.client.Pool.Begin(ctx)
		require.NoError(t, err)

		retID := uuid.New()
		err = env.svc.ProcessReturnDeduction(ctx, tx, retID, env.orderID, []ReturnItemDeduction{
			{OrderItemID: env.orderItemID, Quantity: 1},
		})
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
	}

	// Verify entries in DB
	rows, err := env.client.Pool.Query(ctx, `
		SELECT amount_cents, available_at, metadata->>'reason'
		FROM seller_ledger_entries
		WHERE order_item_id = $1 AND type = 'adjustment'
		ORDER BY created_at ASC, id ASC
	`, env.orderItemID)
	require.NoError(t, err)
	defer rows.Close()

	var amounts []int64
	for rows.Next() {
		var amt int64
		var availAt *time.Time
		var reason string
		err := rows.Scan(&amt, &availAt, &reason)
		require.NoError(t, err)
		assert.Nil(t, availAt, "post_payout adjustment must have available_at == nil")
		assert.Equal(t, "return_post_payout", reason)
		amounts = append(amounts, -amt)
	}

	require.Len(t, amounts, 3)
	assert.Equal(t, int64(233333), amounts[0])
	assert.Equal(t, int64(233333), amounts[1])
	assert.Equal(t, int64(233334), amounts[2])
	assert.Equal(t, int64(700000), amounts[0]+amounts[1]+amounts[2], "Post-payout cumulative sum must equal 700000 exactly")
}
