package staff_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

func setupEMP1B2Harness(t *testing.T) (context.Context, *postgres.Client, *staff.Service, *staff.Repository) {
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

	return ctx, pgClient, svc, staffRepo
}

func createTestStaff(t *testing.T, ctx context.Context, client *postgres.Client, svc *staff.Service, roleCode string) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	email := fmt.Sprintf("staff_%s_%s@test.com", roleCode, userID.String()[:8])
	now := time.Now().UTC()

	t.Cleanup(func() {
		// FK-safe cleanup for this specific exact test fixture ID
		_, _ = client.Pool.Exec(context.Background(), `DELETE FROM staff_member_permissions WHERE user_id = $1`, userID)
		_, _ = client.Pool.Exec(context.Background(), `DELETE FROM staff_members WHERE user_id = $1`, userID)
		_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	var roleID uuid.UUID
	err := client.Pool.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = $1`, roleCode).Scan(&roleID)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'Test ' || $2, $3, 'dummy_hash', 'admin', 'active', false, $4, $4)
	`, userID, roleCode, email, now)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
		VALUES ($1, $2, 'active', $3, $3)
	`, userID, roleID, now)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO staff_member_permissions (user_id, permission, created_at)
		SELECT $1, permission, $2
		FROM staff_role_permissions
		WHERE role_id = $3
	`, userID, now, roleID)
	require.NoError(t, err)

	return userID
}

