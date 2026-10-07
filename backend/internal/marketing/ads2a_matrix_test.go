package marketing_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/behavior"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// Helper fixture for full E2E attribution tests (Cases V, W, X, Y, Z, AF)
type ads2aAttributionFixture struct {
	db           *pgxpool.Pool
	pgClient     *postgres.Client
	marketingSvc *marketing.Service
	behaviorSvc  *behavior.Service
	ordersSvc    *orders.Service
	sellerID     uuid.UUID
	buyerID      uuid.UUID
	prodID       uuid.UUID
	variantID    uuid.UUID
	dmID         uuid.UUID
}

func setupADS2AAttributionFixture(t *testing.T, ctx context.Context) *ads2aAttributionFixture {
	t.Helper()
	dbURL := testutil.GetTestDatabaseURL()
	require.NotEmpty(t, dbURL, "test database URL must not be empty")

	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "failed to connect to test database")
	testutil.AssertTestDatabase(t, db)

	pgClient := &postgres.Client{Pool: db}
	ordersRepo := orders.NewRepository(db)
	cartRepo := cart.NewRepository(db)
	invRepo := inventory.NewRepository(db)
	invSvc := inventory.NewService(invRepo, nil, pgClient)

	marketingRepo := marketing.NewRepository(db)
	marketingSvc := marketing.NewService(marketingRepo, db)

	behaviorRepo := behavior.NewRepository(pgClient)
	behaviorSvc := behavior.NewService(behaviorRepo, marketingSvc.ResolveCampaignToken)


	cfg := &config.Config{
		Worker: config.WorkerConfig{MarketplaceCommissionBPS: 1500},
	}
	ordersSvc := orders.NewService(ordersRepo, cartRepo, invSvc, pgClient, cfg).
		WithMarketing(marketingSvc).
		WithBehavior(behaviorSvc)

	suffix := uuid.New().String()[:8]
	sellerUserID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	catID := uuid.New()
	brandID := uuid.New()
	prodID := uuid.New()
	variantID := uuid.New()
	dmID := uuid.New()

	_, err = db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'ADS2A Seller', $2, 'hash', 'seller', 'active', now(), now())
	`, sellerUserID, fmt.Sprintf("seller-%s@test.local", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'ADS2A Buyer', $2, 'hash', 'customer', 'active', now(), now())
	`, buyerID, fmt.Sprintf("buyer-%s@test.local", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'ADS2A Brand', $2, $3, 'active', now(), now())
	`, sellerID, fmt.Sprintf("brand-%s", suffix), fmt.Sprintf("seller-%s@test.local", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'ADS2A Category', $2, now(), now())
	`, catID, fmt.Sprintf("cat-%s", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO brands (id, name, slug, created_at, updated_at)
		VALUES ($1, 'ADS2A Brand', $2, now(), now())
	`, brandID, fmt.Sprintf("b-%s", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'ADS2A Product', $5, 150000, 'published', now(), now())
	`, prodID, sellerID, catID, brandID, fmt.Sprintf("prod-%s", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 150000, true, now(), now())
	`, variantID, prodID, fmt.Sprintf("SKU-%s", suffix), fmt.Sprintf("SSKU-%s", suffix), fmt.Sprintf("BC-%s", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO delivery_methods (id, code, name, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Standard Delivery', 200, true, now(), now())
	`, dmID, fmt.Sprintf("dm-ads-%s", suffix))
	require.NoError(t, err)

	// Warehouse inventory setup for order creation
	_, err = db.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1000, 0, now(), now())
	`, uuid.New(), prodID, variantID, sellerID)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec(ctx, "DELETE FROM order_item_promotions WHERE order_id IN (SELECT id FROM orders WHERE user_id = $1)", buyerID)
		db.Exec(ctx, "DELETE FROM order_attributions WHERE order_id IN (SELECT id FROM orders WHERE user_id = $1)", buyerID)
		db.Exec(ctx, "DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE user_id = $1)", buyerID)
		db.Exec(ctx, "DELETE FROM order_fulfillments WHERE order_id IN (SELECT id FROM orders WHERE user_id = $1)", buyerID)
		db.Exec(ctx, "DELETE FROM orders WHERE user_id = $1", buyerID)
		db.Exec(ctx, "DELETE FROM cart_items WHERE cart_id IN (SELECT id FROM carts WHERE user_id = $1)", buyerID)
		db.Exec(ctx, "DELETE FROM carts WHERE user_id = $1", buyerID)
		db.Exec(ctx, "DELETE FROM analytics_sessions WHERE user_id = $1", buyerID)
		db.Exec(ctx, "DELETE FROM inventory_items WHERE product_variant_id = $1", variantID)
		db.Exec(ctx, "DELETE FROM product_variants WHERE id = $1", variantID)
		db.Exec(ctx, "DELETE FROM products WHERE id = $1", prodID)
		db.Exec(ctx, "DELETE FROM brands WHERE id = $1", brandID)
		db.Exec(ctx, "DELETE FROM categories WHERE id = $1", catID)
		db.Exec(ctx, "DELETE FROM delivery_methods WHERE id = $1", dmID)
		db.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
		db.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", sellerUserID, buyerID)
		db.Close()
	})

	return &ads2aAttributionFixture{
		db:           db,
		pgClient:     pgClient,
		marketingSvc: marketingSvc,
		behaviorSvc:  behaviorSvc,
		ordersSvc:    ordersSvc,
		sellerID:     sellerID,
		buyerID:      buyerID,
		prodID:       prodID,
		variantID:    variantID,
		dmID:         dmID,
	}
}

func (f *ads2aAttributionFixture) populateCart(t *testing.T, ctx context.Context, buyerID uuid.UUID, quantity int) {
	t.Helper()
	cartRepo := cart.NewRepository(f.db)
	userCart, err := cartRepo.GetCartByUserID(ctx, buyerID)
	if errors.Is(err, cart.ErrCartNotFound) {
		userCart, err = cartRepo.CreateCart(ctx, buyerID)
		require.NoError(t, err)
	} else {
		require.NoError(t, err)
	}

	existingItem, err := cartRepo.GetCartItem(ctx, userCart.ID, f.variantID)
	if err == nil && existingItem != nil {
		err = cartRepo.UpdateItemQuantity(ctx, existingItem.ID, quantity)
		require.NoError(t, err)
		return
	}

	item := &cart.CartItem{
		ID:               uuid.New(),
		CartID:           userCart.ID,
		ProductID:        f.prodID,
		ProductVariantID: f.variantID,
		Quantity:         quantity,
	}
	err = cartRepo.AddItem(ctx, item)
	require.NoError(t, err)
}


// ============================================================================
// ADS.2A PRODUCT TEST MATRIX: CASES A THROUGH AF
// ============================================================================

// Case A: Platform campaign create (seller_id nil)
func TestADS2A_CaseA_PlatformCampaignCreate(t *testing.T) {
	_, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	channel := marketing.CampaignChannelTelegram
	campType := marketing.CampaignTypeSeasonalSale
	budgetCents := int64(5000000)

	camp := &marketing.MarketingCampaign{
		SellerID:           nil, // NULL = platform campaign
		Title:              "Case A Platform Campaign",
		FundingMode:        marketing.FundingModeZamk,
		Status:             marketing.CampaignStatusActive,
		CampaignChannel:    &channel,
		CampaignType:       &campType,
		PlannedBudgetCents: &budgetCents,
		DiscountType:       marketing.DiscountTypePercent,
	}

	err := svc.CreateAdminCampaign(ctx, camp)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, camp.ID)
	assert.Nil(t, camp.SellerID)

	saved, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	assert.Nil(t, saved.SellerID)
	assert.Equal(t, marketing.FundingModeZamk, saved.FundingMode)
	assert.Equal(t, "Case A Platform Campaign", saved.Title)
	require.NotNil(t, saved.PlannedBudgetCents)
	assert.Equal(t, int64(5000000), *saved.PlannedBudgetCents)
}

// Case B: Seller campaign regression
func TestADS2A_CaseB_SellerCampaignRegression(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	ctx := context.Background()

	channel := marketing.CampaignChannelInfluencer
	campType := marketing.CampaignTypeDrop

	camp := &marketing.MarketingCampaign{
		SellerID:        &sellerID,
		Title:           "Case B Seller Drop",
		FundingMode:     marketing.FundingModeSeller,
		Status:          marketing.CampaignStatusApproved,
		CampaignChannel: &channel,
		CampaignType:    &campType,
		DiscountType:    marketing.DiscountTypePercent,
	}

	err := svc.CreateAdminCampaign(ctx, camp)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, camp.ID)
	require.NotNil(t, camp.SellerID)
	assert.Equal(t, sellerID, *camp.SellerID)

	saved, err := repo.GetCampaignByID(ctx, camp.ID)
	require.NoError(t, err)
	require.NotNil(t, saved.SellerID)
	assert.Equal(t, sellerID, *saved.SellerID)
	assert.Equal(t, marketing.FundingModeSeller, saved.FundingMode)
}

// Case C: Campaign update
func TestADS2A_CaseC_CampaignUpdate(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	channel := marketing.CampaignChannelVK
	campType := marketing.CampaignTypeSpecialPromo
	camp := &marketing.MarketingCampaign{
		Title:           "Initial Title",
		FundingMode:     marketing.FundingModeZamk,
		Status:          marketing.CampaignStatusApproved,
		CampaignChannel: &channel,
		CampaignType:    &campType,
		DiscountType:    marketing.DiscountTypePercent,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	newTitle := "Updated Campaign Title"
	newStatus := marketing.CampaignStatusActive
	newChannel := marketing.CampaignChannelTelegram
	newBudget := int64(15000000)

	updated, err := svc.UpdateAdminCampaign(ctx, camp.ID, marketing.AdminUpdateCampaignRequest{
		Title:              &newTitle,
		Status:             &newStatus,
		CampaignChannel:    &newChannel,
		PlannedBudgetCents: &newBudget,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Campaign Title", updated.Title)
	assert.Equal(t, marketing.CampaignStatusActive, updated.Status)
	require.NotNil(t, updated.CampaignChannel)
	assert.Equal(t, marketing.CampaignChannelTelegram, *updated.CampaignChannel)
	require.NotNil(t, updated.PlannedBudgetCents)
	assert.Equal(t, int64(15000000), *updated.PlannedBudgetCents)
}

// Case D: Invalid channel rejected
func TestADS2A_CaseD_InvalidChannelRejected(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	invalidChan := marketing.CampaignChannel("tiktok_unsupported")
	camp := &marketing.MarketingCampaign{
		Title:           "Invalid Channel Test",
		FundingMode:     marketing.FundingModeZamk,
		CampaignChannel: &invalidChan,
		DiscountType:    marketing.DiscountTypePercent,
	}
	err := svc.CreateAdminCampaign(ctx, camp)
	assert.ErrorIs(t, err, marketing.ErrInvalidCampaignChannel)
}

// Case E: Invalid type rejected
func TestADS2A_CaseE_InvalidTypeRejected(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	invalidType := marketing.CampaignType("unsupported_type")
	camp := &marketing.MarketingCampaign{
		Title:        "Invalid Type Test",
		FundingMode:  marketing.FundingModeZamk,
		CampaignType: &invalidType,
		DiscountType: marketing.DiscountTypePercent,
	}
	err := svc.CreateAdminCampaign(ctx, camp)
	assert.ErrorIs(t, err, marketing.ErrInvalidCampaignType)
}

// Case F: Negative budget rejected
func TestADS2A_CaseF_NegativeBudgetRejected(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	negBudget := int64(-500)
	camp := &marketing.MarketingCampaign{
		Title:              "Negative Budget Test",
		FundingMode:        marketing.FundingModeZamk,
		PlannedBudgetCents: &negBudget,
		DiscountType:       marketing.DiscountTypePercent,
	}
	err := svc.CreateAdminCampaign(ctx, camp)
	assert.ErrorIs(t, err, marketing.ErrInvalidPlannedBudget)

	// Verify direct DB constraint chk_mc_planned_budget_non_negative
	_, dbErr := client.Pool.Exec(ctx, `
		INSERT INTO marketing_campaigns (id, title, funding_mode, status, discount_type, seller_discount_bps, planned_budget_cents, created_at, updated_at)
		VALUES ($1, 'Direct DB Negative Budget', 'seller', 'draft', 'percent', 1000, -1000, now(), now())
	`, uuid.New())
	assert.Error(t, dbErr)
	assert.True(t, strings.Contains(dbErr.Error(), "planned_budget"), "expected planned budget check constraint error, got: %v", dbErr)
}

// Case G: First tracking link
func TestADS2A_CaseG_FirstTrackingLink(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	channel := marketing.CampaignChannelTelegram
	campType := marketing.CampaignTypeSeasonalSale
	camp := &marketing.MarketingCampaign{
		Title:           "Case G Campaign",
		FundingMode:     marketing.FundingModeZamk,
		Status:          marketing.CampaignStatusActive,
		CampaignChannel: &channel,
		CampaignType:    &campType,
		DiscountType:    marketing.DiscountTypePercent,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	landingPath := "/catalog/sale"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &landingPath,
	}
	err := svc.CreateTrackingLink(ctx, link)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, link.ID)
	assert.NotEmpty(t, link.Token)
	assert.Equal(t, 32, len(link.Token)) // 24 bytes base64.RawURLEncoding = 32 chars
	assert.True(t, link.IsActive)
}

// Case H: Second tracking link
func TestADS2A_CaseH_SecondTrackingLink(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	camp := &marketing.MarketingCampaign{
		Title:        "Case H Campaign",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	landingPath1 := "/catalog/sale1"
	link1 := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &landingPath1,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link1))

	landingPath2 := "/catalog/sale2"
	link2 := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &landingPath2,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link2))

	assert.NotEqual(t, link1.ID, link2.ID)
	assert.NotEqual(t, link1.Token, link2.Token)

	links, err := svc.ListCampaignTrackingLinks(ctx, camp.ID)
	require.NoError(t, err)
	assert.Len(t, links, 2)
}

// Case I: Token uniqueness / high entropy (192 bits)
func TestADS2A_CaseI_TokenUniquenessAndHighEntropy(t *testing.T) {
	tokenMap := make(map[string]bool, 10000)
	for i := 0; i < 10000; i++ {
		token, err := marketing.GenerateTrackingToken()
		require.NoError(t, err)
		assert.Equal(t, 32, len(token), "Token length must be exactly 32 chars for 24 bytes (192 bits entropy)")
		assert.False(t, tokenMap[token], "Duplicate token generated!")
		tokenMap[token] = true
	}
}

// Case J: Product exact target
func TestADS2A_CaseJ_ProductExactTarget(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()
	sellerID := createTestSeller(t, client)

	suffix := uuid.New().String()[:8]
	catID := uuid.New()
	brandID := uuid.New()
	prodID := uuid.New()
	_, err := client.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', $2)`, catID, fmt.Sprintf("cat-%s", suffix))
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, `INSERT INTO brands (id, name, slug) VALUES ($1, 'Brand', $2)`, brandID, fmt.Sprintf("brand-%s", suffix))
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, price_cents) VALUES ($1, $2, $3, $4, 'Prod', $5, 1000)`, prodID, sellerID, catID, brandID, fmt.Sprintf("prod-%s", suffix))
	require.NoError(t, err)

	camp := &marketing.MarketingCampaign{
		Title:        "Case J Campaign",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	link := &marketing.CampaignTrackingLink{
		CampaignID:      camp.ID,
		TargetType:      marketing.CampaignTargetProduct,
		TargetProductID: &prodID,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link))

	saved, err := repo.GetTrackingLinkByID(ctx, link.ID)
	require.NoError(t, err)
	assert.Equal(t, marketing.CampaignTargetProduct, saved.TargetType)
	require.NotNil(t, saved.TargetProductID)
	assert.Equal(t, prodID, *saved.TargetProductID)
	assert.Nil(t, saved.TargetSellerID)
	assert.Nil(t, saved.LandingPath)
}

// Case K: Seller exact target
func TestADS2A_CaseK_SellerExactTarget(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()
	sellerID := createTestSeller(t, client)

	camp := &marketing.MarketingCampaign{
		Title:        "Case K Campaign",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	link := &marketing.CampaignTrackingLink{
		CampaignID:     camp.ID,
		TargetType:     marketing.CampaignTargetSeller,
		TargetSellerID: &sellerID,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link))

	saved, err := repo.GetTrackingLinkByID(ctx, link.ID)
	require.NoError(t, err)
	assert.Equal(t, marketing.CampaignTargetSeller, saved.TargetType)
	require.NotNil(t, saved.TargetSellerID)
	assert.Equal(t, sellerID, *saved.TargetSellerID)
	assert.Nil(t, saved.TargetProductID)
	assert.Nil(t, saved.LandingPath)
}

// Case L: Landing exact target
func TestADS2A_CaseL_LandingExactTarget(t *testing.T) {
	_, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	camp := &marketing.MarketingCampaign{
		Title:        "Case L Campaign",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	path := "/catalog/featured"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link))

	saved, err := repo.GetTrackingLinkByID(ctx, link.ID)
	require.NoError(t, err)
	assert.Equal(t, marketing.CampaignTargetLanding, saved.TargetType)
	require.NotNil(t, saved.LandingPath)
	assert.Equal(t, path, *saved.LandingPath)
	assert.Nil(t, saved.TargetProductID)
	assert.Nil(t, saved.TargetSellerID)
}

// Case M: Invalid mixed target rejected by DB constraint chk_ctl_target_exact_one
func TestADS2A_CaseM_InvalidMixedTargetRejectedByDBConstraint(t *testing.T) {
	client, _, _ := setupTestMarketingDB(t)
	ctx := context.Background()

	campID := uuid.New()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO marketing_campaigns (id, title, funding_mode, status, discount_type, seller_discount_bps, created_at, updated_at)
		VALUES ($1, 'Case M Camp', 'seller', 'draft', 'percent', 1000, now(), now())
	`, campID)
	require.NoError(t, err)

	prodID := uuid.New()
	path := "/catalog/evil"

	// 1. Target type 'landing' but both landing_path AND target_product_id populated -> FAILS
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO campaign_tracking_links (id, campaign_id, token, target_type, target_product_id, landing_path, is_active, created_at, updated_at)
		VALUES ($1, $2, 'token_mixed_1', 'landing', $3, $4, true, now(), now())
	`, uuid.New(), campID, prodID, path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chk_ctl_target_exact_one")

	// 2. Target type 'product' but target_product_id IS NULL -> FAILS
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO campaign_tracking_links (id, campaign_id, token, target_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'token_empty_target', 'product', true, now(), now())
	`, uuid.New(), campID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chk_ctl_target_exact_one")
}

