package staff

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

var ErrStaffMemberNotFound = errors.New("staff member not found")
var ErrRoleNotFound = errors.New("staff role not found")

type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

// GetStaffMemberByUserID fetches the staff member record and their role.
func (r *Repository) GetStaffMemberByUserID(ctx context.Context, userID uuid.UUID) (*StaffMember, *StaffRole, error) {
	query := `
		SELECT
			sm.user_id, sm.staff_role_id, sm.status, sm.created_by, sm.created_at, sm.updated_at,
			sr.id, sr.code, sr.name, sr.description, sr.is_system, sr.created_at, sr.updated_at
		FROM staff_members sm
		JOIN staff_roles sr ON sr.id = sm.staff_role_id
		WHERE sm.user_id = $1
	`
	var m StaffMember
	var role StaffRole
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&m.UserID, &m.StaffRoleID, &m.Status, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt,
		&role.ID, &role.Code, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrStaffMemberNotFound
		}
		return nil, nil, fmt.Errorf("get staff member by user id: %w", err)
	}
	return &m, &role, nil
}

// GetRolePermissions returns all permissions for a given role.
func (r *Repository) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]string, error) {
	query := `SELECT permission FROM staff_role_permissions WHERE role_id = $1 ORDER BY permission`
	rows, err := r.db.Query(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("get role permissions: %w", err)
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// ListRoles returns all staff roles ordered by name.
func (r *Repository) ListRoles(ctx context.Context) ([]StaffRole, error) {
	query := `
		SELECT id, code, name, description, is_system, created_at, updated_at
		FROM staff_roles
		ORDER BY name
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	var roles []StaffRole
	for rows.Next() {
		var role StaffRole
		if err := rows.Scan(&role.ID, &role.Code, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

// ListRolePermissions returns a map of roleCode → []permission.
func (r *Repository) ListRolePermissions(ctx context.Context) (map[string][]string, error) {
	query := `
		SELECT sr.code, srp.permission
		FROM staff_role_permissions srp
		JOIN staff_roles sr ON sr.id = srp.role_id
		ORDER BY sr.code, srp.permission
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list role permissions: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var code, perm string
		if err := rows.Scan(&code, &perm); err != nil {
			return nil, fmt.Errorf("scan role permission: %w", err)
		}
		result[code] = append(result[code], perm)
	}
	return result, rows.Err()
}

// EnsureOwnerForSeed upserts the given user as owner — used only by dev-seed.
func (r *Repository) EnsureOwnerForSeed(ctx context.Context, userID uuid.UUID) error {
	query := `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
		SELECT $1, id, 'active', now(), now()
		FROM staff_roles
		WHERE code = 'owner'
		ON CONFLICT (user_id) DO UPDATE SET
			staff_role_id = EXCLUDED.staff_role_id,
			status        = 'active',
			updated_at    = now()
	`
	_, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("ensure owner for seed: %w", err)
	}
	return nil
}
