package marketing_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/behavior"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCampaignPurpose_DomainIsolation(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()
	sellerID := createTestSeller(t, client)
	promotion, err := svc.CreateSellerCampaign(ctx, sellerID, marketing.CreateSellerCampaignRequest{
		Title: "Same neutral name", DiscountType: marketing.DiscountTypePercent, SellerDiscountBps: 1000,
	})
	require.NoError(t, err)
	require.Equal(t, marketing.CampaignPurposePromotion, promotion.Purpose)

	advertising := &marketing.MarketingCampaign{
		SellerID: &sellerID, Title: "Промокод в рекламном названии", FundingMode: marketing.FundingModeSeller,
		Status: marketing.CampaignStatusActive, DiscountType: marketing.DiscountTypePercent, SellerDiscountBps: 1000,
		// Caller-supplied purpose cannot override the Ads domain.
		Purpose: marketing.CampaignPurposePromotion,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, advertising))
	saved, err := repo.GetCampaignByID(ctx, advertising.ID)
	require.NoError(t, err)
	require.Equal(t, marketing.CampaignPurposeAdvertising, saved.Purpose)
	views, err := svc.ListAdminCampaigns(ctx)
	require.NoError(t, err)
	found := false
	for _, view := range views {
		require.Equal(t, marketing.CampaignPurposeAdvertising, view.Purpose)
		require.NotEqual(t, promotion.ID, view.ID)
		if view.ID == advertising.ID {
			found = true
		}
	}
	require.True(t, found)
	_, err = svc.GetAdminCampaign(ctx, promotion.ID)
	require.ErrorIs(t, err, marketing.ErrCampaignNotFound)
	title := "Attempted domain escape"
	_, err = svc.UpdateAdminCampaign(ctx, promotion.ID, marketing.AdminUpdateCampaignRequest{Title: &title})
	require.ErrorIs(t, err, marketing.ErrCampaignNotFound)
	unchanged, err := repo.GetCampaignByID(ctx, promotion.ID)
	require.NoError(t, err)
	require.Equal(t, promotion.Title, unchanged.Title)

	_, err = client.Pool.Exec(ctx, "UPDATE marketing_campaigns SET purpose = 'promotion' WHERE id = $1", advertising.ID)
	require.ErrorContains(t, err, "purpose is immutable")
	_, err = client.Pool.Exec(ctx, "UPDATE marketing_campaigns SET purpose = 'advertising' WHERE id = $1", promotion.ID)
	require.ErrorContains(t, err, "purpose is immutable")

	// Explicit promo relation is optional and does not redefine campaign purpose.
	promo := &marketing.PromoCode{
		ID: uuid.New(), CampaignID: promotion.ID, SellerID: sellerID,
		Code:         "PURPOSE" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12],
		DiscountType: marketing.DiscountTypePercent, DiscountValueBps: 1000,
		PerCustomerUsageLimit: 1, IsActive: true,
	}
	require.NoError(t, repo.CreatePromoCode(ctx, promo))
	path := "/catalog"
	link := &marketing.CampaignTrackingLink{CampaignID: advertising.ID, TargetType: marketing.CampaignTargetLanding, LandingPath: &path, PromoCodeID: &promo.ID}
	require.NoError(t, svc.CreateTrackingLink(ctx, link))
	savedLink, err := repo.GetTrackingLinkByID(ctx, link.ID)
	require.NoError(t, err)
	require.Equal(t, &promo.ID, savedLink.PromoCodeID)
	withoutPromo := &marketing.CampaignTrackingLink{CampaignID: advertising.ID, TargetType: marketing.CampaignTargetLanding, LandingPath: &path}
	require.NoError(t, svc.CreateTrackingLink(ctx, withoutPromo))

	// Campaign panel excludes promotion sessions, but global/source truth retains them.
	from := time.Date(2080, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	_, err = client.Pool.Exec(ctx, `DELETE FROM analytics_sessions WHERE started_at >= $1 AND started_at < $2`, from, to)
	require.NoError(t, err)
	for _, id := range []uuid.UUID{advertising.ID, promotion.ID} {
		_, err = client.Pool.Exec(ctx, `INSERT INTO analytics_sessions (id, visitor_id, started_at, last_seen_at, campaign_id, source)
            VALUES ($1, $2, $3, $3, $4, 'purpose-test')`, uuid.New(), uuid.New(), from, id)
		require.NoError(t, err)
	}
	metrics, err := svc.Analytics.GetCampaigns(ctx, from, to)
	require.NoError(t, err)
	require.Len(t, metrics.Campaigns, 1)
	assert.Equal(t, advertising.ID.String(), *metrics.Campaigns[0].CampaignID)
	assert.Equal(t, marketing.CampaignStatusActive, metrics.Campaigns[0].Status)
	overview, err := svc.Analytics.GetOverview(ctx, from, to)
	require.NoError(t, err)
	assert.Equal(t, 2, overview.Current.Visits)
	sources, err := svc.Analytics.GetSources(ctx, from, to)
	require.NoError(t, err)
	require.Len(t, sources.Sources, 1)
	assert.Equal(t, 2, sources.Sources[0].Visits)
}