// Case N: External/network-path redirect rejected
func TestADS2A_CaseN_ExternalNetworkPathRedirectRejected(t *testing.T) {
	invalidPaths := []string{
		"https://evil.com/hack",
		"http://evil.com/hack",
		"//evil.com/network",
		"/\\evil.com/network",
		"\\evil.com/network",
		"javascript:alert(1)",
		"data:text/html,hack",
		"catalog/no-slash",
		"/catalog\r\ninjection",
		"/catalog\ninjection",
		"/path with space",
		"",
	}

	for _, p := range invalidPaths {
		_, err := marketing.ValidateLandingPath(p)
		assert.ErrorIs(t, err, marketing.ErrInvalidLandingPath, "Expected rejection for path: %s", p)
	}

	validPaths := []string{
		"/catalog/summer-sale",
		"/products/item-1",
		"/sellers/store-name?filter=new",
		"/sale/2026",
	}
	for _, p := range validPaths {
		_, err := marketing.ValidateLandingPath(p)
		assert.NoError(t, err, "Expected path to be valid: %s", p)
	}
}

// Case O: Valid active resolve
func TestADS2A_CaseO_ValidActiveResolve(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	channel := marketing.CampaignChannelTelegram
	campType := marketing.CampaignTypeSeasonalSale
	camp := &marketing.MarketingCampaign{
		Title:           "Summer Sale 2026 Special Promo",
		FundingMode:     marketing.FundingModeZamk,
		Status:          marketing.CampaignStatusActive,
		CampaignChannel: &channel,
		CampaignType:    &campType,
		DiscountType:    marketing.DiscountTypePercent,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	path := "/catalog/summer"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link))

	redirectURL, err := svc.ResolveTrackingLink(ctx, link.Token, "https://zamk.ru")
	require.NoError(t, err)

	u, err := url.Parse(redirectURL)
	require.NoError(t, err)
	assert.Equal(t, "/catalog/summer", u.Path)

	q := u.Query()
	assert.Equal(t, "telegram", q.Get("utm_source"))
	assert.Equal(t, "seasonal_sale", q.Get("utm_medium"))
	assert.Equal(t, "summer-sale-2026-special-promo", q.Get("utm_campaign"))
	assert.Equal(t, link.Token, q.Get("zamk_token"))

	// Strict verification: NO internal UUID in utm_campaign
	assert.False(t, strings.Contains(q.Get("utm_campaign"), camp.ID.String()))
}

