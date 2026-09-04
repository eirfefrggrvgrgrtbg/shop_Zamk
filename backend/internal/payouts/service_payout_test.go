package payouts

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func setupTestDB(t *testing.T) *postgres.Client {
	dsn := testutil.GetTestDatabaseURL()
	ctx := context.Background()
	client, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)

	testutil.AssertTestDatabase(t, client.Pool)

	// Guarantee that canonical starter taxonomy required by other suites is present
	require.NoError(t, testutil.EnsureCanonicalStarterTaxonomy(ctx, client.Pool))

	// Truncate only payout financial tables
	_, err = client.Pool.Exec(ctx, "TRUNCATE seller_ledger_entries CASCADE")
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "TRUNCATE payout_batches CASCADE")
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "TRUNCATE seller_commission_rules CASCADE")
	require.NoError(t, err)

	// Clean up only hardcoded test fixtures from service_return_deduction_test
	_, _ = client.Pool.Exec(ctx, "DELETE FROM users WHERE email = 'test@example.com'")
	_, _ = client.Pool.Exec(ctx, "DELETE FROM categories WHERE slug = 'cat'")

	return client
}

func setupUserAndSeller(t *testing.T, client *postgres.Client) (uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	adminID := uuid.New()
	_, err := client.Pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role) VALUES ($1, 'admin', $2, 'hash', 'admin')", adminID, uuid.New().String()+"@admin.local")
	require.NoError(t, err)

	userID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role) VALUES ($1, 'test', $2, 'hash', 'seller')", userID, uuid.New().String()+"@zamk.local")
	require.NoError(t, err)

	sellerID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO sellers (id) VALUES ($1)", sellerID)
	require.NoError(t, err)
	return adminID, sellerID
}

func setupTestOrderWithItem(t *testing.T, client *postgres.Client, sellerID uuid.UUID, priceCents int64) (uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	
	customerID := uuid.New()
	_, err := client.Pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role) VALUES ($1, 'customer', $2, 'hash', 'customer')", customerID, customerID.String()+"@cust.local")
	require.NoError(t, err)

	catID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'cat', $2)", catID, catID.String())
	require.NoError(t, err)

	prodID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, description, status, price_cents) VALUES ($1, $2, $3, 'prod', $4, 'desc', 'published', $5)", prodID, sellerID, catID, prodID.String(), priceCents)
	require.NoError(t, err)

	varID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents) VALUES ($1, $2, 'sku', $3)", varID, prodID, priceCents)
	require.NoError(t, err)

	orderID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'awaiting_payment', $3, 'n', 'p', 'e', 'a')", orderID, customerID, priceCents)
	require.NoError(t, err)

	fullID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'paid')", fullID, orderID, sellerID)
	require.NoError(t, err)

	itemID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'prod', 'slug', $6, 1, $6, $7)", itemID, orderID, prodID, varID, sellerID, priceCents, fullID)
	require.NoError(t, err)

	return orderID, customerID
}

// 1. COMMISSION SNAPSHOT
func TestCommissionSnapshotDoesNotChange(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()

	adminID, sellerID := setupUserAndSeller(t, client)

	err := svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 850, Reason: "initial"}, adminID)
	require.NoError(t, err)

	orderID1, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	err = svc.CreatePendingSalesForOrder(ctx, orderID1)
	require.NoError(t, err)

	err = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 1200, Reason: "change"}, adminID)
	require.NoError(t, err)

	orderID2, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	err = svc.CreatePendingSalesForOrder(ctx, orderID2)
	require.NoError(t, err)

	entries, _, _ := repo.ListSellerLedger(ctx, sellerID, 100, 0)
	var earning1, earning2 SellerLedgerEntry
	for _, e := range entries {
		if e.Type == "seller_earning" {
			if *e.OrderID == orderID1 {
				earning1 = e
			} else if *e.OrderID == orderID2 {
				earning2 = e
			}
		}
	}
	require.Equal(t, int64(9150), earning1.AmountCents)
	require.Equal(t, int64(8800), earning2.AmountCents)

	fmt.Println("COMMISSION CHANGE DOES NOT ALTER HISTORY PASS")
	fmt.Println("COMMISSION SNAPSHOT PASS")
}

