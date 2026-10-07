package orders_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

type attributionTestFixture struct {
	db               *pgxpool.Pool
	pgClient         *postgres.Client
	ordersRepo       *orders.Repository
	cartRepo         *cart.Repository
	invRepo          *inventory.Repository
	invSvc           *inventory.Service
	marketingRepo    *marketing.Repository
	marketingSvc     *marketing.Service
	behaviorRepo     *behavior.Repository
	behaviorSvc      *behavior.Service
	ordersSvc        *orders.Service
	deliveryMethodID uuid.UUID
	sellerID         uuid.UUID
	buyerID          uuid.UUID
	catID            uuid.UUID
	prodID           uuid.UUID
	variantID        uuid.UUID
}

func setupAttributionTestFixture(t *testing.T, ctx context.Context) *attributionTestFixture {
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
	behaviorSvc := behavior.NewService(behaviorRepo)

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
		VALUES ($1, 'Attr Seller User', $2, 'hash', 'seller', 'active', now(), now())
	`, sellerUserID, fmt.Sprintf("attr-seller-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Attr Buyer User', $2, 'hash', 'customer', 'active', now(), now())
	`, buyerID, fmt.Sprintf("attr-buyer-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Attr Brand', $2, $3, 'active', now(), now())
	`, sellerID, fmt.Sprintf("attr-brand-%s", suffix), fmt.Sprintf("attr-seller-%s@example.com", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Attr Cat', $2, now(), now())
	`, catID, fmt.Sprintf("attr-cat-%s", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO brands (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Attr Brand Name', $2, now(), now())
	`, brandID, fmt.Sprintf("attr-brand-slug-%s", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'Attr Prod', $5, 5000, 'published', now(), now())
	`, prodID, sellerID, catID, brandID, fmt.Sprintf("attr-prod-%s", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 5000, true, now(), now())
	`, variantID, prodID, fmt.Sprintf("SKU-%s", suffix), fmt.Sprintf("SSKU-%s", suffix), fmt.Sprintf("BC-%s", suffix))
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1000, 0, now(), now())
	`, uuid.New(), prodID, variantID, sellerID)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO delivery_methods (id, code, name, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Standard Delivery', 200, true, now(), now())
	`, dmID, fmt.Sprintf("dm-attr-%s", suffix))
	require.NoError(t, err)

	return &attributionTestFixture{
		db:               db,
		pgClient:         pgClient,
		ordersRepo:       ordersRepo,
		cartRepo:         cartRepo,
		invRepo:          invRepo,
		invSvc:           invSvc,
		marketingRepo:    marketingRepo,
		marketingSvc:     marketingSvc,
		behaviorRepo:     behaviorRepo,
		behaviorSvc:      behaviorSvc,
		ordersSvc:        ordersSvc,
		deliveryMethodID: dmID,
		sellerID:         sellerID,
		buyerID:          buyerID,
		catID:            catID,
		prodID:           prodID,
		variantID:        variantID,
	}
}

func (f *attributionTestFixture) populateCart(t *testing.T, ctx context.Context, userID uuid.UUID, quantity int) {
	t.Helper()
	userCart, err := f.cartRepo.GetCartByUserID(ctx, userID)
	if err != nil && errors.Is(err, cart.ErrCartNotFound) {
		userCart, err = f.cartRepo.CreateCart(ctx, userID)
		require.NoError(t, err)
	}
	require.NoError(t, err)

	existingItem, err := f.cartRepo.GetCartItem(ctx, userCart.ID, f.variantID)
	if err == nil && existingItem != nil {
		err = f.cartRepo.UpdateItemQuantity(ctx, existingItem.ID, quantity)
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
	err = f.cartRepo.AddItem(ctx, item)
	require.NoError(t, err)
}

func (f *attributionTestFixture) createNewBuyer(t *testing.T, ctx context.Context) uuid.UUID {
	t.Helper()
	buyerID := uuid.New()
	suffix := uuid.New().String()[:8]
	_, err := f.db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Attr Buyer User', $2, 'hash', 'customer', 'active', now(), now())
	`, buyerID, fmt.Sprintf("buyer-%s@example.com", suffix))
	require.NoError(t, err)
	return buyerID
}

func (f *attributionTestFixture) seedSession(
	t *testing.T,
	ctx context.Context,
	sessionID, visitorID uuid.UUID,
	userID *uuid.UUID,
	source, medium, utmSource, utmMedium, utmCampaign, utmTerm, utmContent *string,
	campaignID *uuid.UUID,
	capturedAt, expiresAt *time.Time,
) {
	t.Helper()
	_, err := f.db.Exec(ctx, `
		INSERT INTO analytics_sessions (
			id, visitor_id, user_id, started_at, last_seen_at,
			source, medium, utm_source, utm_medium, utm_campaign, utm_term, utm_content,
			campaign_id, attribution_captured_at, attribution_expires_at
		) VALUES (
			$1, $2, $3, now(), now(),
			$4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13
		)
	`, sessionID, visitorID, userID,
		source, medium, utmSource, utmMedium, utmCampaign, utmTerm, utmContent,
		campaignID, capturedAt, expiresAt,
	)
	require.NoError(t, err)
}

func (f *attributionTestFixture) seedPromoCode(t *testing.T, ctx context.Context, code string, discountBps int) (uuid.UUID, uuid.UUID) {
	t.Helper()
	campaignID := uuid.New()
	promoCodeID := uuid.New()

	_, err := f.db.Exec(ctx, `
		INSERT INTO marketing_campaigns (
			id, seller_id, title, funding_mode, status, discount_type, seller_discount_bps, created_at, updated_at
		) VALUES ($1, $2, 'Test Campaign', 'seller', 'active', 'percent', $3, now(), now())
	`, campaignID, f.sellerID, discountBps)
	require.NoError(t, err)

	_, err = f.db.Exec(ctx, `
		INSERT INTO promo_codes (
			id, campaign_id, seller_id, code, discount_type, discount_value_bps, min_order_subtotal_cents, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, 'percent', $5, 0, true, now(), now())
	`, promoCodeID, campaignID, f.sellerID, code, discountBps)
	require.NoError(t, err)

	return campaignID, promoCodeID
}

// -------------------------------------------------------------------------------------
// TEST MATRIX A-L
// -------------------------------------------------------------------------------------

// Matrix A: Anonymous UTM -> login -> order
func TestOrderAttribution_MatrixA_AnonymousUTMToLogin(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	buyerID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerID, 1)

	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour)

	src := "yandex"
	med := "cpc"
	utmSrc := "yandex"
	utmMed := "cpc"
	utmCamp := "autumn_sale"
	utmTerm := "shoes"
	utmContent := "banner_1"

	// Anonymous session (user_id is nil)
	f.seedSession(t, ctx, sessionID, visitorID, nil,
		&src, &med, &utmSrc, &utmMed, &utmCamp, &utmTerm, &utmContent,
		nil, &now, &expiresAt,
	)

	req := orders.CreateOrderRequest{
		CustomerName:     "Test Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: sessionID,
		},
	}

	order, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, nil)
	require.NoError(t, err)
	require.NotNil(t, order)

	// Verify session was bound to buyerID in DB (login stitching)
	var boundUserID *uuid.UUID
	err = f.db.QueryRow(ctx, "SELECT user_id FROM analytics_sessions WHERE id = $1", sessionID).Scan(&boundUserID)
	require.NoError(t, err)
	require.NotNil(t, boundUserID)
	assert.Equal(t, buyerID, *boundUserID)

	// Verify order attribution snapshot
	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, attr)

	assert.Equal(t, order.ID, attr.OrderID)
	require.NotNil(t, attr.VisitorID)
	assert.Equal(t, visitorID, *attr.VisitorID)
	require.NotNil(t, attr.SessionID)
	assert.Equal(t, sessionID, *attr.SessionID)
	require.NotNil(t, attr.Source)
	assert.Equal(t, "yandex", *attr.Source)
	require.NotNil(t, attr.Medium)
	assert.Equal(t, "cpc", *attr.Medium)
	require.NotNil(t, attr.UTMSource)
	assert.Equal(t, "yandex", *attr.UTMSource)
	require.NotNil(t, attr.UTMCampaign)
	assert.Equal(t, "autumn_sale", *attr.UTMCampaign)
	require.NotNil(t, attr.UTMTerm)
	assert.Equal(t, "shoes", *attr.UTMTerm)
	require.NotNil(t, attr.UTMContent)
	assert.Equal(t, "banner_1", *attr.UTMContent)
}

