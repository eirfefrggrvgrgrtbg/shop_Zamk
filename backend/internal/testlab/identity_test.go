package testlab_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testlab"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
)

func TestIsolatedIdentity(t *testing.T) {
	ctx := context.Background()
	db, err := pgxpool.New(ctx, "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable")
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer db.Close()

	repo := testlab.NewRepository(db)
	
	runID := uuid.New().String()[:8]
	ownerID := uuid.New()
	sellerID := uuid.New()

	ownerUser := &users.User{
		ID:                 ownerID,
		Name:               fmt.Sprintf("TestLab Owner %s", runID),
		Email:              strings.ToLower(fmt.Sprintf("owner-testlab-%s@zamk.ru", runID)),
		Phone:              fmt.Sprintf("+7000%s", runID[:7]),
		PasswordHash:       "hash",
		Role:               users.RoleSeller,
		Status:             users.StatusActive,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	brandName := fmt.Sprintf("TESTLAB %s", runID)
	slug := fmt.Sprintf("testlab-%s", runID)
	desc := "Test Lab Isolated Seller"
	
	seller := &sellers.Seller{
		ID:           sellerID,
		BrandName:    &brandName,
		Slug:         &slug,
		Description:  &desc,
		ContactEmail: &ownerUser.Email,
		ContactPhone: &ownerUser.Phone,
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	sellerUser := &sellers.SellerUser{
		ID:        uuid.New(),
		SellerID:  sellerID,
		UserID:    ownerID,
		Role:      sellers.RoleOwner,
		CreatedAt: time.Now(),
	}

	// A. Create isolated Seller
	err = repo.CreateIsolatedIdentity(ctx, ownerUser, seller, sellerUser)
	if err != nil {
		t.Fatalf("failed to create identity: %v", err)
	}

	// Verify user exists
	var count int
	db.QueryRow(ctx, "SELECT count(*) FROM users WHERE id = $1", ownerID).Scan(&count)
	if count != 1 {
		t.Fatalf("user not found")
	}

	// Verify seller exists
	db.QueryRow(ctx, "SELECT count(*) FROM sellers WHERE id = $1", sellerID).Scan(&count)
	if count != 1 {
		t.Fatalf("seller not found")
	}

	// Verify seller_users exists
	db.QueryRow(ctx, "SELECT count(*) FROM seller_users WHERE user_id = $1 AND seller_id = $2", ownerID, sellerID).Scan(&count)
	if count != 1 {
		t.Fatalf("seller_users not found")
	}

	// B. returned sellerID resolves from linked user
	var resolvedSellerID uuid.UUID
	err = db.QueryRow(ctx, "SELECT seller_id FROM seller_users WHERE user_id = $1", ownerID).Scan(&resolvedSellerID)
	if err != nil || resolvedSellerID != sellerID {
		t.Fatalf("failed to resolve sellerID from userID")
	}

	// Create unrelated seller
	unrelatedRunID := uuid.New().String()[:8]
	unrelatedOwnerID := uuid.New()
	unrelatedSellerID := uuid.New()
	unrelatedOwnerUser := *ownerUser
	unrelatedOwnerUser.ID = unrelatedOwnerID
	unrelatedOwnerUser.Email = fmt.Sprintf("other-%s@zamk.ru", unrelatedRunID)
	unrelatedSeller := *seller
	unrelatedSeller.ID = unrelatedSellerID
	unrelatedBrand := fmt.Sprintf("TESTLAB %s", unrelatedRunID)
	unrelatedSlug := fmt.Sprintf("testlab-%s", unrelatedRunID)
	unrelatedSeller.BrandName = &unrelatedBrand
	unrelatedSeller.Slug = &unrelatedSlug
	unrelatedSellerUser := *sellerUser
	unrelatedSellerUser.ID = uuid.New()
	unrelatedSellerUser.UserID = unrelatedOwnerID
	unrelatedSellerUser.SellerID = unrelatedSellerID
	err = repo.CreateIsolatedIdentity(ctx, &unrelatedOwnerUser, &unrelatedSeller, &unrelatedSellerUser)
	if err != nil {
		t.Fatalf("failed to create unrelated identity: %v", err)
	}

	// C. cleanup removes only run-owned identity
	err = repo.CleanupRun(ctx, runID)
	if err != nil {
		t.Fatalf("failed to cleanup: %v", err)
	}

	db.QueryRow(ctx, "SELECT count(*) FROM sellers WHERE id = $1", sellerID).Scan(&count)
	if count != 0 {
		t.Fatalf("seller should be deleted")
	}
	db.QueryRow(ctx, "SELECT count(*) FROM seller_users WHERE user_id = $1", ownerID).Scan(&count)
	if count != 0 {
		t.Fatalf("seller_users should be deleted")
	}
	// Note: users table cleanup might be done separately or implicitly? Wait, CleanupRun does:
	// DELETE FROM seller_users ... DELETE FROM sellers ...
	// Wait, CleanupRun also deletes users: `DELETE FROM users WHERE id = $1 AND email LIKE '%testlab%'`
	db.QueryRow(ctx, "SELECT count(*) FROM users WHERE id = $1", ownerID).Scan(&count)
	if count != 0 {
		t.Fatalf("user should be deleted")
	}

	// D. unrelated Seller remains
	db.QueryRow(ctx, "SELECT count(*) FROM sellers WHERE id = $1", unrelatedSellerID).Scan(&count)
	if count != 1 {
		t.Fatalf("unrelated seller should remain")
	}
	db.QueryRow(ctx, "SELECT count(*) FROM users WHERE id = $1", unrelatedOwnerID).Scan(&count)
	if count != 1 {
		t.Fatalf("unrelated user should remain")
	}

	// E. creation failure rolls back partial identity graph
	// Try creating with duplicate email to force failure
	duplicateUser := *ownerUser
	duplicateUser.Email = unrelatedOwnerUser.Email // already exists from unrelated
	dupSellerID := uuid.New()
	dupSeller := *seller
	dupSeller.ID = dupSellerID
	dupSellerUser := *sellerUser
	dupSellerUser.ID = uuid.New()
	dupSellerUser.SellerID = dupSellerID
	dupSellerUser.UserID = duplicateUser.ID

	err = repo.CreateIsolatedIdentity(ctx, &duplicateUser, &dupSeller, &dupSellerUser)
	if err == nil {
		t.Fatalf("expected error on duplicate email")
	}

	// Verify rollback
	db.QueryRow(ctx, "SELECT count(*) FROM sellers WHERE id = $1", dupSellerID).Scan(&count)
	if count != 0 {
		t.Fatalf("seller should be rolled back")
	}
}