func TestCommissionEightPointFivePercent(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()

	adminID, sellerID := setupUserAndSeller(t, client)
	err := svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 850, Reason: "initial"}, adminID)
	require.NoError(t, err)

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	err = svc.CreatePendingSalesForOrder(ctx, orderID)
	require.NoError(t, err)

	entries, _, _ := repo.ListSellerLedger(ctx, sellerID, 10, 0)
	var comm, net, gross int64
	for _, e := range entries {
		if e.Type == "zamk_commission" {
			comm = e.AmountCents
		}
		if e.Type == "seller_earning" {
			net = e.AmountCents
		}
		if e.Type == "sale_gross" {
			gross = e.AmountCents
		}
	}
	require.Equal(t, int64(10000), gross)
	require.Equal(t, int64(-850), comm)
	require.Equal(t, int64(9150), net)
	fmt.Println("DECIMAL COMMISSION 8.5% PASS")
}

// 2. 14 DAY HOLD & EXACT 14-DAY BOUNDARY
func TestFourteenDayHold(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerID := setupUserAndSeller(t, client)
	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID)

	// A) Not delivered => unavailable (already tested by TestNotDeliveredNeverAvailable, but we can just use new orders)
	orderA, _ := setupTestOrderWithItem(t, client, sellerID, 1000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderA)

	// B) Delivered at NOW => unavailable
	orderB, _ := setupTestOrderWithItem(t, client, sellerID, 2000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderB)
	
	// C) Delivered at NOW - 13d 23h 59m => unavailable
	orderC, _ := setupTestOrderWithItem(t, client, sellerID, 3000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderC)
	
	// D) Delivered at NOW - 14d => available
	orderD, _ := setupTestOrderWithItem(t, client, sellerID, 4000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderD)

	now := time.Now()
	tx, _ := client.Pool.Begin(ctx)
	_ = svc.MarkOrderDeliveredTx(ctx, tx, orderB, now)
	_ = svc.MarkOrderDeliveredTx(ctx, tx, orderC, now.Add(-14*24*time.Hour).Add(1*time.Minute))
	_ = svc.MarkOrderDeliveredTx(ctx, tx, orderD, now.Add(-14*24*time.Hour))
	tx.Commit(ctx)
	
	_, _ = svc.MakeSellerFundsAvailable(ctx, now, 100)

	bal, _ := svc.GetSellerBalance(ctx, sellerID)
	// Only orderD (4000) should be available. A (1000), B (2000), C (3000) are frozen. Total frozen = 6000.
	require.Equal(t, int64(4000), bal.AvailableCents)
	require.Equal(t, int64(6000), bal.FrozenCents)
	
	entries, _, _ := repo.ListSellerLedger(ctx, sellerID, 100, 0)
	for _, e := range entries {
		if e.OrderID != nil && *e.OrderID == orderD && e.Type == "seller_earning" {
			expectedAvailableAt := now.Add(-14 * 24 * time.Hour).Add(14 * 24 * time.Hour)
			require.NotNil(t, e.AvailableAt)
			require.WithinDuration(t, expectedAvailableAt, *e.AvailableAt, time.Second)
		}
	}

	fmt.Println("14 DAY HOLD PASS")
}

func TestNotDeliveredNeverAvailable(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerID := setupUserAndSeller(t, client)
	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID)

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderID)

	_, _ = svc.MakeSellerFundsAvailable(ctx, time.Now().Add(1000*time.Hour), 100)
	bal, _ := svc.GetSellerBalance(ctx, sellerID)
	require.Equal(t, int64(0), bal.AvailableCents)
	require.Equal(t, int64(10000), bal.FrozenCents)
	fmt.Println("NOT DELIVERED REMAINS FROZEN PASS")
}

