package marketing_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestMarketingDB(t *testing.T) (*postgres.Client, *marketing.Repository, *marketing.Service) {
	ctx := context.Background()
	dbURL := testutil.GetTestDatabaseURL()
	require.True(t, strings.Contains(dbURL, "zamk_test"), "MUST run only against zamk_test")

	client, err := postgres.NewClient(ctx, dbURL)
	require.NoError(t, err)

	testutil.AssertTestDatabase(t, client.Pool)

	// Ensure migrations 000095, 000096, 000097 are applied and up-to-date on zamk_test
	ensureMarketingMigrations(t, client)

	repo := marketing.NewRepository(client.Pool)
	svc := marketing.NewService(repo, client.Pool)

	return client, repo, svc
}

func ensureMarketingMigrations(t *testing.T, client *postgres.Client) {
	ctx := context.Background()

	// Check if latest columns exist in order_item_promotions
	var hasNewCol bool
	_ = client.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns
			WHERE table_name = 'order_item_promotions' AND column_name = 'commission_base_unit_cents'
		)
	`).Scan(&hasNewCol)

	if hasNewCol {
		return
	}

	root := findRepoRoot()
	require.NotEmpty(t, root, "failed to find repo root")

	// Apply down migrations cleanly first if old version of tables exist
	downFiles := []string{
		"000097_create_order_item_promotions.down.sql",
		"000096_create_product_price_history.down.sql",
		"000095_create_marketing_campaigns_and_promotions.down.sql",
	}
	for _, downFile := range downFiles {
		downPath := filepath.Join(root, "migrations", downFile)
		content, err := os.ReadFile(downPath)
		if err == nil {
			_, _ = client.Pool.Exec(ctx, string(content))
		}
	}

	upFiles := []string{
		"000095_create_marketing_campaigns_and_promotions.up.sql",
		"000096_create_product_price_history.up.sql",
		"000097_create_order_item_promotions.up.sql",
	}

	for _, upFile := range upFiles {
		upPath := filepath.Join(root, "migrations", upFile)
		content, err := os.ReadFile(upPath)
		require.NoError(t, err, "failed to read migration %s", upFile)

		_, err = client.Pool.Exec(ctx, string(content))
		require.NoError(t, err, "failed to execute migration %s", upFile)
	}
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func createTestSeller(t *testing.T, client *postgres.Client) uuid.UUID {
	ctx := context.Background()
	sellerID := uuid.New()
	query := `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Test Brand', $4, 'active', now(), now())
	`
	slug := fmt.Sprintf("seller-%s", sellerID.String()[:8])
	email := fmt.Sprintf("seller-%s@example.com", sellerID.String()[:8])
	_, err := client.Pool.Exec(ctx, query, sellerID, "Brand "+slug, slug, email)
	require.NoError(t, err)
	return sellerID
}

func createTestStaffUser(t *testing.T, client *postgres.Client) uuid.UUID {
	ctx := context.Background()
	userID := uuid.New()
	query := `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'Admin Staff', $2, 'hash', 'admin', 'active', false, now(), now())
	`
	email := fmt.Sprintf("admin-%s@zamk.test", userID.String()[:8])
	_, err := client.Pool.Exec(ctx, query, userID, email)
	require.NoError(t, err)
	return userID
}

func createTestOrder(t *testing.T, client *postgres.Client, userID uuid.UUID) uuid.UUID {
	ctx := context.Background()
	orderID := uuid.New()
	query := `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address)
		VALUES ($1, $2, 'created', 500000, 'RUB', 'Customer', '123', 'c@ex.com', 'Moscow')
	`
	_, err := client.Pool.Exec(ctx, query, orderID, userID)
	require.NoError(t, err)
	return orderID
}

func cleanupMarketingFixtures(client *postgres.Client, sellerIDs []uuid.UUID, userIDs []uuid.UUID, campaignIDs []uuid.UUID) {
	ctx := context.Background()
	for _, cid := range campaignIDs {
		client.Pool.Exec(ctx, "DELETE FROM order_item_promotions WHERE campaign_id = $1", cid)
		client.Pool.Exec(ctx, "DELETE FROM promo_code_usages WHERE campaign_id = $1", cid)
		client.Pool.Exec(ctx, "DELETE FROM promo_codes WHERE campaign_id = $1", cid)
		client.Pool.Exec(ctx, "DELETE FROM marketing_campaigns WHERE id = $1", cid)
	}
	for _, sid := range sellerIDs {
		client.Pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE seller_id = $1", sid)
		client.Pool.Exec(ctx, "DELETE FROM product_price_history WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sid)
		client.Pool.Exec(ctx, "DELETE FROM products WHERE seller_id = $1", sid)
		client.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sid)
	}
	for _, uid := range userIDs {
		client.Pool.Exec(ctx, "DELETE FROM orders WHERE user_id = $1", uid)
		client.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", uid)
	}
}

// -------------------------------------------------------------
// 1. ONE ORDER = MAX ONE PROMO (CRITICAL INVARIANT PROOF)
// -------------------------------------------------------------

func TestPromoCodeUsage_OneOrderMaxOnePromo(t *testing.T) {
	client, repo, _ := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestStaffUser(t, client)
	var campaignIDs []uuid.UUID

	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, campaignIDs)

	ctx := context.Background()

	// Create campaign & two different promo codes
	c := &marketing.MarketingCampaign{
		ID:                uuid.New(),
		SellerID:          sellerID,
		Title:             "Promo Usage Constraint Test",
		FundingMode:       marketing.FundingModeSeller,
		Status:            marketing.CampaignStatusActive,
		DiscountType:      marketing.DiscountTypePercent,
		SellerDiscountBps: 1000,
	}
	require.NoError(t, repo.CreateCampaign(ctx, c))
	campaignIDs = append(campaignIDs, c.ID)

	promoA := &marketing.PromoCode{
		ID:                    uuid.New(),
		CampaignID:            c.ID,
		SellerID:              sellerID,
		Code:                  fmt.Sprintf("PROMOA%s", uuid.New().String()[:6]),
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		PerCustomerUsageLimit: 5,
		IsActive:              true,
	}
	require.NoError(t, repo.CreatePromoCode(ctx, promoA))

	promoB := &marketing.PromoCode{
		ID:                    uuid.New(),
		CampaignID:            c.ID,
		SellerID:              sellerID,
		Code:                  fmt.Sprintf("PROMOB%s", uuid.New().String()[:6]),
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		PerCustomerUsageLimit: 5,
		IsActive:              true,
	}
	require.NoError(t, repo.CreatePromoCode(ctx, promoB))

	// Create order X and order Y
	orderX := createTestOrder(t, client, userID)
	orderY := createTestOrder(t, client, userID)

	// Step 1: Reserve Promo A for Order X -> SUCCEEDS
	usageA := `
		INSERT INTO promo_code_usages (id, promo_code_id, campaign_id, order_id, user_id, status, subsidy_cents, seller_discount_cents, reserved_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, 'reserved', 0, 50000, now(), now() + interval '1 hour')
	`
	_, err := client.Pool.Exec(ctx, usageA, uuid.New(), promoA.ID, c.ID, orderX, userID)
	require.NoError(t, err, "First promo reservation for Order X must succeed")

	// Step 2: Attempt to reserve Promo B for SAME Order X -> REJECTED by uq_promo_usage_order
	_, err = client.Pool.Exec(ctx, usageA, uuid.New(), promoB.ID, c.ID, orderX, userID)
	require.Error(t, err, "Second promo reservation for same Order X must fail closed on DB invariant")
	assert.Contains(t, err.Error(), "uq_promo_usage_order", "Must be rejected by exact uq_promo_usage_order constraint")

	// Step 3: Reserve Promo A for different Order Y -> SUCCEEDS
	_, err = client.Pool.Exec(ctx, usageA, uuid.New(), promoA.ID, c.ID, orderY, userID)
	require.NoError(t, err, "Different order Y using promo A must succeed normally")
}

// -------------------------------------------------------------
// 2. PLATFORM FUNDING SEMANTICS (SELLER vs ZAMK vs COFUNDED)
// -------------------------------------------------------------

func TestMarketingCampaign_PlatformFundingSemantics(t *testing.T) {
	client, repo, _ := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID

	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, nil, campaignIDs)

	ctx := context.Background()

	// Invariant: COFUNDED requires seller_discount_bps > 0
	cofundedZeroSeller := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Invalid Cofunded Zero Seller",
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusDraft,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           0, // ILLEGAL
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 1000000,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(cofundedZeroSeller), marketing.ErrCofundedRequiresSellerContribution)
	err := repo.CreateCampaign(ctx, cofundedZeroSeller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_mc_cofunded_seller_contribution")

	// Invariant: Pure ZAMK requires seller discount == 0
	zamkNonZeroSeller := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Invalid Pure ZAMK with Seller Discount",
		FundingMode:                 marketing.FundingModeZamk,
		Status:                      marketing.CampaignStatusDraft,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           500, // ILLEGAL for pure ZAMK
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 1000000,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(zamkNonZeroSeller), marketing.ErrZamkCannotHaveSellerDiscount)
	err = repo.CreateCampaign(ctx, zamkNonZeroSeller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_mc_zamk_seller_zero")

	// Invariant: SELLER requires discount > 0 and 0 ZAMK fields
	sellerZeroDiscount := &marketing.MarketingCampaign{
		ID:                uuid.New(),
		SellerID:          sellerID,
		Title:             "Invalid Seller Zero Discount",
		FundingMode:       marketing.FundingModeSeller,
		Status:            marketing.CampaignStatusDraft,
		DiscountType:      marketing.DiscountTypePercent,
		SellerDiscountBps: 0,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(sellerZeroDiscount), marketing.ErrSellerRequiresDiscount)
	err = repo.CreateCampaign(ctx, sellerZeroDiscount)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_mc_seller_positive_discount")

	sellerWithZamk := &marketing.MarketingCampaign{
		ID:                    uuid.New(),
		SellerID:              sellerID,
		Title:                 "Invalid Seller with ZAMK",
		FundingMode:           marketing.FundingModeSeller,
		Status:                marketing.CampaignStatusDraft,
		DiscountType:          marketing.DiscountTypePercent,
		SellerDiscountBps:     1000,
		RequestedZamkShareBps: 500, // ILLEGAL for seller mode
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(sellerWithZamk), marketing.ErrSellerFundedCannotHaveZamk)
	err = repo.CreateCampaign(ctx, sellerWithZamk)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_mc_seller_zero_zamk")

	// Invariant: SUBMITTED platform-funded requires requested > 0
	submittedZeroReq := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Invalid Submitted Zero Request",
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusSubmitted,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1000,
		RequestedZamkShareBps:       0,
		RequestedZamkBudgetCapCents: 0,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(submittedZeroReq), marketing.ErrSubmittedPlatformRequiresRequested)
	err = repo.CreateCampaign(ctx, submittedZeroReq)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_mc_submitted_platform_req")

	// Invariant: DRAFT or SUBMITTED cannot have approved values
	draftWithApproved := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Invalid Draft with Approved",
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusDraft,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1000,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 1000000,
		ApprovedZamkShareBps:        1000,
		ApprovedZamkBudgetCapCents:  1000000,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(draftWithApproved), marketing.ErrUnreviewedMustHaveZeroApproved)
	err = repo.CreateCampaign(ctx, draftWithApproved)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_mc_unreviewed_zero_approved")
}

// -------------------------------------------------------------
// 3. REQUESTED VS APPROVED AUTHORITY & COUNTER-OFFER LIFECYCLE
// -------------------------------------------------------------

func TestMarketingCampaign_RequestedVsApprovedAuthority(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	staffUserID := createTestStaffUser(t, client)
	var campaignIDs []uuid.UUID

	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{staffUserID}, campaignIDs)

	ctx := context.Background()

	// Invariant: Approved terms cannot exceed requested terms in DB
	cIllegalApp := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Illegal Approved Exceeds Requested",
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusApproved,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1500,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 1000000,
		ApprovedZamkShareBps:        1500, // EXCEEDS 1000 bps requested!
		ApprovedZamkBudgetCapCents: 1000000,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(cIllegalApp), marketing.ErrApprovedExceedsRequested)
	err := repo.CreateCampaign(ctx, cIllegalApp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_mc_approved_ceilings")

	// Pre-seed 35 sales for co-funding eligibility
	for i := 0; i < 35; i++ {
		oid := createTestOrder(t, client, staffUserID)
		_, err := client.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'paid', 100000, 900, 91000)
		`, uuid.New(), oid, sellerID)
		require.NoError(t, err)
	}

	// 1. Create cofunded application (Requested: 15% ZAMK, 50,000 RUB budget)
	draft, err := svc.CreateCofundedApplication(ctx, sellerID, marketing.CreateCofundedApplicationRequest{
		Title:                       "Autumn Warmth Co-funding",
		SellerDiscountBps:           1500,
		RequestedZamkShareBps:       1500,
		RequestedZamkBudgetCapCents: 5000000,
	})
	require.NoError(t, err)
	campaignIDs = append(campaignIDs, draft.ID)

	// Submit application
	submitted, err := svc.SubmitCampaign(ctx, sellerID, draft.ID)
	require.NoError(t, err)
	assert.Equal(t, marketing.CampaignStatusSubmitted, submitted.Status)

	// 2. Admin counter-offers with reduced terms (10% share, 30,000 RUB budget)
	counterShare := 1000
	counterBudget := int64(3000000)
	reviewed, err := svc.AdminReviewCampaign(ctx, staffUserID, submitted.ID, marketing.AdminReviewCampaignRequest{
		Action:                  "approve",
		ApprovedZamkShareBps:    &counterShare,
		ApprovedZamkBudgetCents: &counterBudget,
	})
	require.NoError(t, err)
	assert.Equal(t, marketing.CampaignStatusCounterOffered, reviewed.Status)
	assert.Equal(t, 1000, reviewed.ApprovedZamkShareBps)
	assert.Equal(t, int64(3000000), reviewed.ApprovedZamkBudgetCapCents)

	// 3. Seller accepts counter-offer -> transitions to APPROVED
	approved, err := svc.SellerAcceptCounterOffer(ctx, sellerID, reviewed.ID)
	require.NoError(t, err)
	assert.Equal(t, marketing.CampaignStatusApproved, approved.Status)

	// Invariant: Seller CANNOT mutate approved financial fields
	assert.Equal(t, 1000, approved.ApprovedZamkShareBps)
	assert.Equal(t, int64(3000000), approved.ApprovedZamkBudgetCapCents)
}