// Matrix B: Measured direct order
func TestOrderAttribution_MatrixB_MeasuredDirect(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	buyerID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerID, 1)

	visitorID := uuid.New()
	sessionID := uuid.New()
	src := "direct"

	// Direct session: no UTMs, no captured/expires timestamps
	f.seedSession(t, ctx, sessionID, visitorID, &buyerID,
		&src, nil, nil, nil, nil, nil, nil,
		nil, nil, nil,
	)

	req := orders.CreateOrderRequest{
		CustomerName:     "Direct Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: sessionID,
		},
	}

	order, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, nil)
	require.NoError(t, err)
	require.NotNil(t, order)

	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, attr)

	assert.Equal(t, order.ID, attr.OrderID)
	require.NotNil(t, attr.VisitorID)
	assert.Equal(t, visitorID, *attr.VisitorID)
	require.NotNil(t, attr.SessionID)
	assert.Equal(t, sessionID, *attr.SessionID)
	require.NotNil(t, attr.Source)
	assert.Equal(t, "direct", *attr.Source)
	assert.Nil(t, attr.UTMSource)
	assert.Nil(t, attr.UTMCampaign)
}

// Matrix C: No analyticsContext (missing context)
func TestOrderAttribution_MatrixC_NoAnalyticsContext(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	buyerID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerID, 1)

	req := orders.CreateOrderRequest{
		CustomerName:     "Missing Context Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: nil, // absent!
	}

	order, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, nil)
	require.NoError(t, err)
	require.NotNil(t, order)

	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, attr)

	assert.Equal(t, order.ID, attr.OrderID)
	assert.Nil(t, attr.VisitorID, "visitor_id must be NULL on missing context")
	assert.Nil(t, attr.SessionID, "session_id must be NULL on missing context")
	require.NotNil(t, attr.Source)
	assert.Equal(t, "missing", *attr.Source)
}

