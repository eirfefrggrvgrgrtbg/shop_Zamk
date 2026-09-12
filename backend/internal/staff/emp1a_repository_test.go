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

func TestEMP1A_RepositoryInvariants(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	defer pgClient.Close()

	testutil.AssertTestDatabase(t, pgClient.Pool)

	createTestUser := func(t *testing.T, tx postgres.DBTX) uuid.UUID {
		t.Helper()
		userID := uuid.New()
		phone := fmt.Sprintf("+7999%07d", userID.ID()%10000000)
		email := fmt.Sprintf("test_%s@example.com", userID.String()[:8])
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, phone, name, email, password_hash, created_at, updated_at)
			VALUES ($1, $2, 'Staff User', $3, 'hash', NOW(), NOW())
		`, userID, phone, email)
		require.NoError(t, err)
		return userID
	}

	createTestRole := func(t *testing.T, tx postgres.DBTX, perms ...string) uuid.UUID {
		t.Helper()
		roleID := uuid.New()
		roleCode := fmt.Sprintf("role_%s", roleID.String()[:8])
		_, err := tx.Exec(ctx, `
			INSERT INTO staff_roles (id, code, name, created_at, updated_at)
			VALUES ($1, $2, 'Test Role', NOW(), NOW())
		`, roleID, roleCode)
		require.NoError(t, err)

		for _, p := range perms {
			_, err = tx.Exec(ctx, `
				INSERT INTO staff_role_permissions (role_id, permission, created_at)
				VALUES ($1, $2, NOW())
			`, roleID, p)
			require.NoError(t, err)
		}
		return roleID
	}

	createTestStaff := func(t *testing.T, tx postgres.DBTX, roleID uuid.UUID) uuid.UUID {
		t.Helper()
		userID := createTestUser(t, tx)
		_, err := tx.Exec(ctx, `
			INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
			VALUES ($1, $2, 'active', NOW(), NOW())
		`, userID, roleID)
		require.NoError(t, err)
		return userID
	}

	// A. Direct row exists: HasMemberPermission == true
	t.Run("A_DirectRowExists", func(t *testing.T) {
		tx, err := pgClient.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		roleID := createTestRole(t, tx)
		userID := createTestStaff(t, tx, roleID)

		_, err = tx.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, 'perm.direct_a', NOW())
		`, userID)
		require.NoError(t, err)

		repo := staff.NewRepository(pgClient.Pool).WithTx(tx)
		has, err := repo.HasMemberPermission(ctx, userID, "perm.direct_a")
		require.NoError(t, err)
		require.True(t, has)
	})

	// B. Permission exists in THIS EMPLOYEE'S staff_role_permissions but direct
	// staff_member_permissions row is absent: HasMemberPermission == false.
	// This proves repository direct reads do not fall back to roles.
	t.Run("B_RolePermissionDoesNotFallback", func(t *testing.T) {
		tx, err := pgClient.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		roleID := createTestRole(t, tx, "perm.role_only")
		userID := createTestStaff(t, tx, roleID)

		repo := staff.NewRepository(pgClient.Pool).WithTx(tx)
		has, err := repo.HasMemberPermission(ctx, userID, "perm.role_only")
		require.NoError(t, err)
		require.False(t, has, "HasMemberPermission must not fall back to staff_role_permissions")
	})

	// C. GetMemberPermissions returns exactly the direct set.
	t.Run("C_GetMemberPermissionsExactSet", func(t *testing.T) {
		tx, err := pgClient.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		roleID := createTestRole(t, tx, "perm.role_other")
		userID := createTestStaff(t, tx, roleID)

		_, err = tx.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, 'perm.direct_1', NOW()), ($1, 'perm.direct_2', NOW())
		`, userID)
		require.NoError(t, err)

		repo := staff.NewRepository(pgClient.Pool).WithTx(tx)
		perms, err := repo.GetMemberPermissions(ctx, userID)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"perm.direct_1", "perm.direct_2"}, perms)
	})

	// D. Duplicate: inserting the same (user_id, permission) twice fails due to DB uniqueness.
	t.Run("D_DuplicateFailsUniqueness", func(t *testing.T) {
		tx, err := pgClient.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		roleID := createTestRole(t, tx)
		userID := createTestStaff(t, tx, roleID)

		_, err = tx.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, 'perm.dup', NOW())
		`, userID)
		require.NoError(t, err)

		// Second insert must fail with unique constraint violation
		_, err = tx.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, 'perm.dup', NOW())
		`, userID)
		require.Error(t, err)
	})

	// E. Staff FK: attempting to insert a staff_member_permission for a users.id that has NO
	// staff_members row must fail.
	t.Run("E_StaffFKRejectsNonStaffUser", func(t *testing.T) {
		tx, err := pgClient.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		nonStaffUserID := createTestUser(t, tx)

		_, err = tx.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, 'perm.non_staff', NOW())
		`, nonStaffUserID)
		require.Error(t, err, "must fail foreign key constraint referencing staff_members(user_id)")
	})

	// F. Cascade: delete the staff_members row and verify its staff_member_permissions rows disappear.
	t.Run("F_CascadeDeleteStaffMember", func(t *testing.T) {
		tx, err := pgClient.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		roleID := createTestRole(t, tx)
		userID := createTestStaff(t, tx, roleID)

		_, err = tx.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, 'perm.cascade_1', NOW()), ($1, 'perm.cascade_2', NOW())
		`, userID)
		require.NoError(t, err)

		repo := staff.NewRepository(pgClient.Pool).WithTx(tx)
		perms, err := repo.GetMemberPermissions(ctx, userID)
		require.NoError(t, err)
		require.Len(t, perms, 2)

		// Delete staff member
		_, err = tx.Exec(ctx, `DELETE FROM staff_members WHERE user_id = $1`, userID)
		require.NoError(t, err)

		// Direct permissions must be cascaded
		permsAfter, err := repo.GetMemberPermissions(ctx, userID)
		require.NoError(t, err)
		require.Empty(t, permsAfter, "staff_member_permissions must cascade on staff_members deletion")
	})

	// G. Empty role: backfill yields zero direct rows.
	t.Run("G_EmptyRoleYieldsZeroDirectRows", func(t *testing.T) {
		tx, err := pgClient.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		emptyRoleID := createTestRole(t, tx) // 0 permissions
		userID := createTestStaff(t, tx, emptyRoleID)

		// Backfill query for this member
		_, err = tx.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			SELECT sm.user_id, srp.permission, NOW()
			FROM staff_members sm
			JOIN staff_role_permissions srp ON srp.role_id = sm.staff_role_id
			WHERE sm.user_id = $1
			ON CONFLICT DO NOTHING
		`, userID)
		require.NoError(t, err)

		repo := staff.NewRepository(pgClient.Pool).WithTx(tx)
		perms, err := repo.GetMemberPermissions(ctx, userID)
		require.NoError(t, err)
		require.Empty(t, perms, "empty role backfill must yield zero direct permissions")

		has, err := repo.HasMemberPermission(ctx, userID, "any.permission")
		require.NoError(t, err)
		require.False(t, has)
	})
}