func TestCancelledNeverAvailable(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerID := setupUserAndSeller(t, client)
	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID)

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderID)

	tx, _ := client.Pool.Begin(ctx)
	_ = ordersRepo.SetOrderCancelledTx(ctx, tx, orderID)
	tx.Commit(ctx)

	_, _ = svc.MakeSellerFundsAvailable(ctx, time.Now().Add(1000*time.Hour), 100)
	bal, _ := svc.GetSellerBalance(ctx, sellerID)
	require.Equal(t, int64(0), bal.AvailableCents)
	require.Equal(t, int64(10000), bal.FrozenCents)
	fmt.Println("CANCELLED NEVER AVAILABLE PASS")
}

// 3. DOUBLE PAYOUT PREVENTION
func TestDoublePayoutConcurrent(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerID := setupUserAndSeller(t, client)
	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID)

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderID)
	
	tx, _ := client.Pool.Begin(ctx)
	_ = svc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour))
	tx.Commit(ctx)
	_, _ = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)

	var wg sync.WaitGroup
	var successCount int
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.CreatePayoutBatchForSeller(context.Background(), sellerID)
			mu.Lock()
			if err == nil {
				successCount++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	require.Equal(t, 1, successCount)
	
	batches, _, _ := svc.ListSellerPayoutBatches(ctx, sellerID, 10, 0)
	require.Equal(t, 1, len(batches))
	
	entries, _, _ := svc.ListSellerLedger(ctx, sellerID, 10, 0)
	for _, e := range entries {
		if e.Type == "seller_earning" {
			require.NotNil(t, e.PayoutBatchID)
		}
	}

	bal, _ := svc.GetSellerBalance(ctx, sellerID)
	require.Equal(t, int64(0), bal.AvailableCents)
	fmt.Println("DOUBLE PAYOUT PREVENTION PASS")
}

// 4. PAYOUT LIFECYCLE
func TestPayoutAvailableOnly(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerID := setupUserAndSeller(t, client)
	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID)

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderID)

	_, err := svc.CreatePayoutBatchForSeller(ctx, sellerID)
	require.Error(t, err)
	fmt.Println("PAYOUT AVAILABLE ONLY PASS")
}

func TestFrozenExcludedFromPayout(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerID := setupUserAndSeller(t, client)
	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID)

	order1, _ := setupTestOrderWithItem(t, client, sellerID, 9000)
	_ = svc.CreatePendingSalesForOrder(ctx, order1)
	tx, _ := client.Pool.Begin(ctx)
	_ = svc.MarkOrderDeliveredTx(ctx, tx, order1, time.Now().Add(-15*24*time.Hour))
	tx.Commit(ctx)
	
	order2, _ := setupTestOrderWithItem(t, client, sellerID, 6000)
	_ = svc.CreatePendingSalesForOrder(ctx, order2)
	
	_, _ = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)

	batch, err := svc.CreatePayoutBatchForSeller(ctx, sellerID)
	require.NoError(t, err)
	require.Equal(t, int64(9000), batch.AmountCents)
	
	entries, _, _ := svc.ListSellerLedger(ctx, sellerID, 100, 0)
	for _, e := range entries {
		if e.Type == "seller_earning" {
			if *e.OrderID == order1 {
				require.NotNil(t, e.PayoutBatchID)
			} else if *e.OrderID == order2 {
				require.Nil(t, e.PayoutBatchID)
			}
		}
	}
	fmt.Println("FROZEN EXCLUDED FROM PAYOUT PASS")
}

func TestPaidExcludedFromPayout(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerID := setupUserAndSeller(t, client)
	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID)

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderID)
	tx, _ := client.Pool.Begin(ctx)
	_ = svc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour))
	tx.Commit(ctx)
	_, _ = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)

	batch, err := svc.CreatePayoutBatchForSeller(ctx, sellerID)
	require.NoError(t, err)
	err = svc.ProcessPayoutBatch(ctx, batch.ID)
	require.NoError(t, err)

	_, err = svc.CreatePayoutBatchForSeller(ctx, sellerID)
	require.Error(t, err)
	fmt.Println("PAID EXCLUDED FROM PAYOUT PASS")
}

