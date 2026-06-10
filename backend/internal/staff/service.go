package staff

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

var (
	ErrDuplicateEmail       = errors.New("email already in use")
	ErrCannotBlockLastOwner = errors.New("cannot block or archive the last active owner")
	ErrCannotDemoteOwner    = errors.New("only an owner can change the owner role")
	ErrCannotPromoteToOwner = errors.New("only an owner can assign the owner role")
	ErrTargetNotStaff       = errors.New("target user is not a staff member")
)

type Service struct {
	repo     *Repository
	userRepo *users.Repository
	db       *postgres.Client
}

func NewService(repo *Repository, userRepo *users.Repository, db *postgres.Client) *Service {
	return &Service{repo: repo, userRepo: userRepo, db: db}
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

// ListStaffMembers returns all staff members with their role permissions.
func (s *Service) ListStaffMembers(ctx context.Context) ([]StaffMemberView, error) {
	members, err := s.repo.ListStaffMembers(ctx)
	if err != nil {
		return nil, err
	}
	for i := range members {
		perms, err := s.repo.GetRolePermissions(ctx, members[i].RoleID)
		if err != nil {
			return nil, err
		}
		if perms == nil {
			perms = []string{}
		}
		members[i].Permissions = perms
	}
	return members, nil
}

// CreateStaffMemberInput holds parameters for creating a staff member.
type CreateStaffMemberInput struct {
	Name              string
	Email             string
	Phone             string
	RoleCode          string
	TemporaryPassword string
	CreatedByUserID   uuid.UUID
}

// CreateStaffMemberResult is returned after a successful creation.
// It never contains the plaintext password.
type CreateStaffMemberResult struct {
	UserID   uuid.UUID
	Email    string
	RoleCode string
}

// CreateStaffMember creates a new admin user with a staff role.
func (s *Service) CreateStaffMember(ctx context.Context, input CreateStaffMemberInput) (*CreateStaffMemberResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	// Validate email uniqueness
	_, err := s.userRepo.GetUserByEmail(ctx, email)
	if err == nil {
		return nil, ErrDuplicateEmail
	} else if !errors.Is(err, users.ErrNotFound) {
		return nil, fmt.Errorf("check email: %w", err)
	}

	// Validate roleCode
	role, err := s.repo.GetRoleByCode(ctx, input.RoleCode)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("get role: %w", err)
	}

	// Hash password — never store or return plaintext
	if len(input.TemporaryPassword) < 8 {
		return nil, errors.New("temporary password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.TemporaryPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	newUserID := uuid.New()
	now := time.Now()
	var createdByPtr *uuid.UUID
	if input.CreatedByUserID != uuid.Nil {
		id := input.CreatedByUserID
		createdByPtr = &id
	}

	userRecord := &users.User{
		ID:                 newUserID,
		Name:               strings.TrimSpace(input.Name),
		Email:              email,
		PasswordHash:       string(hash),
		Role:               users.RoleAdmin,
		Status:             users.StatusActive,
		MustChangePassword: true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := s.runCreateStaffTx(ctx, userRecord, role.ID, createdByPtr); err != nil {
		return nil, err
	}

	return &CreateStaffMemberResult{
		UserID:   newUserID,
		Email:    email,
		RoleCode: role.Code,
	}, nil
}

// runCreateStaffTx runs user + staff_member creation in a single transaction.
func (s *Service) runCreateStaffTx(ctx context.Context, user *users.User, roleID uuid.UUID, createdBy *uuid.UUID) error {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.userRepo.WithTx(tx).CreateUser(ctx, user); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	if err := s.repo.WithTx(tx).InsertStaffMember(ctx, user.ID, roleID, createdBy); err != nil {
		return fmt.Errorf("create staff member: %w", err)
	}
	return tx.Commit(ctx)
}

// UpdateStaffRoleInput holds parameters for updating a staff member's role.
type UpdateStaffRoleInput struct {
	TargetUserID uuid.UUID
	NewRoleCode  string
	ActorUserID  uuid.UUID
}

// UpdateStaffRole validates business rules and updates the staff member's role.
func (s *Service) UpdateStaffRole(ctx context.Context, input UpdateStaffRoleInput) error {
	// Get target's current role
	_, targetRole, err := s.repo.GetStaffMemberByUserID(ctx, input.TargetUserID)
	if err != nil {
		if errors.Is(err, ErrStaffMemberNotFound) {
			return ErrTargetNotStaff
		}
		return err
	}

	// Get actor's role to check privileges
	_, actorRole, err := s.repo.GetStaffMemberByUserID(ctx, input.ActorUserID)
	actorIsOwner := err == nil && actorRole != nil && actorRole.Code == "owner"

	// Cannot change the owner role unless actor is owner
	if targetRole.Code == "owner" && !actorIsOwner {
		return ErrCannotDemoteOwner
	}

	// Cannot set role to 'owner' unless actor is owner
	if input.NewRoleCode == "owner" && !actorIsOwner {
		return ErrCannotPromoteToOwner
	}

	// Resolve new role
	newRole, err := s.repo.GetRoleByCode(ctx, input.NewRoleCode)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			return ErrRoleNotFound
		}
		return err
	}

	return s.repo.UpdateStaffRole(ctx, input.TargetUserID, newRole.ID)
}

// UpdateStaffStatusInput holds parameters for updating a staff member's status.
type UpdateStaffStatusInput struct {
	TargetUserID uuid.UUID
	NewStatus    string
	ActorUserID  uuid.UUID
}

// UpdateStaffStatus validates business rules and updates the staff member's status.
func (s *Service) UpdateStaffStatus(ctx context.Context, input UpdateStaffStatusInput) error {
	// Get target's current role
	_, targetRole, err := s.repo.GetStaffMemberByUserID(ctx, input.TargetUserID)
	if err != nil {
		if errors.Is(err, ErrStaffMemberNotFound) {
			return ErrTargetNotStaff
		}
		return err
	}

	// Get actor's role to check privileges
	_, actorRole, err := s.repo.GetStaffMemberByUserID(ctx, input.ActorUserID)
	actorIsOwner := err == nil && actorRole != nil && actorRole.Code == "owner"

	isBlocking := input.NewStatus == string(StatusBlocked) || input.NewStatus == string(StatusArchived)

	// Cannot block/archive owner unless actor is owner
	if targetRole.Code == "owner" && isBlocking && !actorIsOwner {
		return ErrCannotDemoteOwner
	}

	// Cannot block last active owner
	if targetRole.Code == "owner" && isBlocking {
		count, err := s.repo.CountActiveOwners(ctx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrCannotBlockLastOwner
		}
	}

	// Update staff_members status
	if err := s.repo.UpdateStaffStatus(ctx, input.TargetUserID, input.NewStatus); err != nil {
		return err
	}

	// Sync users.status so blocked staff cannot login
	userStatus := users.StatusActive
	if isBlocking {
		userStatus = users.StatusBlocked
	}
	// Best-effort — don't fail the whole operation if user status sync fails
	_ = s.userRepo.UpdateUserStatus(ctx, input.TargetUserID, userStatus)

	return nil
}

// ResetStaffPasswordInput holds parameters for resetting a staff member's password.
type ResetStaffPasswordInput struct {
	TargetUserID      uuid.UUID
	TemporaryPassword string
}

// ResetStaffPassword hashes the new password and sets must_change_password=true.
// The plaintext password is never stored or returned.
func (s *Service) ResetStaffPassword(ctx context.Context, input ResetStaffPasswordInput) error {
	if len(input.TemporaryPassword) < 8 {
		return errors.New("temporary password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.TemporaryPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.userRepo.UpdatePasswordAndMustChange(ctx, input.TargetUserID, string(hash), true)
}