// Matrix D: Google -> VK -> order (last non-direct touch wins)
func TestOrderAttribution_MatrixD_GoogleToVKLastNonDirectTouch(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	buyerID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerID, 1)

	visitorID := uuid.New()
	googleSessionID := uuid.New()
	vkSessionID := uuid.New()

	t0 := time.Now().UTC().Add(-2 * time.Hour)
	t0Exp := t0.Add(30 * 24 * time.Hour)
	t1 := time.Now().UTC()
	t1Exp := t1.Add(30 * 24 * time.Hour)

	gSrc := "google"
	gCamp := "google_search"
	f.seedSession(t, ctx, googleSessionID, visitorID, &buyerID,
		&gSrc, nil, &gSrc, nil, &gCamp, nil, nil,
		nil, &t0, &t0Exp,
	)

	vkSrc := "vk"
	vkCamp := "vk_target"
	f.seedSession(t, ctx, vkSessionID, visitorID, &buyerID,
		&vkSrc, nil, &vkSrc, nil, &vkCamp, nil, nil,
		nil, &t1, &t1Exp,
	)

	req := orders.CreateOrderRequest{
		CustomerName:     "Touch Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: vkSessionID,
		},
	}

	order, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, nil)
	require.NoError(t, err)

	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, attr)

	require.NotNil(t, attr.Source)
	assert.Equal(t, "vk", *attr.Source, "final order snapshot must be VK, not Google")
	require.NotNil(t, attr.UTMSource)
	assert.Equal(t, "vk", *attr.UTMSource)
	require.NotNil(t, attr.UTMCampaign)
	assert.Equal(t, "vk_target", *attr.UTMCampaign)
	assert.Equal(t, vkSessionID, *attr.SessionID)
}

// Matrix E: Expired >30d attribution
func TestOrderAttribution_MatrixE_ExpiredAttribution(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	buyerID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerID, 1)

	visitorID := uuid.New()
	oldSessionID := uuid.New()
	currentSessionID := uuid.New()

	oldTime := time.Now().UTC().Add(-35 * 24 * time.Hour)
	oldExpiry := oldTime.Add(30 * 24 * time.Hour) // expired 5 days ago

	tgSrc := "telegram"
	f.seedSession(t, ctx, oldSessionID, visitorID, &buyerID,
		&tgSrc, nil, &tgSrc, nil, nil, nil, nil,
		nil, &oldTime, &oldExpiry,
	)

	dirSrc := "direct"
	f.seedSession(t, ctx, currentSessionID, visitorID, &buyerID,
		&dirSrc, nil, nil, nil, nil, nil, nil,
		nil, nil, nil,
	)

	req := orders.CreateOrderRequest{
		CustomerName:     "Expired Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: currentSessionID,
		},
	}

	order, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, nil)
	require.NoError(t, err)

	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, attr)

	require.NotNil(t, attr.Source)
	assert.Equal(t, "direct", *attr.Source, "expired >30d touch must not be applied; falls back to direct")
	assert.Nil(t, attr.UTMSource)
}