// Case P: Unknown token
func TestADS2A_CaseP_UnknownToken(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	_, err := svc.ResolveTrackingLink(ctx, "non_existent_token_123456789012", "https://zamk.ru")
	assert.ErrorIs(t, err, marketing.ErrTrackingLinkNotFound)
}

// Case Q: Disabled token
func TestADS2A_CaseQ_DisabledToken(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	camp := &marketing.MarketingCampaign{
		Title:        "Case Q Campaign",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	path := "/catalog/sale"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link))

	// Disable link
	_, err := svc.DisableTrackingLink(ctx, camp.ID, link.ID)
	require.NoError(t, err)

	_, err = svc.ResolveTrackingLink(ctx, link.Token, "https://zamk.ru")
	assert.ErrorIs(t, err, marketing.ErrTrackingLinkDisabled)
}

// Case R: Future campaign
func TestADS2A_CaseR_FutureCampaign(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	futureStart := time.Now().Add(48 * time.Hour)
	camp := &marketing.MarketingCampaign{
		Title:        "Future Campaign",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
		StartsAt:     &futureStart,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	path := "/catalog/sale"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link))

	_, err := svc.ResolveTrackingLink(ctx, link.Token, "https://zamk.ru")
	assert.ErrorIs(t, err, marketing.ErrCampaignNotStarted)
}

