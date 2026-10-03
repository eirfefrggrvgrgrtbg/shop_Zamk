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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

func uniqueCode(prefix string) string {
	return fmt.Sprintf("%s%s", prefix, strings.ToUpper(uuid.New().String()[:8]))
}

func newTestContextWithSeller(sellerID uuid.UUID) context.Context {
	return context.WithValue(context.Background(), "sellerID", sellerID)
}

func cleanupExactFixtures(client *postgres.Client, sellerIDs, userIDs, orderIDs, campaignIDs []uuid.UUID) {
	ctx := context.Background()
	for _, cid := range campaignIDs {
		if cid == uuid.Nil {
			continue
		}
		_, _ = client.Pool.Exec(ctx, "DELETE FROM order_item_promotions WHERE campaign_id = $1", cid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM promo_code_usages WHERE campaign_id = $1", cid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM promo_codes WHERE campaign_id = $1", cid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM marketing_campaigns WHERE id = $1", cid)
	}
	for _, oid := range orderIDs {
		if oid == uuid.Nil {
			continue
		}
		_, _ = client.Pool.Exec(ctx, "DELETE FROM order_reservations WHERE order_id = $1", oid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM order_item_promotions WHERE order_id = $1", oid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM promo_code_usages WHERE order_id = $1", oid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM order_items WHERE order_id = $1", oid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE order_id = $1", oid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", oid)
	}
	for _, sid := range sellerIDs {
		if sid == uuid.Nil {
			continue
		}
		_, _ = client.Pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE seller_id = $1", sid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM product_price_history WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM products WHERE seller_id = $1", sid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sid)
	}
	for _, uid := range userIDs {
		if uid == uuid.Nil {
			continue
		}
		_, _ = client.Pool.Exec(ctx, "DELETE FROM orders WHERE user_id = $1", uid)
		_, _ = client.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", uid)
	}
}

func createTestOrderItemInFulfillment(t *testing.T, client *postgres.Client, orderID, sellerID, fulfillmentID uuid.UUID, priceCents int64, quantity int) (uuid.UUID, uuid.UUID, uuid.UUID) {
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

	var existingFulID uuid.UUID
	err = client.Pool.QueryRow(ctx, `SELECT id FROM order_fulfillments WHERE order_id = $1 AND seller_id = $2`, orderID, sellerID).Scan(&existingFulID)
	if err != nil {
		if fulfillmentID == uuid.Nil {
			fulfillmentID = uuid.New()
		}
		_, err = client.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
			VALUES ($1, $2, $3, 'awaiting_payment', $4, 1500, $4, now(), now())
		`, fulfillmentID, orderID, sellerID, priceCents*int64(quantity))
		require.NoError(t, err)
	} else {
		fulfillmentID = existingFulID
	}

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

func setupMarketingHTTPHandler(svc *marketing.Service) (*marketing.Handler, *chi.Mux) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := marketing.NewHandler(svc, logger)
	r := chi.NewRouter()
	r.Get("/api/seller/promotions", h.ListSellerPromotions)
	r.Post("/api/seller/promotions", h.CreateSellerPromotion)
	r.Patch("/api/seller/promotions/{id}", h.UpdateSellerPromotion)
	return h, r
}

// -------------------------------------------------------------
// MATRIX A: Seller creates PERCENT promo
// -------------------------------------------------------------
func TestMatrixA_SellerCreatesPercentPromo(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs) }()

	ctx := context.Background()
	code := uniqueCode("PCT")
	req := marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		MinOrderSubtotalCents: 50000,
		FirstPaidOrderOnly:    true,
		PerCustomerUsageLimit: 2,
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	campaignIDs = append(campaignIDs, res.CampaignID)

	assert.Equal(t, code, res.Code)
	assert.Equal(t, marketing.DiscountTypePercent, res.DiscountType)
	assert.Equal(t, 1000, res.DiscountValueBps)
	assert.Equal(t, int64(0), res.DiscountValueFixedCents)
	assert.Equal(t, int64(50000), res.MinOrderSubtotalCents)
	assert.True(t, res.FirstPaidOrderOnly)
	assert.Equal(t, 2, res.PerCustomerUsageLimit)
	assert.True(t, res.IsActive)
	assert.Equal(t, "active", res.Status)

	c, err := repo.GetCampaignByID(ctx, res.CampaignID)
	require.NoError(t, err)
	assert.Equal(t, marketing.FundingModeSeller, c.FundingMode)
	assert.Equal(t, marketing.CampaignStatusActive, c.Status)
	assert.Equal(t, 1000, c.SellerDiscountBps)
	assert.Equal(t, int64(0), c.SellerDiscountFixedCents)
	assert.Equal(t, 0, c.RequestedZamkShareBps)
	assert.Equal(t, int64(0), c.RequestedZamkBudgetCapCents)
	assert.Equal(t, 0, c.ApprovedZamkShareBps)
	assert.Equal(t, int64(0), c.ApprovedZamkBudgetCapCents)
	assert.Equal(t, int64(0), c.ZamkReservedCents)
	assert.Equal(t, int64(0), c.ZamkSpentCents)
}

// -------------------------------------------------------------
// MATRIX B: Seller creates FIXED promo
// -------------------------------------------------------------
func TestMatrixB_SellerCreatesFixedPromo(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs) }()

	ctx := context.Background()
	code := uniqueCode("FIX")
	req := marketing.CreateSellerPromoRequest{
		Code:                    code,
		DiscountType:            marketing.DiscountTypeFixed,
		DiscountValueFixedCents: 50000,
		MinOrderSubtotalCents:   200000,
		PerCustomerUsageLimit:   1,
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	campaignIDs = append(campaignIDs, res.CampaignID)

	assert.Equal(t, code, res.Code)
	assert.Equal(t, marketing.DiscountTypeFixed, res.DiscountType)
	assert.Equal(t, int64(50000), res.DiscountValueFixedCents)
	assert.Equal(t, 0, res.DiscountValueBps)
	assert.Equal(t, int64(200000), res.MinOrderSubtotalCents)
	assert.Equal(t, "active", res.Status)

	c, err := repo.GetCampaignByID(ctx, res.CampaignID)
	require.NoError(t, err)
	assert.Equal(t, marketing.FundingModeSeller, c.FundingMode)
	assert.Equal(t, marketing.CampaignStatusActive, c.Status)
	assert.Equal(t, int64(50000), c.SellerDiscountFixedCents)
	assert.Equal(t, 0, c.SellerDiscountBps)
}

// -------------------------------------------------------------
// MATRIX C: Backing campaign + promo created atomically; failure leaves no orphan
// -------------------------------------------------------------
func TestMatrixC_BackingCampaignPlusPromoCreatedAtomically(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, nil)

	ctx := context.Background()
	code := uniqueCode("FAIL")
	req := marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 0, // Invalid: must be > 0
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.Error(t, err)
	require.Nil(t, res)

	var count int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM marketing_campaigns WHERE seller_id = $1", sellerID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "No campaign row should exist after rollback")

	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM promo_codes WHERE seller_id = $1", sellerID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "No promo row should exist after rollback")
}

// -------------------------------------------------------------
// MATRIX D: SELLER funding has zero ZAMK authority/financial fields
// -------------------------------------------------------------
func TestMatrixD_SellerFundingZeroZamkAuthority(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs) }()

	ctx := context.Background()
	code := uniqueCode("ZERO")
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1500,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	camp, err := repo.GetCampaignByID(ctx, res.CampaignID)
	require.NoError(t, err)
	assert.Equal(t, 0, camp.RequestedZamkShareBps)
	assert.Equal(t, int64(0), camp.RequestedZamkBudgetCapCents)
	assert.Equal(t, 0, camp.ApprovedZamkShareBps)
	assert.Equal(t, int64(0), camp.ApprovedZamkBudgetCapCents)
	assert.Equal(t, int64(0), camp.ZamkReservedCents)
	assert.Equal(t, int64(0), camp.ZamkSpentCents)
}

// -------------------------------------------------------------
// MATRIX E: Duplicate code case-insensitive conflict
// -------------------------------------------------------------
func TestMatrixE_DuplicateCodeCaseInsensitiveConflict(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs) }()

	ctx := context.Background()
	code := uniqueCode("DUP")
	res1, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res1.CampaignID)

	// Attempt lowercase duplicate
	_, err = svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             strings.ToLower(code),
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1500,
	})
	require.ErrorIs(t, err, marketing.ErrPromoCodeDuplicate)

	// Attempt mixed-case duplicate
	_, err = svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                    strings.ToUpper(code[:3]) + strings.ToLower(code[3:]),
		DiscountType:            marketing.DiscountTypeFixed,
		DiscountValueFixedCents: 50000,
	})
	require.ErrorIs(t, err, marketing.ErrPromoCodeDuplicate)
}

// -------------------------------------------------------------
// MATRIX F: Seller list contains only own promos
// -------------------------------------------------------------
func TestMatrixF_SellerListContainsOnlyOwnPromos(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	seller1 := createTestSeller(t, client)
	seller2 := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{seller1, seller2}, nil, nil, campaignIDs) }()

	ctx := context.Background()
	code1 := uniqueCode("S1")
	p1, err := svc.CreateSellerPromotion(ctx, seller1, marketing.CreateSellerPromoRequest{
		Code:             code1,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p1.CampaignID)

	code2 := uniqueCode("S2")
	p2, err := svc.CreateSellerPromotion(ctx, seller2, marketing.CreateSellerPromoRequest{
		Code:             code2,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p2.CampaignID)

	s1List, err := svc.ListSellerPromotions(ctx, seller1)
	require.NoError(t, err)
	assert.Len(t, s1List, 1)
	assert.Equal(t, p1.ID, s1List[0].ID)

	s2List, err := svc.ListSellerPromotions(ctx, seller2)
	require.NoError(t, err)
	assert.Len(t, s2List, 1)
	assert.Equal(t, p2.ID, s2List[0].ID)
}

// -------------------------------------------------------------
// MATRIX G: Foreign seller PATCH returns canonical 404 and changes nothing
// -------------------------------------------------------------
func TestMatrixG_ForeignSellerPatchReturnsCanonical404AndChangesNothing(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	seller1 := createTestSeller(t, client)
	seller2 := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{seller1, seller2}, nil, nil, campaignIDs) }()

	ctx := context.Background()
	code := uniqueCode("S1TARG")
	p1, err := svc.CreateSellerPromotion(ctx, seller1, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p1.CampaignID)

	_, router := setupMarketingHTTPHandler(svc)

	// Seller 2 attempts to mutate Seller 1's promo
	reqBody := bytes.NewBufferString(`{"isActive": false}`)
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/seller/promotions/%s", p1.ID.String()), reqBody)
	req = req.WithContext(newTestContextWithSeller(seller2))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "Foreign promo mutation must return 404 Not Found")
	var errResp map[string]any
	err = json.Unmarshal(rr.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "not_found", errResp["code"])

	// Verify Seller 1 promo is unchanged
	promo, err := repo.GetPromoCodeByCode(ctx, code)
	require.NoError(t, err)
	assert.True(t, promo.IsActive, "Seller 1 promo must remain active")
}

// -------------------------------------------------------------
// MATRIX H: Immutable economics cannot be PATCHed
// -------------------------------------------------------------
func TestMatrixH_ImmutableEconomicsCannotBePatched(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	seller := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{seller}, nil, nil, campaignIDs) }()

	ctx := context.Background()
	code := uniqueCode("IMMUT")
	p, err := svc.CreateSellerPromotion(ctx, seller, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	_, router := setupMarketingHTTPHandler(svc)

	forbiddenPayloads := []string{
		`{"code": "NEWCODE"}`,
		`{"discountType": "fixed"}`,
		`{"discountValueBps": 2000}`,
		`{"discountValueFixedCents": 5000}`,
		`{"minOrderSubtotalCents": 10000}`,
		`{"firstPaidOrderOnly": false}`,
		`{"sellerId": "00000000-0000-0000-0000-000000000000"}`,
		`{"campaignId": "00000000-0000-0000-0000-000000000000"}`,
		`{"fundingMode": "zamk"}`,
	}

	for _, payload := range forbiddenPayloads {
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/seller/promotions/%s", p.ID.String()), bytes.NewBufferString(payload))
		req = req.WithContext(newTestContextWithSeller(seller))
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "Payload '%s' must be rejected with 400 Bad Request", payload)
		var errResp map[string]any
		err = json.Unmarshal(rr.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, "immutable_field", errResp["code"])
	}
}

// -------------------------------------------------------------
// MATRIX I: Pause blocks NEW promo application
// -------------------------------------------------------------
func TestMatrixI_PauseBlocksNewPromoApplication(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestStaffUser(t, client)
	orderID := createTestOrder(t, client, userID)
	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
	}()

	ctx := context.Background()
	code := uniqueCode("PAUSE")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	// Pause promo
	f := false
	_, err = svc.UpdateSellerPromotion(ctx, sellerID, p.ID, marketing.UpdateSellerPromoRequest{
		IsActive: &f,
	})
	require.NoError(t, err)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        uuid.New(),
		ProductID:          uuid.New(),
		ProductVariantID:   uuid.New(),
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		return err
	})
	require.ErrorIs(t, err, marketing.ErrPromoInactive, "Paused promo must be rejected during checkout")
}

// -------------------------------------------------------------
// MATRIX J: Pause does not mutate/release an existing reservation
// -------------------------------------------------------------
func TestMatrixJ_PauseDoesNotMutateReleaseExistingReservation(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestStaffUser(t, client)
	orderID := createTestOrder(t, client, userID)
	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, []uuid.UUID{orderID}, campaignIDs)
	}()

	ctx := context.Background()
	code := uniqueCode("HLD")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	itemID, prodID, varID := createTestOrderItem(t, client, orderID, sellerID, 100000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	// Reserve promo during checkout
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, orderID, userID, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Seller pauses promo
	f := false
	_, err = svc.UpdateSellerPromotion(ctx, sellerID, p.ID, marketing.UpdateSellerPromoRequest{
		IsActive: &f,
	})
	require.NoError(t, err)

	// Verify existing reservation remains intact
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, orderID)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, marketing.UsageStatusReserved, usage.Status, "Existing reservation must remain reserved after pause")
}

// -------------------------------------------------------------
// MATRIX K: Reactivate permits new valid application
// -------------------------------------------------------------
func TestMatrixK_ReactivatePermitsNewValidApplication(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestStaffUser(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, nil, campaignIDs) }()

	ctx := context.Background()
	code := uniqueCode("REACTIVE")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	// Pause
	f := false
	_, err = svc.UpdateSellerPromotion(ctx, sellerID, p.ID, marketing.UpdateSellerPromoRequest{IsActive: &f})
	require.NoError(t, err)

	// Reactivate
	tr := true
	_, err = svc.UpdateSellerPromotion(ctx, sellerID, p.ID, marketing.UpdateSellerPromoRequest{IsActive: &tr})
	require.NoError(t, err)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        uuid.New(),
		ProductID:          uuid.New(),
		ProductVariantID:   uuid.New(),
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(10000), calc.TotalSellerDiscountCents)
		return nil
	})
	require.NoError(t, err)
}

// -------------------------------------------------------------
// MATRIX L: Scheduled promo rejected before start
// -------------------------------------------------------------
func TestMatrixL_ScheduledPromoRejectedBeforeStart(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestStaffUser(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, nil, campaignIDs) }()

	ctx := context.Background()
	futureStart := time.Now().UTC().Add(24 * time.Hour)
	code := uniqueCode("SCHED")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
		StartsAt:         &futureStart,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)
	assert.Equal(t, "scheduled", p.Status)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        uuid.New(),
		ProductID:          uuid.New(),
		ProductVariantID:   uuid.New(),
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		return err
	})
	require.ErrorIs(t, err, marketing.ErrPromoNotStarted)
}

// -------------------------------------------------------------
// MATRIX M: Expired promo rejected
// -------------------------------------------------------------
func TestMatrixM_ExpiredPromoRejected(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestStaffUser(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, nil, campaignIDs) }()

	ctx := context.Background()
	nearFuture := time.Now().UTC().Add(100 * time.Millisecond)
	code := uniqueCode("EXP")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
		EndsAt:           &nearFuture,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	time.Sleep(150 * time.Millisecond)

	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        uuid.New(),
		ProductID:          uuid.New(),
		ProductVariantID:   uuid.New(),
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, userID, code, items, 1000, time.Now().UTC())
		return err
	})
	require.ErrorIs(t, err, marketing.ErrPromoExpired)

	list, err := svc.ListSellerPromotions(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, "expired", list[0].Status)
}

// -------------------------------------------------------------
// MATRIX N: Exhausted global limit rejected
// -------------------------------------------------------------
func TestMatrixN_ExhaustedGlobalLimitRejected(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	u1 := createTestStaffUser(t, client)
	u2 := createTestStaffUser(t, client)
	o1 := createTestOrder(t, client, u1)
	o2 := createTestOrder(t, client, u2)
	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{u1, u2}, []uuid.UUID{o1, o2}, campaignIDs)
	}()

	ctx := context.Background()
	limit := 1
	code := uniqueCode("EXH")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
		GlobalUsageLimit: &limit,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	itemID1, prodID1, varID1 := createTestOrderItem(t, client, o1, sellerID, 100000, 1)
	items1 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID1,
		ProductID:          prodID1,
		ProductVariantID:   varID1,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	// Order 1 reserves it
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u1, code, items1, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, o1, u1, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	list, err := svc.ListSellerPromotions(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, "exhausted", list[0].Status)

	itemID2, prodID2, varID2 := createTestOrderItem(t, client, o2, sellerID, 100000, 1)
	items2 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID2,
		ProductID:          prodID2,
		ProductVariantID:   varID2,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	// Order 2 attempts calculation
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u2, code, items2, 1000, time.Now().UTC())
		return err
	})
	require.ErrorIs(t, err, marketing.ErrPromoGlobalLimit)
}

// -------------------------------------------------------------
// MATRIX O: Global limit counts RESERVED + CONSUMED; RELEASED excluded
// -------------------------------------------------------------
func TestMatrixO_GlobalLimitCountsReservedPlusConsumedReleasedExcluded(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	u1 := createTestStaffUser(t, client)
	u2 := createTestStaffUser(t, client)
	u3 := createTestStaffUser(t, client)
	u4 := createTestStaffUser(t, client)
	o1 := createTestOrder(t, client, u1)
	o2 := createTestOrder(t, client, u2)
	o3 := createTestOrder(t, client, u3)
	o4 := createTestOrder(t, client, u4)
	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{u1, u2, u3, u4}, []uuid.UUID{o1, o2, o3, o4}, campaignIDs)
	}()

	ctx := context.Background()
	limit := 2
	code := uniqueCode("RELEXCL")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
		GlobalUsageLimit: &limit,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	itemID1, prodID1, varID1 := createTestOrderItem(t, client, o1, sellerID, 100000, 1)
	items1 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID1,
		ProductID:          prodID1,
		ProductVariantID:   varID1,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	itemID2, prodID2, varID2 := createTestOrderItem(t, client, o2, sellerID, 100000, 1)
	items2 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID2,
		ProductID:          prodID2,
		ProductVariantID:   varID2,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	// Order 1: reserve then consume (consumed = 1)
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u1, code, items1, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, o1, u1, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, o1)
	})
	require.NoError(t, err)

	// Order 2: reserve then release (released = 1)
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u2, code, items2, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, o2, u2, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ReleasePromoForOrderTx(ctx, tx, o2, "user_cancelled")
	})
	require.NoError(t, err)

	// Now committed count is exactly 1 (consumed=1, reserved=0). RELEASED is excluded!
	// Order 3: attempts checkout reservation -> MUST SUCCEED because 1 < limit(2)
	itemID3, prodID3, varID3 := createTestOrderItem(t, client, o3, sellerID, 100000, 1)
	items3 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID3,
		ProductID:          prodID3,
		ProductVariantID:   varID3,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u3, code, items3, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, o3, u3, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err, "Order 3 must succeed because released count does not count against limit")

	// Now committed count is 2 (consumed=1, reserved=1). Limit is reached!
	// Order 4: must be rejected with ErrPromoGlobalLimit
	itemID4, prodID4, varID4 := createTestOrderItem(t, client, o4, sellerID, 100000, 1)
	items4 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID4,
		ProductID:          prodID4,
		ProductVariantID:   varID4,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		_, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u4, code, items4, 1000, time.Now().UTC())
		return err
	})
	require.ErrorIs(t, err, marketing.ErrPromoGlobalLimit)
}

// -------------------------------------------------------------
// MATRIX P: Per-customer limit remains concurrency-safe
// -------------------------------------------------------------
func TestMatrixP_PerCustomerLimitConcurrencySafe(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	u := createTestStaffUser(t, client)
	o1 := createTestOrder(t, client, u)
	o2 := createTestOrder(t, client, u)
	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{u}, []uuid.UUID{o1, o2}, campaignIDs)
	}()

	ctx := context.Background()
	code := uniqueCode("CONCUR")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		PerCustomerUsageLimit: 1,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	itemID1, prodID1, varID1 := createTestOrderItem(t, client, o1, sellerID, 100000, 1)
	items1 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID1,
		ProductID:          prodID1,
		ProductVariantID:   varID1,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	itemID2, prodID2, varID2 := createTestOrderItem(t, client, o2, sellerID, 100000, 1)
	items2 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID2,
		ProductID:          prodID2,
		ProductVariantID:   varID2,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	var wg sync.WaitGroup
	results := make([]error, 2)
	orders := []uuid.UUID{o1, o2}
	itemsList := [][]marketing.PromotedOrderItemInput{items1, items2}

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = client.RunInTx(ctx, func(tx pgx.Tx) error {
				calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u, code, itemsList[idx], 1000, time.Now().UTC())
				if err != nil {
					return err
				}
				return svc.ReserveCheckoutPromoTx(ctx, tx, orders[idx], u, calc, time.Now().UTC().Add(30*time.Minute))
			})
		}(i)
	}
	wg.Wait()

	successCount := 0
	limitErrCount := 0
	for _, res := range results {
		if res == nil {
			successCount++
		} else if assert.ErrorIs(t, res, marketing.ErrPromoCustomerLimit) {
			limitErrCount++
		}
	}
	assert.Equal(t, 1, successCount, "Exactly 1 concurrent reservation should succeed for per-customer limit=1")
	assert.Equal(t, 1, limitErrCount, "Exactly 1 concurrent reservation should be rejected")
}

// -------------------------------------------------------------
// MATRIX Q: PATCH cannot lower global limit below RESERVED + CONSUMED
// -------------------------------------------------------------
func TestMatrixQ_PatchCannotLowerGlobalLimitBelowReservedPlusConsumed(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	u1 := createTestStaffUser(t, client)
	u2 := createTestStaffUser(t, client)
	o1 := createTestOrder(t, client, u1)
	o2 := createTestOrder(t, client, u2)
	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{u1, u2}, []uuid.UUID{o1, o2}, campaignIDs)
	}()

	ctx := context.Background()
	limit := 10
	code := uniqueCode("PTCHGLO")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
		GlobalUsageLimit: &limit,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	itemID1, prodID1, varID1 := createTestOrderItem(t, client, o1, sellerID, 100000, 1)
	items1 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID1,
		ProductID:          prodID1,
		ProductVariantID:   varID1,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	itemID2, prodID2, varID2 := createTestOrderItem(t, client, o2, sellerID, 100000, 1)
	items2 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID2,
		ProductID:          prodID2,
		ProductVariantID:   varID2,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	// Order 1: reserve then consume
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u1, code, items1, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, o1, u1, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, o1)
	})
	require.NoError(t, err)

	// Order 2: reserve (stays reserved)
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u2, code, items2, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, o2, u2, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Committed usage = 2 (1 consumed + 1 reserved)
	tooLow := 1
	_, err = svc.UpdateSellerPromotion(ctx, sellerID, p.ID, marketing.UpdateSellerPromoRequest{
		GlobalUsageLimit: &tooLow,
	})
	require.ErrorIs(t, err, marketing.ErrGlobalLimitBelowUsage)

	validLimit := 2
	res, err := svc.UpdateSellerPromotion(ctx, sellerID, p.ID, marketing.UpdateSellerPromoRequest{
		GlobalUsageLimit: &validLimit,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, *res.GlobalUsageLimit)
}

// -------------------------------------------------------------
// MATRIX R: PATCH cannot lower per-customer limit below existing committed RESERVED + CONSUMED usage
// -------------------------------------------------------------
func TestMatrixR_PatchCannotLowerPerCustomerLimitBelowExistingCommittedUsage(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	u := createTestStaffUser(t, client)
	o1 := createTestOrder(t, client, u)
	o2 := createTestOrder(t, client, u)
	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{u}, []uuid.UUID{o1, o2}, campaignIDs)
	}()

	ctx := context.Background()
	code := uniqueCode("PTCHCUST")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		PerCustomerUsageLimit: 5,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	itemID1, prodID1, varID1 := createTestOrderItem(t, client, o1, sellerID, 100000, 1)
	items1 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID1,
		ProductID:          prodID1,
		ProductVariantID:   varID1,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	itemID2, prodID2, varID2 := createTestOrderItem(t, client, o2, sellerID, 100000, 1)
	items2 := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID2,
		ProductID:          prodID2,
		ProductVariantID:   varID2,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	// Customer reserves 2 orders
	for _, pair := range []struct {
		ord   uuid.UUID
		items []marketing.PromotedOrderItemInput
	}{{o1, items1}, {o2, items2}} {
		err = client.RunInTx(ctx, func(tx pgx.Tx) error {
			calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u, code, pair.items, 1000, time.Now().UTC())
			require.NoError(t, err)
			return svc.ReserveCheckoutPromoTx(ctx, tx, pair.ord, u, calc, time.Now().UTC().Add(30*time.Minute))
		})
		require.NoError(t, err)
	}

	// Lower per-customer limit to 1 (below customer's 2): rejected
	tooLow := 1
	_, err = svc.UpdateSellerPromotion(ctx, sellerID, p.ID, marketing.UpdateSellerPromoRequest{
		PerCustomerUsageLimit: &tooLow,
	})
	require.ErrorIs(t, err, marketing.ErrCustomerLimitBelowUsage)

	validLimit := 2
	res, err := svc.UpdateSellerPromotion(ctx, sellerID, p.ID, marketing.UpdateSellerPromoRequest{
		PerCustomerUsageLimit: &validLimit,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, res.PerCustomerUsageLimit)
}

// -------------------------------------------------------------
// MATRIX S: Seller-created PERCENT promo completes existing checkout/payment lifecycle to consumed
// -------------------------------------------------------------
func TestMatrixS_SellerCreatedPercentPromoCompletesExistingCheckoutPaymentLifecycleToConsumed(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	u := createTestStaffUser(t, client)
	o := createTestOrder(t, client, u)
	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{u}, []uuid.UUID{o}, campaignIDs)
	}()

	ctx := context.Background()
	code := uniqueCode("CYCPCT")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1500, // 15%
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	fulID := uuid.New()
	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, o, sellerID, fulID, 10000, 2)
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, o, sellerID, fulID, 20000, 1)
	items := []marketing.PromotedOrderItemInput{
		{
			OrderItemID:        item1ID,
			ProductID:          prod1ID,
			ProductVariantID:   var1ID,
			SellerID:           sellerID,
			BaseUnitPriceCents: 10000,
			Quantity:           2,
		},
		{
			OrderItemID:        item2ID,
			ProductID:          prod2ID,
			ProductVariantID:   var2ID,
			SellerID:           sellerID,
			BaseUnitPriceCents: 20000,
			Quantity:           1,
		},
	}

	// 1. Checkout calculate & reserve
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(6000), calc.TotalSellerDiscountCents, "15% of (2*100 + 200) = 60 RUB")
		assert.Equal(t, int64(0), calc.TotalZamkSubsidyCents)
		return svc.ReserveCheckoutPromoTx(ctx, tx, o, u, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Verify order_item_promotions rows created
	snap1, err := repo.GetOrderItemPromotion(ctx, item1ID)
	require.NoError(t, err)
	require.NotNil(t, snap1)
	assert.Equal(t, int64(3000), snap1.TotalSellerDiscountCents)
	assert.Equal(t, int64(0), snap1.TotalZamkSubsidyCents)

	// 2. Payment confirmation consumes reservation
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, o)
	})
	require.NoError(t, err)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, o)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)
}

// -------------------------------------------------------------
// MATRIX T: Seller-created FIXED promo completes existing fixed allocation + payment lifecycle to consumed
// -------------------------------------------------------------
func TestMatrixT_SellerCreatedFixedPromoCompletesExistingFixedAllocationPlusPaymentLifecycleToConsumed(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	u := createTestStaffUser(t, client)
	o := createTestOrder(t, client, u)
	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{u}, []uuid.UUID{o}, campaignIDs)
	}()

	ctx := context.Background()
	code := uniqueCode("CYCFIX")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:                    code,
		DiscountType:            marketing.DiscountTypeFixed,
		DiscountValueFixedCents: 30000, // 300 RUB fixed discount
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	fulID := uuid.New()
	item1ID, prod1ID, var1ID := createTestOrderItemInFulfillment(t, client, o, sellerID, fulID, 50000, 1)
	item2ID, prod2ID, var2ID := createTestOrderItemInFulfillment(t, client, o, sellerID, fulID, 50000, 1)
	items := []marketing.PromotedOrderItemInput{
		{
			OrderItemID:        item1ID,
			ProductID:          prod1ID,
			ProductVariantID:   var1ID,
			SellerID:           sellerID,
			BaseUnitPriceCents: 50000,
			Quantity:           1,
		},
		{
			OrderItemID:        item2ID,
			ProductID:          prod2ID,
			ProductVariantID:   var2ID,
			SellerID:           sellerID,
			BaseUnitPriceCents: 50000,
			Quantity:           1,
		},
	}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, int64(30000), calc.TotalSellerDiscountCents)
		assert.Equal(t, int64(0), calc.TotalZamkSubsidyCents)
		return svc.ReserveCheckoutPromoTx(ctx, tx, o, u, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// Verify snapshots
	snap1, err := repo.GetOrderItemPromotion(ctx, item1ID)
	require.NoError(t, err)
	snap2, err := repo.GetOrderItemPromotion(ctx, item2ID)
	require.NoError(t, err)
	assert.Equal(t, int64(30000), snap1.TotalSellerDiscountCents+snap2.TotalSellerDiscountCents)

	// Consume
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, o)
	})
	require.NoError(t, err)

	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, o)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)
}

// -------------------------------------------------------------
// MATRIX U: Historical order_item_promotions / usage economics remain unchanged after allowed operational PATCH
// -------------------------------------------------------------
func TestMatrixU_HistoricalOrderItemPromotionsUsageEconomicsUnchangedAfterOperationalPatch(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	u := createTestStaffUser(t, client)
	o := createTestOrder(t, client, u)
	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{u}, []uuid.UUID{o}, campaignIDs)
	}()

	ctx := context.Background()
	code := uniqueCode("SNAPUNCH")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 2000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	itemID, prodID, varID := createTestOrderItem(t, client, o, sellerID, 100000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, o, u, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	beforeSnap, err := repo.GetOrderItemPromotion(ctx, itemID)
	require.NoError(t, err)
	beforeUsage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, o)
	require.NoError(t, err)

	// Seller updates operational settings: pauses promo, changes limits
	f := false
	newLimit := 100
	newCustLimit := 3
	_, err = svc.UpdateSellerPromotion(ctx, sellerID, p.ID, marketing.UpdateSellerPromoRequest{
		IsActive:              &f,
		GlobalUsageLimit:      &newLimit,
		PerCustomerUsageLimit: &newCustLimit,
	})
	require.NoError(t, err)

	// Verify historical snapshot is 100% field-for-field identical
	afterSnap, err := repo.GetOrderItemPromotion(ctx, itemID)
	require.NoError(t, err)
	assert.Equal(t, beforeSnap.BaseUnitPriceCents, afterSnap.BaseUnitPriceCents)
	assert.Equal(t, beforeSnap.SellerDiscountUnitCents, afterSnap.SellerDiscountUnitCents)
	assert.Equal(t, beforeSnap.TotalSellerDiscountCents, afterSnap.TotalSellerDiscountCents)
	assert.Equal(t, beforeSnap.CustomerPaidUnitPriceCents, afterSnap.CustomerPaidUnitPriceCents)
	assert.Equal(t, beforeSnap.TotalCustomerPaidCents, afterSnap.TotalCustomerPaidCents)
	assert.Equal(t, beforeSnap.CommissionBaseUnitCents, afterSnap.CommissionBaseUnitCents)
	assert.Equal(t, beforeSnap.TotalCommissionBaseCents, afterSnap.TotalCommissionBaseCents)
	assert.Equal(t, beforeSnap.CommissionRateBps, afterSnap.CommissionRateBps)
	assert.Equal(t, beforeSnap.TotalCommissionChargedCents, afterSnap.TotalCommissionChargedCents)

	afterUsage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, o)
	require.NoError(t, err)
	assert.Equal(t, beforeUsage.Status, afterUsage.Status)
	assert.Equal(t, beforeUsage.SellerDiscountCents, afterUsage.SellerDiscountCents)
	assert.Equal(t, beforeUsage.SubsidyCents, afterUsage.SubsidyCents)
}

// -------------------------------------------------------------
// MATRIX V: seller_id cannot be spoofed through Seller HTTP request
// -------------------------------------------------------------
func TestMatrixV_SellerIDCannotBeSpoofedThroughSellerHTTPRequest(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	authSeller := createTestSeller(t, client)
	spoofedSeller := createTestSeller(t, client)
	defer cleanupExactFixtures(client, []uuid.UUID{authSeller, spoofedSeller}, nil, nil, nil)

	_, router := setupMarketingHTTPHandler(svc)

	payload := fmt.Sprintf(`{
		"code": "%s",
		"discountType": "percent",
		"discountValueBps": 1000,
		"sellerId": "%s"
	}`, uniqueCode("SPOOF"), spoofedSeller.String())

	req := httptest.NewRequest("POST", "/api/seller/promotions", bytes.NewBufferString(payload))
	req = req.WithContext(newTestContextWithSeller(authSeller))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Explicit seller_id in body must be rejected with 400 Bad Request")
	var errResp map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_request", errResp["code"])

	// Also test snake_case seller_id
	snakePayload := fmt.Sprintf(`{
		"code": "%s",
		"discountType": "percent",
		"discountValueBps": 1000,
		"seller_id": "%s"
	}`, uniqueCode("SPOOF2"), spoofedSeller.String())

	req2 := httptest.NewRequest("POST", "/api/seller/promotions", bytes.NewBufferString(snakePayload))
	req2 = req2.WithContext(newTestContextWithSeller(authSeller))
	rr2 := httptest.NewRecorder()

	router.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusBadRequest, rr2.Code)

	// Ensure no promo was created under spoofedSeller
	ctx := context.Background()
	var count int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM promo_codes WHERE seller_id = $1", spoofedSeller).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "No promo code must ever be created under the spoofed seller")
}

// -------------------------------------------------------------
// MATRIX W: Canonical maximum valid percent boundary is accepted
// -------------------------------------------------------------
func TestMatrixW_CanonicalMaximumValidPercentBoundaryAccepted(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs) }()

	ctx := context.Background()
	code := uniqueCode("MAXPCT")
	// Canonical upper boundary is 10000 bps (100.00%)
	req := marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 10000,
	}

	res, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.NoError(t, err, "Canonical 10000 bps (100%) must be accepted")
	require.NotNil(t, res)
	campaignIDs = append(campaignIDs, res.CampaignID)

	assert.Equal(t, 10000, res.DiscountValueBps)

	c, err := repo.GetCampaignByID(ctx, res.CampaignID)
	require.NoError(t, err)
	assert.Equal(t, 10000, c.SellerDiscountBps)
}

// -------------------------------------------------------------
// MATRIX X: Value above canonical domain maximum is rejected
// -------------------------------------------------------------
func TestMatrixX_ValueAboveCanonicalDomainMaximumRejected(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	defer cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, nil)

	ctx := context.Background()
	// Value above canonical upper bound (10001 bps)
	req := marketing.CreateSellerPromoRequest{
		Code:             uniqueCode("EXCPCT"),
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 10001,
	}

	_, err := svc.CreateSellerPromotion(ctx, sellerID, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, marketing.ErrInvalidDiscountValue, "Discount above 10000 bps must be rejected")
}

// -------------------------------------------------------------
// MATRIX Y: Failed creation leaves no backing campaign orphan
// -------------------------------------------------------------
func TestMatrixY_FailedCreationLeavesNoBackingCampaignOrphan(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer func() { cleanupExactFixtures(client, []uuid.UUID{sellerID}, nil, nil, campaignIDs) }()

	ctx := context.Background()
	code := uniqueCode("ORPHAN")

	// 1. First promo succeeds
	res, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, res.CampaignID)

	// 2. Second promo with duplicate code fails
	_, err = svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1500,
	})
	require.ErrorIs(t, err, marketing.ErrPromoCodeDuplicate)

	// 3. Third promo with invalid discount fails
	_, err = svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             uniqueCode("INVALID"),
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 10001,
	})
	require.Error(t, err)

	// Verify exact count of campaigns for this seller is exactly 1 (from step 1), zero orphans
	var count int
	err = client.Pool.QueryRow(ctx, "SELECT count(*) FROM marketing_campaigns WHERE seller_id = $1", sellerID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Only the 1 successfully created campaign must exist; zero orphans allowed")
}

// -------------------------------------------------------------
// MATRIX Z: Pause after reservation followed by successful payment consumes existing reservation exactly once
// -------------------------------------------------------------
func TestMatrixZ_PauseAfterReservationFollowedBySuccessfulPaymentConsumesReservationExactlyOnce(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	u := createTestStaffUser(t, client)
	o := createTestOrder(t, client, u)
	var campaignIDs []uuid.UUID
	defer func() {
		cleanupExactFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{u}, []uuid.UUID{o}, campaignIDs)
	}()

	ctx := context.Background()
	code := uniqueCode("PAUSEPAY")
	p, err := svc.CreateSellerPromotion(ctx, sellerID, marketing.CreateSellerPromoRequest{
		Code:             code,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p.CampaignID)

	itemID, prodID, varID := createTestOrderItem(t, client, o, sellerID, 100000, 1)
	items := []marketing.PromotedOrderItemInput{{
		OrderItemID:        itemID,
		ProductID:          prodID,
		ProductVariantID:   varID,
		SellerID:           sellerID,
		BaseUnitPriceCents: 100000,
		Quantity:           1,
	}}

	// 1. Customer reserves promo
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		calc, err := svc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, u, code, items, 1000, time.Now().UTC())
		require.NoError(t, err)
		return svc.ReserveCheckoutPromoTx(ctx, tx, o, u, calc, time.Now().UTC().Add(30*time.Minute))
	})
	require.NoError(t, err)

	// 2. Seller pauses promo
	f := false
	_, err = svc.UpdateSellerPromotion(ctx, sellerID, p.ID, marketing.UpdateSellerPromoRequest{
		IsActive: &f,
	})
	require.NoError(t, err)

	// 3. Payment confirmation occurs: ConsumePromoForOrderTx must succeed even though promo is now paused!
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, o)
	})
	require.NoError(t, err, "Payment confirmation must succeed for already reserved promo even if paused")

	// Verify status is consumed
	usage, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, o)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usage.Status)

	// 4. Duplicate webhook idempotency: calling ConsumePromoForOrderTx again is a no-op
	err = client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ConsumePromoForOrderTx(ctx, tx, o)
	})
	require.NoError(t, err, "Duplicate payment webhook must be idempotent no-op")

	usageAfter, err := repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, client.Pool, o)
	require.NoError(t, err)
	assert.Equal(t, marketing.UsageStatusConsumed, usageAfter.Status)
}

// -------------------------------------------------------------
// MATRIX W: GET /api/seller/promotions runtime HTTP contract & envelope
// -------------------------------------------------------------
func TestMatrixW_ListSellerPromotionsEndpointContract(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	seller1 := createTestSeller(t, client)
	seller2 := createTestSeller(t, client)
	var campaignIDs []uuid.UUID
	defer cleanupExactFixtures(client, []uuid.UUID{seller1, seller2}, nil, nil, campaignIDs)

	ctx := context.Background()
	_, router := setupMarketingHTTPHandler(svc)

	// 1. Unauthorized request without seller context must return 401
	unauthReq := httptest.NewRequest("GET", "/api/seller/promotions", nil)
	unauthRR := httptest.NewRecorder()
	router.ServeHTTP(unauthRR, unauthReq)
	assert.Equal(t, http.StatusUnauthorized, unauthRR.Code, "Request without seller auth must return 401")

	// 2. Empty state for seller1: must return 200 with { items: [], count: 0 } envelope
	req1 := httptest.NewRequest("GET", "/api/seller/promotions", nil)
	req1 = req1.WithContext(newTestContextWithSeller(seller1))
	rr1 := httptest.NewRecorder()
	router.ServeHTTP(rr1, req1)

	require.Equal(t, http.StatusOK, rr1.Code, "Authenticated GET must return 200")
	var resp1 marketing.SellerPromotionsResponse
	err := json.Unmarshal(rr1.Body.Bytes(), &resp1)
	require.NoError(t, err, "Response must unmarshal into SellerPromotionsResponse")
	assert.Equal(t, 0, resp1.Count)
	assert.NotNil(t, resp1.Items, "Items slice must not be nil in JSON envelope")
	assert.Empty(t, resp1.Items)
	// Verify raw JSON contains exact "items" and "count" keys
	var rawMap1 map[string]any
	err = json.Unmarshal(rr1.Body.Bytes(), &rawMap1)
	require.NoError(t, err)
	assert.Contains(t, rawMap1, "items")
	assert.Contains(t, rawMap1, "count")
	assert.Equal(t, float64(0), rawMap1["count"])

	// 3. Create promo for seller1 and verify list envelope includes items
	code := uniqueCode("WLIST")
	p1, err := svc.CreateSellerPromotion(ctx, seller1, marketing.CreateSellerPromoRequest{
		Code:                  code,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1500,
		MinOrderSubtotalCents: 20000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, p1.CampaignID)

	req2 := httptest.NewRequest("GET", "/api/seller/promotions", nil)
	req2 = req2.WithContext(newTestContextWithSeller(seller1))
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)

	require.Equal(t, http.StatusOK, rr2.Code)
	var resp2 marketing.SellerPromotionsResponse
	err = json.Unmarshal(rr2.Body.Bytes(), &resp2)
	require.NoError(t, err)
	assert.Equal(t, 1, resp2.Count)
	require.Len(t, resp2.Items, 1)
	assert.Equal(t, code, resp2.Items[0].Code)
	assert.Equal(t, "active", resp2.Items[0].Status)
	assert.Equal(t, p1.ID, resp2.Items[0].ID)
	assert.Equal(t, 1500, resp2.Items[0].DiscountValueBps)

	// 4. Isolation: seller2 must still see empty list { items: [], count: 0 }
	req3 := httptest.NewRequest("GET", "/api/seller/promotions", nil)
	req3 = req3.WithContext(newTestContextWithSeller(seller2))
	rr3 := httptest.NewRecorder()
	router.ServeHTTP(rr3, req3)

	require.Equal(t, http.StatusOK, rr3.Code)
	var resp3 marketing.SellerPromotionsResponse
	err = json.Unmarshal(rr3.Body.Bytes(), &resp3)
	require.NoError(t, err)
	assert.Equal(t, 0, resp3.Count)
	assert.Empty(t, resp3.Items)
}
