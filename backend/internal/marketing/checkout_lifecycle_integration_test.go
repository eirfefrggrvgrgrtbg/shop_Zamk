package marketing_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

// TestMatrixA_ConcurrentGlobalLimit verifies that when 2 concurrent orders compete
// for the last global usage of a promo code, exactly 1 succeeds and 1 is rejected.
func TestMatrixA_ConcurrentGlobalLimit(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)
	limit := 1

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeSeller, marketing.DiscountTypePercent, 1000, 0, 0, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "GLOBAL1", marketing.DiscountTypePercent, 1000, 0, 0, &limit, 10, false)

	user1 := createTestUser(t, client, "u1")
	user2 := createTestUser(t, client, "u2")
	order1 := createTestOrder(t, client, user1)
	order2 := createTestOrder(t, client, user2)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user1, user2}, []uuid.UUID{camp.ID})

	item1ID, prod1ID, var1ID := createTestOrderItem(t, client, order1, sellerID, 100000, 1)
	item2ID, prod2ID, var2ID := createTestOrderItem(t, client, order2, sellerID, 100000, 1)

	items1 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        item1ID,
		ProductID:          prod1ID,
		ProductVariantID:   var1ID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}
	items2 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        item2ID,
		ProductID:          prod2ID,
		ProductVariantID:   var2ID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	var wg sync.WaitGroup
	var err1, err2 error
	startGate := make(chan struct{})

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-startGate
		_ = client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user1, promo.Code, items1, 900, time.Now().UTC())
			if err != nil {
				err1 = err
				return err
			}
			err1 = svc.ReserveCheckoutPromoTx(ctx, tx, order1, user1, calc, time.Now().UTC().Add(30*time.Minute))
			return err1
		})
	}()

	go func() {
		defer wg.Done()
		<-startGate
		_ = client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user2, promo.Code, items2, 900, time.Now().UTC())
			if err != nil {
				err2 = err
				return err
			}
			err2 = svc.ReserveCheckoutPromoTx(ctx, tx, order2, user2, calc, time.Now().UTC().Add(30*time.Minute))
			return err2
		})
	}()

	close(startGate)
	wg.Wait()

	successCount := 0
	if err1 == nil {
		successCount++
	}
	if err2 == nil {
		successCount++
	}
	assert.Equal(t, 1, successCount, "Exactly one order must succeed under global limit=1")

	if err1 != nil {
		assert.ErrorIs(t, err1, marketing.ErrPromoGlobalLimit)
	}
	if err2 != nil {
		assert.ErrorIs(t, err2, marketing.ErrPromoGlobalLimit)
	}
}

// TestMatrixB_ConcurrentCustomerLimit verifies that when the same customer concurrently
// places two orders with per-customer limit=1, exactly 1 succeeds.
func TestMatrixB_ConcurrentCustomerLimit(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeSeller, marketing.DiscountTypePercent, 1000, 0, 0, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "CUSTLIMIT1", marketing.DiscountTypePercent, 1000, 0, 0, nil, 1, false)

	user := createTestUser(t, client, "cust1")
	order1 := createTestOrder(t, client, user)
	order2 := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	item1ID, prod1ID, var1ID := createTestOrderItem(t, client, order1, sellerID, 100000, 1)
	item2ID, prod2ID, var2ID := createTestOrderItem(t, client, order2, sellerID, 100000, 1)

	items1 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        item1ID,
		ProductID:          prod1ID,
		ProductVariantID:   var1ID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}
	items2 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        item2ID,
		ProductID:          prod2ID,
		ProductVariantID:   var2ID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	var wg sync.WaitGroup
	var err1, err2 error
	startGate := make(chan struct{})

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-startGate
		_ = client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items1, 900, time.Now().UTC())
			if err != nil {
				err1 = err
				return err
			}
			err1 = svc.ReserveCheckoutPromoTx(ctx, tx, order1, user, calc, time.Now().UTC().Add(30*time.Minute))
			return err1
		})
	}()

	go func() {
		defer wg.Done()
		<-startGate
		_ = client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items2, 900, time.Now().UTC())
			if err != nil {
				err2 = err
				return err
			}
			err2 = svc.ReserveCheckoutPromoTx(ctx, tx, order2, user, calc, time.Now().UTC().Add(30*time.Minute))
			return err2
		})
	}()

	close(startGate)
	wg.Wait()

	successCount := 0
	if err1 == nil {
		successCount++
	}
	if err2 == nil {
		successCount++
	}
	assert.Equal(t, 1, successCount, "Exactly one order must succeed under per-customer limit=1")
	if err1 != nil {
		assert.ErrorIs(t, err1, marketing.ErrPromoCustomerLimit)
	}
	if err2 != nil {
		assert.ErrorIs(t, err2, marketing.ErrPromoCustomerLimit)
	}
}

// TestMatrixC_ConcurrentPlatformBudgetContention verifies that when remaining ZAMK budget
// is sufficient for only 1 order, exactly 1 order succeeds in reservation.
func TestMatrixC_ConcurrentPlatformBudgetContention(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	// Budget cap: 600 RUB (60,000 cents). Subsidy per order of 5000 RUB @ 10% is 500 RUB (50,000 cents).
	// Two orders would require 100,000 cents, so only 1 can succeed!
	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 60000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "BUDGETCAP1", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user1 := createTestUser(t, client, "b1")
	user2 := createTestUser(t, client, "b2")
	order1 := createTestOrder(t, client, user1)
	order2 := createTestOrder(t, client, user2)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user1, user2}, []uuid.UUID{camp.ID})

	item1ID, prod1ID, var1ID := createTestOrderItem(t, client, order1, sellerID, 500000, 1)
	item2ID, prod2ID, var2ID := createTestOrderItem(t, client, order2, sellerID, 500000, 1)

	items1 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        item1ID,
		ProductID:          prod1ID,
		ProductVariantID:   var1ID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}
	items2 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        item2ID,
		ProductID:          prod2ID,
		ProductVariantID:   var2ID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	var wg sync.WaitGroup
	var err1, err2 error
	startGate := make(chan struct{})

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-startGate
		_ = client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user1, promo.Code, items1, 900, time.Now().UTC())
			if err != nil {
				err1 = err
				return err
			}
			err1 = svc.ReserveCheckoutPromoTx(ctx, tx, order1, user1, calc, time.Now().UTC().Add(30*time.Minute))
			return err1
		})
	}()

	go func() {
		defer wg.Done()
		<-startGate
		_ = client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user2, promo.Code, items2, 900, time.Now().UTC())
			if err != nil {
				err2 = err
				return err
			}
			err2 = svc.ReserveCheckoutPromoTx(ctx, tx, order2, user2, calc, time.Now().UTC().Add(30*time.Minute))
			return err2
		})
	}()

	close(startGate)
	wg.Wait()

	successCount := 0
	if err1 == nil {
		successCount++
	}
	if err2 == nil {
		successCount++
	}
	assert.Equal(t, 1, successCount, "Exactly one order must succeed under budget contention")
	if err1 != nil {
		assert.ErrorIs(t, err1, marketing.ErrPromoBudgetExhausted)
	}
	if err2 != nil {
		assert.ErrorIs(t, err2, marketing.ErrPromoBudgetExhausted)
	}
}

// TestMatrixD_DuplicatePaymentSuccessWebhook verifies that duplicate payment success webhooks
// consume the promo reservation exactly once and never double-spend budget.
func TestMatrixD_DuplicatePaymentSuccessWebhook(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 200000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "DUPWEBHOOK1", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "dupu")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	// Reserve
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// First payment confirmation
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, orderID)
	})
	require.NoError(t, err)

	updatedCamp, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), updatedCamp.ZamkReservedCents)
	assert.Equal(t, int64(50000), updatedCamp.ZamkSpentCents)

	// Duplicate payment confirmation
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, orderID)
	})
	require.NoError(t, err, "Duplicate webhook must be idempotent no-op")

	dupCamp, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), dupCamp.ZamkReservedCents)
	assert.Equal(t, int64(50000), dupCamp.ZamkSpentCents, "Spent budget must not double-count")
}

// TestMatrixE_DuplicatePaymentFailureWebhook verifies that duplicate payment failure/rejection webhooks
// release the promo reservation exactly once.
func TestMatrixE_DuplicatePaymentFailureWebhook(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 200000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "DUPFAIL1", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "dupfailu")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	// Reserve
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// First failure release
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, orderID, "payment_failed")
	})
	require.NoError(t, err)

	updatedCamp, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), updatedCamp.ZamkReservedCents)
	assert.Equal(t, int64(0), updatedCamp.ZamkSpentCents)

	// Duplicate failure release
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, orderID, "payment_failed")
	})
	require.NoError(t, err, "Duplicate release must be idempotent no-op")

	dupCamp, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), dupCamp.ZamkReservedCents)
	assert.Equal(t, int64(0), dupCamp.ZamkSpentCents)
}

// TestMatrixF_SuccessVsFailureRace verifies that once an order is consumed upon payment success,
// any subsequent or late failure/cancellation release never reverses the consumed state.
func TestMatrixF_SuccessVsFailureRace(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 200000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RACECONS1", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "raceu")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	// Reserve
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Payment succeeded
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, orderID)
	})
	require.NoError(t, err)

	// Late cancellation or failure attempt
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, orderID, "late_cancellation")
	})
	require.NoError(t, err)

	// Verify status remains consumed and spent budget remains intact
	finalCamp, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), finalCamp.ZamkSpentCents)
	assert.Equal(t, int64(0), finalCamp.ZamkReservedCents)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)
}

// TestMatrixG_ConcurrentFirstOrderPromo verifies that when a new customer concurrently
// places two orders using a first-order-only promo, only 1 can acquire the first-order benefit.
func TestMatrixG_ConcurrentFirstOrderPromo(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeSeller, marketing.DiscountTypePercent, 1000, 0, 0, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "FIRSTORDER1", marketing.DiscountTypePercent, 1000, 0, 0, nil, 5, true)

	user := createTestUser(t, client, "newcust")
	order1 := createTestOrder(t, client, user)
	order2 := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	item1ID, prod1ID, var1ID := createTestOrderItem(t, client, order1, sellerID, 100000, 1)
	item2ID, prod2ID, var2ID := createTestOrderItem(t, client, order2, sellerID, 100000, 1)

	items1 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        item1ID,
		ProductID:          prod1ID,
		ProductVariantID:   var1ID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}
	items2 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        item2ID,
		ProductID:          prod2ID,
		ProductVariantID:   var2ID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	var wg sync.WaitGroup
	var err1, err2 error
	startGate := make(chan struct{})

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-startGate
		_ = client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items1, 900, time.Now().UTC())
			if err != nil {
				err1 = err
				return err
			}
			err1 = svc.ReserveCheckoutPromoTx(ctx, tx, order1, user, calc, time.Now().UTC().Add(30*time.Minute))
			return err1
		})
	}()

	go func() {
		defer wg.Done()
		<-startGate
		_ = client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items2, 900, time.Now().UTC())
			if err != nil {
				err2 = err
				return err
			}
			err2 = svc.ReserveCheckoutPromoTx(ctx, tx, order2, user, calc, time.Now().UTC().Add(30*time.Minute))
			return err2
		})
	}()

	close(startGate)
	wg.Wait()

	successCount := 0
	if err1 == nil {
		successCount++
	}
	if err2 == nil {
		successCount++
	}
	assert.Equal(t, 1, successCount, "Exactly one order must acquire first-order reservation")
}

// TestMatrixH_ReleasedUsageCannotDoubleDecrementBudget verifies that released usage
// cannot later double-decrement or leak campaign budget.
func TestMatrixH_ReleasedUsageCannotDoubleDecrementBudget(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 200000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "DOUBLEDEC1", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "doubledec")
	promoOrder := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, promoOrder, sellerID, 500000, 1)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	// 1. Reserve promo on promoOrder (reserves 50000 cents ZAMK budget)
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, promoOrder, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	campReserved, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campReserved.ZamkReservedCents)

	// 2. Initial release upon payment failure
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, promoOrder, "payment_failed")
	})
	require.NoError(t, err)

	campReleased, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campReleased.ZamkReservedCents)
	assert.Equal(t, int64(0), campReleased.ZamkSpentCents)

	// 3. Second release call must be idempotent and must NOT decrement budget below 0
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, promoOrder, "payment_failed")
	})
	require.NoError(t, err, "Duplicate release must be idempotent no-op")

	campAfterDup, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfterDup.ZamkReservedCents, "Reserved budget must not double-decrement")
	assert.Equal(t, int64(0), campAfterDup.ZamkSpentCents, "Spent budget must remain 0")

	// 4. Attempting to consume released usage must fail and NOT decrement budget
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, promoOrder)
	})
	assert.ErrorIs(t, err, marketing.ErrPromoUsageNotReserved, "Cannot consume released usage")

	campFinal, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campFinal.ZamkReservedCents)
	assert.Equal(t, int64(0), campFinal.ZamkSpentCents)
}

// TestMatrixI_SellerPromoNeverMutatesZamkBudget verifies that a purely seller-funded promotion
// never reserves or spends ZAMK platform budget.
func TestMatrixI_SellerPromoNeverMutatesZamkBudget(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeSeller, marketing.DiscountTypePercent, 2000, 0, 0, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "SELLERONLY1", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "sellerp")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(0), calc.TotalZamkSubsidyCents)
		assert.Equal(t, int64(100000), calc.TotalSellerDiscountCents)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	campAfterRes, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfterRes.ZamkReservedCents)
	assert.Equal(t, int64(0), campAfterRes.ZamkSpentCents)

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, orderID)
	})
	require.NoError(t, err)

	campAfterCons, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfterCons.ZamkReservedCents)
	assert.Equal(t, int64(0), campAfterCons.ZamkSpentCents)
}