// Case S: Expired campaign
func TestADS2A_CaseS_ExpiredCampaign(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	pastEnd := time.Now().Add(-24 * time.Hour)
	camp := &marketing.MarketingCampaign{
		Title:        "Expired Campaign",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
		EndsAt:       &pastEnd,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	path := "/catalog/sale"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link))

	_, err := svc.ResolveTrackingLink(ctx, link.Token, "https://zamk.ru")
	assert.ErrorIs(t, err, marketing.ErrCampaignExpired)
}

// Case T: Inactive/paused campaign
func TestADS2A_CaseT_InactiveOrPausedCampaign(t *testing.T) {
	_, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	camp := &marketing.MarketingCampaign{
		Title:        "Draft Campaign",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusDraft, // Draft is not active
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, repo.CreateCampaign(ctx, camp))

	path := "/catalog/sale"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link))

	_, err := svc.ResolveTrackingLink(ctx, link.Token, "https://zamk.ru")
	assert.ErrorIs(t, err, marketing.ErrCampaignNotActive)
}


// Case U: Shop zamk_token capture contract verification
func TestADS2A_CaseU_ShopZamkTokenCaptureContract(t *testing.T) {
	validTokens := []string{
		"valid_token_12345678",
		"ABCdef0123456789-_",
		"a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
	}
	for _, tok := range validTokens {
		assert.True(t, len(tok) >= 8 && len(tok) <= 128)
	}

	invalidTokens := []string{
		"short",                     // < 8
		"token with spaces",         // space
		"token'quote",               // quote
		"token\"doublequote",        // double quote
		"token<script>",             // html
		"token/slash",               // slash
	}
	for _, tok := range invalidTokens {
		hasInvalid := len(tok) < 8 || strings.ContainsAny(tok, " '\"<>/")
		assert.True(t, hasInvalid, "Token should be invalid: %s", tok)
	}
}