func giveDirectPermission(t *testing.T, ctx context.Context, client *postgres.Client, userID uuid.UUID, permission string) {
	t.Helper()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO staff_member_permissions (user_id, permission, created_at)
		VALUES ($1, $2, now())
		ON CONFLICT DO NOTHING
	`, userID, permission)
	require.NoError(t, err)
}

func removeDirectPermission(t *testing.T, ctx context.Context, client *postgres.Client, userID uuid.UUID, permission string) {
	t.Helper()
	_, err := client.Pool.Exec(ctx, `
		DELETE FROM staff_member_permissions
		WHERE user_id = $1 AND permission = $2
	`, userID, permission)
	require.NoError(t, err)
}

// TestEMP1B2_SequentialManagerSafety verifies sequential manager lockout invariants.
func TestEMP1B2_SequentialManagerSafety(t *testing.T) {
	ctx, client, svc, repo := setupEMP1B2Harness(t)

	// Actor for status changes: an active owner who has direct manage removed
	actorOwnerID := createTestStaff(t, ctx, client, svc, "owner")
	removeDirectPermission(t, ctx, client, actorOwnerID, staff.PermissionStaffPermissionsManage)

	t.Run("A_OneActiveManager_StatusUpdateRejected", func(t *testing.T) {
		// Manager A: non-owner (admin) with direct permission
		managerAID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, managerAID, staff.PermissionStaffPermissionsManage)

		count, err := repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count, "must have exactly 1 active manager")

		// Attempt to block Manager A
		err = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: managerAID,
			NewStatus:    string(staff.StatusBlocked),
			ActorUserID:  actorOwnerID,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrCannotRemoveLastPermissionManager)

		// Assert status is still active
		m, _, err := repo.GetStaffMemberByUserID(ctx, managerAID)
		require.NoError(t, err)
		require.Equal(t, string(staff.StatusActive), m.Status)
	})

	t.Run("B_TwoActiveManagers_OneCanBeBlocked", func(t *testing.T) {
		managerAID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, managerAID, staff.PermissionStaffPermissionsManage)

		managerBID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, managerBID, staff.PermissionStaffPermissionsManage)

		count, err := repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count, "must have exactly 2 active managers")

		// Block Manager B -> should succeed
		err = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: managerBID,
			NewStatus:    string(staff.StatusBlocked),
			ActorUserID:  actorOwnerID,
		})
		require.NoError(t, err)

		// Assert Manager B is blocked
		m, _, err := repo.GetStaffMemberByUserID(ctx, managerBID)
		require.NoError(t, err)
		require.Equal(t, string(staff.StatusBlocked), m.Status)

		// Manager count should now be 1
		count, err = repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count, "remaining active managers must be 1")

		// Now attempt to block Manager A -> should be rejected
		err = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: managerAID,
			NewStatus:    string(staff.StatusBlocked),
			ActorUserID:  actorOwnerID,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrCannotRemoveLastPermissionManager)
	})

	t.Run("C_OneActiveManager_RoleUpdateWithoutManageRejected", func(t *testing.T) {
		managerAID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, managerAID, staff.PermissionStaffPermissionsManage)
		giveDirectPermission(t, ctx, client, managerAID, "staff.update")

		count, err := repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		// Demote Manager A to 'support' (which does NOT include staff.permissions.manage)
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: managerAID,
			NewRoleCode:  "support",
			ActorUserID:  managerAID,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrCannotRemoveLastPermissionManager)

		// Assert Manager A permissions intact
		hasPerm, err := repo.HasMemberPermission(ctx, managerAID, staff.PermissionStaffPermissionsManage)
		require.NoError(t, err)
		require.True(t, hasPerm)
	})

	t.Run("D_TwoActiveManagers_OneCanBeDemoted", func(t *testing.T) {
		managerAID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, managerAID, staff.PermissionStaffPermissionsManage)
		giveDirectPermission(t, ctx, client, managerAID, "staff.update")

		managerBID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, managerBID, staff.PermissionStaffPermissionsManage)

		count, err := repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		// Demote Manager B to support -> should succeed
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: managerBID,
			NewRoleCode:  "support",
			ActorUserID:  managerAID,
		})
		require.NoError(t, err)

		// Assert Manager B lost direct manage capability
		hasPerm, err := repo.HasMemberPermission(ctx, managerBID, staff.PermissionStaffPermissionsManage)
		require.NoError(t, err)
		require.False(t, hasPerm)

		// Remaining active managers must be 1
		count, err = repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		// Now attempt to demote Manager A to support -> should fail
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: managerAID,
			NewRoleCode:  "support",
			ActorUserID:  managerAID,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrCannotRemoveLastPermissionManager)
	})
}

// TestEMP1B2_OwnerManagerIndependence verifies that owner and permission-manager invariants operate independently.
func TestEMP1B2_OwnerManagerIndependence(t *testing.T) {
	ctx, client, svc, repo := setupEMP1B2Harness(t)

	actorOwnerID := createTestStaff(t, ctx, client, svc, "owner")
	removeDirectPermission(t, ctx, client, actorOwnerID, staff.PermissionStaffPermissionsManage)

	t.Run("A_OwnerWithoutManage_CannotBeDemotedWhenLastOwner", func(t *testing.T) {
		// Single owner in DB: actorOwnerID (manage removed)
		// Active manager exists elsewhere:
		managerID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, managerID, staff.PermissionStaffPermissionsManage)
		giveDirectPermission(t, ctx, client, managerID, "staff.update")

		// Count owners
		ownerCount, err := repo.CountActiveOwners(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, ownerCount)

		// Manager attempts to demote the single owner to admin
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: actorOwnerID,
			NewRoleCode:  "admin",
			ActorUserID:  managerID,
		})
		require.Error(t, err)
		// Non-owner actor cannot demote owner
		require.True(t, errors.Is(err, staff.ErrCannotDemoteOwner) || errors.Is(err, staff.ErrCannotRemoveLastOwner))
	})

	t.Run("B_ManagerWithoutOwner_BlockedByLastManagerInvariant", func(t *testing.T) {
		// Single active manager: managerID
		managerID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, managerID, staff.PermissionStaffPermissionsManage)

		// Active owner exists (with no manage)
		ownerCount, err := repo.CountActiveOwners(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, ownerCount, 1)

		// Blocking this manager is rejected by ErrCannotRemoveLastPermissionManager,
		// despite active owners existing in the system!
		err = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: managerID,
			NewStatus:    string(staff.StatusBlocked),
			ActorUserID:  actorOwnerID,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrCannotRemoveLastPermissionManager)
	})

	t.Run("C_IndependentSurvival_DemoteManagerLeavingOwner", func(t *testing.T) {
		// Two managers: M1 and M2
		m1 := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, m1, staff.PermissionStaffPermissionsManage)
		giveDirectPermission(t, ctx, client, m1, "staff.update")

		m2 := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, m2, staff.PermissionStaffPermissionsManage)

		// Demote M2 to support -> succeeds, leaves 1 manager and does not affect owner count
		err := svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: m2,
			NewRoleCode:  "support",
			ActorUserID:  m1,
		})
		require.NoError(t, err)

		mCount, err := repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, mCount)
	})
}

// TestEMP1B2_DeterministicConcurrency tests deterministic race conditions for manager protection.
func TestEMP1B2_DeterministicConcurrency(t *testing.T) {
	ctx, client, svc, repo := setupEMP1B2Harness(t)

	// Actor for status updates: an active owner with no direct manage
	actorOwnerID := createTestStaff(t, ctx, client, svc, "owner")
	removeDirectPermission(t, ctx, client, actorOwnerID, staff.PermissionStaffPermissionsManage)

	t.Run("A_Block_Block_Serialized", func(t *testing.T) {
		m1 := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, m1, staff.PermissionStaffPermissionsManage)

		m2 := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, m2, staff.PermissionStaffPermissionsManage)

		count, err := repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		// 1. Acquire owner role lock in a dedicated transaction
		txLock, err := client.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = txLock.Rollback(ctx) }()

		var lockedID uuid.UUID
		err = txLock.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner' FOR UPDATE`).Scan(&lockedID)
		require.NoError(t, err)

		// 2. Launch R1 (block m1) and R2 (block m2)
		var wg sync.WaitGroup
		var err1, err2 error

		wg.Add(2)
		go func() {
			defer wg.Done()
			err1 = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
				TargetUserID: m1,
				NewStatus:    string(staff.StatusBlocked),
				ActorUserID:  actorOwnerID,
			})
		}()
		go func() {
			defer wg.Done()
			err2 = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
				TargetUserID: m2,
				NewStatus:    string(staff.StatusBlocked),
				ActorUserID:  actorOwnerID,
			})
		}()

		// Wait until at least one goroutine is blocked waiting on the lock
		require.Eventually(t, func() bool {
			var waitingCount int
			_ = client.Pool.QueryRow(ctx, `
				SELECT count(*) FROM pg_stat_activity
				WHERE wait_event_type = 'Lock'
				  AND state = 'active'
				  AND query LIKE '%SELECT id FROM staff_roles WHERE code = ''owner'' FOR UPDATE%'
				  AND pid != pg_backend_pid()
			`).Scan(&waitingCount)
			return waitingCount >= 1
		}, 3*time.Second, 10*time.Millisecond, "at least one goroutine must be waiting on lock")

		// Release lock
		err = txLock.Rollback(ctx)
		require.NoError(t, err)

		wg.Wait()

		// Exactly one must succeed, one must fail with ErrCannotRemoveLastPermissionManager
		successCount := 0
		failCount := 0
		if err1 == nil {
			successCount++
		} else if errors.Is(err1, staff.ErrCannotRemoveLastPermissionManager) {
			failCount++
		}
		if err2 == nil {
			successCount++
		} else if errors.Is(err2, staff.ErrCannotRemoveLastPermissionManager) {
			failCount++
		}

		require.Equal(t, 1, successCount, "exactly one block must succeed")
		require.Equal(t, 1, failCount, "exactly one block must fail with ErrCannotRemoveLastPermissionManager")

		remaining, err := repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, remaining, "exactly 1 active manager must survive")
	})

	t.Run("B_Preset_Block_Serialized", func(t *testing.T) {
		m1 := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, m1, staff.PermissionStaffPermissionsManage)
		giveDirectPermission(t, ctx, client, m1, "staff.update")

		m2 := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, m2, staff.PermissionStaffPermissionsManage)
		giveDirectPermission(t, ctx, client, m2, "staff.update")

		count, err := repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		txLock, err := client.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = txLock.Rollback(ctx) }()

		var lockedID uuid.UUID
		err = txLock.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner' FOR UPDATE`).Scan(&lockedID)
		require.NoError(t, err)

		var wg sync.WaitGroup
		var errPreset, errBlock error

		wg.Add(2)
		go func() {
			defer wg.Done()
			errPreset = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
				TargetUserID: m1,
				NewRoleCode:  "support",
				ActorUserID:  m1,
			})
		}()
		go func() {
			defer wg.Done()
			errBlock = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
				TargetUserID: m2,
				NewStatus:    string(staff.StatusBlocked),
				ActorUserID:  actorOwnerID,
			})
		}()

		require.Eventually(t, func() bool {
			var waitingCount int
			_ = client.Pool.QueryRow(ctx, `
				SELECT count(*) FROM pg_stat_activity
				WHERE wait_event_type = 'Lock'
				  AND state = 'active'
				  AND query LIKE '%SELECT id FROM staff_roles WHERE code = ''owner'' FOR UPDATE%'
				  AND pid != pg_backend_pid()
			`).Scan(&waitingCount)
			return waitingCount >= 1
		}, 3*time.Second, 10*time.Millisecond)

		err = txLock.Rollback(ctx)
		require.NoError(t, err)

		wg.Wait()

		successCount := 0
		failCount := 0
		if errPreset == nil {
			successCount++
		} else if errors.Is(errPreset, staff.ErrCannotRemoveLastPermissionManager) {
			failCount++
		}
		if errBlock == nil {
			successCount++
		} else if errors.Is(errBlock, staff.ErrCannotRemoveLastPermissionManager) {
			failCount++
		}

		require.Equal(t, 1, successCount, "exactly one operation must succeed")
		require.Equal(t, 1, failCount, "exactly one operation must fail with ErrCannotRemoveLastPermissionManager")

		remaining, err := repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, remaining, "exactly 1 active manager must survive")
	})

	t.Run("C_Preset_Preset_Serialized", func(t *testing.T) {
		m1 := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, m1, staff.PermissionStaffPermissionsManage)
		giveDirectPermission(t, ctx, client, m1, "staff.update")

		m2 := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, m2, staff.PermissionStaffPermissionsManage)
		giveDirectPermission(t, ctx, client, m2, "staff.update")

		count, err := repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		txLock, err := client.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = txLock.Rollback(ctx) }()

		var lockedID uuid.UUID
		err = txLock.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner' FOR UPDATE`).Scan(&lockedID)
		require.NoError(t, err)

		var wg sync.WaitGroup
		var errP1, errP2 error

		wg.Add(2)
		go func() {
			defer wg.Done()
			errP1 = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
				TargetUserID: m1,
				NewRoleCode:  "support",
				ActorUserID:  m1,
			})
		}()
		go func() {
			defer wg.Done()
			errP2 = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
				TargetUserID: m2,
				NewRoleCode:  "support",
				ActorUserID:  m2,
			})
		}()

		require.Eventually(t, func() bool {
			var waitingCount int
			_ = client.Pool.QueryRow(ctx, `
				SELECT count(*) FROM pg_stat_activity
				WHERE wait_event_type = 'Lock'
				  AND state = 'active'
				  AND query LIKE '%SELECT id FROM staff_roles WHERE code = ''owner'' FOR UPDATE%'
				  AND pid != pg_backend_pid()
			`).Scan(&waitingCount)
			return waitingCount >= 1
		}, 3*time.Second, 10*time.Millisecond)

		err = txLock.Rollback(ctx)
		require.NoError(t, err)

		wg.Wait()

		successCount := 0
		failCount := 0
		if errP1 == nil {
			successCount++
		} else if errors.Is(errP1, staff.ErrCannotRemoveLastPermissionManager) {
			failCount++
		}
		if errP2 == nil {
			successCount++
		} else if errors.Is(errP2, staff.ErrCannotRemoveLastPermissionManager) {
			failCount++
		}

		require.Equal(t, 1, successCount, "exactly one demotion must succeed")
		require.Equal(t, 1, failCount, "exactly one demotion must fail with ErrCannotRemoveLastPermissionManager")

		remaining, err := repo.CountActivePermissionManagers(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, remaining, "exactly 1 active manager must survive")
	})
}