func TestHeldPayoutNotProcessed(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerID := setupUserAndSeller(t, client)
	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID)

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderID)
	tx, _ := client.Pool.Begin(ctx)
	_ = svc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour))
	tx.Commit(ctx)
	_, _ = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)

	batch, _ := svc.CreatePayoutBatchForSeller(ctx, sellerID)
	require.Equal(t, "scheduled", batch.Status)
	
	err := svc.HoldPayoutBatch(ctx, batch.ID)
	require.NoError(t, err)
	
	_ = svc.ProcessPayoutBatch(ctx, batch.ID)
	
	b2, _ := repo.GetPayoutBatch(ctx, batch.ID)
	require.Equal(t, "held", b2.Status)
	fmt.Println("HELD PAYOUT NOT PROCESSED PASS")
}

func TestFailedPayoutRetainsFunds(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerID := setupUserAndSeller(t, client)
	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID)

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderID)
	tx, _ := client.Pool.Begin(ctx)
	_ = svc.MarkOrderDeliveredTx(ctx, tx, orderID, time.Now().Add(-15*24*time.Hour))
	tx.Commit(ctx)
	_, _ = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)

	batch, _ := svc.CreatePayoutBatchForSeller(ctx, sellerID)
	bal, _ := svc.GetSellerBalance(ctx, sellerID)
	require.Equal(t, int64(0), bal.AvailableCents)
	
	err := svc.FailPayoutBatch(ctx, batch.ID)
	require.NoError(t, err)
	
	bal2, _ := svc.GetSellerBalance(ctx, sellerID)
	require.Equal(t, int64(10000), bal2.AvailableCents)
	
	b2, _ := repo.GetPayoutBatch(ctx, batch.ID)
	require.Equal(t, "failed", b2.Status)
	fmt.Println("FAILED PAYOUT RETAINS FUNDS PASS")
}

func TestLedgerFinancialAmountsImmutable(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerID := setupUserAndSeller(t, client)

	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 850, Reason: "initial"}, adminID)

	orderID, _ := setupTestOrderWithItem(t, client, sellerID, 10000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderID)

	entries, _, _ := repo.ListSellerLedger(ctx, sellerID, 10, 0)
	var originalNet int64
	var entryID uuid.UUID
	for _, e := range entries {
		if e.Type == "seller_earning" {
			originalNet = e.AmountCents
			entryID = e.ID
		}
	}

	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 1200, Reason: "x"}, adminID)

	tx2, _ := client.Pool.Begin(ctx)
	_ = svc.MarkOrderDeliveredTx(ctx, tx2, orderID, time.Now().Add(-15*24*time.Hour))
	tx2.Commit(ctx)
	_, _ = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)

	b, _ := svc.CreatePayoutBatchForSeller(ctx, sellerID)
	_ = svc.ProcessPayoutBatch(ctx, b.ID)

	entries2, _, _ := repo.ListSellerLedger(ctx, sellerID, 10, 0)
	var finalNet int64
	for _, e := range entries2 {
		if e.ID == entryID {
			finalNet = e.AmountCents
		}
	}

	require.Equal(t, originalNet, finalNet)
	fmt.Println("COMMISSION HISTORY IMMUTABLE PASS")
	fmt.Println("LEDGER FINANCIAL HISTORY IMMUTABLE PASS")
}