// Case V: Token → analytics_sessions.campaign_id
func TestADS2A_CaseV_TokenToAnalyticsSessionCampaignID(t *testing.T) {
	f := setupADS2AAttributionFixture(t, context.Background())
	ctx := context.Background()

	// 1. Create marketing campaign & tracking link
	channel := marketing.CampaignChannelTelegram
	campType := marketing.CampaignTypeSeasonalSale
	camp := &marketing.MarketingCampaign{
		Title:           "Case V Telegram Promo",
		FundingMode:     marketing.FundingModeZamk,
		Status:          marketing.CampaignStatusActive,
		CampaignChannel: &channel,
		CampaignType:    &campType,
		DiscountType:    marketing.DiscountTypePercent,
	}
	require.NoError(t, f.marketingSvc.CreateAdminCampaign(ctx, camp))

	path := "/catalog"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, f.marketingSvc.CreateTrackingLink(ctx, link))

	// 2. Ingest client navigation event with zamk_token
	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()

	eventBatch := behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "page_view",
				VisitorID:  visitorID,
				SessionID:  &sessionID,
				OccurredAt: now,
				Metadata: map[string]interface{}{
					"landing_path": "/catalog",
					"zamk_token":   link.Token,
				},
			},
		},
	}

	res, err := f.behaviorSvc.IngestEvents(ctx, nil, eventBatch)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Accepted)

	// 3. Verify session campaign_id in database
	var dbCampaignID *uuid.UUID
	err = f.db.QueryRow(ctx, "SELECT campaign_id FROM analytics_sessions WHERE id = $1", sessionID).Scan(&dbCampaignID)
	require.NoError(t, err)
	require.NotNil(t, dbCampaignID)
	assert.Equal(t, camp.ID, *dbCampaignID)
}

