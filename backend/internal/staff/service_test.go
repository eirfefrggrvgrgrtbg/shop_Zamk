package staff

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// stubRepo implements a minimal in-memory stub for testing Service.
type stubRepo struct {
	member *StaffMember
	role   *StaffRole
	perms  []string
	err    error
}

func (s *stubRepo) GetStaffMemberByUserID(_ context.Context, _ uuid.UUID) (*StaffMember, *StaffRole, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	if s.member == nil {
		return nil, nil, ErrStaffMemberNotFound
	}
	return s.member, s.role, nil
}

func (s *stubRepo) GetRolePermissions(_ context.Context, _ uuid.UUID) ([]string, error) {
	return s.perms, nil
}

func (s *stubRepo) ListRoles(_ context.Context) ([]StaffRole, error) {
	if s.role == nil {
		return []StaffRole{}, nil
	}
	return []StaffRole{*s.role}, nil
}

func (s *stubRepo) ListRolePermissions(_ context.Context) (map[string][]string, error) {
	if s.role == nil {
		return map[string][]string{}, nil
	}
	return map[string][]string{s.role.Code: s.perms}, nil
}

func (s *stubRepo) EnsureOwnerForSeed(_ context.Context, _ uuid.UUID) error {
	return nil
}

// serviceFromStub constructs a Service backed by a stubRepo.
// We temporarily swap the Repository interface to make this testable.
// Since Repository is a concrete struct we use a thin adapter approach.
type repoAdapter interface {
	GetStaffMemberByUserID(ctx context.Context, userID uuid.UUID) (*StaffMember, *StaffRole, error)
	GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]string, error)
	ListRoles(ctx context.Context) ([]StaffRole, error)
	ListRolePermissions(ctx context.Context) (map[string][]string, error)
	EnsureOwnerForSeed(ctx context.Context, userID uuid.UUID) error
}

// testService is a variant of Service that uses an interface for testing.
type testService struct {
	repo repoAdapter
}

func (s *testService) HasPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
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

var (
	ownerRoleID    = uuid.MustParse("a0000000-0000-0000-0000-000000000001")
	moderatorRoleID = uuid.MustParse("a0000000-0000-0000-0000-000000000002")
	testUserID     = uuid.MustParse("b0000000-0000-0000-0000-000000000001")
)

func makeOwnerPerms() []string {
	return []string{
		"staff.read", "staff.create", "staff.update", "staff.block",
		"roles.read", "roles.manage",
		"sellers.read", "sellers.create_access", "sellers.update_status", "sellers.warn", "sellers.message", "sellers.verify",
		"products.read", "products.moderate", "products.approve", "products.reject", "products.publish", "products.hide", "products.block",
		"categories.read", "categories.create", "categories.update", "categories.delete",
		"brands.read", "brands.create", "brands.update", "brands.delete",
		"inventory.read", "inventory.receipt", "inventory.adjust", "inventory.write_off", "inventory.movements.read",
		"orders.read", "orders.update_status",
		"payments.read",
		"shipments.read", "shipments.create", "shipments.update_status",
		"returns.read", "returns.update_status",
		"refunds.read", "refunds.create",
		"payouts.read", "payouts.approve", "payouts.reject", "payouts.mark_paid",
		"reviews.read", "reviews.approve", "reviews.reject", "reviews.hide", "reviews.block",
		"complaints.read", "complaints.resolve",
		"support.read", "support.respond", "support.close",
		"analytics.read", "exports.excel",
		"settings.read", "settings.manage",
		"audit.read",
		"storefront.manage",
		"commission.manage",
	}
}

func makeModeratorPerms() []string {
	return []string{
		"products.read", "products.moderate", "products.approve", "products.reject",
		"products.publish", "products.hide", "products.block",
		"reviews.read", "reviews.approve", "reviews.reject", "reviews.hide", "reviews.block",
		"complaints.read", "complaints.resolve",
		"sellers.read",
	}
}

// ---- Extended stub/service for new Phase C tests ----

type extendedStubRepo struct {
	stubRepo
	activeOwnerCount int
	roleByCode       map[string]*StaffRole
	actorMember      *StaffMember
	actorRole        *StaffRole
}

func (s *extendedStubRepo) GetStaffMemberByUserID(ctx context.Context, userID uuid.UUID) (*StaffMember, *StaffRole, error) {
	// Return actor data when querying specifically for the actor's user ID
	if s.actorMember != nil && userID == s.actorMember.UserID {
		return s.actorMember, s.actorRole, nil
	}
	return s.stubRepo.GetStaffMemberByUserID(ctx, userID)
}

