package dashboard_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/admin/dashboard"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

func TestDashboardRepository_GetSummary(t *testing.T) {
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

	// Clean up related tables before testing
	_, err = pgClient.Pool.Exec(ctx, `TRUNCATE TABLE payouts, auction_lots, inventory_items, products, sellers, orders, users CASCADE`)
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	repo := dashboard.NewRepository(pgClient.Pool)

	// Case 1: Empty database
	summary, err := repo.GetSummary(ctx)
	if err != nil {
		t.Fatalf("GetSummary failed on empty db: %v", err)
	}
	if summary.Overview.RevenueTodayCents != 0 {
		t.Errorf("Expected 0 revenue, got %d", summary.Overview.RevenueTodayCents)
	}

	// Create user
	userID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, status, created_at, updated_at) 
		VALUES ($1, 'test_dashboard@zamk.local', 'hash', 'customer', 'active', NOW(), NOW())
	`, userID)
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	// Insert test data
	// Case 2 & 3: Multiple orders and large value > int32 max (2.1 billion)
	var largeValue int64 = 5000000000 // 5 billion cents
	var smallValue int64 = 100000
	
	// Order 1
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) 
		VALUES ($1, $2, 'paid', $3, 'Test', '123', 'test@test.com', 'Address', CURRENT_DATE, NOW())
	`, uuid.New(), userID, largeValue)
	if err != nil {
		t.Fatalf("Failed to insert order: %v", err)
	}

	// Order 2
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) 
		VALUES ($1, $2, 'paid', $3, 'Test2', '456', 'test2@test.com', 'Address2', CURRENT_DATE, NOW())
	`, uuid.New(), userID, smallValue)
	if err != nil {
		t.Fatalf("Failed to insert order: %v", err)
	}

	expectedTotal := largeValue + smallValue
	expectedAOV := expectedTotal / 2

	// Case 5: Re-run query to check Scan doesn't panic on numeric
	summary, err = repo.GetSummary(ctx)
	if err != nil {
		t.Fatalf("GetSummary failed after inserts: %v", err)
	}

	// Check Revenue
	if summary.Overview.RevenueTodayCents != expectedTotal {
		t.Errorf("Expected revenue %d, got %d", expectedTotal, summary.Overview.RevenueTodayCents)
	}

	// Case 4: Average check is correctly transformed
	if summary.Overview.AverageOrderValue7dCents != expectedAOV {
		t.Errorf("Expected AOV %d, got %d", expectedAOV, summary.Overview.AverageOrderValue7dCents)
	}
	
	// Case 6: Types check - if compilation and tests pass, the fields are int64.
}
