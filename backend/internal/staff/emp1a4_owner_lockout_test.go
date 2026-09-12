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

func setupOwnerLockoutHarness(t *testing.T) (context.Context, *postgres.Client, *staff.Service, *staff.Repository, uuid.UUID, uuid.UUID) {
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

	var ownerRoleID, moderatorRoleID uuid.UUID
	err = pgClient.Pool.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner'`).Scan(&ownerRoleID)
	require.NoError(t, err)
	err = pgClient.Pool.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'moderator'`).Scan(&moderatorRoleID)
	require.NoError(t, err)

	return ctx, pgClient, svc, staffRepo, ownerRoleID, moderatorRoleID
}

func createTestOwner(t *testing.T, ctx context.Context, client *postgres.Client, svc *staff.Service, ownerRoleID uuid.UUID) uuid.UUID {
	t.Helper()
	email := fmt.Sprintf("owner_%s@test.com", uuid.New().String()[:8])
	res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
		Name:              "Owner Member",
		Email:             email,
		RoleCode:          "owner",
		TemporaryPassword: "temporaryPassword123!",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
	})
	return res.UserID
}

func createTestNonOwner(t *testing.T, ctx context.Context, client *postgres.Client, svc *staff.Service, roleCode string) uuid.UUID {
	t.Helper()
	email := fmt.Sprintf("staff_%s@test.com", uuid.New().String()[:8])
	res, err := svc.CreateStaffMember(ctx, staff.CreateStaffMemberInput{
		Name:              "Non Owner Member",
		Email:             email,
		RoleCode:          roleCode,
		TemporaryPassword: "temporaryPassword123!",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = client.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, res.UserID)
	})
	return res.UserID
}

// TestEMP1A4_SequentialOwnerSafety verifies sequential owner lockout invariants.
func TestEMP1A4_SequentialOwnerSafety(t *testing.T) {
	// A. SINGLE OWNER SELF-DEMOTION
	t.Run("A_SingleOwnerSelfDemotionRejected", func(t *testing.T) {
		ctx, client, svc, repo, ownerRoleID, _ := setupOwnerLockoutHarness(t)
		_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
		require.NoError(t, err)

		ownerID := createTestOwner(t, ctx, client, svc, ownerRoleID)

		countBefore, err := repo.CountActiveOwners(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, countBefore)

		permsBefore, err := repo.GetMemberPermissions(ctx, ownerID)
		require.NoError(t, err)

		// Owner attempts self-demotion to moderator
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: ownerID,
			NewRoleCode:  "moderator",
			ActorUserID:  ownerID, // self-demotion
		})
		require.Error(t, err)
		require.True(t, errors.Is(err, staff.ErrCannotRemoveLastOwner), "expected ErrCannotRemoveLastOwner, got %v", err)

		// Verify Owner role still owner
		memberAfter, roleAfter, err := repo.GetStaffMemberByUserID(ctx, ownerID)
		require.NoError(t, err)
		require.Equal(t, ownerRoleID, memberAfter.StaffRoleID)
		require.Equal(t, "owner", roleAfter.Code)

		// Direct permissions unchanged (Rollback capability consistency)
		permsAfter, err := repo.GetMemberPermissions(ctx, ownerID)
		require.NoError(t, err)
		require.ElementsMatch(t, permsBefore, permsAfter, "direct permissions must not be replaced when demotion fails")

		// Active owner count still 1
		countAfter, err := repo.CountActiveOwners(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, countAfter)
	})

	// B. SINGLE OWNER DEMOTED BY ANOTHER ATTEMPT
	t.Run("B_SingleOwnerDemotedByAnotherAttemptRejected", func(t *testing.T) {
		ctx, client, svc, repo, ownerRoleID, _ := setupOwnerLockoutHarness(t)
		_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
		require.NoError(t, err)

		ownerID := createTestOwner(t, ctx, client, svc, ownerRoleID)

		countBefore, err := repo.CountActiveOwners(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, countBefore)

		// Even if actor claims owner privileges, cannot remove the last active owner
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: ownerID,
			NewRoleCode:  "moderator",
			ActorUserID:  ownerID,
		})
		require.ErrorIs(t, err, staff.ErrCannotRemoveLastOwner)
	})

	// C. TWO ACTIVE OWNERS — DEMOTE ONE (ALLOWED)
	t.Run("C_TwoActiveOwnersDemoteOneSucceeds", func(t *testing.T) {
		ctx, client, svc, repo, ownerRoleID, moderatorRoleID := setupOwnerLockoutHarness(t)
		_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
		require.NoError(t, err)

		owner1ID := createTestOwner(t, ctx, client, svc, ownerRoleID)
		owner2ID := createTestOwner(t, ctx, client, svc, ownerRoleID)

		countBefore, err := repo.CountActiveOwners(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, countBefore)

		// Owner 1 demotes Owner 2 to moderator
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: owner2ID,
			NewRoleCode:  "moderator",
			ActorUserID:  owner1ID,
		})
		require.NoError(t, err, "demoting one owner when two exist must succeed")

		member2, role2, err := repo.GetStaffMemberByUserID(ctx, owner2ID)
		require.NoError(t, err)
		require.Equal(t, moderatorRoleID, member2.StaffRoleID)
		require.Equal(t, "moderator", role2.Code)

		// Final active owner count = 1
		countAfter, err := repo.CountActiveOwners(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, countAfter)
	})

	// D. LAST OWNER BLOCK (REJECTED)
	t.Run("D_LastOwnerBlockRejected", func(t *testing.T) {
		ctx, client, svc, repo, ownerRoleID, _ := setupOwnerLockoutHarness(t)
		_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
		require.NoError(t, err)

		ownerID := createTestOwner(t, ctx, client, svc, ownerRoleID)

		countBefore, err := repo.CountActiveOwners(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, countBefore)

		// Attempt to block last active owner
		err = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: ownerID,
			NewStatus:    "blocked",
			ActorUserID:  ownerID,
		})
		require.Error(t, err)
		require.True(t, errors.Is(err, staff.ErrCannotBlockLastOwner), "expected ErrCannotBlockLastOwner, got %v", err)

		// Status remains active
		member, _, err := repo.GetStaffMemberByUserID(ctx, ownerID)
		require.NoError(t, err)
		require.Equal(t, "active", member.Status)

		countAfter, err := repo.CountActiveOwners(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, countAfter)
	})

	// E. TWO OWNERS — BLOCK ONE (ALLOWED)
	t.Run("E_TwoOwnersBlockOneSucceeds", func(t *testing.T) {
		ctx, client, svc, repo, ownerRoleID, _ := setupOwnerLockoutHarness(t)
		_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
		require.NoError(t, err)

		owner1ID := createTestOwner(t, ctx, client, svc, ownerRoleID)
		owner2ID := createTestOwner(t, ctx, client, svc, ownerRoleID)

		countBefore, err := repo.CountActiveOwners(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, countBefore)

		// Block Owner 2
		err = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: owner2ID,
			NewStatus:    "blocked",
			ActorUserID:  owner1ID,
		})
		require.NoError(t, err)

		member2, _, err := repo.GetStaffMemberByUserID(ctx, owner2ID)
		require.NoError(t, err)
		require.Equal(t, "blocked", member2.Status)

		// 1 active owner remains
		countAfter, err := repo.CountActiveOwners(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, countAfter)
	})

	// F. NON-OWNER STATUS UPDATE (UNAFFECTED)
	t.Run("F_NonOwnerStatusUpdateUnaffected", func(t *testing.T) {
		ctx, client, svc, repo, ownerRoleID, _ := setupOwnerLockoutHarness(t)
		_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
		require.NoError(t, err)

		ownerID := createTestOwner(t, ctx, client, svc, ownerRoleID)
		modID := createTestNonOwner(t, ctx, client, svc, "moderator")

		// Block non-owner
		err = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: modID,
			NewStatus:    "blocked",
			ActorUserID:  ownerID,
		})
		require.NoError(t, err, "blocking non-owner should not be restricted by owner safety check")

		modMember, _, err := repo.GetStaffMemberByUserID(ctx, modID)
		require.NoError(t, err)
		require.Equal(t, "blocked", modMember.Status)
	})

	// G. BLOCKED OWNER ACTOR CANNOT PERFORM OWNER ACTIONS
	t.Run("G_BlockedOwnerActorCannotPerformOwnerActions", func(t *testing.T) {
		ctx, client, svc, repo, ownerRoleID, _ := setupOwnerLockoutHarness(t)
		_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
		require.NoError(t, err)

		ownerAID := createTestOwner(t, ctx, client, svc, ownerRoleID)
		ownerBID := createTestOwner(t, ctx, client, svc, ownerRoleID)
		modID := createTestNonOwner(t, ctx, client, svc, "moderator")

		// Block Owner A directly in database
		_, err = client.Pool.Exec(ctx, `UPDATE staff_members SET status = 'blocked' WHERE user_id = $1`, ownerAID)
		require.NoError(t, err)

		// 1. Blocked Owner A attempts to demote Owner B -> ErrCannotDemoteOwner
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: ownerBID,
			NewRoleCode:  "moderator",
			ActorUserID:  ownerAID,
		})
		require.ErrorIs(t, err, staff.ErrCannotDemoteOwner, "blocked owner actor must not demote owner")

		// 2. Blocked Owner A attempts to promote moderator to owner -> ErrCannotPromoteToOwner
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: modID,
			NewRoleCode:  "owner",
			ActorUserID:  ownerAID,
		})
		require.ErrorIs(t, err, staff.ErrCannotPromoteToOwner, "blocked owner actor must not promote to owner")

		// 3. Blocked Owner A attempts to block Owner B -> ErrCannotDemoteOwner
		err = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: ownerBID,
			NewStatus:    "blocked",
			ActorUserID:  ownerAID,
		})
		require.ErrorIs(t, err, staff.ErrCannotDemoteOwner, "blocked owner actor must not block owner")

		// Owner B remains active owner
		memberB, roleB, err := repo.GetStaffMemberByUserID(ctx, ownerBID)
		require.NoError(t, err)
		require.Equal(t, "active", memberB.Status)
		require.Equal(t, "owner", roleB.Code)
	})

	// H. ARCHIVED OWNER ACTOR CANNOT PERFORM OWNER ACTIONS
	t.Run("H_ArchivedOwnerActorCannotPerformOwnerActions", func(t *testing.T) {
		ctx, client, svc, repo, ownerRoleID, _ := setupOwnerLockoutHarness(t)
		_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
		require.NoError(t, err)

		ownerAID := createTestOwner(t, ctx, client, svc, ownerRoleID)
		ownerBID := createTestOwner(t, ctx, client, svc, ownerRoleID)
		modID := createTestNonOwner(t, ctx, client, svc, "moderator")

		// Archive Owner A directly in database
		_, err = client.Pool.Exec(ctx, `UPDATE staff_members SET status = 'archived' WHERE user_id = $1`, ownerAID)
		require.NoError(t, err)

		// 1. Archived Owner A attempts to demote Owner B -> ErrCannotDemoteOwner
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: ownerBID,
			NewRoleCode:  "moderator",
			ActorUserID:  ownerAID,
		})
		require.ErrorIs(t, err, staff.ErrCannotDemoteOwner, "archived owner actor must not demote owner")

		// 2. Archived Owner A attempts to promote moderator to owner -> ErrCannotPromoteToOwner
		err = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: modID,
			NewRoleCode:  "owner",
			ActorUserID:  ownerAID,
		})
		require.ErrorIs(t, err, staff.ErrCannotPromoteToOwner, "archived owner actor must not promote to owner")

		// 3. Archived Owner A attempts to block Owner B -> ErrCannotDemoteOwner
		err = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: ownerBID,
			NewStatus:    "blocked",
			ActorUserID:  ownerAID,
		})
		require.ErrorIs(t, err, staff.ErrCannotDemoteOwner, "archived owner actor must not block owner")

		// Owner B remains active owner
		memberB, roleB, err := repo.GetStaffMemberByUserID(ctx, ownerBID)
		require.NoError(t, err)
		require.Equal(t, "active", memberB.Status)
		require.Equal(t, "owner", roleB.Code)
	})
}

// TestEMP1A4_Concurrency_DemoteVsDemote verifies concurrent demotions of 2 owners cannot leave 0 owners.
func TestEMP1A4_Concurrency_DemoteVsDemote(t *testing.T) {
	ctx, client, svc, repo, ownerRoleID, _ := setupOwnerLockoutHarness(t)

	_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
	require.NoError(t, err)

	ownerAID := createTestOwner(t, ctx, client, svc, ownerRoleID)
	ownerBID := createTestOwner(t, ctx, client, svc, ownerRoleID)

	countBefore, err := repo.CountActiveOwners(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, countBefore)

	ready := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	var errA, errB error

	go func() {
		defer wg.Done()
		<-ready
		errA = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: ownerAID,
			NewRoleCode:  "moderator",
			ActorUserID:  ownerAID,
		})
	}()

	go func() {
		defer wg.Done()
		<-ready
		errB = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: ownerBID,
			NewRoleCode:  "moderator",
			ActorUserID:  ownerBID,
		})
	}()

	close(ready)
	wg.Wait()

	// Invariant: Exactly ONE operation succeeded, the other failed with ErrCannotRemoveLastOwner
	oneSuccess := (errA == nil && errB != nil) || (errA != nil && errB == nil)
	require.True(t, oneSuccess, "exactly one demotion must succeed: errA=%v, errB=%v", errA, errB)

	failedErr := errA
	if errB != nil {
		failedErr = errB
	}
	require.True(t, errors.Is(failedErr, staff.ErrCannotRemoveLastOwner), "failed demotion must return ErrCannotRemoveLastOwner, got %v", failedErr)

	// Invariant: Final active owner count is strictly 1 (NEVER 0!)
	countAfter, err := repo.CountActiveOwners(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, countAfter, "final active owner count must be 1, never 0")
}

// TestEMP1A4_Concurrency_DemoteVsBlock verifies concurrent demote and block cannot leave 0 owners.
func TestEMP1A4_Concurrency_DemoteVsBlock(t *testing.T) {
	ctx, client, svc, repo, ownerRoleID, _ := setupOwnerLockoutHarness(t)

	_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
	require.NoError(t, err)

	ownerAID := createTestOwner(t, ctx, client, svc, ownerRoleID)
	ownerBID := createTestOwner(t, ctx, client, svc, ownerRoleID)

	countBefore, err := repo.CountActiveOwners(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, countBefore)

	ready := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	var errDemote, errBlock error

	// T1: Demote Owner A
	go func() {
		defer wg.Done()
		<-ready
		errDemote = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: ownerAID,
			NewRoleCode:  "moderator",
			ActorUserID:  ownerAID,
		})
	}()

	// T2: Block Owner B
	go func() {
		defer wg.Done()
		<-ready
		errBlock = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: ownerBID,
			NewStatus:    "blocked",
			ActorUserID:  ownerBID,
		})
	}()

	close(ready)
	wg.Wait()

	// Invariant: Exactly one succeeded
	oneSuccess := (errDemote == nil && errBlock != nil) || (errDemote != nil && errBlock == nil)
	require.True(t, oneSuccess, "exactly one destructive op must succeed: errDemote=%v, errBlock=%v", errDemote, errBlock)

	// Final active owner count MUST be 1
	countAfter, err := repo.CountActiveOwners(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, countAfter, "active owners must remain 1 after demote vs block race")
}

// TestEMP1A4_Concurrency_BlockVsBlock verifies concurrent block operations cannot leave 0 owners.
func TestEMP1A4_Concurrency_BlockVsBlock(t *testing.T) {
	ctx, client, svc, repo, ownerRoleID, _ := setupOwnerLockoutHarness(t)

	_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
	require.NoError(t, err)

	ownerAID := createTestOwner(t, ctx, client, svc, ownerRoleID)
	ownerBID := createTestOwner(t, ctx, client, svc, ownerRoleID)

	countBefore, err := repo.CountActiveOwners(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, countBefore)

	ready := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	var errA, errB error

	go func() {
		defer wg.Done()
		<-ready
		errA = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: ownerAID,
			NewStatus:    "blocked",
			ActorUserID:  ownerAID,
		})
	}()

	go func() {
		defer wg.Done()
		<-ready
		errB = svc.UpdateStaffStatus(ctx, staff.UpdateStaffStatusInput{
			TargetUserID: ownerBID,
			NewStatus:    "blocked",
			ActorUserID:  ownerBID,
		})
	}()

	close(ready)
	wg.Wait()

	oneSuccess := (errA == nil && errB != nil) || (errA != nil && errB == nil)
	require.True(t, oneSuccess, "exactly one block must succeed: errA=%v, errB=%v", errA, errB)

	failedErr := errA
	if errB != nil {
		failedErr = errB
	}
	require.True(t, errors.Is(failedErr, staff.ErrCannotBlockLastOwner), "failed block must return ErrCannotBlockLastOwner, got %v", failedErr)

	countAfter, err := repo.CountActiveOwners(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, countAfter, "active owners must remain 1 after block vs block race")
}

// TestEMP1A4_Concurrency_BlockedActorRevalidation verifies that an actor concurrently blocked
// before obtaining the owner lock is revalidated and rejected under the lock.
func TestEMP1A4_Concurrency_BlockedActorRevalidation(t *testing.T) {
	ctx, client, svc, repo, ownerRoleID, _ := setupOwnerLockoutHarness(t)

	_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
	require.NoError(t, err)

	ownerAID := createTestOwner(t, ctx, client, svc, ownerRoleID)
	ownerBID := createTestOwner(t, ctx, client, svc, ownerRoleID)
	ownerCID := createTestOwner(t, ctx, client, svc, ownerRoleID)
	require.NotEqual(t, uuid.Nil, ownerCID)

	countBefore, err := repo.CountActiveOwners(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, countBefore)

	// R2 begins transaction on client.Pool and acquires owner lock first
	txR2, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txR2.Rollback(ctx) }()

	var lockedID uuid.UUID
	err = txR2.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner' FOR UPDATE`).Scan(&lockedID)
	require.NoError(t, err)

	// R2 mutates Owner A to 'blocked' under the lock
	_, err = txR2.Exec(ctx, `UPDATE staff_members SET status = 'blocked' WHERE user_id = $1`, ownerAID)
	require.NoError(t, err)

	// R1 starts concurrently: Owner A attempts to demote Owner B
	var wg sync.WaitGroup
	wg.Add(1)
	var errR1 error

	go func() {
		defer wg.Done()
		errR1 = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: ownerBID,
			NewRoleCode:  "moderator",
			ActorUserID:  ownerAID,
		})
	}()

	// Wait deterministically for R1 to arrive and block on LockOwnerRole
	require.Eventually(t, func() bool {
		var waiting bool
		_ = client.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE wait_event_type = 'Lock'
				  AND state = 'active'
				  AND query LIKE '%SELECT id FROM staff_roles WHERE code = ''owner'' FOR UPDATE%'
				  AND pid != pg_backend_pid()
			)
		`).Scan(&waiting)
		return waiting
	}, 3*time.Second, 10*time.Millisecond, "R1 must block waiting on owner role lock")

	// R2 commits, releasing lock to R1
	err = txR2.Commit(ctx)
	require.NoError(t, err)

	wg.Wait()

	// Invariant: R1 re-read Owner A under the lock, discovered A is blocked, and rejected the demotion
	require.Error(t, errR1)
	require.True(t, errors.Is(errR1, staff.ErrCannotDemoteOwner), "expected ErrCannotDemoteOwner, got %v", errR1)

	// Target B remained an active owner
	memberB, roleB, err := repo.GetStaffMemberByUserID(ctx, ownerBID)
	require.NoError(t, err)
	require.Equal(t, "owner", roleB.Code)
	require.Equal(t, "active", memberB.Status)

	// Active owners count is 2 (B and C)
	countAfter, err := repo.CountActiveOwners(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, countAfter)
}

// TestEMP1A4_Concurrency_DemotedActorRevalidation verifies that an actor concurrently demoted
// before obtaining the owner lock is revalidated and rejected under the lock.
func TestEMP1A4_Concurrency_DemotedActorRevalidation(t *testing.T) {
	ctx, client, svc, repo, ownerRoleID, moderatorRoleID := setupOwnerLockoutHarness(t)

	_, err := client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE staff_role_id = $1`, ownerRoleID)
	require.NoError(t, err)

	ownerAID := createTestOwner(t, ctx, client, svc, ownerRoleID)
	ownerBID := createTestOwner(t, ctx, client, svc, ownerRoleID)
	ownerCID := createTestOwner(t, ctx, client, svc, ownerRoleID)
	require.NotEqual(t, uuid.Nil, ownerCID)

	countBefore, err := repo.CountActiveOwners(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, countBefore)

	// R2 begins transaction on client.Pool and acquires owner lock first
	txR2, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txR2.Rollback(ctx) }()

	var lockedID uuid.UUID
	err = txR2.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner' FOR UPDATE`).Scan(&lockedID)
	require.NoError(t, err)

	// R2 demotes Owner A to moderator under the lock
	_, err = txR2.Exec(ctx, `UPDATE staff_members SET staff_role_id = $1 WHERE user_id = $2`, moderatorRoleID, ownerAID)
	require.NoError(t, err)

	// R1 starts concurrently: Owner A attempts to demote Owner B
	var wg sync.WaitGroup
	wg.Add(1)
	var errR1 error

	go func() {
		defer wg.Done()
		errR1 = svc.UpdateStaffRole(ctx, staff.UpdateStaffRoleInput{
			TargetUserID: ownerBID,
			NewRoleCode:  "moderator",
			ActorUserID:  ownerAID,
		})
	}()

	// Wait deterministically for R1 to arrive and block on LockOwnerRole
	require.Eventually(t, func() bool {
		var waiting bool
		_ = client.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE wait_event_type = 'Lock'
				  AND state = 'active'
				  AND query LIKE '%SELECT id FROM staff_roles WHERE code = ''owner'' FOR UPDATE%'
				  AND pid != pg_backend_pid()
			)
		`).Scan(&waiting)
		return waiting
	}, 3*time.Second, 10*time.Millisecond, "R1 must block waiting on owner role lock")

	// R2 commits, releasing lock to R1
	err = txR2.Commit(ctx)
	require.NoError(t, err)

	wg.Wait()

	// Invariant: R1 re-read Owner A under the lock, discovered A is no longer owner, and rejected the demotion
	require.Error(t, errR1)
	require.True(t, errors.Is(errR1, staff.ErrCannotDemoteOwner), "expected ErrCannotDemoteOwner, got %v", errR1)

	// Target B remained an active owner
	memberB, roleB, err := repo.GetStaffMemberByUserID(ctx, ownerBID)
	require.NoError(t, err)
	require.Equal(t, "owner", roleB.Code)
	require.Equal(t, "active", memberB.Status)

	// Active owners count is 2 (B and C)
	countAfter, err := repo.CountActiveOwners(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, countAfter)
}
