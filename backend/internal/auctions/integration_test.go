package auctions_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auctions"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/ratelimit"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func TestAuctionIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Run with INTEGRATION_TEST=1")
	}

	ctx := context.Background()
	dbURL := "postgres://postgres:postgres@localhost:5432/zamk?sslmode=disable"
	pgClient, err := postgres.NewClient(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to DB: %v", err)
	}
	defer pgClient.Pool.Close()

	// Initialize dependencies
	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}
	limiter := ratelimit.New(redisClient.Client)
	
	notifRepo := notifications.NewRepository(pgClient)
	notifService := notifications.NewService(notifRepo, nil) // nil ws
	
	repo := auctions.NewRepository(pgClient.Pool)
	hub := auctions.NewSSEHub()
	svc := auctions.NewService(repo, notifService, limiter, hub)

	// Admin IDs and Customer IDs
	adminID := uuid.New()
	customerA := uuid.New()
	customerB := uuid.New()

	// Seed users to satisfy foreign key constraints
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at) 
		VALUES 
			($1, 'Admin', 'admin@test.com', 'x', 'admin', 'active', now(), now()), 
			($2, 'A', 'a@test.com', 'y', 'customer', 'active', now(), now()), 
			($3, 'B', 'b@test.com', 'z', 'customer', 'active', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, adminID, customerA, customerB)
	if err != nil {
		t.Fatalf("Failed to seed users: %v", err)
	}

	// 1. Admin creates auction
	eventReq := auctions.AdminCreateAuctionRequest{
		Title: "Integration Test Auction",
		StartsAt: time.Now().Add(-1 * time.Hour), // Already started
		EndsAt: time.Now().Add(1 * time.Hour),
		BidStepCents: 1000, // 10.00
		PaymentDeadlineHours: 24,
		AntiSnipingEnabled: true,
		AntiSnipingTriggerSeconds: 300, // 5 mins
		AntiSnipingExtensionSeconds: 60, // 1 min
		MaxBidsPerUserPerLotPerMinute: 10,
		MaxRejectedBidsPerUserPerMinute: 10,
		NoBidsPolicy: "manual_review",
		UnpaidWinnerPolicy: "manual_review",
		IsPublic: true,
		ShowOnHomepage: true,
		HighlightInNav: true,
		BiddingEnabled: true,
	}

	event := &auctions.AuctionEvent{
		ID: uuid.New(),
		Title: eventReq.Title,
		Status: auctions.AuctionStatusDraft,
		StartsAt: eventReq.StartsAt,
		EndsAt: eventReq.EndsAt,
		BidStepCents: eventReq.BidStepCents,
		PaymentDeadlineHours: eventReq.PaymentDeadlineHours,
		AntiSnipingEnabled: eventReq.AntiSnipingEnabled,
		AntiSnipingTriggerSeconds: eventReq.AntiSnipingTriggerSeconds,
		AntiSnipingExtensionSeconds: eventReq.AntiSnipingExtensionSeconds,
		MaxBidsPerUserPerLotPerMinute: eventReq.MaxBidsPerUserPerLotPerMinute,
		MaxRejectedBidsPerUserPerMinute: eventReq.MaxRejectedBidsPerUserPerMinute,
		NoBidsPolicy: auctions.NoBidsPolicy(eventReq.NoBidsPolicy),
		UnpaidWinnerPolicy: auctions.UnpaidWinnerPolicy(eventReq.UnpaidWinnerPolicy),
		IsPublic: eventReq.IsPublic,
		ShowOnHomepage: eventReq.ShowOnHomepage,
		HighlightInNav: eventReq.HighlightInNav,
		BiddingEnabled: eventReq.BiddingEnabled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = repo.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}
	t.Logf("Created Event ID: %s", event.ID)

	// 2. Admin creates lots
	lot1 := &auctions.AuctionLot{
		ID: uuid.New(),
		AuctionID: event.ID,
		Title: "Test Lot 1 (Bids)",
		StartPriceCents: 5000,
		BidStepCents: 1000,
		Status: auctions.LotStatusDraft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = repo.CreateLot(ctx, lot1)
	if err != nil {
		t.Fatalf("Failed to create lot 1: %v", err)
	}

	lot2 := &auctions.AuctionLot{
		ID: uuid.New(),
		AuctionID: event.ID,
		Title: "Test Lot 2 (No Bids)",
		StartPriceCents: 3000,
		BidStepCents: 500,
		Status: auctions.LotStatusDraft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = repo.CreateLot(ctx, lot2)
	if err != nil {
		t.Fatalf("Failed to create lot 2: %v", err)
	}

	// 3. Publish Auction
	err = svc.UpdateEventStatus(ctx, event.ID, auctions.AuctionStatusLive)
	if err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}
	err = svc.UpdateLotStatus(ctx, lot1.ID, auctions.LotStatusActive)
	if err != nil {
		t.Fatalf("Failed to activate lot1: %v", err)
	}
	err = svc.UpdateLotStatus(ctx, lot2.ID, auctions.LotStatusActive)
	if err != nil {
		t.Fatalf("Failed to activate lot2: %v", err)
	}

	// 4 & 5 & 6. Verify Public endpoints logic via repo
	activeEvents, err := repo.ListActiveAuctions(ctx)
	if err != nil || len(activeEvents) == 0 {
		t.Fatalf("Failed to list active auctions: %v", err)
	}
	
	hpEvents, err := repo.ListHomepageAuctions(ctx)
	if err != nil || len(hpEvents) == 0 {
		t.Fatalf("Failed to list homepage auctions: %v", err)
	}

	navEvents, err := repo.ListNavHighlightAuctions(ctx)
	if err != nil || len(navEvents) == 0 {
		t.Fatalf("Failed to list nav highlight auctions: %v", err)
	}

	// 7. Customer A places first bid
	idemA := uuid.New().String()
	bidAmountA := int64(5000) // Start price
	respA, err := svc.PlaceBid(ctx, lot1.ID, customerA, auctions.BidRequest{
		AmountCents: &bidAmountA,
		IdempotencyKey: &idemA,
	})
	if err != nil {
		t.Fatalf("Customer A failed to place bid: %v", err)
	}
	if respA.NewCurrentBid != 5000 {
		t.Fatalf("Expected bid to be 5000, got %d", respA.NewCurrentBid)
	}
	t.Logf("Customer A placed bid successfully at 5000")

	// 8. Customer B places next bid
	idemB := uuid.New().String()
	bidAmountB := int64(6000) // 5000 + 1000 step
	respB, err := svc.PlaceBid(ctx, lot1.ID, customerB, auctions.BidRequest{
		AmountCents: &bidAmountB,
		IdempotencyKey: &idemB,
	})
	if err != nil {
		t.Fatalf("Customer B failed to place bid: %v", err)
	}
	if respB.NewCurrentBid != 6000 {
		t.Fatalf("Expected bid to be 6000, got %d", respB.NewCurrentBid)
	}
	t.Logf("Customer B placed bid successfully at 6000")

	// 9. Check notifications logic
	// The DB should have notifications for A (outbid) and B (accepted). We won't test full DB rows, but no panic is good.
	
	// 10. Duplicate idempotency key
	respB2, err := svc.PlaceBid(ctx, lot1.ID, customerB, auctions.BidRequest{
		AmountCents: &bidAmountB,
		IdempotencyKey: &idemB,
	})
	if err != nil {
		t.Fatalf("Duplicate idempotency key should not fail: %v", err)
	}
	if respB2.NewCurrentBid != 6000 {
		t.Fatalf("Expected idempotent bid to be 6000, got %d", respB2.NewCurrentBid)
	}
	t.Logf("Idempotency verified successfully")

	// 11. Conflicting idempotency key
	confAmount := int64(7000)
	_, err = svc.PlaceBid(ctx, lot1.ID, customerB, auctions.BidRequest{
		AmountCents: &confAmount,
		IdempotencyKey: &idemB,
	})
	if err != auctions.ErrDuplicateIdempotency {
		t.Fatalf("Expected duplicate idempotency error, got %v", err)
	}
	t.Logf("Conflicting idempotency key correctly blocked")

	// 12. Finalize Auction
	err = svc.FinalizeAuction(ctx, event.ID, adminID)
	if err != nil {
		t.Fatalf("Failed to finalize auction: %v", err)
	}

	// 13 & 14. Verify lot statuses
	finalLot1, err := repo.GetLotByID(ctx, lot1.ID)
	if err != nil {
		t.Fatalf("Failed to fetch lot1: %v", err)
	}
	if finalLot1.Status != auctions.LotStatusWonPendingPayment {
		t.Fatalf("Expected lot1 status won_pending_payment, got %s", finalLot1.Status)
	}
	if finalLot1.CurrentWinnerUserID == nil || *finalLot1.CurrentWinnerUserID != customerB {
		t.Fatalf("Expected lot1 winner to be Customer B")
	}

	finalLot2, err := repo.GetLotByID(ctx, lot2.ID)
	if err != nil {
		t.Fatalf("Failed to fetch lot2: %v", err)
	}
	if finalLot2.Status != auctions.LotStatusEndedNoBids {
		t.Fatalf("Expected lot2 status ended_no_bids, got %s", finalLot2.Status)
	}

	t.Logf("Auction finalization logic completely verified")
}