// TestEMP1B2_UpdateRolePrivilegeEscalation verifies that applying a role preset requires direct staff.permissions.manage.
func TestEMP1B2_UpdateRolePrivilegeEscalation(t *testing.T) {
	ctx, client, svc, repo := setupEMP1B2Harness(t)

	// 8. Actor has staff.update only (direct YES), staff.permissions.manage (direct NO)
	t.Run("8A_StaffUpdateOnly_CannotApplyCoOwnerPreset", func(t *testing.T) {
		actorID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, actorID, "staff.update")
		removeDirectPermission(t, ctx, client, actorID, staff.PermissionStaffPermissionsManage)

		// Target: ordinary staff (support)
		targetID := createTestStaff(t, ctx, client, svc, "support")
		targetMemberBefore, targetRoleBefore, err := repo.GetStaffMemberByUserID(ctx, targetID)
		require.NoError(t, err)
		targetPermsBefore, err := repo.GetMemberPermissions(ctx, targetID)
		require.NoError(t, err)

		// Attempt to apply co_owner preset
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: targetID,
			NewRoleCode:  "co_owner",
			ActorUserID:  actorID,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrPermissionManagementForbidden, "must reject actor lacking staff.permissions.manage")

		// Assert target unchanged
		targetMemberAfter, targetRoleAfter, err := repo.GetStaffMemberByUserID(ctx, targetID)
		require.NoError(t, err)
		require.Equal(t, targetRoleBefore.ID, targetRoleAfter.ID, "target role must not change")
		require.Equal(t, targetMemberBefore.StaffRoleID, targetMemberAfter.StaffRoleID)

		targetPermsAfter, err := repo.GetMemberPermissions(ctx, targetID)
		require.NoError(t, err)
		require.ElementsMatch(t, targetPermsBefore, targetPermsAfter, "target permissions must remain unchanged")

		// Specifically assert target does NOT gain staff.permissions.manage
		hasManage, err := repo.HasMemberPermission(ctx, targetID, staff.PermissionStaffPermissionsManage)
		require.NoError(t, err)
		require.False(t, hasManage, "target must not gain staff.permissions.manage via exploit")
	})

	// 9. Actor is an active owner by role, but direct staff.permissions.manage has been removed
	t.Run("9A_OwnerWithoutManage_CannotApplyPermissionPreset", func(t *testing.T) {
		ownerActorID := createTestStaff(t, ctx, client, svc, "owner")
		giveDirectPermission(t, ctx, client, ownerActorID, "staff.update")
		removeDirectPermission(t, ctx, client, ownerActorID, staff.PermissionStaffPermissionsManage)

		targetID := createTestStaff(t, ctx, client, svc, "support")

		err := svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: targetID,
			NewRoleCode:  "admin",
			ActorUserID:  ownerActorID,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrPermissionManagementForbidden, "owner without manage capability must be rejected")
	})

	// 10. Valid manager (active, direct staff.update, direct staff.permissions.manage) can apply preset
	t.Run("10A_ValidManager_CanApplyPreset", func(t *testing.T) {
		managerID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, managerID, "staff.update")
		giveDirectPermission(t, ctx, client, managerID, staff.PermissionStaffPermissionsManage)

		targetID := createTestStaff(t, ctx, client, svc, "support")

		err := svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: targetID,
			NewRoleCode:  "moderator",
			ActorUserID:  managerID,
		})
		require.NoError(t, err)

		// Assert target role updated and direct permissions match moderator role preset
		_, newRole, err := repo.GetStaffMemberByUserID(ctx, targetID)
		require.NoError(t, err)
		require.Equal(t, "moderator", newRole.Code)

		targetPerms, err := repo.GetMemberPermissions(ctx, targetID)
		require.NoError(t, err)

		var rolePerms []string
		rows, err := client.Pool.Query(ctx, `SELECT permission FROM staff_role_permissions WHERE role_id = $1`, newRole.ID)
		require.NoError(t, err)
		defer rows.Close()
		for rows.Next() {
			var p string
			require.NoError(t, rows.Scan(&p))
			rolePerms = append(rolePerms, p)
		}
		require.ElementsMatch(t, rolePerms, targetPerms, "target permissions must exactly match role preset")
	})
}