// TestMatrixJ_CofundedCanonical5000Example verifies the exact financial split of the canonical
// 5000 RUB / 15% seller / 10% ZAMK / 1500 bps commission example.
func TestMatrixJ_CofundedCanonical5000Example(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1500, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "CANON5000", marketing.DiscountTypePercent, 2500, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "canonuser")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000, // 5000 RUB
		Quantity:           1,
	}}

	var calc *marketing.CheckoutPromoCalculation
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		res, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 1500, time.Now().UTC())
		require.NoError(t, err)
		calc = res
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, res, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Verification of canonical values
	assert.Equal(t, int64(75000), calc.TotalSellerDiscountCents, "Seller discount: 750 RUB")
	assert.Equal(t, int64(50000), calc.TotalZamkSubsidyCents, "ZAMK subsidy: 500 RUB")
	assert.Equal(t, int64(375000), calc.TotalCustomerPaidCents, "Customer paid: 3750 RUB")
	assert.Equal(t, int64(125000), calc.TotalOrderDiscountCents, "Total discount: 1250 RUB")

	require.Len(t, calc.PromotedLines, 1)
	line := calc.PromotedLines[0]
	assert.Equal(t, int64(425000), line.TotalCommissionBaseCents, "Commission base: 4250 RUB")
	assert.Equal(t, int64(63750), line.TotalCommissionChargedCents, "Commission charged @ 15%: 637.5 RUB")

	// Persisted database financial truth
	var dbBase, dbSellerDisc, dbZamkSubsidy, dbCustPaid, dbCommBase int64
	err = client.Pool.QueryRow(ctx, `
		SELECT base_unit_price_cents, total_seller_discount_cents, total_zamk_subsidy_cents, total_customer_paid_cents, total_commission_base_cents
		FROM order_item_promotions WHERE order_item_id = $1
	`, itemID).Scan(&dbBase, &dbSellerDisc, &dbZamkSubsidy, &dbCustPaid, &dbCommBase)
	require.NoError(t, err)
	assert.Equal(t, int64(500000), dbBase, "Persisted base: 500000")
	assert.Equal(t, int64(75000), dbSellerDisc, "Persisted seller discount: 75000")
	assert.Equal(t, int64(50000), dbZamkSubsidy, "Persisted ZAMK subsidy: 50000")
	assert.Equal(t, int64(375000), dbCustPaid, "Persisted customer paid: 375000")
	assert.Equal(t, int64(425000), dbCommBase, "Persisted commission base: 425000")

	// Explicit proof that ZAMK subsidy does NOT reduce commission base
	assert.Equal(t, dbBase-dbSellerDisc, dbCommBase, "Commission base is strictly base minus seller discount")
	assert.NotEqual(t, dbBase-dbSellerDisc-dbZamkSubsidy, dbCommBase, "ZAMK subsidy MUST NOT reduce commission base")
}

// TestMatrixK_MultiSellerCart verifies that in a multi-seller cart, promotional discounts
// are strictly applied to items belonging to the promotion's seller and unrelated lines are untouched.
func TestMatrixK_MultiSellerCart(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	seller1 := createTestSeller(t, client)
	seller2 := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, seller1, staffID, marketing.FundingModeSeller, marketing.DiscountTypePercent, 2000, 0, 0, 0)
	promo := createPromoWithLimits(t, svc, seller1, camp.ID, "SELLER1ONLY", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "multicart")
	defer cleanupMarketingFixtures(client, []uuid.UUID{seller1, seller2}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	items := []marketing.PromotedOrderItemInput{
		{
			OrderItemID:        uuid.New(),
			ProductID:          uuid.New(),
			ProductVariantID:   uuid.New(),
			SellerID:           seller1,
			BaseUnitPriceCents: 100000,
			Quantity:           2,
		},
		{
			OrderItemID:        uuid.New(),
			ProductID:          uuid.New(),
			ProductVariantID:   uuid.New(),
			SellerID:           seller2,
			BaseUnitPriceCents: 80000,
			Quantity:           1,
		},
	}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)

		// Promoted lines must contain ONLY seller1's items
		require.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, seller1, calc.PromotedLines[0].SellerID)
		assert.Equal(t, int64(40000), calc.TotalSellerDiscountCents, "20% off 2 * 100000 = 40000")
		assert.Equal(t, int64(160000), calc.TotalCustomerPaidCents)
		return nil
	})
	require.NoError(t, err)
}

// TestMatrixL_SellerFixedCentRemainder verifies that a seller fixed discount allocated across
// quantity > 1 distributes remainders cent-exactly without loss.
func TestMatrixL_SellerFixedCentRemainder(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	// Fixed discount 100 RUB (10,000 cents)
	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeSeller, marketing.DiscountTypeFixed, 0, 0, 0, 10000)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "FIXED100", marketing.DiscountTypeFixed, 0, 10000, 0, nil, 5, false)

	user := createTestUser(t, client, "fixedu")
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	// 1 item with quantity = 3, base price 5000 RUB each
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        uuid.New(),
		ProductID:          uuid.New(),
		ProductVariantID:   uuid.New(),
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           3,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)

		assert.Equal(t, int64(10000), calc.TotalSellerDiscountCents, "Total fixed discount must be exact 10,000 cents")
		assert.Equal(t, int64(1500000-10000), calc.TotalCustomerPaidCents, "Customer paid must be exact 1,490,000 cents")

		require.Len(t, calc.PromotedLines, 1)
		line := calc.PromotedLines[0]
		assert.Equal(t, int64(10000), line.TotalSellerDiscountCents)
		assert.Equal(t, int64(1500000-10000), line.TotalCustomerPaidCents)
		assert.Equal(t, int64(1500000-10000), line.TotalCommissionBaseCents)
		return nil
	})
	require.NoError(t, err)
}

// TestMatrixM_PaymentInitializationFailure verifies that when payment provider fails,
// the promo reservation is safely released and available again.
func TestMatrixM_PaymentInitializationFailure(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 200000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "PAYFAIL1", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "payfailu")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	// Reserve
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Simulate payment initialization failure: release promo
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, orderID, "payment_failed")
	})
	require.NoError(t, err)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)
}

// TestMatrixN_TransactionRollbackDuringReservation verifies that if order transaction rolls back,
// no orphan promo_code_usages exist and platform budget is not leaked.
func TestMatrixN_TransactionRollbackDuringReservation(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 200000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "ROLLBACK1", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "rollu")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	// Transaction attempts to reserve, then encounters error and rolls back
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		if err := svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute)); err != nil {
			return err
		}
		// Simulated downstream error (e.g. inventory conflict)
		return fmt.Errorf("simulated downstream error causing rollback")
	})
	require.Error(t, err)

	// Verify no usage record exists
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Nil(t, usage, "Usage record must not exist after rollback")

	// Verify budget was not locked
	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)
}

// TestMatrixO_CampaignDisabledAfterReservation verifies that if a campaign is disabled
// after order reservation, the already reserved order still proceeds to payment and consumes snapshot.
func TestMatrixO_CampaignDisabledAfterReservation(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 200000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "DISABLEDAFTER1", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "disafteru")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	// 1. Reserve while campaign is active
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// 2. Campaign is cancelled or deactivated subsequently
	_, err = client.Pool.Exec(ctx, "UPDATE marketing_campaigns SET status = 'cancelled' WHERE id = $1", camp.ID)
	require.NoError(t, err)

	// 3. Existing reserved order confirms payment
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, orderID)
	})
	require.NoError(t, err, "Existing reserved order must succeed in consuming hold even if campaign was cancelled")

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)
}

// ---------------------------------------------------------------------
// Test Helpers
// ---------------------------------------------------------------------