func TestMarketingEmptyAnalyticsSerializeArrays(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()
	from := time.Date(2199, 1, 1, 0, 0, 0, 0, time.UTC)
	sources, err := svc.Analytics.GetSources(ctx, from, from.Add(time.Hour))
	require.NoError(t, err)
	encoded, err := json.Marshal(sources)
	require.NoError(t, err)
	assert.JSONEq(t, `{"sources":[]}`, string(encoded))
	campaigns, err := svc.Analytics.GetCampaigns(ctx, from, from.Add(time.Hour))
	require.NoError(t, err)
	encoded, err = json.Marshal(campaigns)
	require.NoError(t, err)
	assert.JSONEq(t, `{"campaigns":[]}`, string(encoded))
}

func TestCampaignPurposeMigrationEvidenceAndAmbiguity(t *testing.T) {
	client, _, _ := setupTestMarketingDB(t)
	ctx := context.Background()
	migration, err := os.ReadFile(filepath.Join(findRepoRoot(), "migrations", "000110_separate_campaign_purpose.up.sql"))
	require.NoError(t, err)
	// Execute the exact migration statements inside an isolated schema and rollback.
	sql := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(string(migration)), "BEGIN;"), "COMMIT;")
	for _, ambiguous := range []bool{false, true} {
		t.Run(map[bool]string{false: "deterministic_backfill", true: "ambiguous_row_aborts"}[ambiguous], func(t *testing.T) {
			tx, err := client.Pool.Begin(ctx)
			require.NoError(t, err)
			defer tx.Rollback(ctx)
			schema := "purpose_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			_, err = tx.Exec(ctx, `CREATE SCHEMA `+schema+`; SET LOCAL search_path TO `+schema+`, public;
                CREATE TABLE marketing_campaigns (
                    id UUID PRIMARY KEY, seller_id UUID, title TEXT DEFAULT 'identical title',
                    campaign_channel TEXT, campaign_type TEXT, planned_budget_cents BIGINT,
                    seller_discount_bps INT DEFAULT 0, seller_discount_fixed_cents BIGINT DEFAULT 0,
                    requested_zamk_share_bps INT DEFAULT 0, requested_zamk_budget_cap_cents BIGINT DEFAULT 0,
                    created_at TIMESTAMPTZ DEFAULT now()
                );
                CREATE TABLE campaign_tracking_links (campaign_id UUID);`)
			require.NoError(t, err)
			seller := uuid.New()
			promotion, fixed, cofunded, platform, channel, kind, budget, linked := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
			_, err = tx.Exec(ctx, `INSERT INTO marketing_campaigns (id, seller_id, seller_discount_bps) VALUES ($1,$2,1500)`, promotion, seller)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `INSERT INTO marketing_campaigns (id, seller_id, seller_discount_fixed_cents) VALUES ($1,$2,50000)`, fixed, seller)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `INSERT INTO marketing_campaigns (id, seller_id, requested_zamk_share_bps) VALUES ($1,$2,1000)`, cofunded, seller)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `INSERT INTO marketing_campaigns (id) VALUES ($1)`, platform)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `INSERT INTO marketing_campaigns (id, seller_id, campaign_channel) VALUES ($1,$2,'telegram')`, channel, seller)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `INSERT INTO marketing_campaigns (id, seller_id, campaign_type) VALUES ($1,$2,'drop')`, kind, seller)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `INSERT INTO marketing_campaigns (id, seller_id, planned_budget_cents) VALUES ($1,$2,0)`, budget, seller)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `INSERT INTO marketing_campaigns (id, seller_id, seller_discount_bps) VALUES ($1,$2,1000)`, linked, seller)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `INSERT INTO campaign_tracking_links VALUES ($1)`, linked)
			require.NoError(t, err)
			uncertain := uuid.New()
			if ambiguous {
				_, err = tx.Exec(ctx, `INSERT INTO marketing_campaigns (id, seller_id) VALUES ($1,$2)`, uncertain, seller)
				require.NoError(t, err)
			}
			_, err = tx.Exec(ctx, sql)
			if ambiguous {
				require.ErrorContains(t, err, uncertain.String())
				require.ErrorContains(t, err, "Ambiguous campaign purpose")
				return
			}
			require.NoError(t, err)
			for _, id := range []uuid.UUID{promotion, fixed, cofunded, platform, channel, kind, budget, linked} {
				var purpose string
				require.NoError(t, tx.QueryRow(ctx, `SELECT purpose FROM marketing_campaigns WHERE id=$1`, id).Scan(&purpose))
				expected := "advertising"
				if id == promotion || id == fixed || id == cofunded {
					expected = "promotion"
				}
				assert.Equal(t, expected, purpose, id.String())
			}
		})
	}
}

func TestCampaignPurpose_SellerAdvertising_ZeroEconomicDiscount(t *testing.T) {
	ctx := context.Background()
	f := setupADS2AAttributionFixture(t, ctx)
	defer f.db.Close()

	// 1. ZAMK-owned advertising creation
	channelZamk := marketing.CampaignChannelTelegram
	typeZamk := marketing.CampaignTypeBrandAwareness
	zamkCamp := &marketing.MarketingCampaign{
		Title:           "Platform Ads Campaign",
		FundingMode:     marketing.FundingModeZamk,
		Status:          marketing.CampaignStatusActive,
		CampaignChannel: &channelZamk,
		CampaignType:    &typeZamk,
		DiscountType:    marketing.DiscountTypePercent,
	}
	require.NoError(t, f.marketingSvc.CreateAdminCampaign(ctx, zamkCamp))
	assert.Equal(t, marketing.CampaignPurposeAdvertising, zamkCamp.Purpose)
	assert.Nil(t, zamkCamp.SellerID)

	// 2. Seller-owned advertising creation
	channelSeller := marketing.CampaignChannelVK
	typeSeller := marketing.CampaignTypeDrop
	sellerCamp := &marketing.MarketingCampaign{
		SellerID:          &f.sellerID,
		Title:             "Seller Ads Campaign",
		FundingMode:       marketing.FundingModeSeller,
		Status:            marketing.CampaignStatusActive,
		CampaignChannel:   &channelSeller,
		CampaignType:      &typeSeller,
		DiscountType:      marketing.DiscountTypePercent,
		SellerDiscountBps: 0, // frontend passes 0
	}
	require.NoError(t, f.marketingSvc.CreateAdminCampaign(ctx, sellerCamp))
	assert.Equal(t, marketing.CampaignPurposeAdvertising, sellerCamp.Purpose)
	require.NotNil(t, sellerCamp.SellerID)
	assert.Equal(t, f.sellerID, *sellerCamp.SellerID)

	// Legacy DB storage requirement check:
	// marketing_campaigns row stores 1000 bps solely to satisfy table check constraint chk_mc_seller_positive_discount.
	savedSellerCamp, err := f.marketingSvc.GetAdminCampaign(ctx, sellerCamp.ID)
	require.NoError(t, err)
	assert.Equal(t, marketing.CampaignPurposeAdvertising, savedSellerCamp.Purpose)
	assert.Equal(t, 1000, savedSellerCamp.SellerDiscountBps)

	// 3. Exact proof that seller advertising does NOT create or apply any commercial discount:
	// A) No promo codes are created automatically
	var promoCount int
	err = f.db.QueryRow(ctx, "SELECT COUNT(*) FROM promo_codes WHERE campaign_id = $1", sellerCamp.ID).Scan(&promoCount)
	require.NoError(t, err)
	assert.Equal(t, 0, promoCount, "Advertising campaign must not create promo codes")

	// B) Attempting to attach a promo code to an advertising campaign fails closed
	_, err = f.marketingSvc.CreatePromoCode(ctx, f.sellerID, marketing.CreatePromoCodeRequest{
		CampaignID:       sellerCamp.ID,
		Code:             "FAILPROMO",
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.ErrorIs(t, err, marketing.ErrCampaignNotFound, "Cannot attach promo code to advertising campaign")

	// C) Customer visits via campaign tracking link and places an order
	path := "/catalog"
	link := &marketing.CampaignTrackingLink{
		CampaignID:  sellerCamp.ID,
		TargetType:  marketing.CampaignTargetLanding,
		LandingPath: &path,
	}
	require.NoError(t, f.marketingSvc.CreateTrackingLink(ctx, link))

	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	_, err = f.behaviorSvc.IngestEvents(ctx, nil, behavior.EventIngestionRequest{
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
		CustomerName:     "Buyer Zero Discount",
		CustomerEmail:    "buyer_zero@test.local",
		CustomerPhone:    "+79990000000",
		DeliveryAddress:  "Moscow, Test Ave 1",
		DeliveryMethodID: f.dmID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: sessionID,
		},
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, order)

	// Zero economic discount applied: total is full product price (150000) + delivery (200)
	assert.Equal(t, int64(150200), order.TotalPriceCents, "Total must be full product price (150000) + delivery (200)")
	require.Len(t, order.Items, 1)
	assert.Equal(t, int64(150000), order.Items[0].PriceCents, "Item price is full base price (150000) without discount")
	assert.Equal(t, int64(150000), order.Items[0].SubtotalPriceCents, "Customer paid full base price without discount")

	// Verify no promotional link was recorded for the order
	var orderPromoCount int
	err = f.db.QueryRow(ctx, "SELECT COUNT(*) FROM promo_code_usages WHERE order_id = $1", order.ID).Scan(&orderPromoCount)
	require.NoError(t, err)
	assert.Equal(t, 0, orderPromoCount, "No promo code usage should exist for non-promo order")

	// Attribution is correctly recorded without introducing any discount
	var attrCampaignID *uuid.UUID
	err = f.db.QueryRow(ctx, "SELECT campaign_id FROM order_attributions WHERE order_id = $1", order.ID).Scan(&attrCampaignID)
	require.NoError(t, err)
	require.NotNil(t, attrCampaignID)
	assert.Equal(t, sellerCamp.ID, *attrCampaignID)

	// 4. Existing promotion discount semantics remain unchanged
	promoCampaign, err := f.marketingSvc.CreateSellerCampaign(ctx, f.sellerID, marketing.CreateSellerCampaignRequest{
		Title:             "Real 10pct Promotion",
		DiscountType:      marketing.DiscountTypePercent,
		SellerDiscountBps: 1000,
	})
	require.NoError(t, err)
	assert.Equal(t, marketing.CampaignPurposePromotion, promoCampaign.Purpose)

	uniqueCode := "SAVE10_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	promoCode, err := f.marketingSvc.CreatePromoCode(ctx, f.sellerID, marketing.CreatePromoCodeRequest{
		CampaignID:       promoCampaign.ID,
		Code:             uniqueCode,
		DiscountType:     marketing.DiscountTypePercent,
		DiscountValueBps: 1000,
	})
	require.NoError(t, err)
	assert.NotNil(t, promoCode)

	_, err = f.db.Exec(ctx, "UPDATE marketing_campaigns SET status = 'active' WHERE id = $1", promoCampaign.ID)
	require.NoError(t, err)

	items := []marketing.PromotedOrderItemInput{
		{
			OrderItemID:        uuid.New(),
			ProductID:          f.prodID,
			ProductVariantID:   f.variantID,
			SellerID:           f.sellerID,
			BaseUnitPriceCents: 150000,
			Quantity:           1,
		},
	}
	tx, err := f.db.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	calc, err := f.marketingSvc.ValidateAndCalculateCheckoutPromoTx(ctx, tx, f.buyerID, uniqueCode, items, 1500, time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, calc)
	assert.Equal(t, int64(15000), calc.TotalSellerDiscountCents, "Promotion must apply exactly 10% discount (15000 cents)")
	assert.Equal(t, int64(135000), calc.TotalCustomerPaidCents, "Customer must pay 135000 cents with promotion")
}