// TestEMP1B2_CreateStaffPrivilegeEscalation verifies create flow privilege escalation protections.
func TestEMP1B2_CreateStaffPrivilegeEscalation(t *testing.T) {
	ctx, client, svc, repo := setupEMP1B2Harness(t)

	// 11A. Actor with staff.create only can create employee with ordinary preset (support)
	t.Run("11A_StaffCreateOnly_OrdinaryPresetAllowed", func(t *testing.T) {
		actorID := createTestStaff(t, ctx, client, svc, "moderator")
		giveDirectPermission(t, ctx, client, actorID, "staff.create")
		removeDirectPermission(t, ctx, client, actorID, staff.PermissionStaffPermissionsManage)

		newEmail := fmt.Sprintf("support_valid_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "New Support",
			Email:             newEmail,
			RoleCode:          "support",
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   actorID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM staff_member_permissions WHERE user_id = $1`, res.UserID)
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM staff_members WHERE user_id = $1`, res.UserID)
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		m, r, err := repo.GetStaffMemberByUserID(ctx, res.UserID)
		require.NoError(t, err)
		require.Equal(t, "support", r.Code)
		require.Equal(t, string(staff.StatusActive), m.Status)
	})

	// 11B. Actor with staff.create only CANNOT create employee with manage-bearing preset (co_owner)
	t.Run("11B_StaffCreateOnly_ManagementPresetRejected_NoPartialRows", func(t *testing.T) {
		actorID := createTestStaff(t, ctx, client, svc, "moderator")
		giveDirectPermission(t, ctx, client, actorID, "staff.create")
		removeDirectPermission(t, ctx, client, actorID, staff.PermissionStaffPermissionsManage)

		newEmail := fmt.Sprintf("co_owner_attempt_%s@test.com", uuid.New().String()[:8])
		_, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Unauthorized CoOwner",
			Email:             newEmail,
			RoleCode:          "co_owner",
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   actorID,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrPermissionManagementForbidden)

		// Assert no partial rows survive
		var userCount int
		err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, newEmail).Scan(&userCount)
		require.NoError(t, err)
		require.Equal(t, 0, userCount, "no partial user row should exist")

		var staffCount int
		err = client.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM staff_members sm
			JOIN users u ON u.id = sm.user_id
			WHERE u.email = $1
		`, newEmail).Scan(&staffCount)
		require.NoError(t, err)
		require.Equal(t, 0, staffCount, "no partial staff_members row should exist")

		var permCount int
		err = client.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM staff_member_permissions smp
			JOIN users u ON u.id = smp.user_id
			WHERE u.email = $1
		`, newEmail).Scan(&permCount)
		require.NoError(t, err)
		require.Equal(t, 0, permCount, "no partial staff_member_permissions row should exist")
	})

	// 11C. Active non-owner actor with staff.create AND staff.permissions.manage can create co_owner
	t.Run("11C_ManagerCanCreateManagementPreset_CoOwner", func(t *testing.T) {
		actorID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, actorID, "staff.create")
		giveDirectPermission(t, ctx, client, actorID, staff.PermissionStaffPermissionsManage)

		newEmail := fmt.Sprintf("valid_co_owner_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Valid CoOwner",
			Email:             newEmail,
			RoleCode:          "co_owner",
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   actorID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM staff_member_permissions WHERE user_id = $1`, res.UserID)
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM staff_members WHERE user_id = $1`, res.UserID)
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		m, r, err := repo.GetStaffMemberByUserID(ctx, res.UserID)
		require.NoError(t, err)
		require.Equal(t, "co_owner", r.Code)
		require.Equal(t, string(staff.StatusActive), m.Status)

		hasManage, err := repo.HasMemberPermission(ctx, res.UserID, staff.PermissionStaffPermissionsManage)
		require.NoError(t, err)
		require.True(t, hasManage, "co_owner must receive staff.permissions.manage")
	})

	// 11D. Active non-owner manager CANNOT create owner role -> rejected with ErrCannotPromoteToOwner
	t.Run("11D_NonOwnerManager_CannotCreateOwner", func(t *testing.T) {
		actorID := createTestStaff(t, ctx, client, svc, "admin")
		giveDirectPermission(t, ctx, client, actorID, "staff.create")
		giveDirectPermission(t, ctx, client, actorID, staff.PermissionStaffPermissionsManage)

		newEmail := fmt.Sprintf("non_owner_create_owner_%s@test.com", uuid.New().String()[:8])
		_, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Exploit Owner",
			Email:             newEmail,
			RoleCode:          "owner",
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   actorID,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrCannotPromoteToOwner, "non-owner manager must not be able to create owner")

		// Assert 0 partial rows exist
		var userCount int
		err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, newEmail).Scan(&userCount)
		require.NoError(t, err)
		require.Equal(t, 0, userCount, "no partial user row should exist")

		var staffCount int
		err = client.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM staff_members sm
			JOIN users u ON u.id = sm.user_id
			WHERE u.email = $1
		`, newEmail).Scan(&staffCount)
		require.NoError(t, err)
		require.Equal(t, 0, staffCount, "no partial staff_members row should exist")

		var permCount int
		err = client.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM staff_member_permissions smp
			JOIN users u ON u.id = smp.user_id
			WHERE u.email = $1
		`, newEmail).Scan(&permCount)
		require.NoError(t, err)
		require.Equal(t, 0, permCount, "no partial staff_member_permissions row should exist")
	})

	// 11E. Active owner actor with staff.permissions.manage CAN create owner role
	t.Run("11E_OwnerManager_CanCreateOwner", func(t *testing.T) {
		ownerActorID := createTestStaff(t, ctx, client, svc, "owner")
		giveDirectPermission(t, ctx, client, ownerActorID, "staff.create")
		giveDirectPermission(t, ctx, client, ownerActorID, staff.PermissionStaffPermissionsManage)

		newEmail := fmt.Sprintf("valid_new_owner_%s@test.com", uuid.New().String()[:8])
		res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Valid New Owner",
			Email:             newEmail,
			RoleCode:          "owner",
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerActorID,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM staff_member_permissions WHERE user_id = $1`, res.UserID)
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM staff_members WHERE user_id = $1`, res.UserID)
			_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
		})

		m, r, err := repo.GetStaffMemberByUserID(ctx, res.UserID)
		require.NoError(t, err)
		require.Equal(t, "owner", r.Code)
		require.Equal(t, string(staff.StatusActive), m.Status)

		hasManage, err := repo.HasMemberPermission(ctx, res.UserID, staff.PermissionStaffPermissionsManage)
		require.NoError(t, err)
		require.True(t, hasManage, "created owner must receive staff.permissions.manage")

		// Direct permissions match owner role preset exactly
		var rolePerms []string
		rows, err := client.Pool.Query(ctx, `SELECT permission FROM staff_role_permissions WHERE role_id = $1`, r.ID)
		require.NoError(t, err)
		defer rows.Close()
		for rows.Next() {
			var p string
			require.NoError(t, rows.Scan(&p))
			rolePerms = append(rolePerms, p)
		}
		targetPerms, err := repo.GetMemberPermissions(ctx, res.UserID)
		require.NoError(t, err)
		require.ElementsMatch(t, rolePerms, targetPerms)
	})

	// 11F. Nil actor attempting to create management-bearing presets (co_owner / owner) fails closed
	t.Run("11F_NilActor_ManagementPresetRejected_NoPartialRows", func(t *testing.T) {
		// Attempt co_owner with uuid.Nil
		coOwnerEmail := fmt.Sprintf("nil_actor_co_owner_%s@test.com", uuid.New().String()[:8])
		_, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Nil Actor CoOwner",
			Email:             coOwnerEmail,
			RoleCode:          "co_owner",
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   uuid.Nil,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrPermissionManagementForbidden, "nil actor creating co_owner must fail closed")

		var userCount int
		err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, coOwnerEmail).Scan(&userCount)
		require.NoError(t, err)
		require.Equal(t, 0, userCount, "no partial user row for co_owner")

		// Attempt owner with uuid.Nil
		ownerEmail := fmt.Sprintf("nil_actor_owner_%s@test.com", uuid.New().String()[:8])
		_, err = svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "Nil Actor Owner",
			Email:             ownerEmail,
			RoleCode:          "owner",
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   uuid.Nil,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrPermissionManagementForbidden, "nil actor creating owner must fail closed")

		err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, ownerEmail).Scan(&userCount)
		require.NoError(t, err)
		require.Equal(t, 0, userCount, "no partial user row for owner")
	})
}