func createTestOrderItem(t *testing.T, client *postgres.Client, orderID, sellerID uuid.UUID, priceCents int64, quantity int) (uuid.UUID, uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	catID := uuid.New()
	_, err := client.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Cat', $2, now(), now())`, catID, uuid.New().String())
	require.NoError(t, err)

	prodID := uuid.New()
	_, err = client.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, status, created_at, updated_at) VALUES ($1, $2, $3, 'Prod', $4, $5, 'published', now(), now())`, prodID, sellerID, catID, uuid.New().String(), priceCents)
	require.NoError(t, err)

	variantID := uuid.New()
	barcode := "BARCODE-" + variantID.String()[:8]
	_, err = client.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, true, now(), now())`, variantID, prodID, "SKU-"+variantID.String()[:8], "SSKU-"+variantID.String()[:8], barcode, priceCents)
	require.NoError(t, err)

	fulfillmentID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
		VALUES ($1, $2, $3, 'awaiting_payment', $4, 1500, $4, now(), now())
	`, fulfillmentID, orderID, sellerID, priceCents*int64(quantity))
	require.NoError(t, err)

	itemID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'Item', 'slug', $7, $8, $9, now())
	`, itemID, orderID, fulfillmentID, prodID, variantID, sellerID, priceCents, quantity, priceCents*int64(quantity))
	require.NoError(t, err)

	invID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 100, 10, now(), now())
	`, invID, prodID, variantID, sellerID)
	require.NoError(t, err)

	resID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, quantity, status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, 'active', now() + interval '30 minutes', now())
	`, resID, invID, prodID, variantID, quantity)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_reservations (id, order_id, reservation_id, created_at)
		VALUES ($1, $2, $3, now())
	`, uuid.New(), orderID, resID)
	require.NoError(t, err)

	return itemID, prodID, variantID
}

func createTestUser(t *testing.T, client *postgres.Client, prefix string) uuid.UUID {
	ctx := context.Background()
	userID := uuid.New()
	query := `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Customer', $2, 'hash', 'customer', 'active', now(), now())
	`
	email := fmt.Sprintf("%s-%s@customer.test", prefix, userID.String()[:8])
	_, err := client.Pool.Exec(ctx, query, userID, email)
	require.NoError(t, err)
	return userID
}

func createApprovedCampaign(
	t *testing.T,
	repo *marketing.Repository,
	sellerID, staffID uuid.UUID,
	mode marketing.FundingMode,
	discType marketing.DiscountType,
	sellerDiscountBps int,
	zamkShareBps int,
	zamkBudgetCap int64,
	fixedDiscount int64,
) *marketing.MarketingCampaign {
	ctx := context.Background()
	c := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Campaign " + uuid.New().String()[:6],
		FundingMode:                 mode,
		Status:                      marketing.CampaignStatusActive,
		DiscountType:                discType,
		SellerDiscountBps:           sellerDiscountBps,
		SellerDiscountFixedCents:    fixedDiscount,
		RequestedZamkShareBps:       zamkShareBps,
		RequestedZamkBudgetCapCents: zamkBudgetCap,
		ApprovedZamkShareBps:        zamkShareBps,
		ApprovedZamkBudgetCapCents:  zamkBudgetCap,
		StartsAt:                    timePtr(time.Now().UTC().Add(-1 * time.Hour)),
		EndsAt:                      timePtr(time.Now().UTC().Add(48 * time.Hour)),
		CreatedAt:                   time.Now().UTC(),
		UpdatedAt:                   time.Now().UTC(),
	}
	err := repo.CreateCampaign(ctx, c)
	require.NoError(t, err)
	return c
}

func createPromoWithLimits(
	t *testing.T,
	svc *marketing.Service,
	sellerID, campaignID uuid.UUID,
	code string,
	discType marketing.DiscountType,
	bps int,
	fixed int64,
	minSubtotal int64,
	globalLimit *int,
	perCustomerLimit int,
	firstOrderOnly bool,
) *marketing.PromoCode {
	ctx := context.Background()
	p, err := svc.CreatePromoCode(ctx, sellerID, marketing.CreatePromoCodeRequest{
		CampaignID:              campaignID,
		Code:                    code,
		DiscountType:            discType,
		DiscountValueBps:        bps,
		DiscountValueFixedCents: fixed,
		MinOrderSubtotalCents:   minSubtotal,
		GlobalUsageLimit:        globalLimit,
		PerCustomerUsageLimit:   perCustomerLimit,
		FirstPaidOrderOnly:      firstOrderOnly,
		StartsAt:                timePtr(time.Now().UTC().Add(-1 * time.Hour)),
		EndsAt:                  timePtr(time.Now().UTC().Add(48 * time.Hour)),
	})
	require.NoError(t, err)
	return p
}

func timePtr(t time.Time) *time.Time {
	return &t
}

type testProviderMock struct {
	createPaymentFn func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error)
	verifyWebhookFn func(ctx context.Context, headers map[string]string, body []byte) error
	parseWebhookFn  func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error)
	modeFn          func(method string) string
	checkOrderFn    func(ctx context.Context, orderID string) (payments.CheckOrderResult, error)
}

func (m *testProviderMock) CreatePayment(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
	if m.createPaymentFn != nil {
		return m.createPaymentFn(ctx, input)
	}
	return payments.ProviderCreatePaymentResult{
		ProviderPaymentID: "mock-prov-" + uuid.NewString(),
		PaymentURL:        "https://mock.tbank.pay/" + uuid.NewString(),
		Status:            "NEW",
	}, nil
}

func (m *testProviderMock) VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) error {
	if m.verifyWebhookFn != nil {
		return m.verifyWebhookFn(ctx, headers, body)
	}
	return nil
}

func (m *testProviderMock) ParseWebhook(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
	if m.parseWebhookFn != nil {
		return m.parseWebhookFn(ctx, body)
	}
	evtKey := string(body)
	if evtKey == "" {
		evtKey = uuid.NewString()
	}
	status := "succeeded"
	provStatus := "CONFIRMED"
	if string(body) == "AUTH_FAIL" {
		status = "pending"
		provStatus = "AUTH_FAIL"
	} else if string(body) == "AUTHORIZED" {
		status = "pending"
		provStatus = "AUTHORIZED"
	} else if string(body) == "REFUNDED" {
		status = "refunded"
		provStatus = "REFUNDED"
	} else if string(body) == "REJECTED" || string(body) == "CANCELED" || string(body) == "DEADLINE_EXPIRED" {
		status = "cancelled"
		provStatus = string(body)
	}
	return payments.ProviderWebhookEvent{
		ProviderPaymentID: "mock-prov",
		EventKey:          evtKey,
		Status:            status,
		ProviderStatus:    provStatus,
		AmountCents:       100000,
		RawPayload:        []byte("{}"),
	}, nil
}

func (m *testProviderMock) GetMode(method string) string {
	if m.modeFn != nil {
		return m.modeFn(method)
	}
	return "hosted_form"
}

func (m *testProviderMock) CheckOrder(ctx context.Context, orderID string) (payments.CheckOrderResult, error) {
	if m.checkOrderFn != nil {
		return m.checkOrderFn(ctx, orderID)
	}
	return payments.CheckOrderResult{
		OrderID: orderID,
	}, nil
}

func setupTestPaymentsService(t *testing.T, client *postgres.Client, marketingSvc *marketing.Service, provider payments.Provider) *payments.Service {
	repo := payments.NewRepository(client.Pool)
	cfg := &config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}}
	svc := payments.NewService(
		repo,
		orders.NewRepository(client.Pool),
		inventory.NewService(inventory.NewRepository(client.Pool), nil, client),
		provider,
		client,
		notifications.NewService(nil, nil, nil),
		nil,
		cfg,
	).WithMarketing(marketingSvc)
	return svc
}

func setupTestOrdersService(t *testing.T, client *postgres.Client, marketingSvc *marketing.Service) *orders.Service {
	cfg := &config.Config{}
	invRepo := inventory.NewRepository(client.Pool)
	invSvc := inventory.NewService(invRepo, nil, client)
	ordersRepo := orders.NewRepository(client.Pool)
	return orders.NewService(ordersRepo, nil, invSvc, client, cfg).WithMarketing(marketingSvc)
}

// ---------------------------------------------------------------------
// MARKETING.1C2A Hardening Tests (P through Z)
// ---------------------------------------------------------------------

// TestMatrixP_Canonical5000Seller15Zamk10Economics explicitly verifies the canonical
// 5000 RUB item / 15% seller discount / 10% ZAMK subsidy / 1500 bps commission example.
// Asserts persisted database values in order_item_promotions and proves ZAMK subsidy does not reduce commission base.
func TestMatrixP_Canonical5000Seller15Zamk10Economics(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1500, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "CANONP", marketing.DiscountTypePercent, 2500, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "canonp")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000, // 5000 RUB
		Quantity:           1,
	}}

	var calc *marketing.CheckoutPromoCalculation
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		res, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 1500, time.Now().UTC())
		require.NoError(t, err)
		calc = res
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, res, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Quantity = 1 exact assertions
	assert.Equal(t, int64(75000), calc.TotalSellerDiscountCents, "Seller discount: 75000 cents (15%)")
	assert.Equal(t, int64(50000), calc.TotalZamkSubsidyCents, "ZAMK subsidy: 50000 cents (10%)")
	assert.Equal(t, int64(375000), calc.TotalCustomerPaidCents, "Customer paid: 375000 cents")
	assert.Equal(t, int64(125000), calc.TotalOrderDiscountCents, "Total discount: 125000 cents (25%)")

	require.Len(t, calc.PromotedLines, 1)
	line := calc.PromotedLines[0]
	assert.Equal(t, int64(425000), line.TotalCommissionBaseCents, "Commission base: 425000 cents")
	assert.Equal(t, int64(63750), line.TotalCommissionChargedCents, "Commission charged @ 15%: 63750 cents")

	// Persisted database financial truth
	var dbBase, dbSellerDisc, dbZamkSubsidy, dbCustPaid, dbCommBase int64
	err = client.Pool.QueryRow(ctx, `
		SELECT base_unit_price_cents, total_seller_discount_cents, total_zamk_subsidy_cents, total_customer_paid_cents, total_commission_base_cents
		FROM order_item_promotions WHERE order_item_id = $1
	`, itemID).Scan(&dbBase, &dbSellerDisc, &dbZamkSubsidy, &dbCustPaid, &dbCommBase)
	require.NoError(t, err)
	assert.Equal(t, int64(500000), dbBase, "Persisted base: 500000")
	assert.Equal(t, int64(75000), dbSellerDisc, "Persisted seller discount: 75000")
	assert.Equal(t, int64(50000), dbZamkSubsidy, "Persisted ZAMK subsidy: 50000")
	assert.Equal(t, int64(375000), dbCustPaid, "Persisted customer paid: 375000")
	assert.Equal(t, int64(425000), dbCommBase, "Persisted commission base: 425000")

	// Prove: ZAMK subsidy does NOT reduce commission base
	assert.Equal(t, dbBase-dbSellerDisc, dbCommBase, "Commission base MUST equal base minus seller discount")
	assert.NotEqual(t, dbBase-dbSellerDisc-dbZamkSubsidy, dbCommBase, "ZAMK subsidy MUST NOT reduce commission base")
}

// TestMatrixQ_FirstOrderPromoReservedNonPromoPaidFirst verifies CASE 2:
// Customer reserves a first-order promo on Order A.
// An ordinary non-promo Order B for the same customer becomes PAID first.
// When Order A attempts payment, EnsureOrderPromoHoldTx detects the existing paid order,
// releases the promo hold, and returns ErrPromoFirstOrderOnly.
func TestMatrixQ_FirstOrderPromoReservedNonPromoPaidFirst(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "FIRSTQ", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "userq")
	orderA := createTestOrder(t, client, user)
	orderB := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemAID, prodAID, varAID := createTestOrderItem(t, client, orderA, sellerID, 500000, 1)
	itemsA := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemAID,
		ProductID:          prodAID,
		ProductVariantID:   varAID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	// 1. Order A reserves the first-order promo
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, itemsA, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderA, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Verify Order A usage is reserved and ZAMK budget is reserved
	campBefore, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campBefore.ZamkReservedCents)

	// 2. Order B (non-promo) becomes PAID
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'paid', updated_at = now() WHERE id = $1", orderB)
	require.NoError(t, err)

	// 3. Order A attempts payment initiation: EnsureOrderPromoHold must reject and release hold
	err = svc.EnsureOrderPromoHold(ctx, orderA)
	require.Error(t, err)
	assert.ErrorIs(t, err, marketing.ErrPromoFirstOrderOnly)

	// 4. Assert in DB that promo usage was released and reserved budget was freed
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderA)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status, "Promo usage must be transitioned to released")

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents, "Reserved budget must be released")
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents, "Consumed budget must remain 0")
}

// TestMatrixR_PromoVsNonPromoSuccessfulPaymentRace verifies Scenario 1:
// Customer has 0 paid orders.
// Order A (first-order promo) initiates payment and acquires customer-level first-payment claim.
// Order B (non-promo) calls CreatePayment and is blocked with ErrFirstPaymentInProgress.
// Order A completes payment (CONFIRMED webhook): marks Order A paid, consumes promo, releases claim.
// Order B calls CreatePayment: customer now has 1 paid order, so claim acquisition is bypassed;
// Order B payment proceeds and completes normally as 2nd order.
func TestMatrixR_PromoVsNonPromoSuccessfulPaymentRace(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "FIRSTR", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "userr")
	promoOrder := createTestOrder(t, client, user)
	nonPromoOrder := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemAID, prodAID, varAID := createTestOrderItem(t, client, promoOrder, sellerID, 500000, 1)
	itemsA := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemAID,
		ProductID:          prodAID,
		ProductVariantID:   varAID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	_, _, _ = createTestOrderItem(t, client, nonPromoOrder, sellerID, 500000, 1)

	// Reserve first order promo on promoOrder
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, itemsA, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, promoOrder, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Set both orders to awaiting_payment
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", promoOrder)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 500000 WHERE id = $1", nonPromoOrder)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: "prov-" + input.OrderID,
				PaymentURL:        "https://pay.tbank/" + input.OrderID,
				Status:            "NEW",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			parts := string(body)
			var oID, st string
			_, _ = fmt.Sscanf(parts, "%s %s", &oID, &st)
			amt := int64(400000)
			if oID == nonPromoOrder.String() {
				amt = 500000
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: "prov-" + oID,
				OrderID:           oID,
				Status:            "succeeded",
				ProviderStatus:    st,
				AmountCents:       amt,
				EventKey:          uuid.NewString(),
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// 1. Promo order initiates payment: acquires first payment claim
	respA, err := paySvc.CreatePayment(ctx, user, promoOrder, "card")
	require.NoError(t, err)
	require.NotNil(t, respA)

	// Verify claim exists in DB for promoOrder
	var claimOrderID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT order_id FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimOrderID)
	require.NoError(t, err)
	assert.Equal(t, promoOrder, claimOrderID)

	// 2. Non-promo order attempts payment: MUST be rejected with ErrFirstPaymentInProgress
	_, err = paySvc.CreatePayment(ctx, user, nonPromoOrder, "card")
	require.Error(t, err)
	assert.ErrorIs(t, err, payments.ErrFirstPaymentInProgress)

	// 3. Promo order completes payment successfully
	err = paySvc.HandleWebhook(ctx, nil, []byte(fmt.Sprintf("%s CONFIRMED", promoOrder.String())))
	require.NoError(t, err)

	// Verify promo order is paid, promo usage is consumed, and claim is released
	var orderAStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", promoOrder).Scan(&orderAStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", orderAStatus)

	usageA, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, promoOrder)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usageA.Status)

	campFinal, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campFinal.ZamkSpentCents)
	assert.Equal(t, int64(0), campFinal.ZamkReservedCents)

	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount, "First-payment claim must be released upon payment completion")

	// 4. Non-promo order retries payment: customer now has 1 paid order, so claim acquisition is bypassed
	respB, err := paySvc.CreatePayment(ctx, user, nonPromoOrder, "card")
	require.NoError(t, err)
	require.NotNil(t, respB)

	// Webhook confirms non-promo order
	err = paySvc.HandleWebhook(ctx, nil, []byte(fmt.Sprintf("%s CONFIRMED", nonPromoOrder.String())))
	require.NoError(t, err)

	var orderBStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", nonPromoOrder).Scan(&orderBStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", orderBStatus)
}

// TestMatrix_Scenario2_NonPromoFirstBlocksPromo verifies Scenario 2:
// Customer has 0 paid orders.
// Order B (non-promo) initiates payment first and acquires customer-level first-payment claim.
// Order A (first-order promo) calls CreatePayment and is blocked with ErrFirstPaymentInProgress.
// Order B completes payment (CONFIRMED webhook): marks Order B paid and releases claim.
// Order A calls CreatePayment: customer now has 1 paid order, so EnsureOrderPromoHold detects
// customer is no longer eligible for first-order promo, releases the promo hold, and rejects with ErrPromoFirstOrderOnly.
func TestMatrix_Scenario2_NonPromoFirstBlocksPromo(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "SCEN2", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "scen2")
	promoOrder := createTestOrder(t, client, user)
	nonPromoOrder := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemAID, prodAID, varAID := createTestOrderItem(t, client, promoOrder, sellerID, 500000, 1)
	itemsA := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemAID,
		ProductID:          prodAID,
		ProductVariantID:   varAID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	_, _, _ = createTestOrderItem(t, client, nonPromoOrder, sellerID, 500000, 1)

	// Reserve first order promo on promoOrder
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, itemsA, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, promoOrder, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", promoOrder)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 500000 WHERE id = $1", nonPromoOrder)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: "prov-" + input.OrderID,
				PaymentURL:        "https://pay.tbank/" + input.OrderID,
				Status:            "NEW",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			parts := string(body)
			var oID, st string
			_, _ = fmt.Sscanf(parts, "%s %s", &oID, &st)
			amt := int64(500000)
			if oID == promoOrder.String() {
				amt = 400000
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: "prov-" + oID,
				OrderID:           oID,
				Status:            "succeeded",
				ProviderStatus:    st,
				AmountCents:       amt,
				EventKey:          uuid.NewString(),
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// 1. Non-promo order initiates payment first: acquires claim
	respB, err := paySvc.CreatePayment(ctx, user, nonPromoOrder, "card")
	require.NoError(t, err)
	require.NotNil(t, respB)

	// 2. Promo order attempts payment: blocked with ErrFirstPaymentInProgress
	_, err = paySvc.CreatePayment(ctx, user, promoOrder, "card")
	require.Error(t, err)
	assert.ErrorIs(t, err, payments.ErrFirstPaymentInProgress)

	// 3. Non-promo order completes payment
	err = paySvc.HandleWebhook(ctx, nil, []byte(fmt.Sprintf("%s CONFIRMED", nonPromoOrder.String())))
	require.NoError(t, err)

	var orderBStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", nonPromoOrder).Scan(&orderBStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", orderBStatus)

	// 4. Promo order attempts payment: rejected by EnsureOrderPromoHold as ErrPromoFirstOrderOnly
	_, err = paySvc.CreatePayment(ctx, user, promoOrder, "card")
	require.Error(t, err)
	assert.ErrorIs(t, err, marketing.ErrPromoFirstOrderOnly)

	// Promo usage must be released and ZAMK budget restored
	usageA, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, promoOrder)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usageA.Status)

	campFinal, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campFinal.ZamkReservedCents)
	assert.Equal(t, int64(0), campFinal.ZamkSpentCents)
}

// TestMatrix_Scenario2B_NonPromoFailsUnblocksPromo verifies Scenario 2B:
// Non-promo order initiates payment first and acquires claim, blocking promo order.
// Non-promo payment fails (REJECTED webhook), releasing the claim.
// Promo order retries payment: since customer still has 0 paid orders, promo hold is valid,
// promo order acquires claim, initiates payment, and completes successfully.
func TestMatrix_Scenario2B_NonPromoFailsUnblocksPromo(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "SCEN2B", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "scen2b")
	promoOrder := createTestOrder(t, client, user)
	nonPromoOrder := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemAID, prodAID, varAID := createTestOrderItem(t, client, promoOrder, sellerID, 500000, 1)
	itemsA := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemAID,
		ProductID:          prodAID,
		ProductVariantID:   varAID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	_, _, _ = createTestOrderItem(t, client, nonPromoOrder, sellerID, 500000, 1)

	// Reserve promo
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, itemsA, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, promoOrder, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", promoOrder)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 500000 WHERE id = $1", nonPromoOrder)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: "prov-" + input.OrderID,
				PaymentURL:        "https://pay.tbank/" + input.OrderID,
				Status:            "NEW",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			parts := string(body)
			var oID, st string
			_, _ = fmt.Sscanf(parts, "%s %s", &oID, &st)
			webhookStatus := "succeeded"
			if st == "REJECTED" {
				webhookStatus = "cancelled"
			}
			amt := int64(500000)
			if oID == promoOrder.String() {
				amt = 400000
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: "prov-" + oID,
				OrderID:           oID,
				Status:            webhookStatus,
				ProviderStatus:    st,
				AmountCents:       amt,
				EventKey:          uuid.NewString(),
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// 1. Non-promo initiates payment and acquires claim
	_, err = paySvc.CreatePayment(ctx, user, nonPromoOrder, "card")
	require.NoError(t, err)

	// 2. Promo order is blocked
	_, err = paySvc.CreatePayment(ctx, user, promoOrder, "card")
	require.Error(t, err)
	assert.ErrorIs(t, err, payments.ErrFirstPaymentInProgress)

	// 3. Non-promo payment fails (REJECTED)
	err = paySvc.HandleWebhook(ctx, nil, []byte(fmt.Sprintf("%s REJECTED", nonPromoOrder.String())))
	require.NoError(t, err)

	// Verify claim was released
	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)

	// 4. Promo order retries payment: customer still has 0 paid orders, so succeeds
	respA, err := paySvc.CreatePayment(ctx, user, promoOrder, "card")
	require.NoError(t, err)
	require.NotNil(t, respA)

	// 5. Promo order payment succeeds
	err = paySvc.HandleWebhook(ctx, nil, []byte(fmt.Sprintf("%s CONFIRMED", promoOrder.String())))
	require.NoError(t, err)

	var orderAStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", promoOrder).Scan(&orderAStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", orderAStatus)

	usageA, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, promoOrder)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usageA.Status)

	campFinal, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campFinal.ZamkSpentCents)
}

// TestMatrix_Scenario1B_PromoFailsUnblocksNonPromo verifies Scenario 1B:
// Promo order initiates payment first and acquires claim, blocking non-promo order.
// Promo payment fails (REJECTED webhook), releasing the claim and releasing promo hold.
// Non-promo order retries payment: acquires claim, initiates payment, and completes successfully as first paid order.
func TestMatrix_Scenario1B_PromoFailsUnblocksNonPromo(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "SCEN1B", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "scen1b")
	promoOrder := createTestOrder(t, client, user)
	nonPromoOrder := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemAID, prodAID, varAID := createTestOrderItem(t, client, promoOrder, sellerID, 500000, 1)
	itemsA := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemAID,
		ProductID:          prodAID,
		ProductVariantID:   varAID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	_, _, _ = createTestOrderItem(t, client, nonPromoOrder, sellerID, 500000, 1)

	// Reserve promo
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, itemsA, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, promoOrder, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", promoOrder)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 500000 WHERE id = $1", nonPromoOrder)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: "prov-" + input.OrderID,
				PaymentURL:        "https://pay.tbank/" + input.OrderID,
				Status:            "NEW",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			parts := string(body)
			var oID, st string
			_, _ = fmt.Sscanf(parts, "%s %s", &oID, &st)
			webhookStatus := "succeeded"
			if st == "REJECTED" {
				webhookStatus = "cancelled"
			}
			amt := int64(500000)
			if oID == promoOrder.String() {
				amt = 400000
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: "prov-" + oID,
				OrderID:           oID,
				Status:            webhookStatus,
				ProviderStatus:    st,
				AmountCents:       amt,
				EventKey:          uuid.NewString(),
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// 1. Promo order initiates payment and acquires claim
	_, err = paySvc.CreatePayment(ctx, user, promoOrder, "card")
	require.NoError(t, err)

	// 2. Non-promo order is blocked
	_, err = paySvc.CreatePayment(ctx, user, nonPromoOrder, "card")
	require.Error(t, err)
	assert.ErrorIs(t, err, payments.ErrFirstPaymentInProgress)

	// 3. Promo payment fails (REJECTED)
	err = paySvc.HandleWebhook(ctx, nil, []byte(fmt.Sprintf("%s REJECTED", promoOrder.String())))
	require.NoError(t, err)

	// Verify claim was released and promo was released
	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)

	usageA, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, promoOrder)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usageA.Status)

	// 4. Non-promo order retries payment: customer still has 0 paid orders, so acquires claim and succeeds
	respB, err := paySvc.CreatePayment(ctx, user, nonPromoOrder, "card")
	require.NoError(t, err)
	require.NotNil(t, respB)

	// 5. Non-promo order payment succeeds
	err = paySvc.HandleWebhook(ctx, nil, []byte(fmt.Sprintf("%s CONFIRMED", nonPromoOrder.String())))
	require.NoError(t, err)

	var orderBStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", nonPromoOrder).Scan(&orderBStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", orderBStatus)
}

// TestMatrix_AuthorizedDoesNotConsumePromoOrMarkPaid verifies P0-A:
// Webhook with AUTHORIZED leaves payment in pending state, order in awaiting_payment state,
// and promo usage in reserved status (never consumes promo or marks order paid).
// Subsequent webhook with CONFIRMED transitions payment to succeeded, marks order paid,
// and consumes promo usage.
func TestMatrix_AuthorizedDoesNotConsumePromoOrMarkPaid(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "AUTHNOTPAID", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "authnp")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: "prov-" + input.OrderID,
				PaymentURL:        "https://pay.tbank/" + input.OrderID,
				Status:            "NEW",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			statusStr := string(body)
			if statusStr == "AUTHORIZED" {
				return payments.ProviderWebhookEvent{
					ProviderPaymentID: "prov-" + orderID.String(),
					OrderID:           orderID.String(),
					Status:            "pending",
					ProviderStatus:    "AUTHORIZED",
					AmountCents:       400000,
					EventKey:          uuid.NewString(),
				}, nil
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: "prov-" + orderID.String(),
				OrderID:           orderID.String(),
				Status:            "succeeded",
				ProviderStatus:    "CONFIRMED",
				AmountCents:       400000,
				EventKey:          uuid.NewString(),
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// 1. Create payment
	resp, err := paySvc.CreatePayment(ctx, user, orderID, "card")
	require.NoError(t, err)
	require.NotNil(t, resp)

	// 2. Deliver AUTHORIZED webhook
	err = paySvc.HandleWebhook(ctx, nil, []byte("AUTHORIZED"))
	require.NoError(t, err)

	// Assertions for AUTHORIZED:
	// - Order remains awaiting_payment
	var orderStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&orderStatus)
	require.NoError(t, err)
	assert.Equal(t, "awaiting_payment", orderStatus, "Order must NOT be marked paid on AUTHORIZED")

	// - Payment remains pending
	var paymentStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", resp.PaymentID).Scan(&paymentStatus)
	require.NoError(t, err)
	assert.Equal(t, "pending", paymentStatus, "Payment must remain pending on AUTHORIZED")

	// - Promo remains reserved
	usageMid, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usageMid.Status, "Promo usage must NOT be consumed on AUTHORIZED")

	// - Budget remains reserved, NOT spent
	campMid, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campMid.ZamkReservedCents)
	assert.Equal(t, int64(0), campMid.ZamkSpentCents, "ZAMK budget must NOT be spent on AUTHORIZED")

	// 3. Deliver CONFIRMED webhook
	err = paySvc.HandleWebhook(ctx, nil, []byte("CONFIRMED"))
	require.NoError(t, err)

	// Assertions for CONFIRMED:
	// - Order is now paid
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&orderStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", orderStatus, "Order must be marked paid on CONFIRMED")

	// - Payment is now succeeded
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", resp.PaymentID).Scan(&paymentStatus)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", paymentStatus, "Payment must be succeeded on CONFIRMED")

	// - Promo is now consumed
	usageFinal, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usageFinal.Status, "Promo usage must be consumed on CONFIRMED")

	// - Budget is now spent
	campFinal, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campFinal.ZamkReservedCents)
	assert.Equal(t, int64(50000), campFinal.ZamkSpentCents, "ZAMK budget must be spent on CONFIRMED")
}


// TestMatrixS_AmbiguousProviderInitTimeout verifies that when payment initialization fails
// due to an ambiguous transport timeout / network drop, promo hold and campaign budget
// are NOT prematurely released (fail closed, zero budget leak).
func TestMatrixS_AmbiguousProviderInitTimeout(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "TIMEOUTS", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "timeouts")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	// Reserve promo
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment' WHERE id = $1", orderID)
	require.NoError(t, err)

	// Create mock provider that simulates network timeout
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// Attempt payment initiation
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Invariant: promo hold must NOT be released!
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status, "Promo hold must remain reserved on ambiguous timeout")

	// Invariant: campaign reserved budget must NOT be leaked
	campCheck, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campCheck.ZamkReservedCents, "Budget must remain reserved")
}

// TestMatrixT_DefinitiveProviderInitRejection verifies that when payment initialization
// definitively fails at the provider, the order-level promo hold is preserved for retry,
// and is only released upon authoritative order cancellation.
func TestMatrixT_DefinitiveProviderInitRejection(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "REJECTT", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "rejectt")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	// Reserve promo
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment' WHERE id = $1", orderID)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, fmt.Errorf("tbank init failed: rejected by risk")
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// Payment initialization fails
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	// Order-level hold is preserved for customer retry
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)

	// When order is explicitly cancelled, promo hold is released
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, orderID, "order_cancelled")
	})
	require.NoError(t, err)

	usageAfter, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usageAfter.Status)
}

// TestMatrixU_TransientPaymentFailureDoesNotRelease verifies that a transient auth failure
// (such as AUTH_FAIL on 3DS where the user can retry entering details) does NOT release promo or budget.
// Subsequent confirmation successfully consumes the single promo hold.
func TestMatrixU_TransientPaymentFailureDoesNotRelease(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "TRANSU", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "transu")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment' WHERE id = $1", orderID)
	require.NoError(t, err)

	// Create payment row in DB
	paymentID := uuid.New()
	provPaymentID := "prov-" + paymentID.String()[:8]
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, payment_method, integration_mode)
		VALUES ($1, $2, 'tbank', $3, 'pending', 400000, 'RUB', $4, 'card', 'hosted_form')
	`, paymentID, orderID, provPaymentID, uuid.NewString())
	require.NoError(t, err)

	mockProv := &testProviderMock{
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			statusStr := string(body)
			if statusStr == "AUTH_FAIL" {
				return payments.ProviderWebhookEvent{
					ProviderPaymentID: provPaymentID,
					OrderID:           orderID.String(),
					Status:            "pending",
					ProviderStatus:    "AUTH_FAIL",
					AmountCents:       400000,
					EventKey:          "key-auth-fail-" + paymentID.String()[:8],
				}, nil
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID,
				OrderID:           orderID.String(),
				Status:            "succeeded",
				ProviderStatus:    "CONFIRMED",
				AmountCents:       400000,
				EventKey:          "key-confirmed-" + paymentID.String()[:8],
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// 1. Deliver AUTH_FAIL webhook (transient failure)
	err = paySvc.HandleWebhook(ctx, nil, []byte("AUTH_FAIL"))
	require.NoError(t, err)

	// Assert promo usage is STILL reserved!
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status, "Transient failure must not release promo hold")

	campMid, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campMid.ZamkReservedCents)

	// 2. Deliver CONFIRMED webhook
	err = paySvc.HandleWebhook(ctx, nil, []byte("CONFIRMED"))
	require.NoError(t, err)

	// Assert promo usage is consumed
	usageAfter, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usageAfter.Status)

	campFinal, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campFinal.ZamkSpentCents)
	assert.Equal(t, int64(0), campFinal.ZamkReservedCents)
}

