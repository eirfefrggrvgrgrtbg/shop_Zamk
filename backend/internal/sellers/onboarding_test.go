package sellers

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, *postgres.Client) {
	dsn := testutil.GetTestDatabaseURL()

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "failed to connect to db")

	testutil.AssertTestDatabase(t, db)
	return db, &postgres.Client{Pool: db}
}

func setupTestService(t *testing.T, db *pgxpool.Pool, client *postgres.Client) (*Service, *Repository, *users.Repository) {
	repo := NewRepository(db)
	userRepo := users.NewRepository(db)
	notifRepo := notifications.NewRepository(client)
	notifSvc := notifications.NewService(notifRepo, userRepo, nil)
	svc := NewService(repo, userRepo, client, notifSvc)
	return svc, repo, userRepo
}

func cleanupTestEntities(ctx context.Context, db *pgxpool.Pool, sellerIDs []uuid.UUID, brandIDs []uuid.UUID, userIDs []uuid.UUID) {
	for _, sID := range sellerIDs {
		_, _ = db.Exec(ctx, "DELETE FROM seller_brands WHERE seller_id = $1", sID)
		_, _ = db.Exec(ctx, "DELETE FROM seller_onboarding_applications WHERE seller_id = $1", sID)
		_, _ = db.Exec(ctx, "DELETE FROM seller_users WHERE seller_id = $1", sID)
		_, _ = db.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sID)
	}
	for _, bID := range brandIDs {
		_, _ = db.Exec(ctx, "DELETE FROM seller_brands WHERE brand_id = $1", bID)
		_, _ = db.Exec(ctx, "DELETE FROM brands WHERE id = $1", bID)
	}
	for _, uID := range userIDs {
		_, _ = db.Exec(ctx, "DELETE FROM users WHERE id = $1", uID)
	}
}