// TestEMP1B2_TOCTOU_UpdateRoleActorManageRevoked (Requirement 12)
// Proves that if an actor's manage capability is revoked concurrently while waiting on serialization,
// the unblocked UpdateStaffRole revalidates actor under the lock and rejects the stale operation.
func TestEMP1B2_TOCTOU_UpdateRoleActorManageRevoked(t *testing.T) {
	ctx, client, svc, repo := setupEMP1B2Harness(t)

	// Actor A: active, staff.update, staff.permissions.manage
	actorID := createTestStaff(t, ctx, client, svc, "admin")
	giveDirectPermission(t, ctx, client, actorID, "staff.update")
	giveDirectPermission(t, ctx, client, actorID, staff.PermissionStaffPermissionsManage)

	// Target B: ordinary staff (support)
	targetID := createTestStaff(t, ctx, client, svc, "support")
	targetMemberBefore, targetRoleBefore, err := repo.GetStaffMemberByUserID(ctx, targetID)
	require.NoError(t, err)
	targetPermsBefore, err := repo.GetMemberPermissions(ctx, targetID)
	require.NoError(t, err)

	// 1. Acquire owner role FOR UPDATE lock in dedicated test transaction
	txLock, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txLock.Rollback(ctx) }()

	var lockedID uuid.UUID
	err = txLock.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner' FOR UPDATE`).Scan(&lockedID)
	require.NoError(t, err)

	// 2. R1: Actor A initiates UpdateStaffRole to co_owner in a separate goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	var errR1 error
	go func() {
		defer wg.Done()
		errR1 = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: targetID,
			NewRoleCode:  "co_owner",
			ActorUserID:  actorID,
		})
	}()

	// 3. Verify R1 blocks waiting on owner role lock
	require.Eventually(t, func() bool {
		var waitingCount int
		_ = client.Pool.QueryRow(ctx, `
			SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock'
			  AND state = 'active'
			  AND query LIKE '%SELECT id FROM staff_roles WHERE code = ''owner'' FOR UPDATE%'
			  AND pid != pg_backend_pid()
		`).Scan(&waitingCount)
		return waitingCount >= 1
	}, 3*time.Second, 10*time.Millisecond, "R1 must block waiting on owner role lock")

	// 4. R2: Revoke A's direct staff.permissions.manage while R1 is waiting
	removeDirectPermission(t, ctx, client, actorID, staff.PermissionStaffPermissionsManage)

	// 5. Release the serialization lock
	err = txLock.Rollback(ctx)
	require.NoError(t, err)

	wg.Wait()

	// 6. Assert R1 rejected due to stale capability
	require.Error(t, errR1)
	require.ErrorIs(t, errR1, staff.ErrPermissionManagementForbidden, "R1 must be rejected with ErrPermissionManagementForbidden")

	// 7. Assert Target B remains unchanged
	targetMemberAfter, targetRoleAfter, err := repo.GetStaffMemberByUserID(ctx, targetID)
	require.NoError(t, err)
	require.Equal(t, targetRoleBefore.ID, targetRoleAfter.ID)
	require.Equal(t, targetMemberBefore.StaffRoleID, targetMemberAfter.StaffRoleID)

	targetPermsAfter, err := repo.GetMemberPermissions(ctx, targetID)
	require.NoError(t, err)
	require.ElementsMatch(t, targetPermsBefore, targetPermsAfter)
}