// TestMatrixV_DefinitiveTerminalPaymentFailureReleasesOnce verifies that definitive terminal
// failures (DEADLINE_EXPIRED, REJECTED, CANCELED) release promo and budget exactly once,
// and duplicate terminal notifications are safely idempotent.
func TestMatrixV_DefinitiveTerminalPaymentFailureReleasesOnce(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "TERMFAILV", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "termfailv")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment' WHERE id = $1", orderID)
	require.NoError(t, err)

	paymentID := uuid.New()
	provPaymentID := "prov-term-" + paymentID.String()[:8]
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, payment_method, integration_mode)
		VALUES ($1, $2, 'tbank', $3, 'pending', 400000, 'RUB', $4, 'card', 'hosted_form')
	`, paymentID, orderID, provPaymentID, uuid.NewString())
	require.NoError(t, err)

	mockProv := &testProviderMock{
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID,
				OrderID:           orderID.String(),
				Status:            "cancelled",
				ProviderStatus:    "DEADLINE_EXPIRED",
				AmountCents:       400000,
				EventKey:          string(body),
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	termKey := "event-term-" + paymentID.String()[:8]
	// 1. Deliver DEADLINE_EXPIRED
	err = paySvc.HandleWebhook(ctx, nil, []byte(termKey))
	require.NoError(t, err)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)

	// 2. Deliver duplicate DEADLINE_EXPIRED
	err = paySvc.HandleWebhook(ctx, nil, []byte(termKey))
	require.NoError(t, err)

	campAfterDup, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfterDup.ZamkReservedCents, "Must not double-decrement below 0")
}

// TestMatrixW_PaymentRetryPreservesOneOrderLevelPromoHold verifies that payment retry
// preserves the order-level promo hold, reuses the immutable discounted total,
// and ensures exactly 1 promo_code_usages row exists for the order.
func TestMatrixW_PaymentRetryPreservesOneOrderLevelPromoHold(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RETRYW", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "retryw")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Set order total to discounted amount
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	var callCount int
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			callCount++
			assert.Equal(t, int64(400000), input.AmountCents, "Retry must use immutable discounted order total")
			if callCount == 1 {
				return payments.ProviderCreatePaymentResult{}, fmt.Errorf("%w: card declined", payments.ErrProviderRejected)
			}
			provID := "retry-prov-" + orderID.String()[:8]
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: provID,
				PaymentURL:        "https://pay.tbank/retry",
				Status:            "NEW",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			provID := "retry-prov-" + orderID.String()[:8]
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provID,
				OrderID:           orderID.String(),
				Status:            "succeeded",
				ProviderStatus:    "CONFIRMED",
				AmountCents:       400000,
				EventKey:          "key-retry-" + orderID.String()[:8],
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// Attempt 1 fails
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	// Definitive Init rejection releases promo hold; retry re-acquires it
	usageMid, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usageMid.Status)

	// Attempt 2 succeeds
	resp, err := paySvc.CreatePayment(ctx, user, orderID, "card")
	require.NoError(t, err)
	assert.Equal(t, int64(400000), resp.AmountCents)

	// Webhook confirms
	err = paySvc.HandleWebhook(ctx, nil, []byte("ok"))
	require.NoError(t, err)

	// Assert exactly 1 promo_code_usages row exists for this order and is consumed
	var usageCount int
	var finalStatus string
	err = client.Pool.QueryRow(ctx, "SELECT count(*), max(status) FROM promo_code_usages WHERE order_id = $1", orderID).Scan(&usageCount, &finalStatus)
	require.NoError(t, err)
	assert.Equal(t, 1, usageCount, "UNIQUE(order_id) must be preserved; no second usage row")
	assert.Equal(t, string(marketing.UsageStatusConsumed), finalStatus)
}

// TestMatrixX_ReleasedDiscountedOrderCannotBePaidWithoutValidHold verifies invariant X:
// A discounted order whose promo hold was released cannot be paid on retry if current capacity
// or budget rules are no longer satisfied.
func TestMatrixX_ReleasedDiscountedOrderCannotBePaidWithoutValidHold(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "HOLDX", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "userx")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	// Release the promo hold (e.g. order-level cancellation or expiration)
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, orderID, "test_release")
	})
	require.NoError(t, err)

	// Now deactivate campaign
	_, err = client.Pool.Exec(ctx, "UPDATE marketing_campaigns SET status = 'cancelled' WHERE id = $1", camp.ID)
	require.NoError(t, err)

	// Attempt EnsureOrderPromoHoldTx on payment initiation: must FAIL!
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.EnsureOrderPromoHoldTx(ctx, tx, orderID)
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, marketing.ErrPromoInactive, "Must reject payment retry when hold cannot be re-established")
}

// TestMatrixY_SuccessFollowedByRefundReversalKeepsUsageConsumed verifies that post-payment
// refunds or reversals do NOT release promo hold or restore spent ZAMK budget.
func TestMatrixY_SuccessFollowedByRefundReversalKeepsUsageConsumed(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "REFUNDY", marketing.DiscountTypePercent, 2000, 0, 0, nil, 5, false)

	user := createTestUser(t, client, "usery")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	paymentID := uuid.New()
	provPaymentID := "prov-ref-" + paymentID.String()[:8]
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, payment_method, integration_mode)
		VALUES ($1, $2, 'tbank', $3, 'pending', 400000, 'RUB', $4, 'card', 'hosted_form')
	`, paymentID, orderID, provPaymentID, uuid.NewString())
	require.NoError(t, err)

	mockProv := &testProviderMock{
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			action := string(body)
			if action == "REFUNDED" {
				return payments.ProviderWebhookEvent{
					ProviderPaymentID: provPaymentID,
					OrderID:           orderID.String(),
					Status:            "refunded",
					ProviderStatus:    "REFUNDED",
					AmountCents:       400000,
					EventKey:          "key-refund-" + paymentID.String()[:8],
				}, nil
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID,
				OrderID:           orderID.String(),
				Status:            "succeeded",
				ProviderStatus:    "CONFIRMED",
				AmountCents:       400000,
				EventKey:          "key-confirm-" + paymentID.String()[:8],
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// 1. Deliver CONFIRMED webhook: payment succeeds, promo consumed, budget spent
	err = paySvc.HandleWebhook(ctx, nil, []byte("CONFIRMED"))
	require.NoError(t, err)

	usageAfterPay, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usageAfterPay.Status)

	campAfterPay, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfterPay.ZamkSpentCents)

	// 2. Deliver REFUNDED webhook
	err = paySvc.HandleWebhook(ctx, nil, []byte("REFUNDED"))
	require.NoError(t, err)

	// Invariant: usage remains consumed, budget remains spent!
	usageAfterRefund, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usageAfterRefund.Status, "Promo usage MUST remain consumed after refund")

	campAfterRefund, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfterRefund.ZamkSpentCents, "ZAMK budget MUST remain spent after refund")
}

