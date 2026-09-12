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
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

func setupEMP1A3TestHarness(t *testing.T) (context.Context, *postgres.Client, *staff.Service, *staff.Repository, *users.Repository, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pgClient.Close() })

	testutil.AssertTestDatabase(t, pgClient.Pool)

	staffRepo := staff.NewRepository(pgClient.Pool)
	userRepo := users.NewRepository(pgClient.Pool)
	svc := staff.NewService(staffRepo, userRepo, pgClient)

	// Ensure an active owner exists as actor
	var ownerRoleID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner'`).Scan(&ownerRoleID)
	require.NoError(t, err)

	ownerID := uuid.New()
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Owner Actor', $2, 'hash', 'admin', 'active', NOW(), NOW())
	`, ownerID, fmt.Sprintf("owner_%s@test.com", ownerID.String()[:8]))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pgClient.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, ownerID)
	})

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
	`, ownerID, ownerRoleID)
	require.NoError(t, err)

	return ctx, pgClient, svc, staffRepo, userRepo, ownerID
}

func createTestRole(t *testing.T, ctx context.Context, client *postgres.Client, perms ...string) (uuid.UUID, string) {
	t.Helper()
	roleID := uuid.New()
	roleCode := fmt.Sprintf("role_%s", roleID.String()[:8])
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO staff_roles (id, code, name, is_system, created_at, updated_at)
		VALUES ($1, $2, 'Test Role', false, NOW(), NOW())
	`, roleID, roleCode)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = client.Pool.Exec(context.Background(), `DELETE FROM staff_roles WHERE id = $1`, roleID)
	})

	for _, p := range perms {
		_, err = client.Pool.Exec(ctx, `
			INSERT INTO staff_role_permissions (role_id, permission, created_at)
			VALUES ($1, $2, NOW())
		`, roleID, p)
		require.NoError(t, err)
	}

	return roleID, roleCode
}

// TestEMP1A3_CreateStaffMember verifies atomic preset copying on staff member creation.
func TestEMP1A3_CreateStaffMember(t *testing.T) {
	ctx, client, svc, repo, _, ownerID := setupEMP1A3TestHarness(t)

	// A. Create with preset {perm.a, perm.b}
	t.Run("A_CreateWithPreset", func(t *testing.T) {
		roleID, roleCode := createTestRole(t, ctx, client, "perm.create_a", "perm.create_b")

		email := fmt.Sprintf("create_staff_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Staff Preset",
			Email:             email,
			Phone:             "+79991234567",
			RoleCode:          roleCode,
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		// Assert staff_members.staff_role_id == roleID
		member, role, err := repo.GetStaffMemberByUserID(ctx, res.UserID)
		require.NoError(t, err)
		require.Equal(t, roleID, member.StaffRoleID)
		require.Equal(t, roleCode, role.Code)

		// Assert direct permissions == {perm.create_a, perm.create_b}
		directPerms, err := repo.GetMemberPermissions(ctx, res.UserID)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"perm.create_a", "perm.create_b"}, directPerms)

		// Assert runtime authorization
		hasA, err := svc.HasPermission(ctx, res.UserID, "perm.create_a")
		require.NoError(t, err)
		require.True(t, hasA)

		hasB, err := svc.HasPermission(ctx, res.UserID, "perm.create_b")
		require.NoError(t, err)
		require.True(t, hasB)

		hasOther, err := svc.HasPermission(ctx, res.UserID, "perm.other")
		require.NoError(t, err)
		require.False(t, hasOther)
	})

	// B. Create with empty preset {}
	t.Run("B_CreateWithEmptyPreset", func(t *testing.T) {
		roleID, roleCode := createTestRole(t, ctx, client) // 0 permissions

		email := fmt.Sprintf("create_empty_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Staff Empty Preset",
			Email:             email,
			Phone:             "+79991234568",
			RoleCode:          roleCode,
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		// Assert employee exists and has zero direct permissions
		member, _, err := repo.GetStaffMemberByUserID(ctx, res.UserID)
		require.NoError(t, err)
		require.Equal(t, roleID, member.StaffRoleID)

		directPerms, err := repo.GetMemberPermissions(ctx, res.UserID)
		require.NoError(t, err)
		require.Empty(t, directPerms)

		has, err := svc.HasPermission(ctx, res.UserID, "any.permission")
		require.NoError(t, err)
		require.False(t, has)
	})

	// C. Create transaction failure: permission sync failure rolls back user and staff_member
	t.Run("C_CreateTransactionFailureRollback", func(t *testing.T) {
		failPerm := fmt.Sprintf("fail_perm_%s", uuid.New().String()[:8])
		_, roleCode := createTestRole(t, ctx, client, failPerm)

		// Add temporary check constraint that rejects failPerm in staff_member_permissions
		constraintName := fmt.Sprintf("chk_fail_%s", uuid.New().String()[:8])
		_, err := client.Pool.Exec(ctx, fmt.Sprintf(`
			ALTER TABLE staff_member_permissions
			ADD CONSTRAINT %s CHECK (permission != '%s')
		`, constraintName, failPerm))
		require.NoError(t, err)
		defer func() {
			_, _ = client.Pool.Exec(context.Background(), fmt.Sprintf(`
				ALTER TABLE staff_member_permissions DROP CONSTRAINT IF EXISTS %s
			`, constraintName))
		}()

		email := fmt.Sprintf("create_fail_%s@test.com", uuid.New().String()[:8])
		_, err = svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Should Rollback",
			Email:             email,
			Phone:             "+79991234569",
			RoleCode:          roleCode,
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerID,
		})
		require.Error(t, err, "CreateStaffMember must fail when permission copy violates constraint")

		// Assert no partially created user or staff_member survived
		var userCount int
		err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, email).Scan(&userCount)
		require.NoError(t, err)
		require.Equal(t, 0, userCount, "user record must not survive transaction rollback")

		var staffCount int
		err = client.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM staff_members sm
			JOIN users u ON u.id = sm.user_id
			WHERE u.email = $1
		`, email).Scan(&staffCount)
		require.NoError(t, err)
		require.Equal(t, 0, staffCount, "staff_members record must not survive transaction rollback")
	})
}