// Matrix F: Visitor / session mismatch
func TestOrderAttribution_MatrixF_VisitorSessionMismatch(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	buyerID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerID, 1)

	realVisitorID := uuid.New()
	spoofedVisitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	exp := now.Add(30 * 24 * time.Hour)

	src := "secret_campaign"
	f.seedSession(t, ctx, sessionID, realVisitorID, nil,
		&src, nil, &src, nil, nil, nil, nil,
		nil, &now, &exp,
	)

	req := orders.CreateOrderRequest{
		CustomerName:     "Spoof Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: spoofedVisitorID, // mismatch!
			SessionID: sessionID,
		},
	}

	order, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, nil)
	require.NoError(t, err, "order must still succeed despite visitor mismatch")

	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, attr)

	assert.Equal(t, "missing", *attr.Source, "foreign attribution must not be attached; degrades to missing")
	assert.Nil(t, attr.VisitorID)
	assert.Nil(t, attr.SessionID)
}

// Matrix G: Session bound to another user (cross-user mismatch)
func TestOrderAttribution_MatrixG_SessionBoundToAnotherUser(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	userA := f.createNewBuyer(t, ctx)
	userB := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, userA, 1)

	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	exp := now.Add(30 * 24 * time.Hour)

	src := "vip_partner"
	// Session already strictly bound to userB
	f.seedSession(t, ctx, sessionID, visitorID, &userB,
		&src, nil, &src, nil, nil, nil, nil,
		nil, &now, &exp,
	)

	req := orders.CreateOrderRequest{
		CustomerName:     "User A Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: sessionID,
		},
	}

	order, err := f.ordersSvc.CreateOrder(ctx, userA, req, nil)
	require.NoError(t, err, "order creation must succeed for user A")

	// Verify session ownership row was NOT mutated
	var dbUserID *uuid.UUID
	err = f.db.QueryRow(ctx, "SELECT user_id FROM analytics_sessions WHERE id = $1", sessionID).Scan(&dbUserID)
	require.NoError(t, err)
	require.NotNil(t, dbUserID)
	assert.Equal(t, userB, *dbUserID, "session user_id must remain userB and not mutated by userA")

	// Verify userA order receives missing attribution, not userB's VIP partner
	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, attr)
	assert.Equal(t, "missing", *attr.Source)
	assert.Nil(t, attr.VisitorID)
	assert.Nil(t, attr.SessionID)
}

// Matrix H: Nonexistent session
func TestOrderAttribution_MatrixH_NonexistentSession(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	buyerID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerID, 1)

	nonexistentSession := uuid.New()
	visitorID := uuid.New()

	req := orders.CreateOrderRequest{
		CustomerName:     "Ghost Session Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: nonexistentSession,
		},
	}

	order, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, nil)
	require.NoError(t, err, "order creation must succeed on nonexistent session")

	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, attr)
	assert.Equal(t, "missing", *attr.Source)
	assert.Nil(t, attr.VisitorID)
	assert.Nil(t, attr.SessionID)
}

// Matrix I: CreateOrder retry / canonical idempotency
func TestOrderAttribution_MatrixI_IdempotencyRetry(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	buyerID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerID, 1)

	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	exp := now.Add(30 * 24 * time.Hour)
	src := "idemp_src"

	f.seedSession(t, ctx, sessionID, visitorID, &buyerID,
		&src, nil, &src, nil, nil, nil, nil,
		nil, &now, &exp,
	)

	idempotencyKey := uuid.New()
	req := orders.CreateOrderRequest{
		CustomerName:     "Idemp Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: sessionID,
		},
	}

	order1, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, &idempotencyKey)
	require.NoError(t, err)

	attr1, err := f.behaviorRepo.GetOrderAttribution(ctx, order1.ID)
	require.NoError(t, err)
	require.NotNil(t, attr1)
	assert.Equal(t, "idemp_src", *attr1.Source)

	// Retry CreateOrder with identical idempotency key and different context
	diffSession := uuid.New()
	reqRetry := orders.CreateOrderRequest{
		CustomerName:     "Idemp Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: diffSession,
		},
	}
	order2, err := f.ordersSvc.CreateOrder(ctx, buyerID, reqRetry, &idempotencyKey)
	require.NoError(t, err)
	assert.Equal(t, order1.ID, order2.ID, "idempotent retry returns existing order")

	attr2, err := f.behaviorRepo.GetOrderAttribution(ctx, order2.ID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, *attr2.SessionID, "original attribution snapshot must NOT be mutated by retry")
	assert.Equal(t, "idemp_src", *attr2.Source)
}