// -------------------------------------------------------------
// 4. ORDER ITEM PROMOTION IMMUTABILITY & FK RESTRICTION
// -------------------------------------------------------------

func TestOrderItemPromotion_ImmutabilityAndFKProtection(t *testing.T) {
	client, repo, _ := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestStaffUser(t, client)
	var campaignIDs []uuid.UUID

	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, campaignIDs)

	ctx := context.Background()

	c := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Snapshot Immutability Campaign",
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusApproved,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1500,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 5000000,
		ApprovedZamkShareBps:        1000,
		ApprovedZamkBudgetCapCents: 5000000,
	}
	require.NoError(t, repo.CreateCampaign(ctx, c))
	campaignIDs = append(campaignIDs, c.ID)

	promo := &marketing.PromoCode{
		ID:                    uuid.New(),
		CampaignID:            c.ID,
		SellerID:              sellerID,
		Code:                  fmt.Sprintf("SNAP%s", uuid.New().String()[:6]),
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      2500,
		PerCustomerUsageLimit: 1,
		IsActive:              true,
	}
	require.NoError(t, repo.CreatePromoCode(ctx, promo))

	// Create test product, variant, order, and order_item
	productID := uuid.New()
	variantID := uuid.New()
	orderID := createTestOrder(t, client, userID)
	orderItemID := uuid.New()

	_, err := client.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, price_cents, status)
		VALUES ($1, $2, 'Hoodie', $3, 500000, 'published')
	`, productID, sellerID, fmt.Sprintf("hoodie-%s", productID.String()[:8]))
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents)
		VALUES ($1, $2, $3, 500000)
	`, variantID, productID, fmt.Sprintf("SKU-%s", variantID.String()[:8]))
	require.NoError(t, err)

	fulID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'paid', 1000000, 900, 910000)
	`, fulID, orderID, sellerID)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents)
		VALUES ($1, $2, $3, $4, $5, $6, 'Hoodie', 'hoodie', 500000, 2, 1000000)
	`, orderItemID, orderID, fulID, productID, variantID, sellerID)
	require.NoError(t, err)

	// Step 1: First snapshot creation SUCCEEDS
	snapshot := &marketing.OrderItemPromotion{
		ID:                          uuid.New(),
		OrderItemID:                 orderItemID,
		OrderID:                     orderID,
		SellerID:                    sellerID,
		CampaignID:                  c.ID,
		PromoCodeID:                 promo.ID,
		BaseUnitPriceCents:          500000, // 5000 RUB
		SellerDiscountUnitCents:     75000,  // 750 RUB
		ZamkSubsidyUnitCents:        50000,  // 500 RUB
		CustomerPaidUnitPriceCents:  375000, // 3750 RUB
		CommissionBaseUnitCents:     425000, // 4250 RUB (5000 - 750)
		Quantity:                    2,
		TotalSellerDiscountCents:    150000, // 1500 RUB
		TotalZamkSubsidyCents:       100000, // 1000 RUB
		TotalCustomerPaidCents:      750000, // 7500 RUB
		TotalCommissionBaseCents:    850000, // 4250 * 2 = 8500 RUB
		CommissionRateBps:           900,
		TotalCommissionChargedCents: 76500, // 850000 * 0.09 = 765 RUB
	}
	require.NoError(t, marketing.ValidateOrderItemPromotion(snapshot))
	require.NoError(t, repo.CreateOrderItemPromotion(ctx, snapshot))

	// Step 2: Second snapshot for SAME order_item_id MUST FAIL CLOSED (Duplicate snapshot rejected)
	duplicateSnapshot := *snapshot
	duplicateSnapshot.ID = uuid.New()
	err = repo.CreateOrderItemPromotion(ctx, &duplicateSnapshot)
	require.Error(t, err, "Duplicate snapshot for same order_item_id must be rejected")
	assert.Contains(t, err.Error(), "order_item_promotions_order_item_id_key", "Must fail on UNIQUE(order_item_id) constraint")

	// Step 3: FK Deletion Protection: Deleting campaign is RESTRICTED
	_, err = client.Pool.Exec(ctx, "DELETE FROM marketing_campaigns WHERE id = $1", c.ID)
	require.Error(t, err, "Deleting campaign with historical order item promotion must be blocked by RESTRICT")
	assert.Contains(t, err.Error(), "violates foreign key constraint", "RESTRICT prevents deleting financial history")

	// Step 4: FK Deletion Protection: Deleting promo_code is RESTRICTED
	_, err = client.Pool.Exec(ctx, "DELETE FROM promo_codes WHERE id = $1", promo.ID)
	require.Error(t, err, "Deleting promo code with historical order item promotion must be blocked by RESTRICT")
	assert.Contains(t, err.Error(), "violates foreign key constraint", "RESTRICT prevents deleting financial history")

	// Step 5: Verify retrieved snapshot matches exact persisted values
	retrieved, err := repo.GetOrderItemPromotion(ctx, orderItemID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, int64(500000), retrieved.BaseUnitPriceCents)
	assert.Equal(t, int64(75000), retrieved.SellerDiscountUnitCents)
	assert.Equal(t, int64(50000), retrieved.ZamkSubsidyUnitCents)
	assert.Equal(t, int64(375000), retrieved.CustomerPaidUnitPriceCents)
	assert.Equal(t, int64(425000), retrieved.CommissionBaseUnitCents)
	assert.Equal(t, 2, retrieved.Quantity)
	assert.Equal(t, int64(150000), retrieved.TotalSellerDiscountCents)
	assert.Equal(t, int64(100000), retrieved.TotalZamkSubsidyCents)
	assert.Equal(t, int64(750000), retrieved.TotalCustomerPaidCents)
	assert.Equal(t, int64(850000), retrieved.TotalCommissionBaseCents)
	assert.Equal(t, int64(76500), retrieved.TotalCommissionChargedCents)
}

