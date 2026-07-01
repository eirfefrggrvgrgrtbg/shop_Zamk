package auctions

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/ratelimit"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestMoveLotToDirectSale(t *testing.T) {
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

	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	limiter := ratelimit.New(redisClient)

	notifRepo := notifications.NewRepository(pgClient)
	notifService := notifications.NewService(notifRepo, nil)

	repo := NewRepository(pgClient.Pool)
	hub := NewSSEHub()
	svc := NewService(repo, notifService, limiter, hub)

	adminID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, first_name, last_name, name)
		VALUES ($1, $2, 'hash', 'admin', 'Admin', 'User', 'Admin User')
	`, adminID, uuid.New().String()+"@admin.local")
	if err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}

	event := &AuctionEvent{
		ID:                   uuid.New(),
		Title:                "Direct Sale Test Auction",
		Status:               AuctionStatusEnded,
		StartsAt:             time.Now().Add(-2 * time.Hour),
		EndsAt:               time.Now().Add(-1 * time.Hour),
		BidStepCents:         500,
		PaymentDeadlineHours: 24,
		BiddingEnabled:       true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
	_ = repo.CreateEvent(ctx, event)

	dsPrice := int64(1000)

	// Lot 1: valid ended_no_bids
	lot1 := &AuctionLot{
		ID:                    uuid.New(),
		AuctionID:             event.ID,
		Title:                 "No Bids Lot",
		Description:           nil,
		StartPriceCents:       1000,
		BidStepCents:          500,
		Status:                LotStatusEndedNoBids,
		CanMoveToDirectSale:   true,
		DirectSalePriceCents:  &dsPrice,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	_ = repo.CreateLot(ctx, lot1)

	// Lot 2: valid unpaid_manual_review
	lot2 := &AuctionLot{
		ID:                    uuid.New(),
		AuctionID:             event.ID,
		Title:                 "Unpaid Lot",
		StartPriceCents:       1000,
		BidStepCents:          500,
		Status:                LotStatusUnpaidManualReview,
		CanMoveToDirectSale:   true,
		DirectSalePriceCents:  &dsPrice,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	_ = repo.CreateLot(ctx, lot2)

	// Lot 3: active rejected
	lot3 := &AuctionLot{
		ID:                    uuid.New(),
		AuctionID:             event.ID,
		Title:                 "Active Lot",
		StartPriceCents:       1000,
		BidStepCents:          500,
		Status:                LotStatusActive,
		CanMoveToDirectSale:   true,
		DirectSalePriceCents:  &dsPrice,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	_ = repo.CreateLot(ctx, lot3)

	// Lot 4: missing price
	lot4 := &AuctionLot{
		ID:                    uuid.New(),
		AuctionID:             event.ID,
		Title:                 "No Price Lot",
		StartPriceCents:       1000,
		BidStepCents:          500,
		Status:                LotStatusEndedNoBids,
		CanMoveToDirectSale:   true,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	if err := repo.CreateLot(ctx, lot4); err != nil {
		t.Fatalf("Failed to create lot4: %v", err)
	}

	// Lot 5: can_move_to_direct_sale=false
	lot5 := &AuctionLot{
		ID:                    uuid.New(),
		AuctionID:             event.ID,
		Title:                 "Cannot Move Lot",
		StartPriceCents:       1000,
		BidStepCents:          500,
		Status:                LotStatusEndedNoBids,
		CanMoveToDirectSale:   false,
		DirectSalePriceCents:  &dsPrice,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	_ = repo.CreateLot(ctx, lot5)

	// Lot 6: concurrency check
	lot6 := &AuctionLot{
		ID:                    uuid.New(),
		AuctionID:             event.ID,
		Title:                 "Concurrent Lot",
		StartPriceCents:       1000,
		BidStepCents:          500,
		Status:                LotStatusEndedNoBids,
		CanMoveToDirectSale:   true,
		DirectSalePriceCents:  &dsPrice,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	_ = repo.CreateLot(ctx, lot6)

	// Tests:
	err = svc.MoveLotToDirectSale(ctx, lot3.ID, adminID)
	if err == nil || err.Error() != "Лот нельзя перевести в прямую продажу." {
		t.Fatalf("Expected lot cannot be moved error for active lot, got: %v", err)
	}

	err = svc.MoveLotToDirectSale(ctx, lot4.ID, adminID)
	if err == nil || err.Error() != "Укажите цену прямой продажи." {
		t.Fatalf("Expected price not set error, got: %v", err)
	}

	err = svc.MoveLotToDirectSale(ctx, lot5.ID, adminID)
	if err == nil || err.Error() != "Лот нельзя перевести в прямую продажу." {
		t.Fatalf("Expected lot cannot be moved error for flag=false, got: %v", err)
	}

	err = svc.MoveLotToDirectSale(ctx, lot1.ID, adminID)
	if err != nil {
		t.Fatalf("Expected success for lot1, got: %v", err)
	}

	err = svc.MoveLotToDirectSale(ctx, lot2.ID, adminID)
	if err != nil {
		t.Fatalf("Expected success for lot2, got: %v", err)
	}

	// Idempotency
	err = svc.MoveLotToDirectSale(ctx, lot1.ID, adminID)
	if err == nil || err.Error() != "Лот уже переведён в прямую продажу." {
		t.Fatalf("Expected already moved error on duplicate move, got: %v", err)
	}

	// Concurrent
	var wg sync.WaitGroup
	errCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- svc.MoveLotToDirectSale(ctx, lot6.ID, adminID)
		}()
	}
	wg.Wait()
	close(errCh)

	successCount := 0
	alreadyMovedCount := 0
	for err := range errCh {
		if err == nil {
			successCount++
		} else if err.Error() == "Лот уже переведён в прямую продажу." || err.Error() == "Лот нельзя перевести в прямую продажу." {
			alreadyMovedCount++
		} else {
			t.Fatalf("Unexpected concurrent error: %v", err)
		}
	}
	if successCount != 1 {
		t.Fatalf("Expected exactly 1 success in concurrent moves, got %d", successCount)
	}

	// Verify product fields
	updatedLot, err := repo.GetLotByID(ctx, lot1.ID)
	if err != nil || updatedLot.Status != LotStatusMovedToDirectSale || updatedLot.DirectSaleProductID == nil {
		t.Fatalf("Lot 1 not properly updated in DB")
	}

	var source string
	var status string
	var price int64
	err = pgClient.Pool.QueryRow(ctx, "SELECT source, status, price_cents FROM products WHERE id = $1", updatedLot.DirectSaleProductID).Scan(&source, &status, &price)
	if err != nil {
		t.Fatalf("Failed to retrieve created product: %v", err)
	}

	if source != "auction_direct_sale" {
		t.Fatalf("Expected source 'auction_direct_sale', got %s", source)
	}
	if status != "published" {
		t.Fatalf("Expected status 'published', got %s", status)
	}
	if price != dsPrice {
		t.Fatalf("Expected price %d, got %d", dsPrice, price)
	}
}
