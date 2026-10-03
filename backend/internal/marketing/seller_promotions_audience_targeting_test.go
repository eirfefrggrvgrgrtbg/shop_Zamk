package marketing_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func createOrderWithStatus(t *testing.T, client *postgres.Client, userID uuid.UUID, status string) uuid.UUID {
	ctx := context.Background()
	testutil.AssertTestDatabase(t, client.Pool)
	orderID := uuid.New()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, 100000, 'RUB', 'Customer', '+79991234567', 'cust@test.com', 'Moscow', now(), now())
	`, orderID, userID, status)
	require.NoError(t, err)
	return orderID
}

func createPaymentWithStatus(t *testing.T, client *postgres.Client, orderID uuid.UUID, status string) uuid.UUID {
	ctx := context.Background()
	testutil.AssertTestDatabase(t, client.Pool)
	paymentID := uuid.New()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, status, amount_cents, currency, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, 'tbank', $3, 100000, 'RUB', $4, now(), now())
	`, paymentID, orderID, status, uuid.New().String())
	require.NoError(t, err)
	return paymentID
}

func cleanupAudienceFixtures(client *postgres.Client, userIDs, orderIDs, campaignIDs, sellerIDs []uuid.UUID) {
	ctx := context.Background()
	for _, oid := range orderIDs {
		if oid != uuid.Nil {
			_, _ = client.Pool.Exec(ctx, "DELETE FROM payments WHERE order_id = $1", oid)
		}
	}
	cleanupExactFixtures(client, sellerIDs, userIDs, orderIDs, campaignIDs)
}

