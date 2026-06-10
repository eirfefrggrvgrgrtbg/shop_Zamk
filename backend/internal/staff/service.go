package staff

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// GetStaffAccess returns the full staff access object for a user.
// Returns nil (not an error) if the user has no staff row.
func (s *Service) GetStaffAccess(ctx context.Context, userID uuid.UUID) (*StaffAccess, error) {
	member, role, err := s.repo.GetStaffMemberByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrStaffMemberNotFound) {
			return nil, nil
		}
		return nil, err
	}

	perms, err := s.repo.GetRolePermissions(ctx, role.ID)
	if err != nil {
		return nil, err
	}

	return &StaffAccess{
		Role:        role,
		Member:      member,
		Permissions: perms,
	}, nil
}

// HasPermission returns true if the user is an active staff member with the given permission.
func (s *Service) HasPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
	member, role, err := s.repo.GetStaffMemberByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrStaffMemberNotFound) {
			return false, nil
		}
		return false, err
	}

	if member.Status != string(StatusActive) {
		return false, nil
	}

	perms, err := s.repo.GetRolePermissions(ctx, role.ID)
	if err != nil {
		return false, err
	}

	for _, p := range perms {
		if p == permission {
			return true, nil
		}
	}
	return false, nil
}

// ListRoles returns all staff roles.
func (s *Service) ListRoles(ctx context.Context) ([]StaffRole, error) {
	return s.repo.ListRoles(ctx)
}

// ListRolesWithPermissions returns a map of roleCode → []permission.
func (s *Service) ListRolesWithPermissions(ctx context.Context) (map[string][]string, error) {
	return s.repo.ListRolePermissions(ctx)
}