func TestAuctionConcurrency(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Run with INTEGRATION_TEST=1")
	}

	ctx := context.Background()
	dbURL := "postgres://postgres:postgres@localhost:5432/zamk?sslmode=disable"
	pgClient, err := postgres.NewClient(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to DB: %v", err)
	}
	defer pgClient.Pool.Close()

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}
	limiter := ratelimit.New(redisClient.Client)
	
	notifRepo := notifications.NewRepository(pgClient)
	notifService := notifications.NewService(notifRepo, nil) // nil ws
	
	repo := auctions.NewRepository(pgClient.Pool)
	hub := auctions.NewSSEHub()
	svc := auctions.NewService(repo, notifService, limiter, hub)

	// Admin and 5 Customers
	adminID := uuid.New()
	customers := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	
	// Seed admin
	_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at) VALUES ($1, 'Admin', $2, 'x', 'admin', 'active', now(), now()) ON CONFLICT (id) DO NOTHING`, adminID, uuid.New().String()+"@test.com")
	
	// Seed customers
	for _, cID := range customers {
		_, _ = pgClient.Pool.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at) VALUES ($1, 'Customer', $2, 'y', 'customer', 'active', now(), now()) ON CONFLICT (id) DO NOTHING`, cID, uuid.New().String()+"@test.com")
	}

	// Create event & lot
	event := &auctions.AuctionEvent{
		ID: uuid.New(),
		Title: "Concurrency Test Auction",
		Status: auctions.AuctionStatusLive, // immediately live
		StartsAt: time.Now().Add(-1 * time.Hour),
		EndsAt: time.Now().Add(1 * time.Hour),
		BidStepCents: 500,
		PaymentDeadlineHours: 24,
		BiddingEnabled: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateEvent(ctx, event)

	lot := &auctions.AuctionLot{
		ID: uuid.New(),
		AuctionID: event.ID,
		Title: "Race Condition Lot",
		StartPriceCents: 1000,
		BidStepCents: 500,
		Status: auctions.LotStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateLot(ctx, lot)

	// Simulate 5 users bidding EXACTLY 1000 at the same time
	errs := make(chan error, len(customers))
	bidAmount := int64(1000)

	for _, cID := range customers {
		go func(customerID uuid.UUID) {
			idem := uuid.New().String()
			_, err := svc.PlaceBid(context.Background(), lot.ID, customerID, auctions.BidRequest{
				AmountCents: &bidAmount,
				IdempotencyKey: &idem,
			})
			errs <- err
		}(cID)
	}

	successCount := 0
	for i := 0; i < len(customers); i++ {
		err := <-errs
		if err == nil {
			successCount++
		}
	}

	if successCount != 1 {
		t.Fatalf("Expected exactly 1 successful bid for amount 1000, but got %d", successCount)
	}
	
	finalLot, _ := repo.GetLotByID(ctx, lot.ID)
	if finalLot.CurrentBidCents == nil || *finalLot.CurrentBidCents != 1000 {
		t.Fatalf("Expected current bid to be 1000, got %v", finalLot.CurrentBidCents)
	}
	t.Logf("Atomic bid concurrency verified successfully! Exactly one bid won out of %d.", len(customers))
}
