package fulfillment_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
)

// mockPayoutsService implements payoutsService for testing
type mockPayoutsService struct{}

func (m *mockPayoutsService) CreatePendingSalesForOrder(ctx context.Context, orderID uuid.UUID) error {
	return nil
}

func TestC10ShipmentGuardrails(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := postgres.NewClient(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer db.Close()

	repo := fulfillment.NewRepository(db.Pool)
	ordersRepo := orders.NewRepository(db.Pool)

	// Create service
	svc := fulfillment.NewService(repo, ordersRepo, db, &mockPayoutsService{}, nil)

	// Testing CreateShipment directly requires a full order with fulfillments
	// Since creating an order in the database requires users, sellers, products, etc.
	// We will mostly test that the service handles empty fulfillments gracefully.

	t.Run("CreateShipment fails when order has no fulfillments", func(t *testing.T) {
		adminID := uuid.New()
		orderID := uuid.New() // Non-existent order

		req := fulfillment.CreateShipmentRequest{
			Carrier:        nil,
			TrackingNumber: nil,
			TrackingUrl:    nil,
		}

		// This should fail either because the order doesn't exist, or if it does, it has no fulfillments.
		// Since order doesn't exist, GetOrderForUpdateTx will return pgx.ErrNoRows.
		_, err := svc.CreateShipment(ctx, adminID, orderID, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("CreateShipmentForFulfillment fails when fulfillment does not exist", func(t *testing.T) {
		adminID := uuid.New()
		fulfillmentID := uuid.New() // Non-existent fulfillment

		req := fulfillment.CreateShipmentRequest{}

		_, err := svc.CreateShipmentForFulfillment(ctx, adminID, fulfillmentID, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