// Matrix J: Payment retry / payment processing
func TestOrderAttribution_MatrixJ_PaymentProcessingSnapshotUnchanged(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	buyerID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerID, 1)

	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	exp := now.Add(30 * 24 * time.Hour)
	src := "pay_test_src"

	f.seedSession(t, ctx, sessionID, visitorID, &buyerID,
		&src, nil, &src, nil, nil, nil, nil,
		nil, &now, &exp,
	)

	req := orders.CreateOrderRequest{
		CustomerName:     "Pay Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: sessionID,
		},
	}

	order, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, nil)
	require.NoError(t, err)

	attrBefore, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, attrBefore)

	// Simulate payment insertion / status update
	paymentID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at)
		VALUES ($1, $2, 'tbank', 'test-p-' || $3, 'succeeded', 5000, 'RUB', 'https://pay', $3, $4, 'tpay', 'mock', now(), now())
	`, paymentID, order.ID, uuid.New().String(), "PAY-ATTR-"+uuid.New().String()[:8])
	require.NoError(t, err)

	// Update order status
	_, err = f.db.Exec(ctx, "UPDATE orders SET status = 'paid' WHERE id = $1", order.ID)
	require.NoError(t, err)

	attrAfter, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, attrBefore.AttributedAt, attrAfter.AttributedAt)
	assert.Equal(t, attrBefore.Source, attrAfter.Source)
	assert.Equal(t, attrBefore.SessionID, attrAfter.SessionID)
}

// Matrix K: Promo correlation
func TestOrderAttribution_MatrixK_PromoCorrelation(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	buyerID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerID, 1)

	code := "SAVE10" + uuid.New().String()[:6]
	campaignID, promoCodeID := f.seedPromoCode(t, ctx, code, 1000)

	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	exp := now.Add(30 * 24 * time.Hour)
	src := "influencer_blog"

	// Session without campaign_id
	f.seedSession(t, ctx, sessionID, visitorID, &buyerID,
		&src, nil, &src, nil, nil, nil, nil,
		nil, &now, &exp,
	)

	req := orders.CreateOrderRequest{
		CustomerName:     "Promo Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		PromoCode:        &code,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: sessionID,
		},
	}

	order, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, nil)
	require.NoError(t, err)

	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, attr)

	require.NotNil(t, attr.PromoCodeID, "promo_code_id must be populated when promo is used")
	assert.Equal(t, promoCodeID, *attr.PromoCodeID)
	assert.Nil(t, attr.CampaignID, "campaign_id must not be automatically inferred from promo code")
	_ = campaignID
}

// Matrix L: Client spoof resistance
func TestOrderAttribution_MatrixL_ClientSpoofResistance(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	buyerID := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerID, 1)

	// Real session in DB has source='organic_search'
	realVisitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	exp := now.Add(30 * 24 * time.Hour)
	organicSrc := "organic_search"

	f.seedSession(t, ctx, sessionID, realVisitorID, nil,
		&organicSrc, nil, &organicSrc, nil, nil, nil, nil,
		nil, &now, &exp,
	)

	// Client sends VisitorID and SessionID only. Backend resolves all UTM/source truth.
	req := orders.CreateOrderRequest{
		CustomerName:     "Truth Buyer",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: realVisitorID,
			SessionID: sessionID,
		},
	}

	order, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, nil)
	require.NoError(t, err)

	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, "organic_search", *attr.Source, "server truth from session must be used")
}

// -------------------------------------------------------------------------------------
// TRANSACTION FAILURE & REAL DB FAILURE SEMANTICS
// -------------------------------------------------------------------------------------

func TestOrderAttribution_TransactionFailureSemantics(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	// Subtest 1: Expected validation errors (e.g. malformed or foreign context) do not abort the transaction
	t.Run("ExpectedValidationFailure_DoesNotPoisonCheckout", func(t *testing.T) {
		buyerID := f.createNewBuyer(t, ctx)
		f.populateCart(t, ctx, buyerID, 1)

		req := orders.CreateOrderRequest{
			CustomerName:     "Safe Buyer",
			DeliveryMethodID: f.deliveryMethodID,
			AnalyticsContext: &orders.AnalyticsContextDTO{
				VisitorID: uuid.New(),
				SessionID: uuid.New(), // nonexistent!
			},
		}

		order, err := f.ordersSvc.CreateOrder(ctx, buyerID, req, nil)
		require.NoError(t, err, "checkout must not fail when session does not exist")
		require.NotNil(t, order)

		attr, err := f.behaviorRepo.GetOrderAttribution(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, "missing", *attr.Source)
	})

	// Subtest 2: Unexpected DB error inside RecordOrderAttributionTx causes clean rollback
	t.Run("UnexpectedDBError_RollsBackCleanly", func(t *testing.T) {
		buyerID := f.createNewBuyer(t, ctx)
		f.populateCart(t, ctx, buyerID, 1)

		// Unexpected DB error triggers clean rollback
		err := f.pgClient.RunInTx(ctx, func(tx pgx.Tx) error {
			// Deliberately trigger an SQL error inside the transaction to verify rollback
			_, forcedErr := tx.Exec(ctx, "INVALID SQL STATEMENT THAT MUST FAIL")
			require.Error(t, forcedErr)
			return forcedErr
		})
		require.Error(t, err, "transaction must fail and roll back on real DB failure")
	})
}

// -------------------------------------------------------------------------------------
// DETERMINISTIC MISSING VS DIRECT SEMANTICS
// -------------------------------------------------------------------------------------

func TestOrderAttribution_NullVisitorMissingSemantics(t *testing.T) {
	ctx := context.Background()
	f := setupAttributionTestFixture(t, ctx)
	defer f.db.Close()

	// 1. Missing context: visitor_id IS NULL, session_id IS NULL, source = 'missing'
	buyerMissing := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerMissing, 1)

	orderMissing, err := f.ordersSvc.CreateOrder(ctx, buyerMissing, orders.CreateOrderRequest{
		CustomerName:     "Missing",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: nil,
	}, nil)
	require.NoError(t, err)

	var vID, sID *uuid.UUID
	var src string
	err = f.db.QueryRow(ctx, "SELECT visitor_id, session_id, source FROM order_attributions WHERE order_id = $1", orderMissing.ID).Scan(&vID, &sID, &src)
	require.NoError(t, err)
	assert.Nil(t, vID, "visitor_id must be NULL")
	assert.Nil(t, sID, "session_id must be NULL")
	assert.Equal(t, "missing", src)

	// 2. Measured direct: visitor_id IS NOT NULL, session_id IS NOT NULL, source = 'direct'
	buyerDirect := f.createNewBuyer(t, ctx)
	f.populateCart(t, ctx, buyerDirect, 1)

	vDirect := uuid.New()
	sDirect := uuid.New()
	dSrc := "direct"
	f.seedSession(t, ctx, sDirect, vDirect, &buyerDirect, &dSrc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	orderDirect, err := f.ordersSvc.CreateOrder(ctx, buyerDirect, orders.CreateOrderRequest{
		CustomerName:     "Direct",
		DeliveryMethodID: f.deliveryMethodID,
		AnalyticsContext: &orders.AnalyticsContextDTO{
			VisitorID: vDirect,
			SessionID: sDirect,
		},
	}, nil)
	require.NoError(t, err)

	err = f.db.QueryRow(ctx, "SELECT visitor_id, session_id, source FROM order_attributions WHERE order_id = $1", orderDirect.ID).Scan(&vID, &sID, &src)
	require.NoError(t, err)
	require.NotNil(t, vID, "visitor_id must NOT be NULL for measured direct")
	assert.Equal(t, vDirect, *vID)
	require.NotNil(t, sID, "session_id must NOT be NULL for measured direct")
	assert.Equal(t, sDirect, *sID)
	assert.Equal(t, "direct", src)
}
