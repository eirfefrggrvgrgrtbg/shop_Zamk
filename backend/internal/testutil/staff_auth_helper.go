package testutil

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// GrantStaffAuthorizationState dual-writes the intended permissions to both
// staff_role_permissions (for current role-based runtime) and
// staff_member_permissions (for future member-based runtime).
// It verifies that the connected database is strictly zamk_test before performing any work.
// If permissions are empty, no permissions are granted, ensuring negative cases remain unauthorized.
func GrantStaffAuthorizationState(ctx context.Context, db DBExecutor, userID uuid.UUID, roleID uuid.UUID, permissions ...string) error {
	if err := VerifyTestDatabase(ctx, db); err != nil {
		return err
	}

	for _, p := range permissions {
		if p == "" {
			continue
		}
		if roleID != uuid.Nil {
			_, err := db.Exec(ctx, `
				INSERT INTO staff_role_permissions (role_id, permission)
				VALUES ($1, $2)
				ON CONFLICT DO NOTHING
			`, roleID, p)
			if err != nil {
				return fmt.Errorf("grant staff_role_permissions: %w", err)
			}
		}

		if userID != uuid.Nil {
			_, err := db.Exec(ctx, `
				INSERT INTO staff_member_permissions (user_id, permission, created_at)
				VALUES ($1, $2, NOW())
				ON CONFLICT DO NOTHING
			`, userID, p)
			if err != nil {
				return fmt.Errorf("grant staff_member_permissions: %w", err)
			}
		}
	}
	return nil
}

// SyncStaffMemberRolePermissions copies all permissions currently associated with roleID
// into staff_member_permissions for userID, ensuring canonical role fixtures (e.g. owner)
// are future-compatible without hardcoding full permission sets.
func SyncStaffMemberRolePermissions(ctx context.Context, db DBExecutor, userID uuid.UUID, roleID uuid.UUID) error {
	if err := VerifyTestDatabase(ctx, db); err != nil {
		return err
	}

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
