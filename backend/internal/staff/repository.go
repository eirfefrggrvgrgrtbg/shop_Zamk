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

// WithTx returns a Repository bound to a transaction.
func (r *Repository) WithTx(tx postgres.DBTX) *Repository {
	return &Repository{db: tx}
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

// ListStaffMembers returns all staff members joined with their users and roles.
func (r *Repository) ListStaffMembers(ctx context.Context) ([]StaffMemberView, error) {
	query := `
		SELECT sm.user_id, u.name, u.email, u.status AS user_status, u.must_change_password,
		       sm.status AS staff_status, sm.created_at, sm.updated_at,
		       sr.code AS role_code, sr.name AS role_name, sr.id AS role_id
		FROM staff_members sm
		JOIN users u ON u.id = sm.user_id
		JOIN staff_roles sr ON sr.id = sm.staff_role_id
		ORDER BY sm.created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list staff members: %w", err)
	}
	defer rows.Close()

	var members []StaffMemberView
	for rows.Next() {
		var m StaffMemberView
		if err := rows.Scan(
			&m.UserID, &m.Name, &m.Email, &m.UserStatus, &m.MustChangePassword,
			&m.StaffStatus, &m.CreatedAt, &m.UpdatedAt,
			&m.RoleCode, &m.RoleName, &m.RoleID,
		); err != nil {
			return nil, fmt.Errorf("scan staff member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// GetRoleByCode returns a StaffRole by its code.
func (r *Repository) GetRoleByCode(ctx context.Context, code string) (*StaffRole, error) {
	query := `
		SELECT id, code, name, description, is_system, created_at, updated_at
		FROM staff_roles
		WHERE code = $1
	`
	var role StaffRole
	err := r.db.QueryRow(ctx, query, code).Scan(
		&role.ID, &role.Code, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("get role by code: %w", err)
	}
	return &role, nil
}

// InsertStaffMember inserts a new row in staff_members.
func (r *Repository) InsertStaffMember(ctx context.Context, userID uuid.UUID, roleID uuid.UUID, createdBy *uuid.UUID) error {
	query := `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'active', $3, now(), now())
	`
	_, err := r.db.Exec(ctx, query, userID, roleID, createdBy)
	if err != nil {
		return fmt.Errorf("insert staff member: %w", err)
	}
	return nil
}

// UpdateStaffRole updates the staff_role_id for a given user in staff_members.
func (r *Repository) UpdateStaffRole(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) error {
	query := `
		UPDATE staff_members
		SET staff_role_id = $1, updated_at = now()
		WHERE user_id = $2
	`
	res, err := r.db.Exec(ctx, query, roleID, userID)
	if err != nil {
		return fmt.Errorf("update staff role: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrStaffMemberNotFound
	}
	return nil
}

// UpdateStaffStatus updates the status column in staff_members for a given user.
func (r *Repository) UpdateStaffStatus(ctx context.Context, userID uuid.UUID, status string) error {
	query := `
		UPDATE staff_members
		SET status = $1, updated_at = now()
		WHERE user_id = $2
	`
	res, err := r.db.Exec(ctx, query, status, userID)
	if err != nil {
		return fmt.Errorf("update staff status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrStaffMemberNotFound
	}
	return nil
}

// CountActiveOwners counts active staff members that have the 'owner' role.
func (r *Repository) CountActiveOwners(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM staff_members sm
		JOIN staff_roles sr ON sr.id = sm.staff_role_id
		WHERE sm.status = 'active' AND sr.code = 'owner'
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active owners: %w", err)
	}
	return count, nil
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