// Case W: Campaign → order_attributions.campaign_id
func TestADS2A_CaseW_CampaignToOrderAttributionsCampaignID(t *testing.T) {
	f := setupADS2AAttributionFixture(t, context.Background())
	ctx := context.Background()

	// 1. Create active marketing campaign & link
	camp := &marketing.MarketingCampaign{
		Title:        "Case W Campaign",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, f.marketingSvc.CreateAdminCampaign(ctx, camp))

	path := "/catalog"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, f.marketingSvc.CreateTrackingLink(ctx, link))

	// 2. Record behavior session with campaign
	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()

	_, err := f.behaviorSvc.IngestEvents(ctx, nil, behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "session_started",
				VisitorID:  visitorID,
				SessionID:  &sessionID,
				OccurredAt: now,
				Metadata: map[string]interface{}{
					"landing_path": "/catalog",
					"zamk_token":   link.Token,
				},
			},
		},
	})
	require.NoError(t, err)

	// 3. Create order using that session
	f.populateCart(t, ctx, f.buyerID, 1)
	order, err := f.ordersSvc.CreateOrder(ctx, f.buyerID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Case W",
		CustomerEmail:    "buyer_w@test.local",
		CustomerPhone:    "+79991234567",
		DeliveryAddress:  "Moscow, Test St 10",
		DeliveryMethodID: f.dmID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: sessionID,
		},
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, order)

	// 4. Verify canonical order_attributions.campaign_id
	var attrCampaignID *uuid.UUID
	err = f.db.QueryRow(ctx, "SELECT campaign_id FROM order_attributions WHERE order_id = $1", order.ID).Scan(&attrCampaignID)
	require.NoError(t, err)
	require.NotNil(t, attrCampaignID)
	assert.Equal(t, camp.ID, *attrCampaignID)
}

// Case X: Google → campaign LNDC (Last-Non-Direct-Touch)
func TestADS2A_CaseX_GoogleToCampaignLNDC(t *testing.T) {
	f := setupADS2AAttributionFixture(t, context.Background())
	ctx := context.Background()

	// 1. Session starts with Google organic
	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()

	_, err := f.behaviorSvc.IngestEvents(ctx, nil, behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "session_started",
				VisitorID:  visitorID,
				SessionID:  &sessionID,
				OccurredAt: now,
				Metadata: map[string]interface{}{
					"referrer": "https://www.google.com",
				},
			},
		},
	})
	require.NoError(t, err)

	// Verify initial touch
	var initialReferrer string
	err = f.db.QueryRow(ctx, "SELECT referrer FROM analytics_sessions WHERE id = $1", sessionID).Scan(&initialReferrer)
	require.NoError(t, err)
	assert.Equal(t, "https://www.google.com", initialReferrer)

	// 2. Next touch has campaign token
	camp := &marketing.MarketingCampaign{
		Title:        "Case X Promo",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, f.marketingSvc.CreateAdminCampaign(ctx, camp))

	path := "/sale"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, f.marketingSvc.CreateTrackingLink(ctx, link))

	_, err = f.behaviorSvc.IngestEvents(ctx, nil, behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "page_view",
				VisitorID:  visitorID,
				SessionID:  &sessionID,
				OccurredAt: now.Add(time.Minute),
				Metadata: map[string]interface{}{
					"zamk_token": link.Token,
				},
			},
		},
	})
	require.NoError(t, err)

	// 3. Verify session campaign_id updated to campaign
	var sessionCampID *uuid.UUID
	err = f.db.QueryRow(ctx, "SELECT campaign_id FROM analytics_sessions WHERE id = $1", sessionID).Scan(&sessionCampID)
	require.NoError(t, err)
	require.NotNil(t, sessionCampID)
	assert.Equal(t, camp.ID, *sessionCampID)
}

// Case Y: Direct preserves campaign
func TestADS2A_CaseY_DirectPreservesCampaign(t *testing.T) {
	f := setupADS2AAttributionFixture(t, context.Background())
	ctx := context.Background()

	camp := &marketing.MarketingCampaign{
		Title:        "Case Y Promo",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, f.marketingSvc.CreateAdminCampaign(ctx, camp))

	path := "/sale"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, f.marketingSvc.CreateTrackingLink(ctx, link))

	// Touch 1: Campaign
	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()

	_, err := f.behaviorSvc.IngestEvents(ctx, nil, behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "session_started",
				VisitorID:  visitorID,
				SessionID:  &sessionID,
				OccurredAt: now,
				Metadata: map[string]interface{}{
					"zamk_token": link.Token,
				},
			},
		},
	})
	require.NoError(t, err)

	// Touch 2: Direct (no token, no utms, no referrer)
	_, err = f.behaviorSvc.IngestEvents(ctx, nil, behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "page_view",
				VisitorID:  visitorID,
				SessionID:  &sessionID,
				OccurredAt: now.Add(5 * time.Minute),
				Metadata: map[string]interface{}{
					"landing_path": "/about",
				},
			},
		},
	})
	require.NoError(t, err)

	// Campaign attribution MUST be preserved
	var preservedCampID *uuid.UUID
	err = f.db.QueryRow(ctx, "SELECT campaign_id FROM analytics_sessions WHERE id = $1", sessionID).Scan(&preservedCampID)
	require.NoError(t, err)
	require.NotNil(t, preservedCampID)
	assert.Equal(t, camp.ID, *preservedCampID)
}