// TestEMP1A3_UpdateStaffRole verifies atomic preset replacement and failure rollback.
func TestEMP1A3_UpdateStaffRole(t *testing.T) {
	ctx, client, svc, repo, _, ownerID := setupEMP1A3TestHarness(t)

	// D. UPDATE ROLE / APPLY PRESET
	// Employee currently: RoleA, direct = {perm.custom, perm.a}
	// RoleB template: {perm.b, perm.c}
	// UpdateStaffRole(RoleB) -> staff_role_id == RoleB, direct == {perm.b, perm.c}, perm.custom and perm.a removed
	t.Run("D_UpdateRoleApplyPreset", func(t *testing.T) {
		roleAID, roleACode := createTestRole(t, ctx, client, "perm.role_a")
		roleBID, roleBCode := createTestRole(t, ctx, client, "perm.role_b", "perm.role_c")

		// Create employee with RoleA
		email := fmt.Sprintf("update_staff_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Staff Update Role",
			Email:             email,
			RoleCode:          roleACode,
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		// Add custom direct permission
		_, err = client.Pool.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, 'perm.custom', NOW())
		`, res.UserID)
		require.NoError(t, err)

		// Verify initial direct state: {perm.role_a, perm.custom}
		permsBefore, err := repo.GetMemberPermissions(ctx, res.UserID)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"perm.role_a", "perm.custom"}, permsBefore)

		// Perform UpdateStaffRole to RoleB
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: res.UserID,
			NewRoleCode:  roleBCode,
			ActorUserID:  ownerID,
		})
		require.NoError(t, err)

		// Assert staff_role_id updated to RoleB
		memberAfter, roleAfter, err := repo.GetStaffMemberByUserID(ctx, res.UserID)
		require.NoError(t, err)
		require.Equal(t, roleBID, memberAfter.StaffRoleID)
		require.Equal(t, roleBCode, roleAfter.Code)

		// Assert direct permissions replaced with exact RoleB preset: {perm.role_b, perm.role_c}
		permsAfter, err := repo.GetMemberPermissions(ctx, res.UserID)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"perm.role_b", "perm.role_c"}, permsAfter)

		// Verify perm.custom and perm.role_a were removed
		hasCustom, err := svc.HasPermission(ctx, res.UserID, "perm.custom")
		require.NoError(t, err)
		require.False(t, hasCustom, "old custom direct permission must be removed upon role preset replacement")

		hasA, err := svc.HasPermission(ctx, res.UserID, "perm.role_a")
		require.NoError(t, err)
		require.False(t, hasA, "old role preset permission must be removed")

		hasB, err := svc.HasPermission(ctx, res.UserID, "perm.role_b")
		require.NoError(t, err)
		require.True(t, hasB)

		hasC, err := svc.HasPermission(ctx, res.UserID, "perm.role_c")
		require.NoError(t, err)
		require.True(t, hasC)

		_ = roleAID
	})

	// E. UPDATE ROLE FAILURE: Force sync failure -> old staff_role_id and old direct perms remain
	t.Run("E_UpdateRoleFailureRollback", func(t *testing.T) {
		roleAID, roleACode := createTestRole(t, ctx, client, "perm.old_a")

		failPerm := fmt.Sprintf("fail_upd_%s", uuid.New().String()[:8])
		_, roleFailCode := createTestRole(t, ctx, client, failPerm)

		// Create employee with RoleA
		email := fmt.Sprintf("update_fail_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Staff Update Fail",
			Email:             email,
			RoleCode:          roleACode,
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		// Add constraint that fails on failPerm
		constraintName := fmt.Sprintf("chk_upd_fail_%s", uuid.New().String()[:8])
		_, err = client.Pool.Exec(ctx, fmt.Sprintf(`
			ALTER TABLE staff_member_permissions
			ADD CONSTRAINT %s CHECK (permission != '%s')
		`, constraintName, failPerm))
		require.NoError(t, err)
		defer func() {
			_, _ = client.Pool.Exec(context.Background(), fmt.Sprintf(`
				ALTER TABLE staff_member_permissions DROP CONSTRAINT IF EXISTS %s
			`, constraintName))
		}()

		// Attempt UpdateStaffRole to roleFailCode
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: res.UserID,
			NewRoleCode:  roleFailCode,
			ActorUserID:  ownerID,
		})
		require.Error(t, err, "UpdateStaffRole must fail when copy violates constraint")

		// Assert old staff_role_id remains intact
		member, role, err := repo.GetStaffMemberByUserID(ctx, res.UserID)
		require.NoError(t, err)
		require.Equal(t, roleAID, member.StaffRoleID, "staff_role_id must rollback to old role")
		require.Equal(t, roleACode, role.Code)

		// Assert old direct permissions remain intact
		perms, err := repo.GetMemberPermissions(ctx, res.UserID)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"perm.old_a"}, perms, "direct permissions must rollback to old set")
	})
}

// TestEMP1A3_RuntimeAuthorizationMatrix verifies runtime authorization invariants.
func TestEMP1A3_RuntimeAuthorizationMatrix(t *testing.T) {
	ctx, client, svc, repo, _, ownerID := setupEMP1A3TestHarness(t)

	// A & B: CRITICAL PROOF OF NO ROLE FALLBACK
	// Active employee with direct permission -> allowed.
	// Permission exists ONLY on role template, NOT in member permissions -> denied!
	t.Run("A_and_B_NoRoleFallback", func(t *testing.T) {
		_, roleCode := createTestRole(t, ctx, client, "perm.in_role_only")

		email := fmt.Sprintf("auth_matrix_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Staff Matrix",
			Email:             email,
			RoleCode:          roleCode,
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		// A: Active + direct permission -> allowed
		hasDirect, err := svc.HasPermission(ctx, res.UserID, "perm.in_role_only")
		require.NoError(t, err)
		require.True(t, hasDirect, "active staff member with direct permission must be allowed")

		// Manually remove direct permission row so permission exists ONLY on role
		_, err = client.Pool.Exec(ctx, `
			DELETE FROM staff_member_permissions
			WHERE user_id = $1 AND permission = 'perm.in_role_only'
		`, res.UserID)
		require.NoError(t, err)

		// B: CRITICAL: permission exists in staff_role_permissions for this employee's role,
		// but NOT in staff_member_permissions -> MUST RETURN FALSE!
		hasRoleOnly, err := svc.HasPermission(ctx, res.UserID, "perm.in_role_only")
		require.NoError(t, err)
		require.False(t, hasRoleOnly, "CRITICAL: HasPermission must NOT fallback to staff_role_permissions")
	})

	// C: Direct permission exists, but role template does not contain it -> allowed.
	t.Run("C_DirectPermWithoutRoleTemplateAllowed", func(t *testing.T) {
		_, roleCode := createTestRole(t, ctx, client) // empty role

		email := fmt.Sprintf("auth_c_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Staff C",
			Email:             email,
			RoleCode:          roleCode,
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		// Grant direct permission not present in role
		_, err = client.Pool.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, 'perm.direct_custom', NOW())
		`, res.UserID)
		require.NoError(t, err)

		has, err := svc.HasPermission(ctx, res.UserID, "perm.direct_custom")
		require.NoError(t, err)
		require.True(t, has, "direct member permission must authorize even if role template lacks it")
	})

	// D: Direct grant / revoke immediacy (Section 7)
	// Direct INSERT -> permission effective immediately without token refresh or role update.
	// Direct DELETE while role still has it -> denied immediately.
	t.Run("D_DirectGrantRevokeImmediacy", func(t *testing.T) {
		roleID, roleCode := createTestRole(t, ctx, client, "perm.immediate_role")

		email := fmt.Sprintf("auth_d_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Staff D",
			Email:             email,
			RoleCode:          roleCode,
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		// Test grant immediacy: insert new direct permission
		newPerm := "perm.immediate_grant"
		_, err = client.Pool.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, $2, NOW())
		`, res.UserID, newPerm)
		require.NoError(t, err)

		hasGranted, err := svc.HasPermission(ctx, res.UserID, newPerm)
		require.NoError(t, err)
		require.True(t, hasGranted, "direct INSERT must be effective immediately without role change or token refresh")

		// Test revoke immediacy: delete direct permission while role still has it
		_, err = client.Pool.Exec(ctx, `
			DELETE FROM staff_member_permissions
			WHERE user_id = $1 AND permission = 'perm.immediate_role'
		`, res.UserID)
		require.NoError(t, err)

		// Assert role still has it
		var roleHasCount int
		err = client.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM staff_role_permissions WHERE role_id = $1 AND permission = 'perm.immediate_role'
		`, roleID).Scan(&roleHasCount)
		require.NoError(t, err)
		require.Equal(t, 1, roleHasCount)

		// But member check is denied immediately
		hasRevoked, err := svc.HasPermission(ctx, res.UserID, "perm.immediate_role")
		require.NoError(t, err)
		require.False(t, hasRevoked, "direct DELETE must revoke permission immediately even if role still contains it")
	})

	// E: Role template independence (Section 6)
	// Changing staff_role_permissions must NOT automatically change an existing employee's effective permissions.
	t.Run("E_RoleTemplateIndependence", func(t *testing.T) {
		roleID, roleCode := createTestRole(t, ctx, client, "perm.initial")

		email := fmt.Sprintf("auth_e_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Staff E",
			Email:             email,
			RoleCode:          roleCode,
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		// Verify initial permission
		hasInitial, err := svc.HasPermission(ctx, res.UserID, "perm.initial")
		require.NoError(t, err)
		require.True(t, hasInitial)

		// Add a new permission to the role template
		_, err = client.Pool.Exec(ctx, `
			INSERT INTO staff_role_permissions (role_id, permission, created_at)
			VALUES ($1, 'perm.added_to_template_later', NOW())
		`, roleID)
		require.NoError(t, err)

		// Assert existing employee does NOT automatically acquire the newly added role permission
		hasNewRolePerm, err := svc.HasPermission(ctx, res.UserID, "perm.added_to_template_later")
		require.NoError(t, err)
		require.False(t, hasNewRolePerm, "role template modification must NOT change existing employee's effective authorization")

		// Remove perm.initial from role template
		_, err = client.Pool.Exec(ctx, `
			DELETE FROM staff_role_permissions WHERE role_id = $1 AND permission = 'perm.initial'
		`, roleID)
		require.NoError(t, err)

		// Assert existing employee still retains perm.initial because their direct permission was untouched
		hasInitialStill, err := svc.HasPermission(ctx, res.UserID, "perm.initial")
		require.NoError(t, err)
		require.True(t, hasInitialStill, "existing employee retains direct permission even if removed from role template")
	})

	// F: Blocked / Inactive staff member with direct permission -> denied
	t.Run("F_BlockedStaffMemberDeniedEvenWithDirectPerm", func(t *testing.T) {
		_, roleCode := createTestRole(t, ctx, client, "perm.blocked_test")

		email := fmt.Sprintf("auth_f_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Staff F",
			Email:             email,
			RoleCode:          roleCode,
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		// Block the staff member
		err = repo.UpdateStaffStatus(ctx, res.UserID, "blocked")
		require.NoError(t, err)

		// Verify member has direct permission row
		hasDirectRow, err := repo.HasMemberPermission(ctx, res.UserID, "perm.blocked_test")
		require.NoError(t, err)
		require.True(t, hasDirectRow)

		// But HasPermission returns false because status is blocked
		has, err := svc.HasPermission(ctx, res.UserID, "perm.blocked_test")
		require.NoError(t, err)
		require.False(t, has, "blocked staff member must be denied even if direct permission row exists")
	})

	// G: Non-staff Admin user -> denied by HasPermission
	t.Run("G_NonStaffAdminUserDenied", func(t *testing.T) {
		nonStaffUserID := uuid.New()
		_, err := client.Pool.Exec(ctx, `
			INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
			VALUES ($1, 'Non-Staff Admin', $2, 'hash', 'admin', 'active', NOW(), NOW())
		`, nonStaffUserID, fmt.Sprintf("nonstaff_%s@test.com", nonStaffUserID.String()[:8]))
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, nonStaffUserID)
		})

		has, err := svc.HasPermission(ctx, nonStaffUserID, "any.permission")
		require.NoError(t, err)
		require.False(t, has, "non-staff admin user must be denied permission check")
	})
}