func TestSellerFinanceIsolation(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerA := setupUserAndSeller(t, client)
	_, sellerB := setupUserAndSeller(t, client)

	_ = svc.SetCommissionRate(ctx, sellerA, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID)
	_ = svc.SetCommissionRate(ctx, sellerB, AdminSellerCommissionRequest{RateBPS: 0, Reason: "initial"}, adminID)

	orderA, _ := setupTestOrderWithItem(t, client, sellerA, 9100)
	_ = svc.CreatePendingSalesForOrder(ctx, orderA)
	txA, _ := client.Pool.Begin(ctx)
	_ = svc.MarkOrderDeliveredTx(ctx, txA, orderA, time.Now().Add(-15*24*time.Hour))
	txA.Commit(ctx)
	
	orderB1, _ := setupTestOrderWithItem(t, client, sellerB, 10000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderB1)
	orderB2, _ := setupTestOrderWithItem(t, client, sellerB, 8200)
	_ = svc.CreatePendingSalesForOrder(ctx, orderB2)
	txB, _ := client.Pool.Begin(ctx)
	_ = svc.MarkOrderDeliveredTx(ctx, txB, orderB1, time.Now().Add(-15*24*time.Hour))
	_ = svc.MarkOrderDeliveredTx(ctx, txB, orderB2, time.Now().Add(-15*24*time.Hour))
	txB.Commit(ctx)
	
	_, _ = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)
	
	bBal, _ := svc.GetSellerBalance(ctx, sellerB)
	require.Equal(t, int64(18200), bBal.AvailableCents)
	
	aBal, _ := svc.GetSellerBalance(ctx, sellerA)
	require.Equal(t, int64(9100), aBal.AvailableCents)
	fmt.Println("SELLER BALANCE ISOLATION PASS")
	
	aLedger, _, _ := svc.ListSellerLedger(ctx, sellerA, 100, 0)
	for _, l := range aLedger {
		require.Equal(t, sellerA, l.SellerID)
	}
	fmt.Println("SELLER LEDGER ISOLATION PASS")
	
	_, _ = svc.CreatePayoutBatchForSeller(ctx, sellerA)
	_, _ = svc.CreatePayoutBatchForSeller(ctx, sellerB)
	
	aPayouts, _, _ := svc.ListSellerPayoutBatches(ctx, sellerA, 100, 0)
	for _, p := range aPayouts {
		require.Equal(t, sellerA, p.SellerID)
	}
	fmt.Println("SELLER PAYOUT ISOLATION PASS")
}

func TestRealSummaryFixture(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	repo := NewRepository(client.Pool)
	ordersRepo := orders.NewRepository(client.Pool)
	svc := NewService(repo, client, nil, ordersRepo, nil, nil)
	ctx := context.Background()
	adminID, sellerID := setupUserAndSeller(t, client)
	
	_ = svc.SetCommissionRate(ctx, sellerID, AdminSellerCommissionRequest{RateBPS: 800, Reason: "initial"}, adminID)

	orderA, _ := setupTestOrderWithItem(t, client, sellerID, 1000000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderA)
	tx, _ := client.Pool.Begin(ctx)
	_ = svc.MarkOrderDeliveredTx(ctx, tx, orderA, time.Now().Add(-15*24*time.Hour))
	tx.Commit(ctx)
	
	orderB, _ := setupTestOrderWithItem(t, client, sellerID, 500000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderB)
	
	orderC, _ := setupTestOrderWithItem(t, client, sellerID, 200000)
	_ = svc.CreatePendingSalesForOrder(ctx, orderC)
	
	_, _ = svc.MakeSellerFundsAvailable(ctx, time.Now(), 100)

	bal, _ := svc.GetSellerBalance(ctx, sellerID)
	require.Equal(t, int64(1700000), bal.GrossSalesCents)
	require.Equal(t, int64(-136000), bal.CommissionCents)
	require.Equal(t, int64(920000), bal.AvailableCents)
	require.Equal(t, int64(644000), bal.FrozenCents)
	
	fmt.Println("GROSS 1700000 PASS")
	fmt.Println("COMMISSION 136000 PASS")
	fmt.Println("AVAILABLE 920000 PASS")
	fmt.Println("FROZEN 644000 PASS")
}