func TestInviteSellerCreatesAllRecords(t *testing.T) {
	db, client := setupTestDB(t)
	defer db.Close()
	svc, repo, _ := setupTestService(t, db, client)

	ctx := context.Background()
	suffix := uuid.New().String()[:8]
	email := fmt.Sprintf("seller-%s@example.com", suffix)

	req := &InviteSellerRequest{
		FirstName:         "Ivan",
		LastName:          "Petrov",
		MiddleName:        "Sergeevich",
		Email:             email,
		TemporaryPassword: "TempPassword123!",
		Phone:             "+79991234567",
	}

	res, err := svc.InviteSeller(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.NotEqual(t, uuid.Nil, res.SellerID)
	assert.NotEqual(t, uuid.Nil, res.OwnerUserID)
	assert.True(t, res.TemporaryPasswordReturned)

	t.Cleanup(func() {
		cleanupTestEntities(ctx, db, []uuid.UUID{res.SellerID}, nil, []uuid.UUID{res.OwnerUserID})
	})

	// Verify User record
	var userRole, userStatus string
	var mustChangePassword bool
	err = db.QueryRow(ctx, "SELECT role, status, must_change_password FROM users WHERE id = $1", res.OwnerUserID).
		Scan(&userRole, &userStatus, &mustChangePassword)
	require.NoError(t, err)
	assert.Equal(t, string(users.RoleSeller), userRole)
	assert.Equal(t, string(users.StatusActive), userStatus)
	assert.True(t, mustChangePassword)

	// Verify Seller record
	seller, err := repo.GetSellerByID(ctx, res.SellerID)
	require.NoError(t, err)
	require.NotNil(t, seller)
	assert.Equal(t, StatusPendingSetup, seller.Status)
	assert.NotNil(t, seller.BrandName)
	assert.Equal(t, "Draft Store", *seller.BrandName)
	assert.NotNil(t, seller.ContactEmail)
	assert.Equal(t, email, *seller.ContactEmail)

	// Verify SellerUser link
	var sellerUserRole string
	err = db.QueryRow(ctx, "SELECT role FROM seller_users WHERE seller_id = $1 AND user_id = $2", res.SellerID, res.OwnerUserID).
		Scan(&sellerUserRole)
	require.NoError(t, err)
	assert.Equal(t, string(RoleOwner), sellerUserRole)

	// Verify Onboarding Application
	app, err := repo.GetOnboardingApplicationBySellerID(ctx, res.SellerID)
	require.NoError(t, err)
	require.NotNil(t, app)
	assert.Equal(t, OnboardingStatusNotStarted, app.Status)
	assert.Equal(t, 1, app.CurrentStep)
	assert.Equal(t, "Ivan", app.Payload.Owner.FirstName)
	assert.Equal(t, "Petrov", app.Payload.Owner.LastName)
	assert.Equal(t, "Sergeevich", app.Payload.Owner.MiddleName)
	assert.Equal(t, "+79991234567", app.Payload.Owner.Phone)
}

func TestOnlyOnePrimaryBrandPerSeller(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	sellerID := uuid.New()
	suffix := sellerID.String()[:8]
	sellerSlug := fmt.Sprintf("seller-%s", suffix)

	_, err := db.Exec(ctx,
		"INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, $2, $3, $4, 'active')",
		sellerID, "Seller Primary Test", sellerSlug, fmt.Sprintf("%s@ex.com", sellerSlug),
	)
	require.NoError(t, err, "insert seller should succeed")

	b1 := uuid.New()
	b2 := uuid.New()
	b1Slug := fmt.Sprintf("brand-b1-%s", suffix)
	b2Slug := fmt.Sprintf("brand-b2-%s", suffix)

	_, err = db.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, $2, $3)", b1, "Brand 1", b1Slug)
	require.NoError(t, err, "insert brand 1 should succeed")
	_, err = db.Exec(ctx, "INSERT INTO brands (id, name, slug) VALUES ($1, $2, $3)", b2, "Brand 2", b2Slug)
	require.NoError(t, err, "insert brand 2 should succeed")

	t.Cleanup(func() {
		cleanupTestEntities(ctx, db, []uuid.UUID{sellerID}, []uuid.UUID{b1, b2}, nil)
	})

	// First primary brand insert succeeds
	_, err = db.Exec(ctx,
		"INSERT INTO seller_brands (id, seller_id, brand_id, relationship_type, is_primary, status) VALUES ($1, $2, $3, 'owner', true, 'active')",
		uuid.New(), sellerID, b1,
	)
	require.NoError(t, err, "first primary brand insert must succeed")

	// Second primary brand insert for the same seller MUST fail due to seller_brands_primary_brand_idx unique constraint
	_, err = db.Exec(ctx,
		"INSERT INTO seller_brands (id, seller_id, brand_id, relationship_type, is_primary, status) VALUES ($1, $2, $3, 'owner', true, 'active')",
		uuid.New(), sellerID, b2,
	)
	require.Error(t, err, "second primary brand insert for the same seller must fail")

	// Non-primary brand insert for the same seller succeeds
	_, err = db.Exec(ctx,
		"INSERT INTO seller_brands (id, seller_id, brand_id, relationship_type, is_primary, status) VALUES ($1, $2, $3, 'owner', false, 'active')",
		uuid.New(), sellerID, b2,
	)
	require.NoError(t, err, "non-primary brand insert for the same seller must succeed")
}