// TestMatrixZ_DBInvariantPreventsSimultaneousFirstOrderReservedAndConsumed verifies that
// database index uq_first_order_active_per_customer prevents any invalid simultaneous state
// of (consumed + reserved) or (reserved + reserved) first-order claims for the same customer.
func TestMatrixZ_DBInvariantPreventsSimultaneousFirstOrderReservedAndConsumed(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "INDEXZ", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "userz")
	order1 := createTestOrder(t, client, user)
	order2 := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	// 1. Insert row 1: is_first_order = true, status = 'consumed'
	usage1ID := uuid.New()
	queryInsert := `
		INSERT INTO promo_code_usages (id, promo_code_id, campaign_id, user_id, order_id, is_first_order, status, seller_discount_cents, subsidy_cents, reserved_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, true, $6, 1000, 1000, now(), now() + interval '30 minutes')
	`
	_, err := client.Pool.Exec(ctx, queryInsert, usage1ID, promo.ID, camp.ID, user, order1, "consumed")
	require.NoError(t, err)

	// 2. Attempt to insert row 2 for same customer: is_first_order = true, status = 'reserved'
	usage2ID := uuid.New()
	_, err = client.Pool.Exec(ctx, queryInsert, usage2ID, promo.ID, camp.ID, user, order2, "reserved")
	require.Error(t, err, "Must violate uq_first_order_active_per_customer unique index")
	assert.Contains(t, err.Error(), "uq_first_order_active_per_customer")

	// 3. Attempt to insert row 3 for same customer: is_first_order = true, status = 'consumed'
	usage3ID := uuid.New()
	_, err = client.Pool.Exec(ctx, queryInsert, usage3ID, promo.ID, camp.ID, user, order2, "consumed")
	require.Error(t, err, "Must violate uq_first_order_active_per_customer unique index")
	assert.Contains(t, err.Error(), "uq_first_order_active_per_customer")

	// 4. When row 1 transitions to 'released', a new 'reserved' row can be created
	_, err = client.Pool.Exec(ctx, "UPDATE promo_code_usages SET status = 'released', released_at = now() WHERE id = $1", usage1ID)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, queryInsert, usage2ID, promo.ID, camp.ID, user, order2, "reserved")
	require.NoError(t, err, "After release of row 1, row 2 can be inserted in reserved status")
}

// ---------------------------------------------------------------------
// MARKETING.1C2A-P0B Ambiguous Provider Init Lifecycle Tests (AA through AK)
// ---------------------------------------------------------------------

// TestMatrixAA_TX1Durability_PreInitCommit verifies that customer_first_payment_claims
// and the initial payment claim (status 'created', init_outcome 'pending') are durably
// committed in PostgreSQL and visible from a separate DB pool connection before the provider
// Init network call completes.
func TestMatrixAA_TX1Durability_PreInitCommit(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "TX1DUR", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-aa")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	verifiedInsideProviderCall := false
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			// Query from separate pool connection while provider call is executing
			var claimOrderID uuid.UUID
			err := client.Pool.QueryRow(ctx, "SELECT order_id FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimOrderID)
			assert.NoError(t, err)
			assert.Equal(t, orderID, claimOrderID, "customer_first_payment_claims must be visible during provider Init")

			var payStatus, payInitOutcome string
			err = client.Pool.QueryRow(ctx, "SELECT status, init_outcome FROM payments WHERE order_id = $1", orderID).Scan(&payStatus, &payInitOutcome)
			assert.NoError(t, err)
			assert.Equal(t, "created", payStatus)
			assert.Equal(t, "pending", payInitOutcome)

			verifiedInsideProviderCall = true
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: "prov-aa-" + uuid.NewString(),
				PaymentURL:        "https://pay.test/aa-1",
				Status:            "NEW",
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	resp, err := paySvc.CreatePayment(ctx, user, orderID, "card")
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, verifiedInsideProviderCall)
}

// TestMatrixAB_AmbiguousInit_RetainsClaim verifies that when provider Init returns an ambiguous
// transport failure (context.DeadlineExceeded), CreatePayment returns ErrPaymentOutcomeUncertain,
// and the customer first-payment claim remains durably held in PostgreSQL.
func TestMatrixAB_AmbiguousInit_RetainsClaim(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "AMBIGAB", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ab")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	resp, err := paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, payments.ErrPaymentOutcomeUncertain))

	// Assert first-payment claim remains durably stored
	var claimOrderID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT order_id FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimOrderID)
	require.NoError(t, err)
	assert.Equal(t, orderID, claimOrderID)

	// Assert payment row status = 'created' and init_outcome = 'unknown'
	var payStatus, payInitOutcome string
	err = client.Pool.QueryRow(ctx, "SELECT status, init_outcome FROM payments WHERE order_id = $1", orderID).Scan(&payStatus, &payInitOutcome)
	require.NoError(t, err)
	assert.Equal(t, "created", payStatus)
	assert.Equal(t, "unknown", payInitOutcome)
}

// TestMatrixAC_AmbiguousInit_PreservesPromoHold verifies that an ambiguous Init outcome
// preserves the promo reservation in 'reserved' status.
func TestMatrixAC_AmbiguousInit_PreservesPromoHold(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "AMBIGAC", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ac")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	// Invariant: promo usage remains reserved
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)
}

// TestMatrixAD_AmbiguousInit_PreservesBudget verifies that an ambiguous Init outcome
// preserves ZAMK reserved budget intact.
func TestMatrixAD_AmbiguousInit_PreservesBudget(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "AMBIGAD", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ad")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	// Invariant: ZAMK reserved cents is intact (50000 cents), spent is 0
	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkReservedCents)
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents)
}

// TestMatrixAE_CompetingOrderBlocked_UnderAmbiguousInit verifies that a competing order for
// the same customer cannot acquire the first-payment claim while an ambiguous Init attempt exists.
func TestMatrixAE_CompetingOrderBlocked_UnderAmbiguousInit(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "AMBIGAE", marketing.DiscountTypePercent, 2000, 0, 0, nil, 2, true)

	user := createTestUser(t, client, "user-ae")
	order1 := createTestOrder(t, client, user)
	order2 := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	item1ID, prod1ID, var1ID := createTestOrderItem(t, client, order1, sellerID, 500000, 1)
	items1 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        item1ID,
		ProductID:          prod1ID,
		ProductVariantID:   var1ID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items1, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, order1, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", order1)
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 500000 WHERE id = $1", order2)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// Order 1 enters ambiguous Init
	_, err = paySvc.CreatePayment(ctx, user, order1, "card")
	require.Error(t, err)

	// Order 2 tries to initiate payment
	_, err = paySvc.CreatePayment(ctx, user, order2, "card")
	require.Error(t, err)
	assert.True(t, errors.Is(err, payments.ErrFirstPaymentInProgress), "Competing order must be blocked by active first payment claim")
}

// TestMatrixAF_CancellationBlocked_UnderAmbiguousInit verifies that order cancellation is rejected
// if an outcome-uncertain payment attempt exists for the order.
func TestMatrixAF_CancellationBlocked_UnderAmbiguousInit(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "AMBIGAF", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-af")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)
	ordersSvc := setupTestOrdersService(t, client, svc)

	// Order enters ambiguous Init
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	// Customer tries to cancel order
	err = ordersSvc.CancelCustomerOrder(ctx, user, orderID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, orders.ErrOrderNotCancellable), "Cancellation must be blocked while payment outcome is uncertain")

	// Verify holds and claims remain intact
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)

	var claimOrderID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT order_id FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimOrderID)
	require.NoError(t, err)
	assert.Equal(t, orderID, claimOrderID)
}

// TestMatrixAG_LateConfirmedWebhookReconciles verifies that when a late CONFIRMED webhook
// arrives after an ambiguous Init, it matches the payment by order_id, marks the payment succeeded,
// order paid, consumes the promo, spends ZAMK budget, and releases the first-payment claim.
func TestMatrixAG_LateConfirmedWebhookReconciles(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "AMBIGAG", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ag")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	lateProvID := "tbank-late-ag-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: lateProvID,
				OrderID:           orderID.String(),
				Status:            "succeeded",
				ProviderStatus:    "CONFIRMED",
				AmountCents:       400000,
				EventKey:          "key-late-ag-" + uuid.NewString(),
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// Order enters ambiguous Init
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	// Late webhook arrives with CONFIRMED
	err = paySvc.HandleWebhook(ctx, nil, []byte("CONFIRMED"))
	require.NoError(t, err)

	// Assert payment succeeded and provider_payment_id backfilled
	var pStatus string
	var pProvID *string
	err = client.Pool.QueryRow(ctx, "SELECT status, provider_payment_id FROM payments WHERE order_id = $1", orderID).Scan(&pStatus, &pProvID)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pStatus)
	assert.NotNil(t, pProvID)
	assert.Equal(t, lateProvID, *pProvID)

	// Assert order status is paid
	var oStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", oStatus)

	// Assert promo consumed
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)

	// Assert budget spent
	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkSpentCents)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)

	// Assert claim released
	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)
}

// TestMatrixAH_LateTerminalFailureWebhookReconciles verifies that a late terminal failure
// webhook after an ambiguous Init releases promo reservation, budget, and first-payment claim,
// and unblocks order cancellation.
func TestMatrixAH_LateTerminalFailureWebhookReconciles(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "AMBIGAH", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ah")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	lateProvIDAH := "tbank-late-ah-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: lateProvIDAH,
				OrderID:           orderID.String(),
				Status:            "failed",
				ProviderStatus:    "REJECTED",
				AmountCents:       400000,
				EventKey:          "key-late-ah-" + uuid.NewString(),
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)
	ordersSvc := setupTestOrdersService(t, client, svc)

	// Order enters ambiguous Init
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	// Late terminal failure webhook arrives
	err = paySvc.HandleWebhook(ctx, nil, []byte("REJECTED"))
	require.NoError(t, err)

	// Assert payment marked failed
	var pStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", orderID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", pStatus)

	// Assert promo reservation released
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status)

	// Assert budget released
	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents)

	// Assert claim released
	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)

	// Order cancellation is now unblocked
	err = ordersSvc.CancelCustomerOrder(ctx, user, orderID)
	require.NoError(t, err)
}

// TestMatrixAI_DuplicateWebhookIdempotency verifies that duplicate late CONFIRMED webhooks
// are idempotent and do not double-spend campaign budget or corrupt promo usage state.
func TestMatrixAI_DuplicateWebhookIdempotency(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "AMBIGAI", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ai")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	aiEventKey := "key-late-ai-" + uuid.NewString()
	lateProvIDAI := "tbank-late-ai-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: lateProvIDAI,
				OrderID:           orderID.String(),
				Status:            "succeeded",
				ProviderStatus:    "CONFIRMED",
				AmountCents:       400000,
				EventKey:          aiEventKey,
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// Order enters ambiguous Init
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	// Webhook 1
	err = paySvc.HandleWebhook(ctx, nil, []byte("CONFIRMED"))
	require.NoError(t, err)

	// Webhook 2 (duplicate)
	err = paySvc.HandleWebhook(ctx, nil, []byte("CONFIRMED"))
	require.NoError(t, err)

	// Assert budget spent only once (50000 cents, NOT 100000 cents)
	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkSpentCents)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)
}

// TestMatrixAJ_SameOrderRetryBlocked_UnderAmbiguousInit verifies that repeating CreatePayment
// on the same order while its payment attempt is outcome-uncertain does NOT invoke the provider a second time.
func TestMatrixAJ_SameOrderRetryBlocked_UnderAmbiguousInit(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "AMBIGAJ", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-aj")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	providerCallCount := 0
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			providerCallCount++
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// First attempt: times out
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)
	assert.Equal(t, 1, providerCallCount)

	// Second attempt: must fail fast without calling provider again
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)
	assert.True(t, errors.Is(err, payments.ErrPaymentOutcomeUncertain))
	assert.Equal(t, 1, providerCallCount, "Provider must NOT be called a second time on ambiguous payment retry")
}

