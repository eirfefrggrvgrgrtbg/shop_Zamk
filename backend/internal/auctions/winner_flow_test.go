package auctions

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/ratelimit"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestWinnerFlow(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Run with INTEGRATION_TEST=1")
	}

	ctx := context.Background()
	dbURL := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to DB: %v", err)
	}
	defer pgClient.Pool.Close()

	testutil.AssertTestDatabase(t, pgClient.Pool)

	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	limiter := ratelimit.New(redisClient)

	notifRepo := notifications.NewRepository(pgClient)
	notifService := notifications.NewService(notifRepo, nil, nil)

	repo := NewRepository(pgClient.Pool)
	hub := NewSSEHub()
	svc := NewService(repo, notifService, limiter, hub)

	winnerID := uuid.New()
	nonWinnerID := uuid.New()

	// Seed users
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at) VALUES ($1, 'Winner', $2, 'y', 'customer', 'active', now(), now()) ON CONFLICT (id) DO NOTHING`, winnerID, uuid.New().String()+"@test.com")
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at) VALUES ($1, 'Loser', $2, 'y', 'customer', 'active', now(), now()) ON CONFLICT (id) DO NOTHING`, nonWinnerID, uuid.New().String()+"@test.com")

	event := &AuctionEvent{
		ID:                   uuid.New(),
		Title:                "Winner Flow Test Auction",
		Status:               AuctionStatusLive,
		StartsAt:             time.Now().Add(-1 * time.Hour),
		EndsAt:               time.Now().Add(1 * time.Hour),
		BidStepCents:         500,
		PaymentDeadlineHours: 24,
		BiddingEnabled:       true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
	_ = repo.CreateEvent(ctx, event)

	amountCents := int64(1500)
	deadline := time.Now().Add(24 * time.Hour)
	expiredDeadline := time.Now().Add(-24 * time.Hour)

	// Lot 1: Valid
	lot1 := &AuctionLot{
		ID:                  uuid.New(),
		AuctionID:           event.ID,
		Title:               "Valid Lot",
		StartPriceCents:     1000,
		CurrentBidCents:     &amountCents,
		BidStepCents:        500,
		CurrentWinnerUserID: &winnerID,
		Status:              LotStatusWonPendingPayment,
		PaymentDeadlineAt:   &deadline,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = repo.CreateLot(ctx, lot1)

	// Lot 2: Expired deadline
	lot2 := &AuctionLot{
		ID:                  uuid.New(),
		AuctionID:           event.ID,
		Title:               "Expired Lot",
		StartPriceCents:     1000,
		CurrentBidCents:     &amountCents,
		BidStepCents:        500,
		CurrentWinnerUserID: &winnerID,
		Status:              LotStatusWonPendingPayment,
		PaymentDeadlineAt:   &expiredDeadline,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = repo.CreateLot(ctx, lot2)

	// Lot 3: Wrong Status
	lot3 := &AuctionLot{
		ID:                  uuid.New(),
		AuctionID:           event.ID,
		Title:               "Wrong Status Lot",
		StartPriceCents:     1000,
		CurrentBidCents:     &amountCents,
		BidStepCents:        500,
		CurrentWinnerUserID: &winnerID,
		Status:              LotStatusActive,
		PaymentDeadlineAt:   &deadline,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = repo.CreateLot(ctx, lot3)

	// Test 1: Non-winner rejected
	_, err = svc.CreateOrderForLot(ctx, lot1.ID, nonWinnerID)
	if err == nil || err.Error() != "not the winner of this lot" {
		t.Fatalf("Expected non-winner error, got: %v", err)
	}

	// Test 2: Expired deadline rejected
	_, err = svc.CreateOrderForLot(ctx, lot2.ID, winnerID)
	if err == nil || err.Error() != "payment deadline expired" {
		t.Fatalf("Expected deadline expired error, got: %v", err)
	}

	// Test 3: Wrong status rejected
	_, err = svc.CreateOrderForLot(ctx, lot3.ID, winnerID)
	if err == nil || err.Error() != "lot is not pending payment" {
		t.Fatalf("Expected lot not pending payment error, got: %v", err)
	}

	// Test 4: Winner can create order
	res, err := svc.CreateOrderForLot(ctx, lot1.ID, winnerID)
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if res.OrderID == uuid.Nil {
		t.Fatal("Expected valid OrderID")
	}

	// Test 5: Existing order returned idempotently
	res2, err := svc.CreateOrderForLot(ctx, lot1.ID, winnerID)
	if err != nil {
		t.Fatalf("Expected idempotent success, got: %v", err)
	}
	if res.OrderID != res2.OrderID {
		t.Fatalf("Expected same order id %v, got %v", res.OrderID, res2.OrderID)
	}
}