func TestApproveBrandSlugConflictRollsBack(t *testing.T) {
	db, client := setupTestDB(t)
	defer db.Close()
	svc, repo, _ := setupTestService(t, db, client)
	ctx := context.Background()

	sellerID := uuid.New()
	suffix := sellerID.String()[:8]
	initialSellerSlug := fmt.Sprintf("pending-store-%s", suffix)
	initialBrandName := "Initial Store"

	_, err := db.Exec(ctx,
		"INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, $2, $3, $4, 'pending_setup')",
		sellerID, initialBrandName, initialSellerSlug, fmt.Sprintf("%s@ex.com", initialSellerSlug),
	)
	require.NoError(t, err)

	conflictBrandID := uuid.New()
	conflictBrandSlug := fmt.Sprintf("conflict-brand-%s", suffix)
	_, err = db.Exec(ctx,
		"INSERT INTO brands (id, name, slug) VALUES ($1, 'Existing Brand', $2)",
		conflictBrandID, conflictBrandSlug,
	)
	require.NoError(t, err)

	adminID := uuid.New()
	_, err = db.Exec(ctx,
		"INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'admin', 'Admin User')",
		adminID, fmt.Sprintf("admin-%s@ex.com", suffix),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupTestEntities(ctx, db, []uuid.UUID{sellerID}, []uuid.UUID{conflictBrandID}, []uuid.UUID{adminID})
	})

	appID := uuid.New()
	newStoreSlug := fmt.Sprintf("new-store-%s", suffix)
	app := &SellerOnboardingApplication{
		ID:          appID,
		SellerID:    sellerID,
		Status:      OnboardingStatusPendingReview,
		CurrentStep: 4,
		Payload: OnboardingPayload{
			Store: StorePayload{Slug: newStoreSlug, Name: "New Store Name", Description: "New Store Desc"},
			Brand: BrandPayload{Slug: conflictBrandSlug, Name: "Conflict Brand Name", Description: "Brand Desc"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = repo.CreateOnboardingApplication(ctx, app)
	require.NoError(t, err)

	// Attempt approval: brand slug is already taken
	err = svc.ApproveOnboarding(ctx, adminID, appID, &ApproveOnboardingRequest{ReviewComment: "Approved"})
	require.Error(t, err, "approval must fail on conflicting brand slug")
	assert.True(t, errors.Is(err, ErrBrandSlugTaken), "expected ErrBrandSlugTaken, got: %v", err)

	// Verify TRANSACTION ROLLBACK:
	// 1. Application status remains pending_review (NOT approved)
	savedApp, err := repo.GetOnboardingApplicationByID(ctx, appID)
	require.NoError(t, err)
	assert.Equal(t, OnboardingStatusPendingReview, savedApp.Status, "application status must not change on rollback")
	assert.Nil(t, savedApp.ReviewedAt, "reviewed_at must remain nil on rollback")

	// 2. Seller remains in initial status and slug (NOT active, NOT new store slug)
	seller, err := repo.GetSellerByID(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, StatusPendingSetup, seller.Status, "seller status must not transition to active on rollback")
	assert.Equal(t, initialSellerSlug, *seller.Slug, "seller slug must remain unchanged on rollback")
	assert.Equal(t, initialBrandName, *seller.BrandName, "seller brand_name must remain unchanged on rollback")

	// 3. No new brand or seller_brand link created
	var sellerBrandCount int
	err = db.QueryRow(ctx, "SELECT count(*) FROM seller_brands WHERE seller_id = $1", sellerID).Scan(&sellerBrandCount)
	require.NoError(t, err)
	assert.Equal(t, 0, sellerBrandCount, "no seller_brands row should be committed on rollback")

	// Also verify Store slug conflict rollback
	existingStoreSellerID := uuid.New()
	existingStoreSlug := fmt.Sprintf("taken-store-%s", suffix)
	_, err = db.Exec(ctx,
		"INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Taken Store', $2, $3, 'active')",
		existingStoreSellerID, existingStoreSlug, fmt.Sprintf("%s@ex.com", existingStoreSlug),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupTestEntities(ctx, db, []uuid.UUID{existingStoreSellerID}, nil, nil)
	})

	app.Payload.Store.Slug = existingStoreSlug
	app.Payload.Brand.Slug = fmt.Sprintf("non-conflicting-brand-%s", suffix)
	err = repo.UpdateOnboardingApplication(ctx, app)
	require.NoError(t, err)

	err = svc.ApproveOnboarding(ctx, adminID, appID, &ApproveOnboardingRequest{})
	require.Error(t, err, "approval must fail on conflicting store slug")
	assert.True(t, errors.Is(err, ErrStoreSlugTaken), "expected ErrStoreSlugTaken, got: %v", err)

	// Seller remains unchanged
	sellerAfterStoreConflict, err := repo.GetSellerByID(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, StatusPendingSetup, sellerAfterStoreConflict.Status)
}

func TestUpdateOnboardingStepPersistsPayload(t *testing.T) {
	db, client := setupTestDB(t)
	defer db.Close()
	svc, repo, _ := setupTestService(t, db, client)
	ctx := context.Background()

	sellerID := uuid.New()
	suffix := sellerID.String()[:8]
	sellerSlug := fmt.Sprintf("seller-step-%s", suffix)

	_, err := db.Exec(ctx,
		"INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Step Test Seller', $2, $3, 'pending_setup')",
		sellerID, sellerSlug, fmt.Sprintf("%s@ex.com", sellerSlug),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupTestEntities(ctx, db, []uuid.UUID{sellerID}, nil, nil)
	})

	app := &SellerOnboardingApplication{
		ID:          uuid.New(),
		SellerID:    sellerID,
		Status:      OnboardingStatusNotStarted,
		CurrentStep: 1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = repo.CreateOnboardingApplication(ctx, app)
	require.NoError(t, err)

	// Step 2 update with full payload
	updateReq := &UpdateOnboardingStepRequest{
		CurrentStep: 2,
		Payload: OnboardingPayload{
			Owner: OwnerPayload{
				FirstName:  "Alexey",
				LastName:   "Smirnov",
				MiddleName: "Ivanovich",
				Phone:      "+79998887766",
				Contact:    "telegram:@asmirnov",
			},
			Store: StorePayload{
				Name:             "Alexey Store",
				Slug:             fmt.Sprintf("alexey-store-%s", suffix),
				ShortDescription: "Short store desc",
				Description:      "Full detailed description of the store",
				LogoURL:          "https://example.com/logo.png",
				CoverURL:         "https://example.com/cover.png",
				PublicEmail:      "store@example.com",
				PublicPhone:      "+79991112233",
				SocialLinks:      map[string]string{"telegram": "https://t.me/alexey"},
			},
			Brand: BrandPayload{
				Name:        "Alexey Brand",
				Slug:        fmt.Sprintf("alexey-brand-%s", suffix),
				Description: "Brand description",
				LogoURL:     "https://example.com/brand-logo.png",
				Country:     "RU",
			},
			Legal: LegalPayload{
				SellerType:              "individual",
				LegalName:               "IP Smirnov A.I.",
				TaxID:                   "770123456789",
				RegistrationNumber:      "321774600000000",
				PayoutDetailsConfigured: true,
			},
		},
	}

	updatedApp, err := svc.UpdateOnboardingStep(ctx, sellerID, updateReq)
	require.NoError(t, err)
	require.NotNil(t, updatedApp)
	assert.Equal(t, OnboardingStatusDraft, updatedApp.Status, "not_started status must transition to draft")
	assert.Equal(t, 2, updatedApp.CurrentStep)
	assert.Equal(t, "Alexey", updatedApp.Payload.Owner.FirstName)
	assert.Equal(t, "Alexey Store", updatedApp.Payload.Store.Name)
	assert.Equal(t, "770123456789", updatedApp.Payload.Legal.TaxID)

	// Verify persistence in DB
	saved, err := repo.GetOnboardingApplicationBySellerID(ctx, sellerID)
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, OnboardingStatusDraft, saved.Status)
	assert.Equal(t, 2, saved.CurrentStep)
	assert.Equal(t, "Alexey", saved.Payload.Owner.FirstName)
	assert.Equal(t, "Smirnov", saved.Payload.Owner.LastName)
	assert.Equal(t, "Ivanovich", saved.Payload.Owner.MiddleName)
	assert.Equal(t, "Alexey Store", saved.Payload.Store.Name)
	assert.Equal(t, "https://t.me/alexey", saved.Payload.Store.SocialLinks["telegram"])
	assert.Equal(t, "IP Smirnov A.I.", saved.Payload.Legal.LegalName)
	assert.True(t, saved.Payload.Legal.PayoutDetailsConfigured)

	// Nil / partial fields handling: optional fields omitted should not panic or fail
	partialReq := &UpdateOnboardingStepRequest{
		CurrentStep: 3,
		Payload: OnboardingPayload{
			Owner: OwnerPayload{
				FirstName: "Alexey",
				LastName:  "Smirnov",
				// MiddleName, Phone, Contact omitted
			},
			Store: StorePayload{
				Name: "Alexey Store",
				Slug: fmt.Sprintf("alexey-store-%s", suffix),
				// ShortDescription, Description, LogoURL, CoverURL, SocialLinks omitted
			},
		},
	}

	partialApp, err := svc.UpdateOnboardingStep(ctx, sellerID, partialReq)
	require.NoError(t, err)
	assert.Equal(t, 3, partialApp.CurrentStep)
	assert.Empty(t, partialApp.Payload.Owner.MiddleName)
	assert.Nil(t, partialApp.Payload.Store.SocialLinks)

	// Guard: cannot update approved onboarding application
	saved.Status = OnboardingStatusApproved
	err = repo.UpdateOnboardingApplication(ctx, saved)
	require.NoError(t, err)

	_, err = svc.UpdateOnboardingStep(ctx, sellerID, updateReq)
	require.Error(t, err, "updating approved application must fail")
}