// TestMatrixAK_DefinitiveInitRejection_AllowsCancellationAndRelease verifies that a definitive
// provider rejection on Init marks the payment failed with init_outcome 'rejected', releases
// the first-payment claim, and allows order cancellation and promo release.
func TestMatrixAK_DefinitiveInitRejection_AllowsCancellationAndRelease(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "AMBIGAK", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ak")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, fmt.Errorf("%w: terminal card declined", payments.ErrProviderRejected)
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)
	ordersSvc := setupTestOrdersService(t, client, svc)

	// Definitive rejection on Init
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)
	assert.True(t, errors.Is(err, payments.ErrProviderRejected))

	// Assert payment marked failed with init_outcome = rejected
	var pStatus, pInitOutcome string
	err = client.Pool.QueryRow(ctx, "SELECT status, init_outcome FROM payments WHERE order_id = $1", orderID).Scan(&pStatus, &pInitOutcome)
	require.NoError(t, err)
	assert.Equal(t, "failed", pStatus)
	assert.Equal(t, "rejected", pInitOutcome)

	// Assert first-payment claim released
	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)

	// Order cancellation is permitted
	err = ordersSvc.CancelCustomerOrder(ctx, user, orderID)
	require.NoError(t, err)

	// Promo usage released
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status)
}

// TestMatrixAL_DefinitiveInitRejection_ReleasesPromoAndBudgetImmediately verifies that when provider Init
// fails with ErrProviderRejected, the promo reservation, ZAMK reserved budget, and first-payment claim
// are immediately released in CreatePayment without waiting for customer order cancellation.
func TestMatrixAL_DefinitiveInitRejection_ReleasesPromoAndBudgetImmediately(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "REJECTAL", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-al")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, fmt.Errorf("%w: terminal card declined", payments.ErrProviderRejected)
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// Definitive rejection on Init
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)
	assert.True(t, errors.Is(err, payments.ErrProviderRejected))

	// Assert payment marked failed with init_outcome = rejected
	var pStatus, pInitOutcome string
	err = client.Pool.QueryRow(ctx, "SELECT status, init_outcome FROM payments WHERE order_id = $1", orderID).Scan(&pStatus, &pInitOutcome)
	require.NoError(t, err)
	assert.Equal(t, "failed", pStatus)
	assert.Equal(t, "rejected", pInitOutcome)

	// Assert promo usage released immediately WITHOUT calling CancelOrder
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status)

	// Assert ZAMK reserved budget released
	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents)

	// Assert first-payment claim released
	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)

	// Assert order status remains awaiting_payment (inventory hold remains for retry)
	var oStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "awaiting_payment", oStatus)
}

// TestMatrixAM_CheckOrder_UnknownToConfirmed_ReconcilesConsistentlyWithWebhook verifies that
// when an ambiguous payment is reconciled against provider returning CONFIRMED, it transitions
// to succeeded, marks order paid, consumes the promo, spends ZAMK budget, and releases the first-payment claim.
func TestMatrixAM_CheckOrder_UnknownToConfirmed_ReconcilesConsistentlyWithWebhook(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAM", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-am")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	reconPID := "tb-recon-am-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       400000,
						Status:            "succeeded",
						ProviderStatus:    "CONFIRMED",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// Enter ambiguous state
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	// Authoritative reconciliation
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	// Assert payment succeeded, provider_payment_id populated, reconciliation_attempted_at stamped
	var pStatus, pInitOutcome string
	var pProvID *string
	var pReconAt *time.Time
	err = client.Pool.QueryRow(ctx, "SELECT status, init_outcome, provider_payment_id, reconciliation_attempted_at FROM payments WHERE id = $1", pID).Scan(&pStatus, &pInitOutcome, &pProvID, &pReconAt)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pStatus)
	assert.Equal(t, "successful", pInitOutcome)
	assert.NotNil(t, pProvID)
	assert.Equal(t, reconPID, *pProvID)

	// Assert order status paid
	var oStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", oStatus)

	// Assert promo consumed
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)

	// Assert ZAMK budget spent
	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkSpentCents)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)

	// Assert first-payment claim released
	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)
}

// TestMatrixAN_CheckOrder_UnknownToRejected_ReleasesOnce verifies that reconciliation
// discovering a terminal provider rejection releases promo reservation, budget, and first-payment claim once,
// and idempotent subsequent reconciliations do not double-release.
func TestMatrixAN_CheckOrder_UnknownToRejected_ReleasesOnce(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAN", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-an")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	reconPID := "tb-recon-an-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       400000,
						Status:            "cancelled",
						ProviderStatus:    "REJECTED",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// Enter ambiguous state
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	// First reconciliation: terminal failure
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	var pStatus, pInitOutcome string
	err = client.Pool.QueryRow(ctx, "SELECT status, init_outcome FROM payments WHERE id = $1", pID).Scan(&pStatus, &pInitOutcome)
	require.NoError(t, err)
	assert.Equal(t, "failed", pStatus)
	assert.Equal(t, "rejected", pInitOutcome)

	// Promo released
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status)

	// Budget released
	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents)

	// Claim released
	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)

	// Second reconciliation: idempotent no-op
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	campSecond, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campSecond.ZamkReservedCents)
	assert.Equal(t, int64(0), campSecond.ZamkSpentCents)
}

// TestMatrixAO_CheckOrder_Authorized_HoldsPromoBudgetClaimWithoutConsumingOrReleasing verifies
// that reconciliation finding an AUTHORIZED state marks payment pending/authorized, but keeps
// the promo reserved, budget reserved, and claim held without consuming or releasing.
func TestMatrixAO_CheckOrder_Authorized_HoldsPromoBudgetClaimWithoutConsumingOrReleasing(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAO", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ao")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	reconPID := "tb-recon-ao-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       400000,
						Status:            "pending",
						ProviderStatus:    "AUTHORIZED",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// Enter ambiguous state
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	// Reconcile AUTHORIZED
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	var pStatus, pInitOutcome string
	err = client.Pool.QueryRow(ctx, "SELECT status, init_outcome FROM payments WHERE id = $1", pID).Scan(&pStatus, &pInitOutcome)
	require.NoError(t, err)
	assert.Equal(t, "pending", pStatus)
	assert.Equal(t, "successful", pInitOutcome)

	// Promo remains reserved
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)

	// Budget remains reserved
	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkReservedCents)
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents)

	// Claim remains held
	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount)

	// Order status remains awaiting_payment
	var oStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "awaiting_payment", oStatus)
}

// TestMatrixAP_CheckOrder_ZeroPayments_StaysUnknownNoRelease verifies that when CheckOrder
// returns 0 payments, local payment stays created/unknown and nothing is consumed or released.
func TestMatrixAP_CheckOrder_ZeroPayments_StaysUnknownNoRelease(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAP", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ap")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{OrderID: oID, Payments: nil}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	var pStatus, pInitOutcome string
	err = client.Pool.QueryRow(ctx, "SELECT status, init_outcome FROM payments WHERE id = $1", pID).Scan(&pStatus, &pInitOutcome)
	require.NoError(t, err)
	assert.Equal(t, "created", pStatus)
	assert.Equal(t, "unknown", pInitOutcome)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkReservedCents)

	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount)
}

// TestMatrixAQ_CheckOrder_TransportTimeout_StaysUnknownNoRelease verifies that when CheckOrder
// encounters a network timeout, the payment remains unknown and no budget or promo is mutated.
func TestMatrixAQ_CheckOrder_TransportTimeout_StaysUnknownNoRelease(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAQ", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-aq")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{}, context.DeadlineExceeded
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	err = paySvc.ReconcilePayment(ctx, pID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))

	var pStatus, pInitOutcome string
	err = client.Pool.QueryRow(ctx, "SELECT status, init_outcome FROM payments WHERE id = $1", pID).Scan(&pStatus, &pInitOutcome)
	require.NoError(t, err)
	assert.Equal(t, "created", pStatus)
	assert.Equal(t, "unknown", pInitOutcome)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)
}

// TestMatrixAR_CheckOrder_AmountMismatch_FailsClosed verifies that when CheckOrder returns
// a payment whose amount does not match local payment amount, reconciliation fails closed.
func TestMatrixAR_CheckOrder_AmountMismatch_FailsClosed(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAR", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ar")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	reconPID := "tb-recon-ar-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       399900, // Mismatch: 399900 != 400000
						Status:            "succeeded",
						ProviderStatus:    "CONFIRMED",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	err = paySvc.ReconcilePayment(ctx, pID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, payments.ErrPaymentAmountMismatch))

	var pStatus, pInitOutcome string
	err = client.Pool.QueryRow(ctx, "SELECT status, init_outcome FROM payments WHERE id = $1", pID).Scan(&pStatus, &pInitOutcome)
	require.NoError(t, err)
	assert.Equal(t, "created", pStatus)
	assert.Equal(t, "unknown", pInitOutcome)
}

// TestMatrixAS_CheckOrder_MultiplePlausiblePayments_FailsClosed verifies that when CheckOrder
// returns multiple plausible unassigned payments for an order, reconciliation fails closed without guessing.
func TestMatrixAS_CheckOrder_MultiplePlausiblePayments_FailsClosed(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAS", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-as")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: "tb-as-1-" + uuid.NewString(),
						AmountCents:       400000,
						Status:            "succeeded",
						ProviderStatus:    "CONFIRMED",
					},
					{
						ProviderPaymentID: "tb-as-2-" + uuid.NewString(),
						AmountCents:       400000,
						Status:            "succeeded",
						ProviderStatus:    "CONFIRMED",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	err = paySvc.ReconcilePayment(ctx, pID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, payments.ErrMultipleProviderPaymentsAmbiguous))

	var pStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", pID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "created", pStatus)
}

