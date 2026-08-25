package sellers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, *postgres.Client) {
	dsn := testutil.GetTestDatabaseURL()

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}

	testutil.AssertTestDatabase(t, db)
	
	return db, &postgres.Client{Pool: db}
}

func cleanTestDB(t *testing.T, db *pgxpool.Pool) {
	testutil.AssertTestDatabase(t, db)

	ctx := context.Background()
	db.Exec(ctx, "DELETE FROM seller_brands")
	db.Exec(ctx, "DELETE FROM seller_onboarding_applications")
	db.Exec(ctx, "DELETE FROM brands")
	db.Exec(ctx, "DELETE FROM sellers")
	db.Exec(ctx, "DELETE FROM users")
}

func TestInviteSellerCreatesAllRecords(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	cleanTestDB(t, db)

	repo := NewRepository(db)
	
	ctx := context.Background()
	sellerID := uuid.New()
	
	_, err := db.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'test', 'test', 'test@ex.com', 'pending')", sellerID)
	if err != nil {
		t.Fatalf("insert seller: %v", err)
	}

	appID := uuid.New()
	app := &SellerOnboardingApplication{
		ID:       appID,
		SellerID: sellerID,
		Status:   OnboardingStatusNotStarted,
		CurrentStep: 1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = repo.CreateOnboardingApplication(ctx, app)
	if err != nil {
		t.Fatalf("CreateOnboardingApplication failed: %v", err)
	}

	saved, err := repo.GetOnboardingApplicationBySellerID(ctx, sellerID)
	if err != nil {
		t.Fatalf("GetOnboardingApplicationBySellerID failed: %v", err)
	}
	if saved.ID != appID {
		t.Fatalf("expected ID %v, got %v", appID, saved.ID)
	}
}

func TestOnlyOnePrimaryBrandPerSeller(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	cleanTestDB(t, db)
	ctx := context.Background()

	sellerID := uuid.New()
	db.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 's1', 's1', 's1@ex.com', 'active')", sellerID)
	
	b1 := uuid.New()
	b2 := uuid.New()
	db.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, 'b1', 'b1')", b1)
	db.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, 'b2', 'b2')", b2)

	_, err := db.Exec(ctx, "INSERT INTO seller_brands (id, seller_id, brand_id, relationship_type, is_primary, status) VALUES ($1, $2, $3, 'owner', true, 'active')", uuid.New(), sellerID, b1)
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = db.Exec(ctx, "INSERT INTO seller_brands (id, seller_id, brand_id, relationship_type, is_primary, status) VALUES ($1, $2, $3, 'owner', true, 'active')", uuid.New(), sellerID, b2)
	if err == nil {
		t.Fatalf("second primary insert should have failed!")
	}
}

func TestApproveBrandSlugConflictRollsBack(t *testing.T) {
	db, pgClient := setupTestDB(t)
	defer db.Close()
	cleanTestDB(t, db)
	ctx := context.Background()
	repo := NewRepository(db)

	sellerID := uuid.New()
	db.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 's1', 's1', 's1@ex.com', 'pending')", sellerID)
	
	conflictBrandID := uuid.New()
	db.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, 'Conflict', 'conflict-slug')", conflictBrandID)

	appID := uuid.New()
	app := &SellerOnboardingApplication{
		ID:       appID,
		SellerID: sellerID,
		Status:   OnboardingStatusPendingReview,
		Payload:  OnboardingPayload{
			Store: StorePayload{Slug: "valid-store", Name: "Valid"},
			Brand: BrandPayload{Slug: "conflict-slug", Name: "Conflict"},
		},
		CurrentStep: 4,
	}
	repo.CreateOnboardingApplication(ctx, app)

	err := pgClient.RunInTx(ctx, func(tx pgx.Tx) error {
		lockedApp, err := repo.WithTx(tx).GetOnboardingApplicationBySellerIDForUpdate(ctx, sellerID)
		if err != nil { return err }

		_, err = tx.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, $2, $3)", uuid.New(), lockedApp.Payload.Brand.Name, lockedApp.Payload.Brand.Slug)
		return err
	})

	if err == nil {
		t.Fatalf("Expected conflict error")
	}

	var status string
	db.QueryRow(ctx, "SELECT status FROM seller_onboarding_applications WHERE id = $1", appID).Scan(&status)
	if status != string(OnboardingStatusPendingReview) {
		t.Fatalf("Status should not change, got: %v", status)
	}
}

func TestUpdateOnboardingStepPersistsPayload(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	cleanTestDB(t, db)
	ctx := context.Background()
	repo := NewRepository(db)

	sellerID := uuid.New()
	db.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 's1', 's1', 's1@ex.com', 'pending')", sellerID)
	app := &SellerOnboardingApplication{ID: uuid.New(), SellerID: sellerID, Status: OnboardingStatusNotStarted, CurrentStep: 1}
	repo.CreateOnboardingApplication(ctx, app)

	app.Payload.Owner = OwnerPayload{FirstName: "A", LastName: "B"}
	app.Status = OnboardingStatusDraft
	app.CurrentStep = 2
	repo.UpdateOnboardingApplication(ctx, app)

	saved, _ := repo.GetOnboardingApplicationBySellerID(ctx, sellerID)
	if saved.Payload.Owner.FirstName != "A" {
		t.Fatalf("payload not saved")
	}
}