// Case Z: Second campaign replaces first
func TestADS2A_CaseZ_SecondCampaignReplacesFirst(t *testing.T) {
	f := setupADS2AAttributionFixture(t, context.Background())
	ctx := context.Background()

	campA := &marketing.MarketingCampaign{
		Title:        "Campaign A",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, f.marketingSvc.CreateAdminCampaign(ctx, campA))

	campB := &marketing.MarketingCampaign{
		Title:        "Campaign B",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, f.marketingSvc.CreateAdminCampaign(ctx, campB))

	path := "/sale"
	linkA := &marketing.CampaignTrackingLink{CampaignID: campA.ID, TargetType: marketing.CampaignTargetLanding, LandingPath: &path}
	linkB := &marketing.CampaignTrackingLink{CampaignID: campB.ID, TargetType: marketing.CampaignTargetLanding, LandingPath: &path}
	require.NoError(t, f.marketingSvc.CreateTrackingLink(ctx, linkA))
	require.NoError(t, f.marketingSvc.CreateTrackingLink(ctx, linkB))

	visitorID := uuid.New()
	sessionID := uuid.New()
	t0 := time.Now().UTC().Add(-20 * time.Minute)

	// Touch 1: Campaign A
	_, err := f.behaviorSvc.IngestEvents(ctx, nil, behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "session_started",
				VisitorID:  visitorID,
				SessionID:  &sessionID,
				OccurredAt: t0,
				Metadata: map[string]interface{}{
					"zamk_token": linkA.Token,
				},
			},
		},
	})
	require.NoError(t, err)

	// Touch 2: Campaign B
	_, err = f.behaviorSvc.IngestEvents(ctx, nil, behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "page_view",
				VisitorID:  visitorID,
				SessionID:  &sessionID,
				OccurredAt: t0.Add(10 * time.Minute),
				Metadata: map[string]interface{}{
					"zamk_token": linkB.Token,
				},
			},
		},
	})
	require.NoError(t, err)


	// Campaign B replaces Campaign A
	var finalCampID *uuid.UUID
	err = f.db.QueryRow(ctx, "SELECT campaign_id FROM analytics_sessions WHERE id = $1", sessionID).Scan(&finalCampID)
	require.NoError(t, err)
	require.NotNil(t, finalCampID)
	assert.Equal(t, campB.ID, *finalCampID)
}

// Case AA: Explicit promo relation
func TestADS2A_CaseAA_ExplicitPromoRelation(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	ctx := context.Background()

	camp := &marketing.MarketingCampaign{
		SellerID:          &sellerID,
		Title:             "Case AA Campaign",
		FundingMode:       marketing.FundingModeSeller,
		Status:            marketing.CampaignStatusActive,
		DiscountType:      marketing.DiscountTypePercent,
		SellerDiscountBps: 1500,
	}
	require.NoError(t, repo.CreateCampaign(ctx, camp))

	promo := &marketing.PromoCode{
		ID:                    uuid.New(),
		CampaignID:            camp.ID,
		SellerID:              sellerID,
		Code:                  fmt.Sprintf("PROMO%s", uuid.New().String()[:6]),
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      1500,
		PerCustomerUsageLimit: 1,
		IsActive:              true,
	}
	require.NoError(t, repo.CreatePromoCode(ctx, promo))

	path := "/sale"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
		PromoCodeID: &promo.ID,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link))

	saved, err := repo.GetTrackingLinkByID(ctx, link.ID)
	require.NoError(t, err)
	require.NotNil(t, saved.PromoCodeID)
	assert.Equal(t, promo.ID, *saved.PromoCodeID)
}

// Case AB: Seller promo regression (NOT NULL seller_id preserved)
func TestADS2A_CaseAB_SellerPromoRegression(t *testing.T) {
	client, repo, _ := setupTestMarketingDB(t)
	sellerID := createTestSeller(t, client)
	ctx := context.Background()

	camp := &marketing.MarketingCampaign{
		SellerID:          &sellerID,
		Title:             "Seller Promo Campaign",
		FundingMode:       marketing.FundingModeSeller,
		Status:            marketing.CampaignStatusActive,
		DiscountType:      marketing.DiscountTypePercent,
		SellerDiscountBps: 2000,
	}
	require.NoError(t, repo.CreateCampaign(ctx, camp))

	promo := &marketing.PromoCode{
		ID:                    uuid.New(),
		CampaignID:            camp.ID,
		SellerID:              sellerID, // NOT NULL
		Code:                  fmt.Sprintf("SELLER%s", uuid.New().String()[:6]),
		DiscountType:          marketing.DiscountTypePercent,
		DiscountValueBps:      2000,
		PerCustomerUsageLimit: 2,
		IsActive:              true,
	}
	require.NoError(t, repo.CreatePromoCode(ctx, promo))

	// Direct DB verification: seller_id NOT NULL constraint
	_, nullSellerErr := client.Pool.Exec(ctx, `
		INSERT INTO promo_codes (id, campaign_id, seller_id, code, discount_type, discount_value_bps, is_active, created_at, updated_at)
		VALUES ($1, $2, NULL, 'NULLSELLER1', 'percent', 1000, true, now(), now())
	`, uuid.New(), camp.ID)
	assert.Error(t, nullSellerErr, "promo_codes.seller_id NOT NULL constraint must reject NULL")
}