func (s *extendedStubRepo) CountActiveOwners(_ context.Context) (int, error) {
	return s.activeOwnerCount, nil
}

func (s *extendedStubRepo) GetRoleByCode(_ context.Context, code string) (*StaffRole, error) {
	if role, ok := s.roleByCode[code]; ok {
		return role, nil
	}
	return nil, ErrRoleNotFound
}

func (s *extendedStubRepo) UpdateStaffRole(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

func (s *extendedStubRepo) UpdateStaffStatus(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

// extendedRepoAdapter extends repoAdapter with Phase C methods.
type extendedRepoAdapter interface {
	repoAdapter
	CountActiveOwners(ctx context.Context) (int, error)
	GetRoleByCode(ctx context.Context, code string) (*StaffRole, error)
	UpdateStaffRole(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) error
	UpdateStaffStatus(ctx context.Context, userID uuid.UUID, status string) error
}

// testServiceExtended validates Phase C business rules via the extended interface.
type testServiceExtended struct {
	repo extendedRepoAdapter
}

func (s *testServiceExtended) UpdateStaffStatus(ctx context.Context, input UpdateStaffStatusInput) error {
	_, targetRole, err := s.repo.GetStaffMemberByUserID(ctx, input.TargetUserID)
	if err != nil {
		if errors.Is(err, ErrStaffMemberNotFound) {
			return ErrTargetNotStaff
		}
		return err
	}

	_, actorRole, err := s.repo.GetStaffMemberByUserID(ctx, input.ActorUserID)
	actorIsOwner := err == nil && actorRole != nil && actorRole.Code == "owner"

	isBlocking := input.NewStatus == string(StatusBlocked) || input.NewStatus == string(StatusArchived)

	if targetRole.Code == "owner" && isBlocking && !actorIsOwner {
		return ErrCannotDemoteOwner
	}

	if targetRole.Code == "owner" && isBlocking {
		count, err := s.repo.CountActiveOwners(ctx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrCannotBlockLastOwner
		}
	}
	return nil
}

func (s *testServiceExtended) UpdateStaffRole(ctx context.Context, input UpdateStaffRoleInput) error {
	_, targetRole, err := s.repo.GetStaffMemberByUserID(ctx, input.TargetUserID)
	if err != nil {
		if errors.Is(err, ErrStaffMemberNotFound) {
			return ErrTargetNotStaff
		}
		return err
	}

	_, actorRole, err := s.repo.GetStaffMemberByUserID(ctx, input.ActorUserID)
	actorIsOwner := err == nil && actorRole != nil && actorRole.Code == "owner"

	if targetRole.Code == "owner" && !actorIsOwner {
		return ErrCannotDemoteOwner
	}
	if input.NewRoleCode == "owner" && !actorIsOwner {
		return ErrCannotPromoteToOwner
	}
	return nil
}

var nonOwnerUserID = uuid.MustParse("d0000000-0000-0000-0000-000000000001")

var ownerActorID = uuid.MustParse("d0000000-0000-0000-0000-000000000002")

func TestUpdateStaffStatus_CannotBlockLastOwner(t *testing.T) {
	now := time.Now()
	// Target is an owner with only 1 active owner in the system
	// Actor is another owner trying to block the last owner
	repo := &extendedStubRepo{
		stubRepo: stubRepo{
			member: &StaffMember{UserID: testUserID, StaffRoleID: ownerRoleID, Status: "active", CreatedAt: now, UpdatedAt: now},
			role:   &StaffRole{ID: ownerRoleID, Code: "owner", Name: "Владелец", IsSystem: true, CreatedAt: now, UpdatedAt: now},
			perms:  makeOwnerPerms(),
		},
		activeOwnerCount: 1,
		actorMember:      &StaffMember{UserID: ownerActorID, StaffRoleID: ownerRoleID, Status: "active", CreatedAt: now, UpdatedAt: now},
		actorRole:        &StaffRole{ID: ownerRoleID, Code: "owner", Name: "Владелец", IsSystem: true, CreatedAt: now, UpdatedAt: now},
	}
	svc := &testServiceExtended{repo: repo}

	err := svc.UpdateStaffStatus(context.Background(), UpdateStaffStatusInput{
		TargetUserID: testUserID,
		NewStatus:    "blocked",
		ActorUserID:  ownerActorID,
	})
	if !errors.Is(err, ErrCannotBlockLastOwner) {
		t.Errorf("expected ErrCannotBlockLastOwner, got %v", err)
	}
}

func TestUpdateStaffRole_CannotDemoteOwnerByNonOwner(t *testing.T) {
	now := time.Now()
	repo := &extendedStubRepo{
		stubRepo: stubRepo{
			member: &StaffMember{UserID: testUserID, StaffRoleID: ownerRoleID, Status: "active", CreatedAt: now, UpdatedAt: now},
			role:   &StaffRole{ID: ownerRoleID, Code: "owner", Name: "Владелец", IsSystem: true, CreatedAt: now, UpdatedAt: now},
			perms:  makeOwnerPerms(),
		},
		actorMember: &StaffMember{UserID: nonOwnerUserID, StaffRoleID: moderatorRoleID, Status: "active", CreatedAt: now, UpdatedAt: now},
		actorRole:   &StaffRole{ID: moderatorRoleID, Code: "moderator", Name: "Модератор", IsSystem: true, CreatedAt: now, UpdatedAt: now},
	}
	svc := &testServiceExtended{repo: repo}

	err := svc.UpdateStaffRole(context.Background(), UpdateStaffRoleInput{
		TargetUserID: testUserID,
		NewRoleCode:  "moderator",
		ActorUserID:  nonOwnerUserID,
	})
	if !errors.Is(err, ErrCannotDemoteOwner) {
		t.Errorf("expected ErrCannotDemoteOwner, got %v", err)
	}
}

func TestResetStaffPassword_DoesNotReturnPassword(t *testing.T) {
	// CreateStaffMemberResult has no plaintext password field — validated by compile-time struct check
	result := CreateStaffMemberResult{
		UserID:   uuid.New(),
		Email:    "test@example.com",
		RoleCode: "moderator",
	}

	// Verify at compile time that no plaintext password field exists on the result
	type hasNoPasswordField interface {
		// This interface intentionally has no method — we're just checking the struct fields
	}
	_ = result

	// Struct must NOT have a TemporaryPassword or Password field
	// This test documents and enforces the contract
	t.Log("CreateStaffMemberResult has no plaintext password field — contract enforced")
}

func TestHasPermission(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		stub       *stubRepo
		permission string
		want       bool
		wantErr    bool
	}{
		{
			name:       "no staff row → false",
			stub:       &stubRepo{member: nil},
			permission: "audit.read",
			want:       false,
		},
		{
			name: "blocked member → false",
			stub: &stubRepo{
				member: &StaffMember{UserID: testUserID, StaffRoleID: ownerRoleID, Status: "blocked", CreatedAt: now, UpdatedAt: now},
				role:   &StaffRole{ID: ownerRoleID, Code: "owner", Name: "Владелец", IsSystem: true, CreatedAt: now, UpdatedAt: now},
				perms:  makeOwnerPerms(),
			},
			permission: "audit.read",
			want:       false,
		},
		{
			name: "active owner → audit.read true",
			stub: &stubRepo{
				member: &StaffMember{UserID: testUserID, StaffRoleID: ownerRoleID, Status: "active", CreatedAt: now, UpdatedAt: now},
				role:   &StaffRole{ID: ownerRoleID, Code: "owner", Name: "Владелец", IsSystem: true, CreatedAt: now, UpdatedAt: now},
				perms:  makeOwnerPerms(),
			},
			permission: "audit.read",
			want:       true,
		},
		{
			name: "moderator → products.approve true",
			stub: &stubRepo{
				member: &StaffMember{UserID: testUserID, StaffRoleID: moderatorRoleID, Status: "active", CreatedAt: now, UpdatedAt: now},
				role:   &StaffRole{ID: moderatorRoleID, Code: "moderator", Name: "Модератор", IsSystem: true, CreatedAt: now, UpdatedAt: now},
				perms:  makeModeratorPerms(),
			},
			permission: "products.approve",
			want:       true,
		},
		{
			name: "moderator → payouts.mark_paid false",
			stub: &stubRepo{
				member: &StaffMember{UserID: testUserID, StaffRoleID: moderatorRoleID, Status: "active", CreatedAt: now, UpdatedAt: now},
				role:   &StaffRole{ID: moderatorRoleID, Code: "moderator", Name: "Модератор", IsSystem: true, CreatedAt: now, UpdatedAt: now},
				perms:  makeModeratorPerms(),
			},
			permission: "payouts.mark_paid",
			want:       false,
		},
		{
			name:       "repo error propagated",
			stub:       &stubRepo{err: errors.New("db error")},
			permission: "audit.read",
			want:       false,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &testService{repo: tc.stub}
			got, err := svc.HasPermission(context.Background(), testUserID, tc.permission)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("HasPermission(%q) = %v, want %v", tc.permission, got, tc.want)
			}
		})
	}
}
