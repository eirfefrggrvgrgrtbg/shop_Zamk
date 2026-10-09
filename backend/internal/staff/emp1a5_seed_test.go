package staff_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func TestRepository_EnsureOwnerForSeed(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	defer pgClient.Close()
	testutil.AssertTestDatabase(t, pgClient.Pool)

	repo := staff.NewRepository(pgClient.Pool)

	// Create a dummy user
	userID := uuid.New()
	email1 := fmt.Sprintf("seed_%s@test.local", userID.String())
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, phone, name, email, password_hash, created_at, updated_at)
		VALUES ($1, $2, 'Test Seed Owner', $3, 'hash', now(), now())
	`, userID, "+12345678900", email1)
	require.NoError(t, err)

	// Create an unrelated user to verify they are unchanged
	unrelatedID := uuid.New()
	email2 := fmt.Sprintf("unrelated_%s@test.local", unrelatedID.String())
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, phone, name, email, password_hash, created_at, updated_at)
		VALUES ($1, $2, 'Test Unrelated', $3, 'hash', now(), now())
	`, unrelatedID, "+12345678901", email2)
	require.NoError(t, err)

	// Give unrelated user some role
	supportRole, err := repo.GetRoleByCode(ctx, "support")
	require.NoError(t, err)
	err = repo.InsertStaffMember(ctx, unrelatedID, supportRole.ID, nil)
	require.NoError(t, err)
	err = repo.CopyRolePermissionsToMember(ctx, unrelatedID, supportRole.ID)
	require.NoError(t, err)
	unrelatedPermsBefore, err := repo.GetMemberPermissions(ctx, unrelatedID)
	require.NoError(t, err)

	// 1. Run EnsureOwnerForSeed
	err = repo.EnsureOwnerForSeed(ctx, userID)
	require.NoError(t, err)

	// Verify 1: seeded owner has owner role
	member, role, err := repo.GetStaffMemberByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, "owner", role.Code)
	require.Equal(t, string(staff.StatusActive), member.Status)

	// Verify 2: seeded owner receives canonical owner role permissions
	perms, err := repo.GetMemberPermissions(ctx, userID)
	require.NoError(t, err)
	require.NotEmpty(t, perms)

	// Verify 4: marketing.campaigns.read is present
	require.Contains(t, perms, "marketing.campaigns.read")

	// 5. Run EnsureOwnerForSeed AGAIN (idempotency)
	err = repo.EnsureOwnerForSeed(ctx, userID)
	require.NoError(t, err)

	// 6. Verify no duplicate permission rows
	permsAfter, err := repo.GetMemberPermissions(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, perms, permsAfter)

	// 7. Verify unrelated staff member is unchanged
	unrelatedPermsAfter, err := repo.GetMemberPermissions(ctx, unrelatedID)
	require.NoError(t, err)
	require.Equal(t, unrelatedPermsBefore, unrelatedPermsAfter)
}