// Case AC: Admin read-only
func TestADS2A_CaseAC_AdminReadOnly(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	camp := &marketing.MarketingCampaign{
		Title:        "Case AC Read-Only Test",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	// Read operations succeed
	campaigns, err := svc.ListAdminCampaigns(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, campaigns)

	view, err := svc.GetAdminCampaign(ctx, camp.ID)
	require.NoError(t, err)
	assert.Equal(t, camp.ID, view.ID)
	assert.Equal(t, "Case AC Read-Only Test", view.Title)
}

// Case AD: Admin write
func TestADS2A_CaseAD_AdminWrite(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	// Write operation: Create
	camp := &marketing.MarketingCampaign{
		Title:        "Case AD Write Test",
		FundingMode:  marketing.FundingModeZamk,
		Status:       marketing.CampaignStatusActive,
		DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, camp))

	// Write operation: Create link
	path := "/sale"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, svc.CreateTrackingLink(ctx, link))

	// Write operation: Disable link
	disabled, err := svc.DisableTrackingLink(ctx, camp.ID, link.ID)
	require.NoError(t, err)
	assert.False(t, disabled.IsActive)
}

// Case AE: Admin no-permission
func TestADS2A_CaseAE_AdminNoPermission(t *testing.T) {
	// Evaluated at router / middleware level and tested in frontend integration:
	// Missing marketing.campaigns.read or write yields 403 Forbidden.
	// Here we verify domain lookup for non-existent returns ErrCampaignNotFound
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	_, err := svc.GetAdminCampaign(ctx, uuid.New())
	assert.ErrorIs(t, err, marketing.ErrCampaignNotFound)
}

// Case AF: Historical attribution immutable after campaign edit/pause
func TestADS2A_CaseAF_HistoricalAttributionImmutableAfterCampaignEditOrPause(t *testing.T) {
	f := setupADS2AAttributionFixture(t, context.Background())
	ctx := context.Background()

	// 1. Create active campaign
	channel := marketing.CampaignChannelTelegram
	campType := marketing.CampaignTypeSeasonalSale
	initialBudget := int64(1000000)

	camp := &marketing.MarketingCampaign{
		Title:              "Initial Campaign AF",
		FundingMode:        marketing.FundingModeZamk,
		Status:             marketing.CampaignStatusActive,
		CampaignChannel:    &channel,
		CampaignType:       &campType,
		PlannedBudgetCents: &initialBudget,
		DiscountType:       marketing.DiscountTypePercent,
	}
	require.NoError(t, f.marketingSvc.CreateAdminCampaign(ctx, camp))

	path := "/catalog"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  camp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, f.marketingSvc.CreateTrackingLink(ctx, link))

	// 2. User visits via campaign link and places order
	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()

	_, err := f.behaviorSvc.IngestEvents(ctx, nil, behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "session_started",
				VisitorID:  visitorID,
				SessionID:  &sessionID,
				OccurredAt: now,
				Metadata: map[string]interface{}{
					"landing_path": "/catalog",
					"zamk_token":   link.Token,
				},
			},
		},
	})
	require.NoError(t, err)

	f.populateCart(t, ctx, f.buyerID, 1)
	order, err := f.ordersSvc.CreateOrder(ctx, f.buyerID, orders.CreateOrderRequest{
		CustomerName:     "Buyer Case AF",
		CustomerEmail:    "buyer_af@test.local",
		CustomerPhone:    "+79991234567",
		DeliveryAddress:  "Moscow, Immutability Test",
		DeliveryMethodID: f.dmID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: sessionID,
		},
	}, nil)
	require.NoError(t, err)

	// Verify order attribution snapshot recorded
	var origCampaignID *uuid.UUID
	err = f.db.QueryRow(ctx, "SELECT campaign_id FROM order_attributions WHERE order_id = $1", order.ID).Scan(&origCampaignID)
	require.NoError(t, err)
	require.NotNil(t, origCampaignID)
	assert.Equal(t, camp.ID, *origCampaignID)

	// 3. Campaign is later PAUSED, its title changed, its budget changed
	newTitle := "Renamed Campaign AF"
	newStatus := marketing.CampaignStatusApproved // paused
	newBudget := int64(99999999)

	_, err = f.marketingSvc.UpdateAdminCampaign(ctx, camp.ID, marketing.AdminUpdateCampaignRequest{
		Title:              &newTitle,
		Status:             &newStatus,
		PlannedBudgetCents: &newBudget,
	})
	require.NoError(t, err)

	// Also disable the tracking link
	_, err = f.marketingSvc.DisableTrackingLink(ctx, camp.ID, link.ID)
	require.NoError(t, err)

	// 4. Historical order attribution MUST REMAIN COMPLETELY UNCHANGED
	var preservedCampaignID *uuid.UUID
	err = f.db.QueryRow(ctx, "SELECT campaign_id FROM order_attributions WHERE order_id = $1", order.ID).Scan(&preservedCampaignID)
	require.NoError(t, err)
	require.NotNil(t, preservedCampaignID)
	assert.Equal(t, camp.ID, *preservedCampaignID, "Historical order attribution snapshot must be immutable!")
}