// -------------------------------------------------------------
// 5. EXISTING CONSTRAINTS & REPOSITORY BEHAVIOR
// -------------------------------------------------------------

func TestMarketingCampaign_SingleOpenPlatformFundingConstraint(t *testing.T) {
	client, repo, _ := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID

	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, nil, campaignIDs)

	ctx := context.Background()

	// 1. Create first cofunded campaign in SUBMITTED status -> Succeeds
	c1 := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Spring Promo 1",
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusSubmitted,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1500,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 5000000,
	}
	require.NoError(t, repo.CreateCampaign(ctx, c1))
	campaignIDs = append(campaignIDs, c1.ID)

	// 2. Attempt to create a SECOND cofunded campaign in SUBMITTED status for same seller -> Rejected
	c2 := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Spring Promo 2",
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusSubmitted,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1500,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 5000000,
	}
	err := repo.CreateCampaign(ctx, c2)
	assert.ErrorIs(t, err, marketing.ErrOpenPlatformFundingExists, "Second open platform application must be rejected by partial unique index")
}

func TestMarketingCampaign_DraftDoesNotReservePlatformSlot(t *testing.T) {
	client, repo, _ := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID

	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, nil, campaignIDs)

	ctx := context.Background()

	// Draft 1
	d1 := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Draft 1",
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusDraft,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1500,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 5000000,
	}
	require.NoError(t, repo.CreateCampaign(ctx, d1))
	campaignIDs = append(campaignIDs, d1.ID)

	// Draft 2 -> Must succeed because drafts do NOT reserve the slot
	d2 := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Draft 2",
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusDraft,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1500,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 5000000,
	}
	require.NoError(t, repo.CreateCampaign(ctx, d2))
	campaignIDs = append(campaignIDs, d2.ID)
}

