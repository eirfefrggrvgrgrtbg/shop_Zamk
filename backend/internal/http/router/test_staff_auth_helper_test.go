package router_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// grantRouterStaffPermissions dual-writes permissions to both staff_role_permissions (for current
// role-based runtime) and staff_member_permissions (for future member-based runtime).
func grantRouterStaffPermissions(ctx context.Context, db postgres.DBTX, userID uuid.UUID, roleID uuid.UUID, perms []string) error {
	for _, p := range perms {
		if p == "" {
			continue
		}
		_, err := db.Exec(ctx, `
			INSERT INTO staff_role_permissions (role_id, permission)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, roleID, p)
		if err != nil {
			return fmt.Errorf("grant staff_role_permissions: %w", err)
		}

		_, err = db.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT DO NOTHING
		`, userID, p)
		if err != nil {
			return fmt.Errorf("grant staff_member_permissions: %w", err)
		}
	}
	return nil
}

// setupRouterStaffMember creates or updates the staff role, active staff member, and dual-writes
// the granted permissions to both staff_role_permissions and staff_member_permissions.
func setupRouterStaffMember(ctx context.Context, db postgres.DBTX, userID uuid.UUID, roleID uuid.UUID, roleCode, roleName string, perms []string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO staff_roles (id, code, name)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING
	`, roleID, roleCode, roleName)
	if err != nil {
		return fmt.Errorf("insert staff_roles: %w", err)
	}

	_, err = db.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status)
		VALUES ($1, $2, 'active')
		ON CONFLICT (user_id) DO UPDATE SET staff_role_id = EXCLUDED.staff_role_id, status = 'active'
	`, userID, roleID)
	if err != nil {
		return fmt.Errorf("insert staff_members: %w", err)
	}

	return grantRouterStaffPermissions(ctx, db, userID, roleID, perms)
}

// setupRouterStaffWithPermissions generates a new random role with the given roleName,
// assigns it to the user as an active staff member, and dual-writes the permissions.
func setupRouterStaffWithPermissions(ctx context.Context, db postgres.DBTX, userID uuid.UUID, roleName string, perms []string) (uuid.UUID, error) {
	roleID := uuid.New()
	code := roleID.String()[:8]
	err := setupRouterStaffMember(ctx, db, userID, roleID, code, roleName, perms)
	if err != nil {
		return uuid.Nil, err
	}
	return roleID, nil
}

// syncRouterStaffRolePermissions copies all permissions currently associated with roleID
// into staff_member_permissions for userID, ensuring canonical role fixtures are future-compatible.
func syncRouterStaffRolePermissions(ctx context.Context, db postgres.DBTX, userID uuid.UUID, roleID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		INSERT INTO staff_member_permissions (user_id, permission, created_at)
		SELECT $1, srp.permission, NOW()
		FROM staff_role_permissions srp
		WHERE srp.role_id = $2
		ON CONFLICT DO NOTHING
	`, userID, roleID)
	if err != nil {
		return fmt.Errorf("sync staff_member_permissions from role: %w", err)
	}
	return nil
}

// TestRouterStaffAuthHelper_DualWriteInvariant verifies that the shared helper establishes
// equivalent authorization state across both staff_role_permissions and staff_member_permissions.
func TestRouterStaffAuthHelper_DualWriteInvariant(t *testing.T) {
	ctx := context.Background()
	testDBURL := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)
	defer pgClient.Close()

	testutil.AssertTestDatabase(t, pgClient.Pool)

	tx, err := pgClient.Pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	// 1. Employee with permissions
	userWithPermsID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'DualWrite Staff', 'hash', 'admin', 'active', NOW(), NOW())
	`, userWithPermsID, "dualwrite_"+userWithPermsID.String()[:8]+"@test.com", "7999"+userWithPermsID.String()[:7])
	require.NoError(t, err)

	roleID, err := setupRouterStaffWithPermissions(ctx, tx, userWithPermsID, "DualWriteRole", []string{"perm.a", "perm.b"})
	require.NoError(t, err)

	// Verify role-derived permissions == {"perm.a", "perm.b"}
	rolePermRows, err := tx.Query(ctx, `SELECT permission FROM staff_role_permissions WHERE role_id = $1 ORDER BY permission`, roleID)
	require.NoError(t, err)
	defer rolePermRows.Close()

	var rolePerms []string
	for rolePermRows.Next() {
		var p string
		require.NoError(t, rolePermRows.Scan(&p))
		rolePerms = append(rolePerms, p)
	}
	require.ElementsMatch(t, []string{"perm.a", "perm.b"}, rolePerms)

	// Verify staff_member_permissions == {"perm.a", "perm.b"}
	memberPermRows, err := tx.Query(ctx, `SELECT permission FROM staff_member_permissions WHERE user_id = $1 ORDER BY permission`, userWithPermsID)
	require.NoError(t, err)
	defer memberPermRows.Close()

	var memberPerms []string
	for memberPermRows.Next() {
		var p string
		require.NoError(t, memberPermRows.Scan(&p))
		memberPerms = append(memberPerms, p)
	}
	require.ElementsMatch(t, []string{"perm.a", "perm.b"}, memberPerms)

	// 2. Employee with NO permissions (negative auth preservation)
	userNoPermsID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'NoPerm Staff', 'hash', 'admin', 'active', NOW(), NOW())
	`, userNoPermsID, "noperm_"+userNoPermsID.String()[:8]+"@test.com", "7999"+userNoPermsID.String()[:7])
	require.NoError(t, err)

	noPermRoleID, err := setupRouterStaffWithPermissions(ctx, tx, userNoPermsID, "NoPermRole", nil)
	require.NoError(t, err)

	var roleCount int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM staff_role_permissions WHERE role_id = $1`, noPermRoleID).Scan(&roleCount)
	require.NoError(t, err)
	require.Equal(t, 0, roleCount)

	var memberCount int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM staff_member_permissions WHERE user_id = $1`, userNoPermsID).Scan(&memberCount)
	require.NoError(t, err)
	require.Equal(t, 0, memberCount, "employee intended with no permissions must have 0 direct rows")
}
