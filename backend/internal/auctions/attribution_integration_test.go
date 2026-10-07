package auctions_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auctions"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/behavior"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type auctionAttributionFixture struct {
	db           *pgxpool.Pool
	pgClient     *postgres.Client
	auctionsRepo *auctions.Repository
	auctionsSvc  *auctions.Service
	behaviorRepo *behavior.Repository
	behaviorSvc  *behavior.Service
	eventID      uuid.UUID
}

func setupAuctionAttributionFixture(t *testing.T, ctx context.Context) *auctionAttributionFixture {
	t.Helper()
	dbURL := testutil.GetTestDatabaseURL()
	require.NotEmpty(t, dbURL)

	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	testutil.AssertTestDatabase(t, db)

	pgClient := &postgres.Client{Pool: db}
	repo := auctions.NewRepository(db)
	bRepo := behavior.NewRepository(pgClient)
	bSvc := behavior.NewService(bRepo)
	hub := auctions.NewSSEHub()
	svc := auctions.NewService(repo, nil, nil, hub).WithBehavior(bSvc)

	eventID := uuid.New()
	event := &auctions.AuctionEvent{
		ID:                   eventID,
		Title:                "Attribution Proof Auction",
		Status:               auctions.AuctionStatusLive,
		StartsAt:             time.Now().Add(-1 * time.Hour),
		EndsAt:               time.Now().Add(24 * time.Hour),
		BidStepCents:         500,
		PaymentDeadlineHours: 24,
		BiddingEnabled:       true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
	err = repo.CreateEvent(ctx, event)
	require.NoError(t, err)

	return &auctionAttributionFixture{
		db:           db,
		pgClient:     pgClient,
		auctionsRepo: repo,
		auctionsSvc:  svc,
		behaviorRepo: bRepo,
		behaviorSvc:  bSvc,
		eventID:      eventID,
	}
}

func (f *auctionAttributionFixture) createWinningLot(t *testing.T, ctx context.Context, winnerID uuid.UUID) *auctions.AuctionLot {
	t.Helper()
	lotID := uuid.New()
	amountCents := int64(12000)
	deadline := time.Now().Add(24 * time.Hour)

	lot := &auctions.AuctionLot{
		ID:                  lotID,
		AuctionID:           f.eventID,
		Title:               fmt.Sprintf("Winning Lot %s", lotID.String()[:8]),
		StartPriceCents:     10000,
		CurrentBidCents:     &amountCents,
		BidStepCents:        500,
		CurrentWinnerUserID: &winnerID,
		Status:              auctions.LotStatusWonPendingPayment,
		PaymentDeadlineAt:   &deadline,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	err := f.auctionsRepo.CreateLot(ctx, lot)
	require.NoError(t, err)
	return lot
}

func (f *auctionAttributionFixture) createUser(t *testing.T, ctx context.Context, prefix string) uuid.UUID {
	t.Helper()
	uID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'hash', 'customer', 'active', now(), now())
	`, uID, prefix+" User", fmt.Sprintf("%s-%s@zamk.local", prefix, uID.String()[:8]))
	require.NoError(t, err)
	return uID
}

// 1. Valid analyticsContext -> canonical auction-created order receives snapshot
func TestAuctionAttribution_ValidContext(t *testing.T) {
	ctx := context.Background()
	f := setupAuctionAttributionFixture(t, ctx)
	defer f.db.Close()

	winnerID := f.createUser(t, ctx, "AuctionWinner")
	lot := f.createWinningLot(t, ctx, winnerID)

	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	exp := now.Add(30 * 24 * time.Hour)
	src := "telegram_ads"

	_, err := f.db.Exec(ctx, `
		INSERT INTO analytics_sessions (
			id, visitor_id, user_id, started_at, last_seen_at,
			source, medium, utm_source, utm_medium, utm_campaign,
			attribution_captured_at, attribution_expires_at
		) VALUES (
			$1, $2, $3, now(), now(),
			$4, 'cpc', $4, 'cpc', 'auction_promo',
			$5, $6
		)
	`, sessionID, visitorID, winnerID, src, now, exp)
	require.NoError(t, err)

	req := auctions.CreateAuctionOrderRequest{
		AnalyticsContext: &auctions.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: sessionID,
		},
	}

	result, err := f.auctionsSvc.CreateOrderForLot(ctx, lot.ID, winnerID, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEqual(t, uuid.Nil, result.OrderID)

	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, result.OrderID)
	require.NoError(t, err)
	require.NotNil(t, attr)

	assert.Equal(t, result.OrderID, attr.OrderID)
	require.NotNil(t, attr.VisitorID)
	assert.Equal(t, visitorID, *attr.VisitorID)
	require.NotNil(t, attr.SessionID)
	assert.Equal(t, sessionID, *attr.SessionID)
	require.NotNil(t, attr.Source)
	assert.Equal(t, "telegram_ads", *attr.Source)
	require.NotNil(t, attr.UTMCampaign)
	assert.Equal(t, "auction_promo", *attr.UTMCampaign)
}

// 2. Missing / invalid context -> auction order still succeeds
func TestAuctionAttribution_MissingContext(t *testing.T) {
	ctx := context.Background()
	f := setupAuctionAttributionFixture(t, ctx)
	defer f.db.Close()

	winnerID := f.createUser(t, ctx, "AuctionWinnerMissing")
	lot := f.createWinningLot(t, ctx, winnerID)

	// Context completely nil
	req := auctions.CreateAuctionOrderRequest{
		AnalyticsContext: nil,
	}

	result, err := f.auctionsSvc.CreateOrderForLot(ctx, lot.ID, winnerID, req)
	require.NoError(t, err, "auction order must succeed even when analytics context is absent")
	require.NotNil(t, result)

	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, result.OrderID)
	require.NoError(t, err)
	require.NotNil(t, attr)

	assert.Equal(t, result.OrderID, attr.OrderID)
	assert.Nil(t, attr.VisitorID, "visitor_id must be NULL for missing context")
	assert.Nil(t, attr.SessionID, "session_id must be NULL for missing context")
	require.NotNil(t, attr.Source)
	assert.Equal(t, "missing", *attr.Source)
}

// 3. Attribution cannot cross user/session ownership
func TestAuctionAttribution_CrossUserMismatch(t *testing.T) {
	ctx := context.Background()
	f := setupAuctionAttributionFixture(t, ctx)
	defer f.db.Close()

	winnerID := f.createUser(t, ctx, "AuctionWinnerCross")
	otherUserID := f.createUser(t, ctx, "OtherUser")
	lot := f.createWinningLot(t, ctx, winnerID)

	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	exp := now.Add(30 * 24 * time.Hour)
	src := "other_user_vip"

	// Session is strictly bound to otherUserID
	_, err := f.db.Exec(ctx, `
		INSERT INTO analytics_sessions (
			id, visitor_id, user_id, started_at, last_seen_at,
			source, medium, utm_source,
			attribution_captured_at, attribution_expires_at
		) VALUES (
			$1, $2, $3, now(), now(),
			$4, 'cpc', $4,
			$5, $6
		)
	`, sessionID, visitorID, otherUserID, src, now, exp)
	require.NoError(t, err)

	req := auctions.CreateAuctionOrderRequest{
		AnalyticsContext: &auctions.AnalyticsContextDTO{
			VisitorID: visitorID,
			SessionID: sessionID,
		},
	}

	result, err := f.auctionsSvc.CreateOrderForLot(ctx, lot.ID, winnerID, req)
	require.NoError(t, err, "auction order must succeed for winner")
	require.NotNil(t, result)

	// Verify session ownership was not stolen
	var dbUserID *uuid.UUID
	err = f.db.QueryRow(ctx, "SELECT user_id FROM analytics_sessions WHERE id = $1", sessionID).Scan(&dbUserID)
	require.NoError(t, err)
	require.NotNil(t, dbUserID)
	assert.Equal(t, otherUserID, *dbUserID, "session user_id must remain otherUserID")

	// Verify attribution degraded safely to missing and did not attach otherUser's VIP attribution
	attr, err := f.behaviorRepo.GetOrderAttribution(ctx, result.OrderID)
	require.NoError(t, err)
	require.NotNil(t, attr)
	assert.Equal(t, "missing", *attr.Source)
	assert.Nil(t, attr.VisitorID)
	assert.Nil(t, attr.SessionID)
}