// TestMatrixAT_CheckOrder_ProviderPIDAssignedToOtherPayment_FailsClosed verifies that if
// a remote payment ID is already assigned to a different local payment, reconciliation fails closed.
func TestMatrixAT_CheckOrder_ProviderPIDAssignedToOtherPayment_FailsClosed(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAT", marketing.DiscountTypePercent, 2000, 0, 0, nil, 2, false)
	_ = promo

	user1 := createTestUser(t, client, "user-at1")
	user2 := createTestUser(t, client, "user-at2")
	order1 := createTestOrder(t, client, user1)
	order2 := createTestOrder(t, client, user2)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user1, user2}, []uuid.UUID{camp.ID})

	_, err := client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id IN ($1, $2)", order1, order2)
	require.NoError(t, err)

	existingPID := "tb-shared-at-" + uuid.NewString()

	// Insert Payment 1 with existingPID
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_number, created_at, updated_at, idempotency_key)
		VALUES ($1, $2, 'tbank', $3, 'succeeded', 400000, 'RUB', $4, now(), now(), $5)
	`, uuid.New(), order1, existingPID, "PAY-"+uuid.NewString()[:6], uuid.NewString())
	require.NoError(t, err)

	// Payment 2 is ambiguous
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: existingPID,
						AmountCents:       400000,
						Status:            "succeeded",
						ProviderStatus:    "CONFIRMED",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user2, order2, "card")
	require.Error(t, err)

	var p2ID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", order2).Scan(&p2ID)
	require.NoError(t, err)

	err = paySvc.ReconcilePayment(ctx, p2ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, payments.ErrProviderPaymentAlreadyAssigned))

	var p2Status string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", p2ID).Scan(&p2Status)
	require.NoError(t, err)
	assert.Equal(t, "created", p2Status)
}

// TestMatrixAU_ConcurrentWebhookAndReconciliation_ConfirmedExactlyOnce verifies that when
// a late CONFIRMED webhook and a background reconciliation run concurrently for the same ambiguous
// payment, both complete cleanly and financial budget/promo effects apply exactly once without double spending.
func TestMatrixAU_ConcurrentWebhookAndReconciliation_ConfirmedExactlyOnce(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAU", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-au")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	sharedPID := "tb-shared-au-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: sharedPID,
						AmountCents:       400000,
						Status:            "succeeded",
						ProviderStatus:    "CONFIRMED",
					},
				},
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: sharedPID,
				OrderID:           orderID.String(),
				Status:            "succeeded",
				ProviderStatus:    "CONFIRMED",
				AmountCents:       400000,
				EventKey:          "key-au-" + uuid.NewString(),
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)
	var errWebhook, errRecon error

	gate := make(chan struct{})

	go func() {
		defer wg.Done()
		<-gate
		errWebhook = paySvc.HandleWebhook(ctx, nil, []byte("CONFIRMED"))
	}()

	go func() {
		defer wg.Done()
		<-gate
		errRecon = paySvc.ReconcilePayment(ctx, pID)
	}()

	close(gate)
	wg.Wait()

	require.NoError(t, errWebhook)
	require.NoError(t, errRecon)

	// Verify exact single financial application
	var pStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", pID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pStatus)

	var oStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", oStatus)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkSpentCents, "ZAMK budget spent must be exactly 50,000 cents (no double spend)")
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)

	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)
}

// TestMatrixAV_ConcurrentWebhookAndReconciliation_TerminalFailureReleaseOnce verifies that
// concurrent webhook rejection and reconciliation release the reservation and budget exactly once without deadlock.
func TestMatrixAV_ConcurrentWebhookAndReconciliation_TerminalFailureReleaseOnce(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAV", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-av")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	sharedPID := "tb-shared-av-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: sharedPID,
						AmountCents:       400000,
						Status:            "cancelled",
						ProviderStatus:    "REJECTED",
					},
				},
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: sharedPID,
				OrderID:           orderID.String(),
				Status:            "cancelled",
				ProviderStatus:    "REJECTED",
				AmountCents:       400000,
				EventKey:          "key-av-" + uuid.NewString(),
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)
	var errWebhook, errRecon error

	gate := make(chan struct{})

	go func() {
		defer wg.Done()
		<-gate
		errWebhook = paySvc.HandleWebhook(ctx, nil, []byte("REJECTED"))
	}()

	go func() {
		defer wg.Done()
		<-gate
		errRecon = paySvc.ReconcilePayment(ctx, pID)
	}()

	close(gate)
	wg.Wait()

	require.NoError(t, errWebhook)
	require.NoError(t, errRecon)

	var pStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", pID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", pStatus)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents)
}

// TestMatrixAW_Reconciliation_BackfillsProviderPaymentIDExactlyOnce verifies that reconciliation
// cleanly backfills provider_payment_id on an ambiguous payment row where it was previously NULL.
func TestMatrixAW_Reconciliation_BackfillsProviderPaymentIDExactlyOnce(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAW", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)
	_ = promo

	user := createTestUser(t, client, "user-aw")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	_, _, _ = createTestOrderItem(t, client, orderID, sellerID, 400000, 1)

	_, err := client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	backfillPID := "tb-backfill-aw-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: backfillPID,
						AmountCents:       400000,
						Status:            "succeeded",
						ProviderStatus:    "CONFIRMED",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	var initialPID *string
	err = client.Pool.QueryRow(ctx, "SELECT id, provider_payment_id FROM payments WHERE order_id = $1", orderID).Scan(&pID, &initialPID)
	require.NoError(t, err)
	assert.Nil(t, initialPID, "ProviderPaymentID must be NULL on ambiguous Init")

	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	var backfilledPID *string
	err = client.Pool.QueryRow(ctx, "SELECT provider_payment_id FROM payments WHERE id = $1", pID).Scan(&backfilledPID)
	require.NoError(t, err)
	require.NotNil(t, backfilledPID)
	assert.Equal(t, backfillPID, *backfilledPID)
}

// TestMatrixAX_SameOrderCreatePaymentWhileUnknown_MakesZeroSecondInitCalls verifies that
// retry of CreatePayment on an order with unknown init outcome is blocked without making any provider calls.
func TestMatrixAX_SameOrderCreatePaymentWhileUnknown_MakesZeroSecondInitCalls(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAX", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)
	_ = promo

	user := createTestUser(t, client, "user-ax")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	_, err := client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	providerCalls := 0
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			providerCalls++
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// First call times out
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)
	assert.Equal(t, 1, providerCalls)

	// Second call fails fast
	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)
	assert.True(t, errors.Is(err, payments.ErrPaymentOutcomeUncertain))
	assert.Equal(t, 1, providerCalls, "Must make zero second provider Init calls while state is UNKNOWN")
}

// TestMatrixAY_WorkerRestart_DuplicateReconciliation_IsIdempotent verifies that worker restarts
// or repeated reconciliation runs ignore already-settled payments and execute idempotently.
func TestMatrixAY_WorkerRestart_DuplicateReconciliation_IsIdempotent(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAY", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ay")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	// Isolate from previous test unknown payments in shared test database
	_, _ = client.Pool.Exec(ctx, "UPDATE payments SET init_outcome = 'settled_test' WHERE init_outcome = 'unknown'")

	reconPID := "tb-recon-ay-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       400000,
						Status:            "succeeded",
						ProviderStatus:    "CONFIRMED",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	// First pass reconciles to succeeded
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	// Second pass via batch runner: claims 0 candidates
	processed, err := paySvc.ReconcileUnknownPayments(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, processed, "Candidate claimer must skip settled payments")

	// Direct call to ReconcilePayment is safe no-op
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkSpentCents)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)
}

// TestMatrixAZ_TwoWorkerInstances_CannotDoubleApplyTransition verifies that two worker instances
// running ReconcileUnknownPayments concurrently with SKIP LOCKED never double-apply transitions.
func TestMatrixAZ_TwoWorkerInstances_CannotDoubleApplyTransition(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONAZ", marketing.DiscountTypePercent, 2000, 0, 0, nil, 10, false)

	user1 := createTestUser(t, client, "u-az-1")
	user2 := createTestUser(t, client, "u-az-2")
	user3 := createTestUser(t, client, "u-az-3")
	order1 := createTestOrder(t, client, user1)
	order2 := createTestOrder(t, client, user2)
	order3 := createTestOrder(t, client, user3)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user1, user2, user3}, []uuid.UUID{camp.ID})

	// Isolate from previous test unknown payments in shared test database
	_, _ = client.Pool.Exec(ctx, "UPDATE payments SET init_outcome = 'settled_test' WHERE status = 'created' AND init_outcome = 'unknown'")

	for _, oID := range []uuid.UUID{order1, order2, order3} {
		itemID, prodID, varID := createTestOrderItem(t, client, oID, sellerID, 500000, 1)
		items := []marketing.PromotedOrderItemInput{{
			OrderItemID:        itemID,
			ProductID:          prodID,
			ProductVariantID:   varID,
			SellerID:           sellerID,
			BaseUnitPriceCents: 500000,
			Quantity:           1,
		}}
		uID := user1
		if oID == order2 {
			uID = user2
		} else if oID == order3 {
			uID = user3
		}
		err := client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, uID, promo.Code, items, 900, time.Now().UTC())
			require.NoError(t, err)
			return svc.ReserveCheckoutPromoTx(ctx, tx, oID, uID, calc, time.Now().UTC().Add(30*time.Minute))
		})
		require.NoError(t, err)

		_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", oID)
		require.NoError(t, err)
	}

	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: "tb-recon-az-" + oID,
						AmountCents:       400000,
						Status:            "succeeded",
						ProviderStatus:    "CONFIRMED",
					},
				},
			}, nil
		},
	}
	paySvc1 := setupTestPaymentsService(t, client, svc, mockProv)
	paySvc2 := setupTestPaymentsService(t, client, svc, mockProv)

	// Create 3 ambiguous payments
	_, _ = paySvc1.CreatePayment(ctx, user1, order1, "card")
	_, _ = paySvc1.CreatePayment(ctx, user2, order2, "card")
	_, _ = paySvc1.CreatePayment(ctx, user3, order3, "card")

	var wg sync.WaitGroup
	wg.Add(2)
	gate := make(chan struct{})

	go func() {
		defer wg.Done()
		<-gate
		_, _ = paySvc1.ReconcileUnknownPayments(ctx, 10)
	}()

	go func() {
		defer wg.Done()
		<-gate
		_, _ = paySvc2.ReconcileUnknownPayments(ctx, 10)
	}()

	close(gate)
	wg.Wait()

	// Verify all 3 orders paid and exactly 3 * 50,000 = 150,000 cents spent
	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(150000), campAfter.ZamkSpentCents, "Total ZAMK spent must be exactly 150,000 cents (no double spend)")
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)

	for _, oID := range []uuid.UUID{order1, order2, order3} {
		var oStatus string
		err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", oID).Scan(&oStatus)
		require.NoError(t, err)
		assert.Equal(t, "paid", oStatus)
	}
}

// TestMatrixBA_CustomerCancellationBlockedForNonTerminalReconciledState verifies that
// after reconciliation updates payment to AUTHORIZED, customer cancellation remains strictly blocked.
func TestMatrixBA_CustomerCancellationBlockedForNonTerminalReconciledState(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONBA", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-ba")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	reconPID := "tb-recon-ba-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       400000,
						Status:            "pending",
						ProviderStatus:    "AUTHORIZED",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)
	ordersSvc := setupTestOrdersService(t, client, svc)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	// Cancellation must be rejected
	err = ordersSvc.CancelCustomerOrder(ctx, user, orderID)
	require.Error(t, err)

	var oStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "awaiting_payment", oStatus)
}

// TestMatrixBB_AfterAuthoritativeTerminalReconciliation_NormalCancellationRestored verifies
// that once an ambiguous payment is authoritatively reconciled to a terminal failure, normal order cancellation succeeds.
func TestMatrixBB_AfterAuthoritativeTerminalReconciliation_NormalCancellationRestored(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONBB", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bb")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	reconPID := "tb-recon-bb-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       400000,
						Status:            "cancelled",
						ProviderStatus:    "REJECTED",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)
	ordersSvc := setupTestOrdersService(t, client, svc)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	// Reconcile to terminal failure
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	// Order cancellation is restored and succeeds
	err = ordersSvc.CancelCustomerOrder(ctx, user, orderID)
	require.NoError(t, err)

	var oStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", oStatus)
}

// TestMatrixBC_CanonicalPromotionEconomics_RemainUnchanged verifies the exact locked promotion
// economics across the entire lifecycle: base 500,000 cents (5000 RUB), seller promo 75,000 cents (750 RUB),
// commission base 425,000 cents (4250 RUB), ZAMK budget 50,000 cents (500 RUB), customer payable 375,000 cents (3750 RUB).
func TestMatrixBC_CanonicalPromotionEconomics_RemainUnchanged(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	// Co-funded campaign: seller discount 15% (1500 bps), ZAMK share 10% (1000 bps)
	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1500, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "CANONBC", marketing.DiscountTypePercent, 2500, 0, 0, nil, 1, false)

	user := createTestUser(t, client, "user-bc")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	// 1. Calculate & Reserve promo
	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)

		// Assert pre-reservation financial values
		assert.Equal(t, int64(75000), calc.TotalSellerDiscountCents, "Seller discount must be 75,000 cents")
		assert.Equal(t, int64(50000), calc.TotalZamkSubsidyCents, "ZAMK discount must be 50,000 cents")
		assert.Equal(t, int64(375000), calc.TotalCustomerPaidCents, "Customer paid must be 375,000 cents")
		require.Len(t, calc.PromotedLines, 1)
		assert.Equal(t, int64(425000), calc.PromotedLines[0].TotalCommissionBaseCents, "Commission base must be 425,000 cents")

		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Set order total_price_cents to customer payable (375000)
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 375000 WHERE id = $1", orderID)
	require.NoError(t, err)

	// Verify order_item_promotions allocation before payment
	var sellerDisc, zamkDisc, custPaid, commBase int64
	err = client.Pool.QueryRow(ctx, `
		SELECT total_seller_discount_cents, total_zamk_subsidy_cents, total_customer_paid_cents, total_commission_base_cents
		FROM order_item_promotions WHERE order_item_id = $1
	`, itemID).Scan(&sellerDisc, &zamkDisc, &custPaid, &commBase)
	require.NoError(t, err)
	assert.Equal(t, int64(75000), sellerDisc)
	assert.Equal(t, int64(50000), zamkDisc)
	assert.Equal(t, int64(375000), custPaid)
	assert.Equal(t, int64(425000), commBase)

	// 2. Provider Init ambiguous
	reconPID := "tb-recon-bc-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       375000,
						Status:            "succeeded",
						ProviderStatus:    "CONFIRMED",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	// 3. Reconcile to CONFIRMED
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	// 4. Assert post-reconciliation financial state
	var oStatus string
	var oTotal int64
	err = client.Pool.QueryRow(ctx, "SELECT status, total_price_cents FROM orders WHERE id = $1", orderID).Scan(&oStatus, &oTotal)
	require.NoError(t, err)
	assert.Equal(t, "paid", oStatus)
	assert.Equal(t, int64(375000), oTotal)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkSpentCents, "ZAMK budget spent must be exactly 50,000 cents")
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)
}

// TestMatrixBD_CheckOrder_AuthFail_RemainsNonTerminal verifies that when CheckOrder returns AUTH_FAIL,
// the local payment remains non-terminal, promo/budget/claim remain held, and cancellation is blocked.
func TestMatrixBD_CheckOrder_AuthFail_RemainsNonTerminal(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONBD", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bd")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	reconPID := "tb-recon-bd-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       400000,
						Status:            "pending",
						ProviderStatus:    "AUTH_FAIL",
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)
	ordersSvc := setupTestOrdersService(t, client, svc)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	// Reconcile with AUTH_FAIL
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	// 1. Payment remains non-terminal
	var pStatus, pInitOutcome string
	var provPID *string
	err = client.Pool.QueryRow(ctx, "SELECT status, init_outcome, provider_payment_id FROM payments WHERE id = $1", pID).Scan(&pStatus, &pInitOutcome, &provPID)
	require.NoError(t, err)
	assert.True(t, pStatus == "created" || pStatus == "pending", "Payment status MUST be non-terminal (created or pending)")
	assert.Equal(t, "unknown", pInitOutcome, "InitOutcome must remain unknown for future reconciliation")
	require.NotNil(t, provPID)
	assert.Equal(t, reconPID, *provPID)

	// 2. Promo remains RESERVED
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)

	// 3. ZAMK budget remains RESERVED
	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkReservedCents)
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents)

	// 4. Claim remains held
	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount)

	// 5. Cancellation blocked
	err = ordersSvc.CancelCustomerOrder(ctx, user, orderID)
	require.Error(t, err, "Cancellation MUST remain blocked while provider payment session is in retryable AUTH_FAIL")
}

// TestMatrixBE_CheckOrder_AuthFail_ThenConfirmed_SuccessOnce verifies that after AUTH_FAIL,
// a subsequent CheckOrder finding CONFIRMED transitions the payment to success exactly once.
func TestMatrixBE_CheckOrder_AuthFail_ThenConfirmed_SuccessOnce(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONBE", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-be")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	reconPID := "tb-recon-be-" + uuid.NewString()
	providerStatus := "AUTH_FAIL"
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			normStatus := "pending"
			if providerStatus == "CONFIRMED" {
				normStatus = "succeeded"
			}
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       400000,
						Status:            normStatus,
						ProviderStatus:    providerStatus,
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	// Step 1: Reconcile AUTH_FAIL
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	// Step 2: Customer successfully retries on bank form -> provider returns CONFIRMED
	providerStatus = "CONFIRMED"
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	// Assert final success
	var pStatus, oStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", pID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pStatus)

	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", oStatus)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkSpentCents)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)

	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)
}

// TestMatrixBF_CheckOrder_AuthFail_ThenRejected_ReleaseOnce verifies that after AUTH_FAIL,
// a subsequent CheckOrder finding REJECTED executes canonical terminal release exactly once.
func TestMatrixBF_CheckOrder_AuthFail_ThenRejected_ReleaseOnce(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "RECONBF", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bf")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	reconPID := "tb-recon-bf-" + uuid.NewString()
	providerStatus := "AUTH_FAIL"
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			normStatus := "pending"
			if providerStatus == "REJECTED" {
				normStatus = "cancelled"
			}
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       400000,
						Status:            normStatus,
						ProviderStatus:    providerStatus,
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	// Step 1: Reconcile AUTH_FAIL
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	// Step 2: Provider marks payment REJECTED
	providerStatus = "REJECTED"
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	// Assert terminal failure
	var pStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", pID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", pStatus)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)

	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)
}

// TestMatrixBG_Webhook_AuthFail_RemainsNonTerminal verifies that when an incoming webhook carries AUTH_FAIL,
// the payment remains non-terminal and holds are preserved.
func TestMatrixBG_Webhook_AuthFail_RemainsNonTerminal(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "HOOKBG", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bg")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	provPaymentID := "tb-hook-bg-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: provPaymentID,
				PaymentURL:        "https://pay.tbank.ru/bg",
				Status:            "pending",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID,
				OrderID:           orderID.String(),
				Status:            "pending",
				ProviderStatus:    "AUTH_FAIL",
				AmountCents:       400000,
				EventKey:          "key-auth-fail-" + provPaymentID,
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)
	ordersSvc := setupTestOrdersService(t, client, svc)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.NoError(t, err)

	// Deliver webhook with AUTH_FAIL
	err = paySvc.HandleWebhook(ctx, nil, []byte("AUTH_FAIL"))
	require.NoError(t, err)

	// Assert payment remains pending (non-terminal)
	var pStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", orderID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "pending", pStatus)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkReservedCents)

	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount)

	// Cancellation remains blocked
	err = ordersSvc.CancelCustomerOrder(ctx, user, orderID)
	require.Error(t, err)
}

// TestMatrixBH_Webhook_AuthFail_ThenConfirmed_SuccessOnce verifies that after an AUTH_FAIL webhook,
// a subsequent CONFIRMED webhook is accepted and processes success exactly once.
func TestMatrixBH_Webhook_AuthFail_ThenConfirmed_SuccessOnce(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "HOOKBH", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bh")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	provPaymentID := "tb-hook-bh-" + uuid.NewString()
	currentStatus := "AUTH_FAIL"
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: provPaymentID,
				PaymentURL:        "https://pay.tbank.ru/bh",
				Status:            "pending",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			normStatus := "pending"
			if currentStatus == "CONFIRMED" {
				normStatus = "succeeded"
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID,
				OrderID:           orderID.String(),
				Status:            normStatus,
				ProviderStatus:    currentStatus,
				AmountCents:       400000,
				EventKey:          "key-" + currentStatus + "-" + provPaymentID,
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.NoError(t, err)

	// Step 1: Deliver AUTH_FAIL webhook
	err = paySvc.HandleWebhook(ctx, nil, []byte("AUTH_FAIL"))
	require.NoError(t, err)

	// Step 2: Deliver CONFIRMED webhook
	currentStatus = "CONFIRMED"
	err = paySvc.HandleWebhook(ctx, nil, []byte("CONFIRMED"))
	require.NoError(t, err)

	// Assert final success
	var pStatus, oStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", orderID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pStatus)

	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", oStatus)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkSpentCents)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)

	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)
}

// TestMatrixBI_Webhook_AuthFail_ThenRejected_ReleaseOnce verifies that after an AUTH_FAIL webhook,
// a subsequent REJECTED webhook performs terminal release exactly once.
func TestMatrixBI_Webhook_AuthFail_ThenRejected_ReleaseOnce(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "HOOKBI", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bi")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	provPaymentID := "tb-hook-bi-" + uuid.NewString()
	currentStatus := "AUTH_FAIL"
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: provPaymentID,
				PaymentURL:        "https://pay.tbank.ru/bi",
				Status:            "pending",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			normStatus := "pending"
			if currentStatus == "REJECTED" {
				normStatus = "cancelled"
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID,
				OrderID:           orderID.String(),
				Status:            normStatus,
				ProviderStatus:    currentStatus,
				AmountCents:       400000,
				EventKey:          "key-" + currentStatus + "-" + provPaymentID,
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.NoError(t, err)

	// Step 1: Deliver AUTH_FAIL webhook
	err = paySvc.HandleWebhook(ctx, nil, []byte("AUTH_FAIL"))
	require.NoError(t, err)

	// Step 2: Deliver REJECTED webhook
	currentStatus = "REJECTED"
	err = paySvc.HandleWebhook(ctx, nil, []byte("REJECTED"))
	require.NoError(t, err)

	// Assert terminal failure
	var pStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", orderID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", pStatus)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents)
	assert.Equal(t, int64(0), campAfter.ZamkReservedCents)

	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount)
}

// TestMatrixBJ_DuplicateAuthFail_ZeroFinancialSideEffects verifies that duplicate AUTH_FAIL events
// cause zero financial mutations (no releases, no consumption).
func TestMatrixBJ_DuplicateAuthFail_ZeroFinancialSideEffects(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "HOOKBJ", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bj")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	provPaymentID := "tb-hook-bj-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: provPaymentID,
				PaymentURL:        "https://pay.tbank.ru/bj",
				Status:            "pending",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID,
				OrderID:           orderID.String(),
				Status:            "pending",
				ProviderStatus:    "AUTH_FAIL",
				AmountCents:       400000,
				EventKey:          "key-auth-fail-" + string(body),
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.NoError(t, err)

	// Deliver AUTH_FAIL 3 times
	for i := 1; i <= 3; i++ {
		err = paySvc.HandleWebhook(ctx, nil, []byte(fmt.Sprintf("AUTH_FAIL_%d", i)))
		require.NoError(t, err)
	}

	// Financial state remains exactly as reserved
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status)

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkReservedCents)
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents)

	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount)
}

// TestMatrixBK_AuthFail_CompetingOrderBlockedByClaim verifies that while an order is in AUTH_FAIL,
// a competing order for the same customer cannot enter the external payable lifecycle.
func TestMatrixBK_AuthFail_CompetingOrderBlockedByClaim(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "CLAIMBK", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bk")
	order1 := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID1, prodID1, varID1 := createTestOrderItem(t, client, order1, sellerID, 500000, 1)
	items1 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID1,
		ProductID:          prodID1,
		ProductVariantID:   varID1,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items1, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, order1, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", order1)
	require.NoError(t, err)

	provPID1 := "tb-bk-1-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: provPID1,
				PaymentURL:        "https://pay.tbank.ru/bk1",
				Status:            "pending",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPID1,
				OrderID:           order1.String(),
				Status:            "pending",
				ProviderStatus:    "AUTH_FAIL",
				AmountCents:       400000,
				EventKey:          "key-auth-fail-bk",
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	// Init Order 1
	_, err = paySvc.CreatePayment(ctx, user, order1, "card")
	require.NoError(t, err)

	// Deliver AUTH_FAIL to Order 1
	err = paySvc.HandleWebhook(ctx, nil, []byte("AUTH_FAIL"))
	require.NoError(t, err)

	// Now customer creates Order 2 (competing first order)
	order2 := createTestOrder(t, client, user)
	createTestOrderItem(t, client, order2, sellerID, 500000, 1)
	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 500000 WHERE id = $1", order2)
	require.NoError(t, err)

	// Attempting payment on Order 2 must be blocked by customer_first_payment_claims
	_, err = paySvc.CreatePayment(ctx, user, order2, "card")
	require.Error(t, err, "Competing order MUST be blocked while Order 1 is in AUTH_FAIL")
	assert.ErrorIs(t, err, payments.ErrFirstPaymentInProgress)
}

// TestMatrixBL_Confirmed_ThenStaleAuthFail_NoDowngrade verifies that if a payment is already CONFIRMED,
// a subsequent stale AUTH_FAIL event cannot downgrade the payment, restore budget, or release promo.
func TestMatrixBL_Confirmed_ThenStaleAuthFail_NoDowngrade(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "HOOKBL", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bl")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	provPaymentID := "tb-hook-bl-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: provPaymentID,
				PaymentURL:        "https://pay.tbank.ru/bl",
				Status:            "pending",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			statusStr := string(body)
			normStatus := "pending"
			if statusStr == "CONFIRMED" {
				normStatus = "succeeded"
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID,
				OrderID:           orderID.String(),
				Status:            normStatus,
				ProviderStatus:    statusStr,
				AmountCents:       400000,
				EventKey:          "key-" + statusStr + "-" + provPaymentID,
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.NoError(t, err)

	// Step 1: CONFIRMED arrives first
	err = paySvc.HandleWebhook(ctx, nil, []byte("CONFIRMED"))
	require.NoError(t, err)

	// Verify confirmed state
	var pStatus, oStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", orderID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pStatus)

	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", oStatus)

	// Step 2: Out-of-order stale AUTH_FAIL arrives
	err = paySvc.HandleWebhook(ctx, nil, []byte("AUTH_FAIL"))
	require.NoError(t, err)

	// Payment must REMAIN succeeded, order must REMAIN paid, promo must REMAIN consumed
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", orderID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pStatus, "Payment must not be downgraded by stale AUTH_FAIL")

	err = client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&oStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", oStatus, "Order must remain paid")

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status, "Promo usage must not be downgraded")

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkSpentCents)
}

// TestMatrixBM_Rejected_ThenStaleAuthFail_NoReopening verifies that if a payment is already REJECTED,
// a subsequent stale AUTH_FAIL event cannot reopen the terminal payment, re-reserve promo, or recreate claims.
func TestMatrixBM_Rejected_ThenStaleAuthFail_NoReopening(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "HOOKBM", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bm")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	provPaymentID := "tb-hook-bm-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: provPaymentID,
				PaymentURL:        "https://pay.tbank.ru/bm",
				Status:            "pending",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			statusStr := string(body)
			normStatus := "pending"
			if statusStr == "REJECTED" {
				normStatus = "cancelled"
			}
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID,
				OrderID:           orderID.String(),
				Status:            normStatus,
				ProviderStatus:    statusStr,
				AmountCents:       400000,
				EventKey:          "key-" + statusStr + "-" + provPaymentID,
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.NoError(t, err)

	// Step 1: REJECTED arrives
	err = paySvc.HandleWebhook(ctx, nil, []byte("REJECTED"))
	require.NoError(t, err)

	var pStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", orderID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", pStatus)

	// Step 2: Stale AUTH_FAIL arrives
	err = paySvc.HandleWebhook(ctx, nil, []byte("AUTH_FAIL"))
	require.NoError(t, err)

	// Must remain terminal failed
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", orderID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", pStatus, "Payment must not be reopened by stale AUTH_FAIL")

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReleased, usage.Status, "Promo usage must remain released")

	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount, "Claim must not be recreated")
}

// TestMatrixBN_AuthFail_RemainsEligibleForLaterCheckOrderReconciliation verifies that an ambiguous payment
// that received AUTH_FAIL remains eligible for candidate selection and subsequent reconciliation.
func TestMatrixBN_AuthFail_RemainsEligibleForLaterCheckOrderReconciliation(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "POLLBN", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bn")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	reconPID := "tb-recon-bn-" + uuid.NewString()
	providerStatus := "AUTH_FAIL"
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{}, context.DeadlineExceeded
		},
		checkOrderFn: func(ctx context.Context, oID string) (payments.CheckOrderResult, error) {
			normStatus := "pending"
			if providerStatus == "CONFIRMED" {
				normStatus = "succeeded"
			}
			return payments.CheckOrderResult{
				OrderID: oID,
				Payments: []payments.ProviderPaymentItem{
					{
						ProviderPaymentID: reconPID,
						AmountCents:       400000,
						Status:            normStatus,
						ProviderStatus:    providerStatus,
					},
				},
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)
	payRepo := payments.NewRepository(client.Pool)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.Error(t, err)

	var pID uuid.UUID
	err = client.Pool.QueryRow(ctx, "SELECT id FROM payments WHERE order_id = $1", orderID).Scan(&pID)
	require.NoError(t, err)

	// Step 1: Reconcile AUTH_FAIL
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	// Step 2: Invalidate reconciliation_attempted_at to simulate scheduling backoff elapsing
	_, err = client.Pool.Exec(ctx, "UPDATE payments SET reconciliation_attempted_at = now() - interval '1 minute' WHERE id = $1", pID)
	require.NoError(t, err)

	// Step 3: Verify candidate selection claims this payment again!
	candidates, err := payRepo.ClaimReconciliationCandidates(ctx, 10, 30*time.Second)
	require.NoError(t, err)

	var found bool
	for _, c := range candidates {
		if c.ID == pID {
			found = true
			break
		}
	}
	assert.True(t, found, "Payment with AUTH_FAIL MUST remain eligible for subsequent candidate selection")

	// Step 4: Subsequent reconciliation with CONFIRMED succeeds
	providerStatus = "CONFIRMED"
	err = paySvc.ReconcilePayment(ctx, pID)
	require.NoError(t, err)

	var pStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE id = $1", pID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pStatus)
}

// TestMatrixBO_UnknownProviderStatus_FailsClosed verifies that unknown or unrecognized provider statuses
// fail closed with zero financial mutations (no consume, no release, no claim release).
func TestMatrixBO_UnknownProviderStatus_FailsClosed(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	sellerID := createTestSeller(t, client)
	staffID := createTestStaffUser(t, client)

	camp := createApprovedCampaign(t, repo, sellerID, staffID, marketing.FundingModeCofunded, marketing.DiscountTypePercent, 1000, 1000, 1000000, 0)
	promo := createPromoWithLimits(t, svc, sellerID, camp.ID, "HOOKBO", marketing.DiscountTypePercent, 2000, 0, 0, nil, 1, true)

	user := createTestUser(t, client, "user-bo")
	orderID := createTestOrder(t, client, user)
	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{user}, []uuid.UUID{camp.ID})

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 500000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 500000,
		Quantity:           1,
	}}

	err := client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user, promo.Code, items, 900, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, user, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "UPDATE orders SET status = 'awaiting_payment', total_price_cents = 400000 WHERE id = $1", orderID)
	require.NoError(t, err)

	provPaymentID := "tb-hook-bo-" + uuid.NewString()
	mockProv := &testProviderMock{
		createPaymentFn: func(ctx context.Context, input payments.CreatePaymentInput) (payments.ProviderCreatePaymentResult, error) {
			return payments.ProviderCreatePaymentResult{
				ProviderPaymentID: provPaymentID,
				PaymentURL:        "https://pay.tbank.ru/bo",
				Status:            "pending",
			}, nil
		},
		parseWebhookFn: func(ctx context.Context, body []byte) (payments.ProviderWebhookEvent, error) {
			return payments.ProviderWebhookEvent{
				ProviderPaymentID: provPaymentID,
				OrderID:           orderID.String(),
				Status:            "pending",
				ProviderStatus:    "UNRECOGNIZED_STRANGE_STATUS",
				AmountCents:       400000,
				EventKey:          "key-strange-" + provPaymentID,
			}, nil
		},
	}
	paySvc := setupTestPaymentsService(t, client, svc, mockProv)

	_, err = paySvc.CreatePayment(ctx, user, orderID, "card")
	require.NoError(t, err)

	// Deliver webhook with unrecognized status
	err = paySvc.HandleWebhook(ctx, nil, []byte("UNRECOGNIZED_STRANGE_STATUS"))
	require.NoError(t, err)

	// Payment must remain non-terminal (pending), promo reserved, budget reserved, claim held
	var pStatus string
	err = client.Pool.QueryRow(ctx, "SELECT status FROM payments WHERE order_id = $1", orderID).Scan(&pStatus)
	require.NoError(t, err)
	assert.Equal(t, "pending", pStatus)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status, "Promo usage MUST remain reserved")

	campAfter, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), campAfter.ZamkReservedCents, "ZAMK budget MUST remain reserved")
	assert.Equal(t, int64(0), campAfter.ZamkSpentCents, "ZAMK budget MUST NOT be spent")

	var claimCount int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM customer_first_payment_claims WHERE user_id = $1", user).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount, "First payment claim MUST remain held")
}