// ============================================================================
// PART 1: TESTS A - R (Direct repository tests on CountCustomerSuccessfullyPaidOrdersTx)
// ============================================================================
func TestAudienceTargeting_CanonicalEverPaidHistory_DirectRepo(t *testing.T) {
	client, _, _ := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)
	ctx := context.Background()

	t.Run("TestA_UserWith0Orders", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-a")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, nil, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("TestB_OrderPaidStatusWithoutPaymentRowMustNotCount", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-b")
		orderID := createOrderWithStatus(t, client, userID, "paid")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		// Anti-loophole: orders.status = 'paid' without a payment row MUST NOT COUNT!
		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "orders.status = 'paid' without payments row must not count as paid history")
	})

	t.Run("TestC_OrderPaidStatusWithFailedPaymentMustNotCount", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-c")
		orderID := createOrderWithStatus(t, client, userID, "paid")
		createPaymentWithStatus(t, client, orderID, "failed")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("TestD_OrderPaidStatusWithPendingPaymentMustNotCount", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-d")
		orderID := createOrderWithStatus(t, client, userID, "paid")
		createPaymentWithStatus(t, client, orderID, "pending")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("TestE_OrderPaidStatusWithSucceededPaymentCounts", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-e")
		orderID := createOrderWithStatus(t, client, userID, "paid")
		createPaymentWithStatus(t, client, orderID, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("TestF_OrderAssemblingWithSucceededPaymentCounts", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-f")
		orderID := createOrderWithStatus(t, client, userID, "assembling")
		createPaymentWithStatus(t, client, orderID, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("TestG_OrderPackedWithSucceededPaymentCounts", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-g")
		orderID := createOrderWithStatus(t, client, userID, "packed")
		createPaymentWithStatus(t, client, orderID, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("TestH_OrderShippedWithSucceededPaymentCounts", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-h")
		orderID := createOrderWithStatus(t, client, userID, "shipped")
		createPaymentWithStatus(t, client, orderID, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("TestI_OrderDeliveredWithSucceededPaymentCounts", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-i")
		orderID := createOrderWithStatus(t, client, userID, "delivered")
		createPaymentWithStatus(t, client, orderID, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("TestJ_OrderAwaitingPaymentWithSucceededPaymentCounts", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-j")
		orderID := createOrderWithStatus(t, client, userID, "awaiting_payment")
		createPaymentWithStatus(t, client, orderID, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("TestK_OrderCreatedStatusWithSucceededPaymentCounts", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-k")
		orderID := createOrderWithStatus(t, client, userID, "created")
		createPaymentWithStatus(t, client, orderID, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("TestL_OrderCancelledAfterSucceededPaymentCounts", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-l")
		orderID := createOrderWithStatus(t, client, userID, "cancelled")
		createPaymentWithStatus(t, client, orderID, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "post-payment cancellation still represents a historically paid customer")
	})

	t.Run("TestM_OrderCancelledBeforePaymentDoesNotCount", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-m")
		orderID := createOrderWithStatus(t, client, userID, "cancelled")
		createPaymentWithStatus(t, client, orderID, "failed")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("TestN_MultiplePaymentAttemptsForSameOrderCountsOnce", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-n")
		orderID := createOrderWithStatus(t, client, userID, "paid")
		createPaymentWithStatus(t, client, orderID, "failed")
		createPaymentWithStatus(t, client, orderID, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "multiple payments on same order must count distinctly as 1 order")
	})

	t.Run("TestO_MultipleSucceededPaymentsForSameOrderCountsOnce", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-o")
		orderID := createOrderWithStatus(t, client, userID, "paid")
		createPaymentWithStatus(t, client, orderID, "succeeded")
		createPaymentWithStatus(t, client, orderID, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderID}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "multiple succeeded payments on same order must count distinctly as 1 order")
	})

	t.Run("TestP_MultipleOrdersEachWithSucceededPaymentCountsN", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-p")
		order1 := createOrderWithStatus(t, client, userID, "delivered")
		createPaymentWithStatus(t, client, order1, "succeeded")
		order2 := createOrderWithStatus(t, client, userID, "shipped")
		createPaymentWithStatus(t, client, order2, "succeeded")
		order3 := createOrderWithStatus(t, client, userID, "paid")
		createPaymentWithStatus(t, client, order3, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{order1, order2, order3}, nil, nil)

		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("TestQ_ExcludeOrderIDWorksProperly", func(t *testing.T) {
		userID := createTestUser(t, client, "aud-q")
		orderCurrent := createOrderWithStatus(t, client, userID, "created")
		createPaymentWithStatus(t, client, orderCurrent, "succeeded")
		orderPast := createOrderWithStatus(t, client, userID, "delivered")
		createPaymentWithStatus(t, client, orderPast, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, []uuid.UUID{orderCurrent, orderPast}, nil, nil)

		// Excluding current order yields 1
		count, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, orderCurrent)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Without exclude yields 2
		countAll, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userID, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 2, countAll)
	})

	t.Run("TestR_UserIsolation", func(t *testing.T) {
		userA := createTestUser(t, client, "aud-ra")
		userB := createTestUser(t, client, "aud-rb")
		orderA := createOrderWithStatus(t, client, userA, "delivered")
		createPaymentWithStatus(t, client, orderA, "succeeded")
		defer cleanupAudienceFixtures(client, []uuid.UUID{userA, userB}, []uuid.UUID{orderA}, nil, nil)

		countA, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userA, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 1, countA)

		countB, err := marketing.CountCustomerSuccessfullyPaidOrdersTx(ctx, client.Pool, userB, uuid.Nil)
		require.NoError(t, err)
		assert.Equal(t, 0, countB, "User A payments must never count towards User B")
	})
}

// ============================================================================
// PART 2: TESTS S - AC (FIRST_PAID_ORDER promo in checkout calculation)
// ============================================================================
func TestAudienceTargeting_FirstPaidOrder(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)
	sellerID := createTestSeller(t, client)
	ctx := context.Background()

	code := uniqueCode("FIRST")
	req := marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		MinOrderSubtotalCents: 1000,
		AudienceType:          marketing.AudienceFirstPaidOrder,
	}
	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignID := res.CampaignID

	defer cleanupAudienceFixtures(client, nil, nil, []uuid.UUID{campaignID}, []uuid.UUID{sellerID})

	testCases := []struct {
		name          string
		orderStatus   string
		paymentStatus string
		noPaymentRow  bool
		expectErr     error
	}{
		{name: "TestS_PassesWith0Orders", expectErr: nil},
		{name: "TestT_RejectedWhenPaidWithSucceededPayment", orderStatus: "paid", paymentStatus: "succeeded", expectErr: marketing.ErrPromoFirstOrderOnly},
		{name: "TestU_RejectedWhenAssemblingWithSucceededPayment", orderStatus: "assembling", paymentStatus: "succeeded", expectErr: marketing.ErrPromoFirstOrderOnly},
		{name: "TestV_RejectedWhenPackedWithSucceededPayment", orderStatus: "packed", paymentStatus: "succeeded", expectErr: marketing.ErrPromoFirstOrderOnly},
		{name: "TestW_RejectedWhenShippedWithSucceededPayment", orderStatus: "shipped", paymentStatus: "succeeded", expectErr: marketing.ErrPromoFirstOrderOnly},
		{name: "TestX_RejectedWhenDeliveredWithSucceededPayment", orderStatus: "delivered", paymentStatus: "succeeded", expectErr: marketing.ErrPromoFirstOrderOnly},
		{name: "TestY_RejectedWhenAwaitingPaymentWithSucceededPayment", orderStatus: "awaiting_payment", paymentStatus: "succeeded", expectErr: marketing.ErrPromoFirstOrderOnly},
		{name: "TestZ_RejectedWhenCreatedWithSucceededPayment", orderStatus: "created", paymentStatus: "succeeded", expectErr: marketing.ErrPromoFirstOrderOnly},
		{name: "TestAA_RejectedWhenCancelledAfterSucceededPayment", orderStatus: "cancelled", paymentStatus: "succeeded", expectErr: marketing.ErrPromoFirstOrderOnly},
		{name: "TestAB_PassesWhenCancelledBeforePayment", orderStatus: "cancelled", paymentStatus: "failed", expectErr: nil},
		{name: "TestAC_PassesWhenStatusPaidWithoutPaymentRow", orderStatus: "paid", noPaymentRow: true, expectErr: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			userID := createTestUser(t, client, "aud-fpo")
			checkoutOrderID := createTestOrder(t, client, userID)
			itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, checkoutOrderID, sellerID, uuid.Nil, 100000, 1)

			var createdOrderIDs = []uuid.UUID{checkoutOrderID}
			if tc.orderStatus != "" {
				histOrderID := createOrderWithStatus(t, client, userID, tc.orderStatus)
				createdOrderIDs = append(createdOrderIDs, histOrderID)
				if !tc.noPaymentRow && tc.paymentStatus != "" {
					createPaymentWithStatus(t, client, histOrderID, tc.paymentStatus)
				}
			}
			defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, createdOrderIDs, nil, nil)

			items := []marketing.PromotedOrderItemInput{
				{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
			}

			err = client.RunInTx(ctx, func(tx pgx.Tx) error {
				calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
				if tc.expectErr != nil {
					require.Error(t, err)
					assert.ErrorIs(t, err, tc.expectErr)
				} else {
					require.NoError(t, err)
					assert.NotNil(t, calc)
					assert.True(t, calc.IsFirstOrder)
					assert.Equal(t, int64(10000), calc.TotalSellerDiscountCents)
				}
				return nil
			})
			require.NoError(t, err)
		})
	}
}

// ============================================================================
// PART 3: TESTS AD - AM (REPEAT_CUSTOMERS promo in checkout calculation)
// ============================================================================
func TestAudienceTargeting_RepeatCustomers(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)
	sellerID := createTestSeller(t, client)
	ctx := context.Background()

	code := uniqueCode("REPEAT")
	req := marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1500,
		MinOrderSubtotalCents: 1000,
		AudienceType:          marketing.AudienceRepeatCustomer,
	}
	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	campaignID := res.CampaignID

	defer cleanupAudienceFixtures(client, nil, nil, []uuid.UUID{campaignID}, []uuid.UUID{sellerID})

	testCases := []struct {
		name          string
		orderStatus   string
		paymentStatus string
		noPaymentRow  bool
		expectErr     error
	}{
		{name: "TestAD_RejectedWhenUserHas0Orders", expectErr: marketing.ErrPromoRepeatCustomerRequired},
		{name: "TestAE_RejectedWhenStatusPaidWithoutPaymentRow", orderStatus: "paid", noPaymentRow: true, expectErr: marketing.ErrPromoRepeatCustomerRequired},
		{name: "TestAF_RejectedWhenOnlyFailedPayments", orderStatus: "created", paymentStatus: "failed", expectErr: marketing.ErrPromoRepeatCustomerRequired},
		{name: "TestAG_PassesWhenStatusPaidWithSucceededPayment", orderStatus: "paid", paymentStatus: "succeeded", expectErr: nil},
		{name: "TestAH_PassesWhenStatusAssemblingWithSucceededPayment", orderStatus: "assembling", paymentStatus: "succeeded", expectErr: nil},
		{name: "TestAI_PassesWhenStatusPackedWithSucceededPayment", orderStatus: "packed", paymentStatus: "succeeded", expectErr: nil},
		{name: "TestAJ_PassesWhenStatusShippedWithSucceededPayment", orderStatus: "shipped", paymentStatus: "succeeded", expectErr: nil},
		{name: "TestAK_PassesWhenStatusDeliveredWithSucceededPayment", orderStatus: "delivered", paymentStatus: "succeeded", expectErr: nil},
		{name: "TestAL_PassesWhenStatusAwaitingPaymentWithSucceededPayment", orderStatus: "awaiting_payment", paymentStatus: "succeeded", expectErr: nil},
		{name: "TestAM_PassesWhenStatusCancelledWithSucceededPayment", orderStatus: "cancelled", paymentStatus: "succeeded", expectErr: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			userID := createTestUser(t, client, "aud-rep")
			checkoutOrderID := createTestOrder(t, client, userID)
			itemID, prodID, varID := createTestOrderItemInFulfillment(t, client, checkoutOrderID, sellerID, uuid.Nil, 100000, 1)

			var createdOrderIDs = []uuid.UUID{checkoutOrderID}
			if tc.orderStatus != "" {
				histOrderID := createOrderWithStatus(t, client, userID, tc.orderStatus)
				createdOrderIDs = append(createdOrderIDs, histOrderID)
				if !tc.noPaymentRow && tc.paymentStatus != "" {
					createPaymentWithStatus(t, client, histOrderID, tc.paymentStatus)
				}
			}
			defer cleanupAudienceFixtures(client, []uuid.UUID{userID}, createdOrderIDs, nil, nil)

			items := []marketing.PromotedOrderItemInput{
				{OrderItemID: itemID, ProductID: prodID, ProductVariantID: varID, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
			}

			err = client.RunInTx(ctx, func(tx pgx.Tx) error {
				calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
				if tc.expectErr != nil {
					require.Error(t, err)
					assert.ErrorIs(t, err, tc.expectErr)
				} else {
					require.NoError(t, err)
					assert.NotNil(t, calc)
					assert.False(t, calc.IsFirstOrder)
					assert.Equal(t, int64(15000), calc.TotalSellerDiscountCents)
				}
				return nil
			})
			require.NoError(t, err)
		})
	}
}

// ============================================================================
// PART 4: TESTS AN - AO (ALL_CUSTOMERS and Immutability)
// ============================================================================
func TestAudienceTargeting_AllCustomers_And_Immutability(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	testutil.AssertTestDatabase(t, client.Pool)
	ctx := context.Background()

	t.Run("TestAN_AllCustomersAppliesRegardlessOfOrderHistory", func(t *testing.T) {
		sellerID := createTestSeller(t, client)
		code := uniqueCode("ALL")
		req := marketing.CreateSellerPromoRequest{
			Code:                  code,
			DiscountType:          marketing.DiscountTypePercent,
			DiscountValueBps:      1000,
			MinOrderSubtotalCents: 1000,
			AudienceType:          marketing.AudienceAllCustomers,
		}
		res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
		require.NoError(t, err)
		defer cleanupAudienceFixtures(client, nil, nil, []uuid.UUID{res.CampaignID}, []uuid.UUID{sellerID})

		// 1. User with 0 orders
		user0 := createTestUser(t, client, "aud-all0")
		order0 := createTestOrder(t, client, user0)
		item0, p0, v0 := createTestOrderItemInFulfillment(t, client, order0, sellerID, uuid.Nil, 100000, 1)
		defer cleanupAudienceFixtures(client, []uuid.UUID{user0}, []uuid.UUID{order0}, nil, nil)

		err = client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user0, code, []marketing.PromotedOrderItemInput{
				{OrderItemID: item0, ProductID: p0, ProductVariantID: v0, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
			}, 1000, time.Now().UTC())
			require.NoError(t, err)
			assert.NotNil(t, calc)
			assert.False(t, calc.IsFirstOrder)
			return nil
		})
		require.NoError(t, err)

		// 2. User with historical paid order
		user1 := createTestUser(t, client, "aud-all1")
		hist1 := createOrderWithStatus(t, client, user1, "delivered")
		createPaymentWithStatus(t, client, hist1, "succeeded")
		order1 := createTestOrder(t, client, user1)
		item1, p1, v1 := createTestOrderItemInFulfillment(t, client, order1, sellerID, uuid.Nil, 100000, 1)
		defer cleanupAudienceFixtures(client, []uuid.UUID{user1}, []uuid.UUID{hist1, order1}, nil, nil)

		err = client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, user1, code, []marketing.PromotedOrderItemInput{
				{OrderItemID: item1, ProductID: p1, ProductVariantID: v1, SellerID: sellerID, BaseUnitPriceCents: 100000, Quantity: 1},
			}, 1000, time.Now().UTC())
			require.NoError(t, err)
			assert.NotNil(t, calc)
			assert.False(t, calc.IsFirstOrder)
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("TestAO_AudienceImmutabilityRejectedByAPI", func(t *testing.T) {
		sellerID := createTestSeller(t, client)
		code := uniqueCode("IMMUT")
		req := marketing.CreateSellerPromoRequest{
			Code:                  code,
			DiscountType:          marketing.DiscountTypePercent,
			DiscountValueBps:      1000,
			MinOrderSubtotalCents: 1000,
			AudienceType:          marketing.AudienceFirstPaidOrder,
		}
		res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
		require.NoError(t, err)
		defer cleanupAudienceFixtures(client, nil, nil, []uuid.UUID{res.CampaignID}, []uuid.UUID{sellerID})

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		handler := marketing.NewHandler(svc, logger)

		// Attempt to modify audienceType via update endpoint
		payload := []byte(`{"audienceType": "REPEAT_CUSTOMERS"}`)
		httpReq := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/seller/promotions/%s", res.ID.String()), bytes.NewReader(payload))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq = httpReq.WithContext(newTestContextWithSeller(sellerID))

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", res.ID.String())
		httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		handler.UpdateSellerPromotion(rr, httpReq)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var respBody map[string]any
		err = json.Unmarshal(rr.Body.Bytes(), &respBody)
		require.NoError(t, err)
		assert.Equal(t, "immutable_field", respBody["code"])
	})
}

func TestMigration000102_UpDownLossyRepeatCustomers(t *testing.T) {
	ctx := context.Background()
	client, err := postgres.NewClient(ctx, testutil.GetTestDatabaseURL())
	require.NoError(t, err)
	defer client.Close()

	testutil.AssertTestDatabase(t, client.Pool)

	// Ensure we start from an up-migrated state
	_, _ = client.Pool.Exec(ctx, `
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'promo_codes' AND column_name = 'audience_type'
			) THEN
				ALTER TABLE promo_codes ADD COLUMN audience_type VARCHAR(32) NOT NULL DEFAULT 'ALL_CUSTOMERS';
				IF EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_name = 'promo_codes' AND column_name = 'first_paid_order_only'
				) THEN
					UPDATE promo_codes SET audience_type = 'FIRST_PAID_ORDER' WHERE first_paid_order_only = true;
					ALTER TABLE promo_codes DROP COLUMN first_paid_order_only;
				END IF;
				ALTER TABLE promo_codes ADD CONSTRAINT chk_promo_codes_audience_type CHECK (audience_type IN ('ALL_CUSTOMERS', 'FIRST_PAID_ORDER', 'REPEAT_CUSTOMERS'));
			END IF;
		END $$;
	`)

	repo := marketing.NewRepository(client.Pool)
	svc := marketing.NewService(repo, client.Pool)

	sellerID := createTestSeller(t, client)
	basePromo, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                  uniqueCode("MIGBASE"),
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		MinOrderSubtotalCents: 1000,
		AudienceType:          marketing.AudienceAllCustomers,
	})
	require.NoError(t, err)
	campaignID := basePromo.CampaignID
	defer cleanupAudienceFixtures(client, nil, nil, []uuid.UUID{campaignID}, []uuid.UUID{sellerID})

	idAll := uuid.New()
	idFirst := uuid.New()
	idRepeat := uuid.New()

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO promo_codes (id, campaign_id, seller_id, code, discount_type, discount_value_bps, discount_value_fixed_cents, min_order_subtotal_cents, audience_type, is_active, created_at, updated_at)
		VALUES
			($1, $4, $8, $5, 'percent', 1000, 0, 0, 'ALL_CUSTOMERS', true, now(), now()),
			($2, $4, $8, $6, 'percent', 1000, 0, 0, 'FIRST_PAID_ORDER', true, now(), now()),
			($3, $4, $8, $7, 'percent', 1000, 0, 0, 'REPEAT_CUSTOMERS', true, now(), now())
	`, idAll, idFirst, idRepeat, campaignID, uniqueCode("MIGALL"), uniqueCode("MIGFIRST"), uniqueCode("MIGREP"), sellerID)
	require.NoError(t, err)

	// --- 1. RUN DOWN MIGRATION (000102 DOWN) ---
	_, err = client.Pool.Exec(ctx, `
		ALTER TABLE promo_codes ADD COLUMN first_paid_order_only BOOLEAN NOT NULL DEFAULT false;
		UPDATE promo_codes SET first_paid_order_only = (audience_type = 'FIRST_PAID_ORDER');
		ALTER TABLE promo_codes DROP CONSTRAINT IF EXISTS chk_promo_codes_audience_type;
		ALTER TABLE promo_codes DROP COLUMN audience_type;
	`)
	require.NoError(t, err)

	// Verify DOWN mappings:
	// ALL_CUSTOMERS -> first_paid_order_only = false
	var flagAll bool
	err = client.Pool.QueryRow(ctx, "SELECT first_paid_order_only FROM promo_codes WHERE id = $1", idAll).Scan(&flagAll)
	require.NoError(t, err)
	assert.False(t, flagAll, "ALL_CUSTOMERS must map to first_paid_order_only = false in down migration")

	// FIRST_PAID_ORDER -> first_paid_order_only = true
	var flagFirst bool
	err = client.Pool.QueryRow(ctx, "SELECT first_paid_order_only FROM promo_codes WHERE id = $1", idFirst).Scan(&flagFirst)
	require.NoError(t, err)
	assert.True(t, flagFirst, "FIRST_PAID_ORDER must map to first_paid_order_only = true in down migration")

	// REPEAT_CUSTOMERS -> first_paid_order_only = false (LOSSY!)
	var flagRepeat bool
	err = client.Pool.QueryRow(ctx, "SELECT first_paid_order_only FROM promo_codes WHERE id = $1", idRepeat).Scan(&flagRepeat)
	require.NoError(t, err)
	assert.False(t, flagRepeat, "REPEAT_CUSTOMERS must map to first_paid_order_only = false in down migration (lossy conversion)")

	// --- 2. RUN UP MIGRATION (000102 UP) ---
	_, err = client.Pool.Exec(ctx, `
		ALTER TABLE promo_codes ADD COLUMN audience_type VARCHAR(32) NOT NULL DEFAULT 'ALL_CUSTOMERS';
		UPDATE promo_codes SET audience_type = 'FIRST_PAID_ORDER' WHERE first_paid_order_only = true;
		ALTER TABLE promo_codes ADD CONSTRAINT chk_promo_codes_audience_type CHECK (audience_type IN ('ALL_CUSTOMERS', 'FIRST_PAID_ORDER', 'REPEAT_CUSTOMERS'));
		ALTER TABLE promo_codes DROP COLUMN first_paid_order_only;
	`)
	require.NoError(t, err)

	// Verify UP mappings:
	// false -> ALL_CUSTOMERS
	var audAll string
	err = client.Pool.QueryRow(ctx, "SELECT audience_type FROM promo_codes WHERE id = $1", idAll).Scan(&audAll)
	require.NoError(t, err)
	assert.Equal(t, "ALL_CUSTOMERS", audAll)

	// true -> FIRST_PAID_ORDER
	var audFirst string
	err = client.Pool.QueryRow(ctx, "SELECT audience_type FROM promo_codes WHERE id = $1", idFirst).Scan(&audFirst)
	require.NoError(t, err)
	assert.Equal(t, "FIRST_PAID_ORDER", audFirst)

	// former REPEAT_CUSTOMERS that was downgraded to false now becomes ALL_CUSTOMERS (demonstrating lossy round-trip!)
	var audRepeatLossy string
	err = client.Pool.QueryRow(ctx, "SELECT audience_type FROM promo_codes WHERE id = $1", idRepeat).Scan(&audRepeatLossy)
	require.NoError(t, err)
	assert.Equal(t, "ALL_CUSTOMERS", audRepeatLossy, "Proves lossy round-trip: former REPEAT_CUSTOMERS cannot be restored from legacy boolean")
}