// TestEMP1B2_TOCTOU_CreateStaffActorManageRevoked (Requirement 13)
// Proves that if an actor's manage capability is revoked concurrently while waiting on serialization,
// the unblocked CreateStaffMember revalidates actor under the lock and rejects creation without partial rows.
func TestEMP1B2_TOCTOU_CreateStaffActorManageRevoked(t *testing.T) {
	ctx, client, svc, _ := setupEMP1B2Harness(t)

	// Actor A: active, staff.create, staff.permissions.manage
	actorID := createTestStaff(t, ctx, client, svc, "admin")
	giveDirectPermission(t, ctx, client, actorID, "staff.create")
	giveDirectPermission(t, ctx, client, actorID, staff.PermissionStaffPermissionsManage)

	newEmail := fmt.Sprintf("toctou_co_owner_%s@test.com", uuid.New().String()[:8])

	// 1. Acquire owner role FOR UPDATE lock in dedicated test transaction
	txLock, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txLock.Rollback(ctx) }()

	var lockedID uuid.UUID
	err = txLock.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner' FOR UPDATE`).Scan(&lockedID)
	require.NoError(t, err)

	// 2. R1: Actor A initiates CreateStaffMember for co_owner in a separate goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	var errR1 error
	go func() {
		defer wg.Done()
		_, errR1 = svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "TOCTOU CoOwner",
			Email:             newEmail,
			RoleCode:          "co_owner",
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   actorID,
		})
	}()

	// 3. Verify R1 blocks waiting on owner role lock
	require.Eventually(t, func() bool {
		var waitingCount int
		_ = client.Pool.QueryRow(ctx, `
			SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock'
			  AND state = 'active'
			  AND query LIKE '%SELECT id FROM staff_roles WHERE code = ''owner'' FOR UPDATE%'
			  AND pid != pg_backend_pid()
		`).Scan(&waitingCount)
		return waitingCount >= 1
	}, 3*time.Second, 10*time.Millisecond, "R1 must block waiting on owner role lock")

	// 4. R2: Revoke A's direct staff.permissions.manage while R1 is waiting
	removeDirectPermission(t, ctx, client, actorID, staff.PermissionStaffPermissionsManage)

	// 5. Release the serialization lock
	err = txLock.Rollback(ctx)
	require.NoError(t, err)

	wg.Wait()

	// 6. Assert R1 rejected
	require.Error(t, errR1)
	require.ErrorIs(t, errR1, staff.ErrPermissionManagementForbidden, "R1 must be rejected with ErrPermissionManagementForbidden")

	// 7. Assert no partial rows exist
	var count int
	err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, newEmail).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "no partial user row should exist")
}