func TestMarketingCampaign_SellerFundedNotBlockedBySingleSlot(t *testing.T) {
	client, repo, _ := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID

	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, nil, campaignIDs)

	ctx := context.Background()

	// 1. One active cofunded campaign
	c1 := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Active Cofunded",
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusActive,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1500,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 5000000,
		ApprovedZamkShareBps:        1000,
		ApprovedZamkBudgetCapCents: 5000000,
	}
	require.NoError(t, repo.CreateCampaign(ctx, c1))
	campaignIDs = append(campaignIDs, c1.ID)

	// 2. Purely seller-funded campaigns can be created in parallel without limit
	for i := 1; i <= 3; i++ {
		sCamp := &marketing.MarketingCampaign{
			ID:                uuid.New(),
			SellerID:          sellerID,
			Title:             fmt.Sprintf("Seller Parallel %d", i),
			FundingMode:       marketing.FundingModeSeller,
			Status:            marketing.CampaignStatusActive,
			DiscountType:      marketing.DiscountTypePercent,
			SellerDiscountBps: 1000,
		}
		require.NoError(t, repo.CreateCampaign(ctx, sCamp))
		campaignIDs = append(campaignIDs, sCamp.ID)
	}
}

func TestPromoCode_CaseInsensitiveUniqueness(t *testing.T) {
	client, repo, _ := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID

	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, nil, campaignIDs)

	ctx := context.Background()

	c := &marketing.MarketingCampaign{
		ID:                uuid.New(),
		SellerID:          sellerID,
		Title:             "Promo Test",
		FundingMode:       marketing.FundingModeSeller,
		Status:            marketing.CampaignStatusActive,
		DiscountType:      marketing.DiscountTypePercent,
		SellerDiscountBps: 1000,
	}
	require.NoError(t, repo.CreateCampaign(ctx, c))
	campaignIDs = append(campaignIDs, c.ID)

	baseCode := fmt.Sprintf("WINTER%s", uuid.New().String()[:6])
	p1 := &marketing.PromoCode{
		ID:                    uuid.New(),
		CampaignID:            c.ID,
		SellerID:              sellerID,
		Code:                  baseCode,
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		PerCustomerUsageLimit: 1,
		IsActive:              true,
	}
	require.NoError(t, repo.CreatePromoCode(ctx, p1))

	// Attempting to create lowercase must fail on unique normalized code index
	p2 := &marketing.PromoCode{
		ID:                    uuid.New(),
		CampaignID:            c.ID,
		SellerID:              sellerID,
		Code:                  strings.ToLower(baseCode),
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1000,
		PerCustomerUsageLimit: 1,
		IsActive:              true,
	}
	err := repo.CreatePromoCode(ctx, p2)
	assert.ErrorIs(t, err, marketing.ErrPromoCodeDuplicate)

	// Fetching case-insensitively
	fetched, err := repo.GetPromoCodeByCode(ctx, strings.ToLower(baseCode))
	require.NoError(t, err)
	assert.Equal(t, p1.ID, fetched.ID)
}

