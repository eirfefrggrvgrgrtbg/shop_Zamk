package testutil_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func TestGrantStaffAuthorizationState_DualState(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	testutil.AssertTestDatabase(t, pool)

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	// 1. Employee A intended with permissions: "perm.a", "perm.b"
	userAID := uuid.New()
	roleAID := uuid.New()

	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Staff A', 'hash', 'admin', 'active', NOW(), NOW())
	`, userAID, "staff_a_"+userAID.String()[:8]+"@zamk.local", "7999"+userAID.String()[:7])
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO staff_roles (id, code, name, is_system)
		VALUES ($1, $2, 'Role A', false)
	`, roleAID, "role_a_"+roleAID.String()[:8])
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status)
		VALUES ($1, $2, 'active')
	`, userAID, roleAID)
	require.NoError(t, err)

	err = testutil.GrantStaffAuthorizationState(ctx, tx, userAID, roleAID, "perm.a", "perm.b")
	require.NoError(t, err)

	// Query role permissions for Role A
	rowsA, err := tx.Query(ctx, `SELECT permission FROM staff_role_permissions WHERE role_id = $1 ORDER BY permission`, roleAID)
	require.NoError(t, err)
	defer rowsA.Close()

	var rolePermsA []string
	for rowsA.Next() {
		var p string
		require.NoError(t, rowsA.Scan(&p))
		rolePermsA = append(rolePermsA, p)
	}
	require.ElementsMatch(t, []string{"perm.a", "perm.b"}, rolePermsA)

	// Query member permissions for User A
	memRowsA, err := tx.Query(ctx, `SELECT permission FROM staff_member_permissions WHERE user_id = $1 ORDER BY permission`, userAID)
	require.NoError(t, err)
	defer memRowsA.Close()

	var memberPermsA []string
	for memRowsA.Next() {
		var p string
		require.NoError(t, memRowsA.Scan(&p))
		memberPermsA = append(memberPermsA, p)
	}
	require.ElementsMatch(t, []string{"perm.a", "perm.b"}, memberPermsA)

	// 2. Employee B intended with NO permissions
	userBID := uuid.New()
	roleBID := uuid.New()

	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Staff B', 'hash', 'admin', 'active', NOW(), NOW())
	`, userBID, "staff_b_"+userBID.String()[:8]+"@zamk.local", "7999"+userBID.String()[:7])
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO staff_roles (id, code, name, is_system)
		VALUES ($1, $2, 'Role B', false)
	`, roleBID, "role_b_"+roleBID.String()[:8])
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status)
		VALUES ($1, $2, 'active')
	`, userBID, roleBID)
	require.NoError(t, err)

	err = testutil.GrantStaffAuthorizationState(ctx, tx, userBID, roleBID)
	require.NoError(t, err)

	var countRoleB int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM staff_role_permissions WHERE role_id = $1`, roleBID).Scan(&countRoleB)
	require.NoError(t, err)
	require.Equal(t, 0, countRoleB, "Employee B role representation must have 0 permissions")

	var countMemberB int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM staff_member_permissions WHERE user_id = $1`, userBID).Scan(&countMemberB)
	require.NoError(t, err)
	require.Equal(t, 0, countMemberB, "Employee B member representation must have 0 permissions")
}