// TestEMP1B2_TOCTOU_CreateOwnerActorOwnerRoleDemoted
// Proves that if an owner actor's owner role is demoted concurrently while waiting on serialization,
// the unblocked CreateStaffMember revalidates the actor's role under the lock and rejects owner creation with ErrCannotPromoteToOwner.
func TestEMP1B2_TOCTOU_CreateOwnerActorOwnerRoleDemoted(t *testing.T) {
	ctx, client, svc, _ := setupEMP1B2Harness(t)

	// Actor A: active owner with direct manage and create capabilities
	ownerActorID := createTestStaff(t, ctx, client, svc, "owner")
	giveDirectPermission(t, ctx, client, ownerActorID, "staff.create")
	giveDirectPermission(t, ctx, client, ownerActorID, staff.PermissionStaffPermissionsManage)

	candidateEmail := fmt.Sprintf("toctou_new_owner_%s@test.com", uuid.New().String()[:8])

	// 1. Acquire owner role FOR UPDATE lock in dedicated test transaction
	txLock, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txLock.Rollback(ctx) }()

	var lockedID uuid.UUID
	err = txLock.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner' FOR UPDATE`).Scan(&lockedID)
	require.NoError(t, err)

	// 2. R1: Actor A initiates CreateStaffMember for owner in a separate goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	var errR1 error
	go func() {
		defer wg.Done()
		_, errR1 = svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
			Name:              "TOCTOU Owner Candidate",
			Email:             candidateEmail,
			RoleCode:          "owner",
			TemporaryPassword: "temporaryPassword123!",
			CreatedByUserID:   ownerActorID,
		})
	}()

	// 3. Verify R1 blocks waiting on owner role lock
	require.Eventually(t, func() bool {
		var waitingCount int
		_ = client.Pool.QueryRow(ctx, `
			SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock'
			  AND state = 'active'
			  AND query LIKE '%SELECT id FROM staff_roles WHERE code = ''owner'' FOR UPDATE%'
			  AND pid != pg_backend_pid()
		`).Scan(&waitingCount)
		return waitingCount >= 1
	}, 3*time.Second, 10*time.Millisecond, "R1 must block waiting on owner role lock")

	// 4. R2: Demote Actor A from owner to admin while R1 is waiting
	var adminRoleID uuid.UUID
	err = client.Pool.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'admin'`).Scan(&adminRoleID)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `UPDATE staff_members SET staff_role_id = $1 WHERE user_id = $2`, adminRoleID, ownerActorID)
	require.NoError(t, err)

	// 5. Release the serialization lock
	err = txLock.Rollback(ctx)
	require.NoError(t, err)

	wg.Wait()

	// 6. Assert R1 rejected because actor is no longer an owner
	require.Error(t, errR1)
	require.ErrorIs(t, errR1, staff.ErrCannotPromoteToOwner, "R1 must be rejected with ErrCannotPromoteToOwner")

	// 7. Assert no partial rows exist for candidate owner
	var userCount int
	err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, candidateEmail).Scan(&userCount)
	require.NoError(t, err)
	require.Equal(t, 0, userCount, "no partial user row should exist for candidate owner")

	var staffCount int
	err = client.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM staff_members sm
		JOIN users u ON u.id = sm.user_id
		WHERE u.email = $1
	`, candidateEmail).Scan(&staffCount)
	require.NoError(t, err)
	require.Equal(t, 0, staffCount, "no partial staff_members row should exist for candidate owner")
}