func TestOnboardingReviewLifecycle(t *testing.T) {
	db, client := setupTestDB(t)
	defer db.Close()
	svc, repo, _ := setupTestService(t, db, client)
	ctx := context.Background()

	sellerID := uuid.New()
	suffix := sellerID.String()[:8]
	sellerSlug := fmt.Sprintf("lifecycle-seller-%s", suffix)

	_, err := db.Exec(ctx,
		"INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Lifecycle Seller', $2, $3, 'pending_setup')",
		sellerID, sellerSlug, fmt.Sprintf("%s@ex.com", sellerSlug),
	)
	require.NoError(t, err)

	adminID := uuid.New()
	_, err = db.Exec(ctx,
		"INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'admin', 'Admin Reviewer')",
		adminID, fmt.Sprintf("admin-rev-%s@ex.com", suffix),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupTestEntities(ctx, db, []uuid.UUID{sellerID}, nil, []uuid.UUID{adminID})
	})

	app := &SellerOnboardingApplication{
		ID:          uuid.New(),
		SellerID:    sellerID,
		Status:      OnboardingStatusDraft,
		CurrentStep: 4,
		Payload: OnboardingPayload{
			Store: StorePayload{Name: "Final Store", Slug: fmt.Sprintf("final-store-%s", suffix), Description: "Desc"},
			Brand: BrandPayload{Name: "Final Brand", Slug: fmt.Sprintf("final-brand-%s", suffix), Description: "Brand Desc"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = repo.CreateOnboardingApplication(ctx, app)
	require.NoError(t, err)

	// 1. SubmitOnboarding -> transitions draft to pending_review
	submittedApp, err := svc.SubmitOnboarding(ctx, sellerID, &SubmitOnboardingRequest{Payload: app.Payload})
	require.NoError(t, err)
	assert.Equal(t, OnboardingStatusPendingReview, submittedApp.Status)
	assert.NotNil(t, submittedApp.SubmittedAt)

	// 2. RequestChangesOnboarding -> transitions pending_review to changes_requested
	err = svc.RequestChangesOnboarding(ctx, adminID, app.ID, &RequestChangesOnboardingRequest{ReviewComment: "Please clarify legal address"})
	require.NoError(t, err)

	changesApp, err := repo.GetOnboardingApplicationByID(ctx, app.ID)
	require.NoError(t, err)
	assert.Equal(t, OnboardingStatusChangesRequested, changesApp.Status)
	assert.NotNil(t, changesApp.ReviewComment)
	assert.Equal(t, "Please clarify legal address", *changesApp.ReviewComment)

	// 3. Re-submit after changes requested
	submittedAgain, err := svc.SubmitOnboarding(ctx, sellerID, &SubmitOnboardingRequest{Payload: app.Payload})
	require.NoError(t, err)
	assert.Equal(t, OnboardingStatusPendingReview, submittedAgain.Status)

	// 4. ApproveOnboarding -> transitions to approved, activates seller, creates brand and seller_brand link
	err = svc.ApproveOnboarding(ctx, adminID, app.ID, &ApproveOnboardingRequest{ReviewComment: "Welcome aboard"})
	require.NoError(t, err)

	approvedApp, err := repo.GetOnboardingApplicationByID(ctx, app.ID)
	require.NoError(t, err)
	assert.Equal(t, OnboardingStatusApproved, approvedApp.Status)
	assert.Equal(t, "Welcome aboard", *approvedApp.ReviewComment)

	// Verify Seller became Active with store name and slug
	seller, err := repo.GetSellerByID(ctx, sellerID)
	require.NoError(t, err)
	assert.Equal(t, StatusActive, seller.Status)
	assert.Equal(t, "Final Store", *seller.BrandName)
	assert.Equal(t, fmt.Sprintf("final-store-%s", suffix), *seller.Slug)

	// Verify Brand created in brands table
	var brandID uuid.UUID
	err = db.QueryRow(ctx, "SELECT id FROM brands WHERE slug = $1", fmt.Sprintf("final-brand-%s", suffix)).Scan(&brandID)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, brandID)
	t.Cleanup(func() {
		cleanupTestEntities(ctx, db, nil, []uuid.UUID{brandID}, nil)
	})

	// Verify SellerBrand link
	var isPrimary bool
	var relType string
	err = db.QueryRow(ctx, "SELECT is_primary, relationship_type FROM seller_brands WHERE seller_id = $1 AND brand_id = $2", sellerID, brandID).
		Scan(&isPrimary, &relType)
	require.NoError(t, err)
	assert.True(t, isPrimary)
	assert.Equal(t, "owner", relType)

	// Idempotency: approving again returns nil without error
	err = svc.ApproveOnboarding(ctx, adminID, app.ID, &ApproveOnboardingRequest{})
	require.NoError(t, err, "approving already approved application should be idempotent")
}