func TestProductPriceHistory_RecordAndList(t *testing.T) {
	client, repo, _ := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestStaffUser(t, client)

	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, nil)

	ctx := context.Background()

	productID := uuid.New()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, price_cents, status)
		VALUES ($1, $2, 'Dress', $3, 1000000, 'published')
	`, productID, sellerID, fmt.Sprintf("dress-%s", productID.String()[:8]))
	require.NoError(t, err)

	// Record price change 10,000 RUB -> 8,500 RUB
	reason := "Seasonal sale"
	h1 := &marketing.ProductPriceHistory{
		ID:              uuid.New(),
		ProductID:       productID,
		OldPriceCents:   1000000,
		NewPriceCents:   850000,
		ChangedByUserID: &userID,
		Source:          marketing.PriceChangeSourceSeller,
		Reason:          &reason,
		CreatedAt:       time.Now().UTC().Add(-1 * time.Hour),
	}
	require.NoError(t, repo.RecordPriceChange(ctx, h1))

	// Record price change 8,500 RUB -> 9,000 RUB
	h2 := &marketing.ProductPriceHistory{
		ID:              uuid.New(),
		ProductID:       productID,
		OldPriceCents:   850000,
		NewPriceCents:   900000,
		ChangedByUserID: &userID,
		Source:          marketing.PriceChangeSourceSeller,
		CreatedAt:       time.Now().UTC(),
	}
	require.NoError(t, repo.RecordPriceChange(ctx, h2))

	// Query history
	hist, err := repo.ListProductPriceHistory(ctx, productID, 10)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	assert.Equal(t, int64(900000), hist[0].NewPriceCents, "Latest change first")
	assert.Equal(t, int64(850000), hist[1].NewPriceCents)
}

func TestSellerEligibility_SuccessfulPaidSalesCount(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	userID := createTestStaffUser(t, client)

	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, []uuid.UUID{userID}, nil)

	ctx := context.Background()

	// Initially 0 sales -> Not eligible
	eligibility, err := svc.GetSellerEligibility(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, 0, eligibility.SuccessfulSalesCount)
	assert.False(t, eligibility.IsEligible)

	// Seed 34 paid order fulfillments
	for i := 0; i < 34; i++ {
		oid := createTestOrder(t, client, userID)
		_, err := client.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
			VALUES ($1, $2, $3, 'paid', 100000, 900, 91000)
		`, uuid.New(), oid, sellerID)
		require.NoError(t, err)
	}

	count, err := repo.CountSuccessfulSellerSales(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, 34, count)

	eligibility, err = svc.GetSellerEligibility(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, 34, eligibility.SuccessfulSalesCount)
	assert.False(t, eligibility.IsEligible, "34 is below threshold of 35")

	// Add 35th sale: PO rule confirms 'returned' also counts as successful historical paid sale
	returnedOID := createTestOrder(t, client, userID)
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
		VALUES ($1, $2, $3, 'returned', 100000, 900, 91000)
	`, uuid.New(), returnedOID, sellerID)
	require.NoError(t, err)

	// Now exactly 35 sales -> Eligible!
	count, err = repo.CountSuccessfulSellerSales(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, 35, count)

	eligibility, err = svc.GetSellerEligibility(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, 35, eligibility.SuccessfulSalesCount)
	assert.True(t, eligibility.IsEligible, "35 paid sales threshold reached")
}

func TestMarketingCampaign_BudgetCapConstraint(t *testing.T) {
	client, repo, _ := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	var campaignIDs []uuid.UUID

	defer cleanupMarketingFixtures(client, []uuid.UUID{sellerID}, nil, campaignIDs)

	ctx := context.Background()

	// Approved campaign with 50,000 RUB budget cap
	c := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Budget Test Campaign",
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusApproved,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1500,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 5000000,
		ApprovedZamkShareBps:        1000,
		ApprovedZamkBudgetCapCents: 5000000,
		ZamkSpentCents:              0,
		ZamkReservedCents:           0,
	}
	require.NoError(t, repo.CreateCampaign(ctx, c))
	campaignIDs = append(campaignIDs, c.ID)

	// Attempting to mutate spent beyond budget cap directly in DB violates chk_mc_budget_spent
	_, err := client.Pool.Exec(ctx, `
		UPDATE marketing_campaigns
		SET zamk_spent_cents = 6000000
		WHERE id = $1
	`, c.ID)
	assert.Error(t, err, "Exceeding approved_zamk_budget_cap_cents must violate DB CHECK constraint")
}
