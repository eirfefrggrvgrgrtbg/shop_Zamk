package staff_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

type emp1b3Harness struct {
	ctx          context.Context
	client       *postgres.Client
	svc          *staff.Service
	staffRepo    *staff.Repository
	handler      *staff.Handler
	router       *chi.Mux
	tokenService *auth.TokenService
}

func setupEMP1B3Harness(t *testing.T) *emp1b3Harness {
	t.Helper()
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pgClient.Close() })

	testutil.AssertTestDatabase(t, pgClient.Pool)

	staffRepo := staff.NewRepository(pgClient.Pool)
	userRepo := users.NewRepository(pgClient.Pool)
	auditRepo := staff.NewAuditRepository(pgClient.Pool)
	svc := staff.NewService(staffRepo, userRepo, auditRepo, pgClient)
	handler := staff.NewHandler(svc, auditRepo, userRepo)

	cfg := &config.Config{}
	cfg.JWT.AccessTokenSecret = "test-jwt-secret-for-emp1b3"
	cfg.JWT.RefreshTokenSecret = "test-jwt-refresh-secret-emp1b3"
	cfg.JWT.AccessTokenTTLMinutes = 60
	cfg.JWT.RefreshTokenTTLDays = 7
	cfg.RateLimit.Enabled = false
	cfg.App.Env = "test"

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dummyRedis := &redis.Client{Client: goredis.NewClient(&goredis.Options{})}
	realRouter, cancel := app.BuildRouter(ctx, cfg, pgClient, dummyRedis, logger)
	t.Cleanup(cancel)

	tokenService := auth.NewTokenService(cfg.JWT.AccessTokenSecret, cfg.JWT.RefreshTokenSecret, cfg.JWT.AccessTokenTTLMinutes)

	return &emp1b3Harness{
		ctx:          ctx,
		client:       pgClient,
		svc:          svc,
		staffRepo:    staffRepo,
		handler:      handler,
		router:       realRouter,
		tokenService: tokenService,
	}
}

func createB3Staff(t *testing.T, ctx context.Context, client *postgres.Client, roleCode string) (uuid.UUID, string) {
	t.Helper()
	userID := uuid.New()
	email := fmt.Sprintf("b3_%s_%s@test.com", roleCode, userID.String()[:8])
	now := time.Now().UTC()

	t.Cleanup(func() {
		c := context.Background()
		_, _ = client.Pool.Exec(c, `DELETE FROM audit_logs WHERE entity_id = $1`, userID.String())
		_, _ = client.Pool.Exec(c, `DELETE FROM staff_member_permissions WHERE user_id = $1`, userID)
		_, _ = client.Pool.Exec(c, `DELETE FROM staff_members WHERE user_id = $1`, userID)
		_, _ = client.Pool.Exec(c, `DELETE FROM users WHERE id = $1`, userID)
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

	return userID, email
}

func setDirectPerms(t *testing.T, ctx context.Context, client *postgres.Client, userID uuid.UUID, permissions ...string) {
	t.Helper()
	_, _ = client.Pool.Exec(ctx, `DELETE FROM staff_member_permissions WHERE user_id = $1`, userID)
	for _, p := range permissions {
		_, err := client.Pool.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, $2, now())
			ON CONFLICT DO NOTHING
		`, userID, p)
		require.NoError(t, err)
	}
}

func makeStaffToken(t *testing.T, tokenService *auth.TokenService, userID uuid.UUID, email string) string {
	t.Helper()
	tok, err := tokenService.GenerateAccessToken(userID, email, "admin")
	require.NoError(t, err)
	return tok
}

// 1. GET /staff/members/{userId}/permissions — Access Control
func TestEMP1B3_GetPermissions_AccessControl(t *testing.T) {
	h := setupEMP1B3Harness(t)

	managerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, managerID, staff.PermissionStaffPermissionsManage)

	nonManagerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, nonManagerID, "analytics.read")

	inactiveManagerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, inactiveManagerID, staff.PermissionStaffPermissionsManage)
	_, err := h.client.Pool.Exec(h.ctx, `UPDATE staff_members SET status = 'blocked' WHERE user_id = $1`, inactiveManagerID)
	require.NoError(t, err)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")
	setDirectPerms(t, h.ctx, h.client, targetID, "support.read", "support.respond")

	t.Run("ActorWithoutManage_Forbidden", func(t *testing.T) {
		_, err := h.svc.GetStaffMemberPermissions(h.ctx, targetID, nonManagerID)
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrPermissionManagementForbidden)
	})

	t.Run("InactiveManager_Forbidden", func(t *testing.T) {
		_, err := h.svc.GetStaffMemberPermissions(h.ctx, targetID, inactiveManagerID)
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrPermissionManagementForbidden)
	})

	t.Run("ValidManager_Success", func(t *testing.T) {
		perms, err := h.svc.GetStaffMemberPermissions(h.ctx, targetID, managerID)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"support.read", "support.respond"}, perms)
	})

	t.Run("TargetNotFound_Error", func(t *testing.T) {
		_, err := h.svc.GetStaffMemberPermissions(h.ctx, uuid.New(), managerID)
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrTargetNotStaff)
	})
}

// 2. GET /staff/members/{userId}/permissions — Direct-Only Proof (Section 3)
func TestEMP1B3_GetPermissions_DirectOnly(t *testing.T) {
	h := setupEMP1B3Harness(t)

	managerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, managerID, staff.PermissionStaffPermissionsManage)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")

	// Verify target's role preset contains "support.read"
	var presetContains bool
	err := h.client.Pool.QueryRow(h.ctx, `
		SELECT EXISTS (
			SELECT 1 FROM staff_role_permissions srp
			JOIN staff_members sm ON sm.staff_role_id = srp.role_id
			WHERE sm.user_id = $1 AND srp.permission = 'support.read'
		)
	`, targetID).Scan(&presetContains)
	require.NoError(t, err)
	require.True(t, presetContains, "target role template must contain support.read")

	// Explicitly clear target's direct permissions
	_, err = h.client.Pool.Exec(h.ctx, `DELETE FROM staff_member_permissions WHERE user_id = $1`, targetID)
	require.NoError(t, err)

	// GET target permissions: support.read MUST NOT be returned!
	permsBefore, err := h.svc.GetStaffMemberPermissions(h.ctx, targetID, managerID)
	require.NoError(t, err)
	require.Empty(t, permsBefore, "direct permissions must be empty; must not fall back to role preset")

	// Insert direct permission X ("support.read")
	_, err = h.client.Pool.Exec(h.ctx, `
		INSERT INTO staff_member_permissions (user_id, permission, created_at)
		VALUES ($1, 'support.read', now())
	`, targetID)
	require.NoError(t, err)

	// GET again: support.read MUST be returned!
	permsAfter, err := h.svc.GetStaffMemberPermissions(h.ctx, targetID, managerID)
	require.NoError(t, err)
	require.Equal(t, []string{"support.read"}, permsAfter)

	// Prove returned permissions are deterministically sorted
	setDirectPerms(t, h.ctx, h.client, targetID, "products.read", "analytics.read", "brands.read")
	sortedPerms, err := h.svc.GetStaffMemberPermissions(h.ctx, targetID, managerID)
	require.NoError(t, err)
	require.Equal(t, []string{"analytics.read", "brands.read", "products.read"}, sortedPerms)
}

// 3. PUT /staff/members/{userId}/permissions — Access Control
func TestEMP1B3_PutPermissions_AccessControl(t *testing.T) {
	h := setupEMP1B3Harness(t)

	managerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, managerID, staff.PermissionStaffPermissionsManage)

	nonManagerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, nonManagerID, "orders.read")

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")

	t.Run("ActorWithoutManage_Rejected", func(t *testing.T) {
		_, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: targetID,
			ActorUserID:  nonManagerID,
			Permissions:  []string{"orders.read"},
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrPermissionManagementForbidden)
	})
}

// 4. PUT /staff/members/{userId}/permissions — Canonicalization Proof (Section 4)
func TestEMP1B3_PutPermissions_Canonicalization(t *testing.T) {
	h := setupEMP1B3Harness(t)

	managerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, managerID, staff.PermissionStaffPermissionsManage)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")

	// Input with duplicate permissions and unordered
	inputPerms := []string{"returns.read", "orders.read", "orders.read"}
	updated, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
		TargetUserID: targetID,
		ActorUserID:  managerID,
		Permissions:  inputPerms,
	})
	require.NoError(t, err)
	// Response must be deterministically sorted and deduplicated
	require.Equal(t, []string{"orders.read", "returns.read"}, updated)

	// DB contains exactly 2 rows
	var dbCount int
	err = h.client.Pool.QueryRow(h.ctx, `SELECT COUNT(*) FROM staff_member_permissions WHERE user_id = $1`, targetID).Scan(&dbCount)
	require.NoError(t, err)
	require.Equal(t, 2, dbCount, "DB must contain exactly 2 deduplicated rows")

	// Repeat identical canonical PUT -> idempotent
	updated2, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
		TargetUserID: targetID,
		ActorUserID:  managerID,
		Permissions:  inputPerms,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"orders.read", "returns.read"}, updated2)

	err = h.client.Pool.QueryRow(h.ctx, `SELECT COUNT(*) FROM staff_member_permissions WHERE user_id = $1`, targetID).Scan(&dbCount)
	require.NoError(t, err)
	require.Equal(t, 2, dbCount, "DB must remain exactly 2 rows after idempotent PUT")
}

// 5. PUT /staff/members/{userId}/permissions — Null / Missing Request Contract (Section 5)
func TestEMP1B3_PutPermissions_NullAndMissing(t *testing.T) {
	h := setupEMP1B3Harness(t)

	managerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, managerID, staff.PermissionStaffPermissionsManage)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")
	setDirectPerms(t, h.ctx, h.client, targetID, "support.read")

	// Direct handler test with router context
	callHandler := func(body string) (int, map[string]any) {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ctx := context.WithValue(req.Context(), "userID", managerID)
				next.ServeHTTP(w, req.WithContext(ctx))
			})
		})
		r.Put("/api/admin/staff/members/{userId}/permissions", h.handler.UpdateStaffMemberPermissions)

		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/staff/members/%s/permissions", targetID), bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		return rec.Code, resp
	}

	t.Run("ValidEmptyArray_Allowed", func(t *testing.T) {
		code, resp := callHandler(`{"permissions": []}`)
		require.Equal(t, http.StatusOK, code)
		perms, ok := resp["permissions"].([]any)
		require.True(t, ok)
		require.Empty(t, perms)
	})

	t.Run("NullPermissions_400ValidationError", func(t *testing.T) {
		code, resp := callHandler(`{"permissions": null}`)
		require.Equal(t, http.StatusBadRequest, code)
		require.Equal(t, "validation_error", resp["error"])
	})

	t.Run("MissingPermissionsField_400ValidationError", func(t *testing.T) {
		code, resp := callHandler(`{}`)
		require.Equal(t, http.StatusBadRequest, code)
		require.Equal(t, "validation_error", resp["error"])
	})

	t.Run("MalformedJSON_400InvalidRequest", func(t *testing.T) {
		code, resp := callHandler(`{"permissions": [unterminated`)
		require.Equal(t, http.StatusBadRequest, code)
		require.Equal(t, "invalid_request", resp["error"])
	})
}

// 6. PUT /staff/members/{userId}/permissions — Capability Validation
func TestEMP1B3_PutPermissions_CapabilityValidation(t *testing.T) {
	h := setupEMP1B3Harness(t)

	managerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, managerID, staff.PermissionStaffPermissionsManage)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")
	setDirectPerms(t, h.ctx, h.client, targetID, "support.read")

	t.Run("ValidKnownCapabilities_ReplacesSet", func(t *testing.T) {
		newPerms := []string{"analytics.read", "brands.read", "products.read"}
		updated, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: targetID,
			ActorUserID:  managerID,
			Permissions:  newPerms,
		})
		require.NoError(t, err)
		require.ElementsMatch(t, newPerms, updated)

		directPerms, err := h.staffRepo.GetMemberPermissions(h.ctx, targetID)
		require.NoError(t, err)
		require.ElementsMatch(t, newPerms, directPerms)
	})

	t.Run("UnknownCapability_RejectedWithErrInvalidPermission", func(t *testing.T) {
		permsBefore, err := h.staffRepo.GetMemberPermissions(h.ctx, targetID)
		require.NoError(t, err)

		invalidPerms := []string{"analytics.read", "invalid.bogus.permission"}
		_, err = h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: targetID,
			ActorUserID:  managerID,
			Permissions:  invalidPerms,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrInvalidPermission)

		// Assert target permissions unchanged
		permsAfter, err := h.staffRepo.GetMemberPermissions(h.ctx, targetID)
		require.NoError(t, err)
		require.ElementsMatch(t, permsBefore, permsAfter)
	})
}

// 7. PUT /staff/members/{userId}/permissions — Role Preset Independence
func TestEMP1B3_PutPermissions_RolePresetIndependence(t *testing.T) {
	h := setupEMP1B3Harness(t)

	managerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, managerID, staff.PermissionStaffPermissionsManage)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")
	memberBefore, roleBefore, err := h.staffRepo.GetStaffMemberByUserID(h.ctx, targetID)
	require.NoError(t, err)

	// Update to permissions completely different from support preset
	customPerms := []string{"inventory.read", "orders.read"}
	updated, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
		TargetUserID: targetID,
		ActorUserID:  managerID,
		Permissions:  customPerms,
	})
	require.NoError(t, err)
	require.ElementsMatch(t, customPerms, updated)

	// Assert staff_role_id was NOT modified
	memberAfter, roleAfter, err := h.staffRepo.GetStaffMemberByUserID(h.ctx, targetID)
	require.NoError(t, err)
	require.Equal(t, roleBefore.ID, roleAfter.ID)
	require.Equal(t, "support", roleAfter.Code)
	require.Equal(t, memberBefore.StaffRoleID, memberAfter.StaffRoleID)
}

// 8. PUT /staff/members/{userId}/permissions — Role Independence (Section 8)
func TestEMP1B3_PutPermissions_RoleIndependence(t *testing.T) {
	h := setupEMP1B3Harness(t)

	// Actor A: role != owner (admin), active, direct staff.permissions.manage = YES
	actorA, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, actorA, staff.PermissionStaffPermissionsManage)

	// Actor B: role = owner, active, direct staff.permissions.manage = NO
	actorB, _ := createB3Staff(t, h.ctx, h.client, "owner")
	_, err := h.client.Pool.Exec(h.ctx, `DELETE FROM staff_member_permissions WHERE user_id = $1 AND permission = $2`, actorB, staff.PermissionStaffPermissionsManage)
	require.NoError(t, err)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")

	// Actor A can use GET and PUT
	perms, err := h.svc.GetStaffMemberPermissions(h.ctx, targetID, actorA)
	require.NoError(t, err)
	require.NotNil(t, perms)

	_, err = h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
		TargetUserID: targetID,
		ActorUserID:  actorA,
		Permissions:  []string{"orders.read"},
	})
	require.NoError(t, err)

	// Actor B CANNOT use GET or PUT (rejected with ErrPermissionManagementForbidden)
	_, err = h.svc.GetStaffMemberPermissions(h.ctx, targetID, actorB)
	require.Error(t, err)
	require.ErrorIs(t, err, staff.ErrPermissionManagementForbidden)

	_, err = h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
		TargetUserID: targetID,
		ActorUserID:  actorB,
		Permissions:  []string{"analytics.read"},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, staff.ErrPermissionManagementForbidden)
}

// 9. PUT /staff/members/{userId}/permissions — Grant Any Known Capability (Section 9)
func TestEMP1B3_PutPermissions_GrantAnyKnownCapability(t *testing.T) {
	h := setupEMP1B3Harness(t)

	// Actor has staff.permissions.manage, but does NOT have warehouse.receiving
	actorID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, actorID, staff.PermissionStaffPermissionsManage)

	hasWarehouse, err := h.staffRepo.HasMemberPermission(h.ctx, actorID, "warehouse.receiving")
	require.NoError(t, err)
	require.False(t, hasWarehouse, "actor must not personally have warehouse.receiving")

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")

	// Actor grants warehouse.receiving to target -> MUST SUCCEED
	updated, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
		TargetUserID: targetID,
		ActorUserID:  actorID,
		Permissions:  []string{"warehouse.receiving"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"warehouse.receiving"}, updated)

	targetHasWarehouse, err := h.staffRepo.HasMemberPermission(h.ctx, targetID, "warehouse.receiving")
	require.NoError(t, err)
	require.True(t, targetHasWarehouse, "target must have gained warehouse.receiving")
}

// 10. PUT /staff/members/{userId}/permissions — Empty List `[]`
func TestEMP1B3_PutPermissions_EmptyList(t *testing.T) {
	h := setupEMP1B3Harness(t)

	managerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, managerID, staff.PermissionStaffPermissionsManage)

	t.Run("EmptyList_AllowedForOrdinaryStaff", func(t *testing.T) {
		targetID, _ := createB3Staff(t, h.ctx, h.client, "support")
		setDirectPerms(t, h.ctx, h.client, targetID, "support.read")

		updated, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: targetID,
			ActorUserID:  managerID,
			Permissions:  []string{},
		})
		require.NoError(t, err)
		require.Empty(t, updated)

		directPerms, err := h.staffRepo.GetMemberPermissions(h.ctx, targetID)
		require.NoError(t, err)
		require.Empty(t, directPerms)
	})

	t.Run("EmptyList_AllowedForManagerWhenAnotherManagerExists", func(t *testing.T) {
		m2, _ := createB3Staff(t, h.ctx, h.client, "admin")
		setDirectPerms(t, h.ctx, h.client, m2, staff.PermissionStaffPermissionsManage)

		count, err := h.staffRepo.CountActivePermissionManagers(h.ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		updated, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: m2,
			ActorUserID:  managerID,
			Permissions:  []string{},
		})
		require.NoError(t, err)
		require.Empty(t, updated)

		countAfter, err := h.staffRepo.CountActivePermissionManagers(h.ctx)
		require.NoError(t, err)
		require.Equal(t, 1, countAfter)
	})

	t.Run("EmptyList_RejectedWhenRemovingLastManager", func(t *testing.T) {
		count, err := h.staffRepo.CountActivePermissionManagers(h.ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		_, err = h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: managerID,
			ActorUserID:  managerID,
			Permissions:  []string{},
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrCannotRemoveLastPermissionManager)
	})
}

// 11. PUT /staff/members/{userId}/permissions — Self-Edit & Self-Revoke
func TestEMP1B3_PutPermissions_SelfEditAndSelfRevoke(t *testing.T) {
	h := setupEMP1B3Harness(t)

	mA, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, mA, staff.PermissionStaffPermissionsManage)

	mB, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, mB, staff.PermissionStaffPermissionsManage)

	t.Run("SelfEdit_Allowed", func(t *testing.T) {
		newPerms := []string{staff.PermissionStaffPermissionsManage, "analytics.read", "orders.read"}
		updated, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: mA,
			ActorUserID:  mA,
			Permissions:  newPerms,
		})
		require.NoError(t, err)
		require.ElementsMatch(t, newPerms, updated)
	})

	t.Run("SelfRevoke_AllowedWhenAnotherManagerExists", func(t *testing.T) {
		updated, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: mA,
			ActorUserID:  mA,
			Permissions:  []string{"analytics.read"},
		})
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"analytics.read"}, updated)

		hasManage, err := h.staffRepo.HasMemberPermission(h.ctx, mA, staff.PermissionStaffPermissionsManage)
		require.NoError(t, err)
		require.False(t, hasManage)

		remaining, err := h.staffRepo.CountActivePermissionManagers(h.ctx)
		require.NoError(t, err)
		require.Equal(t, 1, remaining)
	})

	t.Run("SelfRevoke_RejectedWhenLastManager", func(t *testing.T) {
		_, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: mB,
			ActorUserID:  mB,
			Permissions:  []string{"analytics.read"},
		})
		require.Error(t, err)
		require.ErrorIs(t, err, staff.ErrCannotRemoveLastPermissionManager)
	})
}

// 12. PUT /staff/members/{userId}/permissions — Blocked/Archived Target
func TestEMP1B3_PutPermissions_BlockedTarget(t *testing.T) {
	h := setupEMP1B3Harness(t)

	managerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, managerID, staff.PermissionStaffPermissionsManage)

	blockedTargetID, _ := createB3Staff(t, h.ctx, h.client, "support")
	_, err := h.client.Pool.Exec(h.ctx, `UPDATE staff_members SET status = 'blocked' WHERE user_id = $1`, blockedTargetID)
	require.NoError(t, err)

	t.Run("CanEditBlockedTarget_PermissionsUpdated", func(t *testing.T) {
		newPerms := []string{"inventory.read", "orders.read"}
		updated, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: blockedTargetID,
			ActorUserID:  managerID,
			Permissions:  newPerms,
		})
		require.NoError(t, err)
		require.ElementsMatch(t, newPerms, updated)
	})

	t.Run("GrantingManageToBlockedTarget_DoesNotMakeActiveManager", func(t *testing.T) {
		countBefore, err := h.staffRepo.CountActivePermissionManagers(h.ctx)
		require.NoError(t, err)
		require.Equal(t, 1, countBefore)

		_, err = h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: blockedTargetID,
			ActorUserID:  managerID,
			Permissions:  []string{staff.PermissionStaffPermissionsManage},
		})
		require.NoError(t, err)

		countAfter, err := h.staffRepo.CountActivePermissionManagers(h.ctx)
		require.NoError(t, err)
		require.Equal(t, 1, countAfter)

		hasRuntime, err := h.svc.HasPermission(h.ctx, blockedTargetID, staff.PermissionStaffPermissionsManage)
		require.NoError(t, err)
		require.False(t, hasRuntime, "blocked staff must have 0 runtime access")
	})

	t.Run("RevokingManageFromBlockedTarget_AllowedEvenWith1ActiveManager", func(t *testing.T) {
		updated, err := h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: blockedTargetID,
			ActorUserID:  managerID,
			Permissions:  []string{},
		})
		require.NoError(t, err)
		require.Empty(t, updated)
	})
}

// 13. PUT /staff/members/{userId}/permissions — Deterministic Concurrency: Two Self-Revokes (Section 10)
func TestEMP1B3_PutPermissions_DeterministicConcurrency_TwoSelfRevokes(t *testing.T) {
	h := setupEMP1B3Harness(t)

	m1, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, m1, staff.PermissionStaffPermissionsManage)

	m2, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, m2, staff.PermissionStaffPermissionsManage)

	count, err := h.staffRepo.CountActivePermissionManagers(h.ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	txLock, err := h.client.Pool.Begin(h.ctx)
	require.NoError(t, err)
	defer func() { _ = txLock.Rollback(h.ctx) }()

	var lockedID uuid.UUID
	err = txLock.QueryRow(h.ctx, `SELECT id FROM staff_roles WHERE code = 'owner' FOR UPDATE`).Scan(&lockedID)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)
	var err1, err2 error

	go func() {
		defer wg.Done()
		_, err1 = h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: m1,
			ActorUserID:  m1,
			Permissions:  []string{},
		})
	}()
	go func() {
		defer wg.Done()
		_, err2 = h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: m2,
			ActorUserID:  m2,
			Permissions:  []string{},
		})
	}()

	require.Eventually(t, func() bool {
		var waitingCount int
		_ = h.client.Pool.QueryRow(h.ctx, `
			SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock'
			  AND state = 'active'
			  AND query LIKE '%SELECT id FROM staff_roles WHERE code = ''owner'' FOR UPDATE%'
			  AND pid != pg_backend_pid()
		`).Scan(&waitingCount)
		return waitingCount >= 1
	}, 3*time.Second, 10*time.Millisecond)

	err = txLock.Rollback(h.ctx)
	require.NoError(t, err)

	wg.Wait()

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

	require.Equal(t, 1, successCount, "exactly 1 self-revoke must succeed")
	require.Equal(t, 1, failCount, "exactly 1 self-revoke must fail with ErrCannotRemoveLastPermissionManager")

	remaining, err := h.staffRepo.CountActivePermissionManagers(h.ctx)
	require.NoError(t, err)
	require.Equal(t, 1, remaining, "final active permission manager count must be exactly 1, never 0")
}

// 14. PUT /staff/members/{userId}/permissions — Actor Manage-Revoked TOCTOU (Section 11)
func TestEMP1B3_PutPermissions_TOCTOU_ActorManageRevoked(t *testing.T) {
	h := setupEMP1B3Harness(t)

	actorID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, actorID, staff.PermissionStaffPermissionsManage)

	otherManagerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, otherManagerID, staff.PermissionStaffPermissionsManage)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")
	setDirectPerms(t, h.ctx, h.client, targetID, "support.read")

	txLock, err := h.client.Pool.Begin(h.ctx)
	require.NoError(t, err)
	defer func() { _ = txLock.Rollback(h.ctx) }()

	var lockedID uuid.UUID
	err = txLock.QueryRow(h.ctx, `SELECT id FROM staff_roles WHERE code = 'owner' FOR UPDATE`).Scan(&lockedID)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	var errR1 error
	go func() {
		defer wg.Done()
		_, errR1 = h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: targetID,
			ActorUserID:  actorID,
			Permissions:  []string{"analytics.read"},
		})
	}()

	require.Eventually(t, func() bool {
		var waitingCount int
		_ = h.client.Pool.QueryRow(h.ctx, `
			SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock'
			  AND state = 'active'
			  AND query LIKE '%SELECT id FROM staff_roles WHERE code = ''owner'' FOR UPDATE%'
			  AND pid != pg_backend_pid()
		`).Scan(&waitingCount)
		return waitingCount >= 1
	}, 3*time.Second, 10*time.Millisecond)

	// Revoke actor's manage capability while waiting
	_, err = h.client.Pool.Exec(h.ctx, `DELETE FROM staff_member_permissions WHERE user_id = $1 AND permission = $2`, actorID, staff.PermissionStaffPermissionsManage)
	require.NoError(t, err)

	err = txLock.Rollback(h.ctx)
	require.NoError(t, err)

	wg.Wait()

	require.Error(t, errR1)
	require.ErrorIs(t, errR1, staff.ErrPermissionManagementForbidden)

	// Target C permissions must remain unchanged
	targetPerms, err := h.staffRepo.GetMemberPermissions(h.ctx, targetID)
	require.NoError(t, err)
	require.Equal(t, []string{"support.read"}, targetPerms)
}

// 15. PUT /staff/members/{userId}/permissions — Actor Blocked TOCTOU (Section 12)
func TestEMP1B3_PutPermissions_TOCTOU_ActorBlocked(t *testing.T) {
	h := setupEMP1B3Harness(t)

	actorID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, actorID, staff.PermissionStaffPermissionsManage)

	otherManagerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, otherManagerID, staff.PermissionStaffPermissionsManage)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")
	setDirectPerms(t, h.ctx, h.client, targetID, "support.read")

	txLock, err := h.client.Pool.Begin(h.ctx)
	require.NoError(t, err)
	defer func() { _ = txLock.Rollback(h.ctx) }()

	var lockedID uuid.UUID
	err = txLock.QueryRow(h.ctx, `SELECT id FROM staff_roles WHERE code = 'owner' FOR UPDATE`).Scan(&lockedID)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	var errR1 error
	go func() {
		defer wg.Done()
		_, errR1 = h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: targetID,
			ActorUserID:  actorID,
			Permissions:  []string{"analytics.read"},
		})
	}()

	require.Eventually(t, func() bool {
		var waitingCount int
		_ = h.client.Pool.QueryRow(h.ctx, `
			SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock'
			  AND state = 'active'
			  AND query LIKE '%SELECT id FROM staff_roles WHERE code = ''owner'' FOR UPDATE%'
			  AND pid != pg_backend_pid()
		`).Scan(&waitingCount)
		return waitingCount >= 1
	}, 3*time.Second, 10*time.Millisecond)

	// Block actor while waiting
	_, err = h.client.Pool.Exec(h.ctx, `UPDATE staff_members SET status = 'blocked' WHERE user_id = $1`, actorID)
	require.NoError(t, err)

	err = txLock.Rollback(h.ctx)
	require.NoError(t, err)

	wg.Wait()

	require.Error(t, errR1)
	require.ErrorIs(t, errR1, staff.ErrPermissionManagementForbidden)

	targetPerms, err := h.staffRepo.GetMemberPermissions(h.ctx, targetID)
	require.NoError(t, err)
	require.Equal(t, []string{"support.read"}, targetPerms)
}

// 16. PUT /staff/members/{userId}/permissions — Concurrent PUT to Same Target (Section 13)
func TestEMP1B3_PutPermissions_ConcurrentSameTarget(t *testing.T) {
	h := setupEMP1B3Harness(t)

	mA, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, mA, staff.PermissionStaffPermissionsManage)

	mB, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, mB, staff.PermissionStaffPermissionsManage)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")

	setX := []string{"orders.read"}
	setY := []string{"analytics.read"}

	var wg sync.WaitGroup
	wg.Add(2)
	var errA, errB error

	go func() {
		defer wg.Done()
		_, errA = h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: targetID,
			ActorUserID:  mA,
			Permissions:  setX,
		})
	}()
	go func() {
		defer wg.Done()
		_, errB = h.svc.UpdateStaffMemberPermissions(h.ctx, staff.UpdateStaffMemberPermissionsInput{
			TargetUserID: targetID,
			ActorUserID:  mB,
			Permissions:  setY,
		})
	}()

	wg.Wait()
	require.NoError(t, errA)
	require.NoError(t, errB)

	finalPerms, err := h.staffRepo.GetMemberPermissions(h.ctx, targetID)
	require.NoError(t, err)

	// Due to serialization under LockOwnerRole, contract is serial last-writer-wins.
	// Final state must equal exactly setX OR exactly setY, never union or partial!
	isX := len(finalPerms) == 1 && finalPerms[0] == "orders.read"
	isY := len(finalPerms) == 1 && finalPerms[0] == "analytics.read"
	require.True(t, isX || isY, "final permissions must equal exactly setX or setY; got: %v", finalPerms)
}

// 17. PUT /staff/members/{userId}/permissions — Atomic Rollback (Section 14)
func TestEMP1B3_PutPermissions_AtomicRollback(t *testing.T) {
	h := setupEMP1B3Harness(t)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")
	originalPerms := []string{"orders.read", "products.read"}
	setDirectPerms(t, h.ctx, h.client, targetID, originalPerms...)

	// Begin manual transaction to demonstrate rollback atomicity
	tx, err := h.client.Pool.Begin(h.ctx)
	require.NoError(t, err)

	txRepo := h.staffRepo.WithTx(tx)
	// Mutate inside transaction
	err = txRepo.ReplaceMemberPermissions(h.ctx, targetID, []string{"analytics.read"})
	require.NoError(t, err)

	// Verify uncommitted state has new permission
	uncommittedPerms, err := txRepo.GetMemberPermissions(h.ctx, targetID)
	require.NoError(t, err)
	require.Equal(t, []string{"analytics.read"}, uncommittedPerms)

	// Abort/rollback transaction
	err = tx.Rollback(h.ctx)
	require.NoError(t, err)

	// Assert that outside transaction, exact original set remains
	permsAfter, err := h.staffRepo.GetMemberPermissions(h.ctx, targetID)
	require.NoError(t, err)
	require.ElementsMatch(t, originalPerms, permsAfter, "rollback must restore exact original direct permissions")
}

// 18. PUT /staff/members/{userId}/permissions — Audit Event (Section 15)
func TestEMP1B3_PutPermissions_AuditEvent(t *testing.T) {
	h := setupEMP1B3Harness(t)

	managerID, _ := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, managerID, staff.PermissionStaffPermissionsManage)

	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), "userID", managerID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Put("/api/admin/staff/members/{userId}/permissions", h.handler.UpdateStaffMemberPermissions)

	callPut := func(body string) int {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/staff/members/%s/permissions", targetID), bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	// 1. Successful PUT produces staff.permissions_update audit event
	code := callPut(`{"permissions": ["orders.read"]}`)
	require.Equal(t, http.StatusOK, code)

	var auditCount int
	err := h.client.Pool.QueryRow(h.ctx, `
		SELECT count(*) FROM audit_logs
		WHERE action = 'staff.permissions_update' AND entity_id = $1
	`, targetID.String()).Scan(&auditCount)
	require.NoError(t, err)
	require.Equal(t, 1, auditCount)

	// 2. Failed PUT (unknown capability) must NOT record an audit event
	code = callPut(`{"permissions": ["bogus.perm"]}`)
	require.Equal(t, http.StatusBadRequest, code)

	var auditCountAfter int
	err = h.client.Pool.QueryRow(h.ctx, `
		SELECT count(*) FROM audit_logs
		WHERE action = 'staff.permissions_update' AND entity_id = $1
	`, targetID.String()).Scan(&auditCountAfter)
	require.NoError(t, err)
	require.Equal(t, 1, auditCountAfter, "audit count must not increase on failed mutation")
}

// 19. REAL ROUTER ACCEPTANCE MATRIX A-J (Section 6, 7, 8)
func TestEMP1B3_RealRouter_AcceptanceMatrix(t *testing.T) {
	h := setupEMP1B3Harness(t)

	// Active non-owner actor WITH direct staff.permissions.manage
	managerID, managerEmail := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, managerID, staff.PermissionStaffPermissionsManage)
	managerToken := makeStaffToken(t, h.tokenService, managerID, managerEmail)

	// Owner role actor WITHOUT direct staff.permissions.manage
	ownerID, ownerEmail := createB3Staff(t, h.ctx, h.client, "owner")
	_, err := h.client.Pool.Exec(h.ctx, `DELETE FROM staff_member_permissions WHERE user_id = $1 AND permission = $2`, ownerID, staff.PermissionStaffPermissionsManage)
	require.NoError(t, err)
	ownerNoManageToken := makeStaffToken(t, h.tokenService, ownerID, ownerEmail)

	// Active actor WITHOUT direct staff.permissions.manage (ordinary admin)
	ordinaryAdminID, ordinaryAdminEmail := createB3Staff(t, h.ctx, h.client, "admin")
	setDirectPerms(t, h.ctx, h.client, ordinaryAdminID, "orders.read")
	ordinaryAdminToken := makeStaffToken(t, h.tokenService, ordinaryAdminID, ordinaryAdminEmail)

	// Target user (support)
	targetID, _ := createB3Staff(t, h.ctx, h.client, "support")
	setDirectPerms(t, h.ctx, h.client, targetID, "support.read")

	initialMember, initialRole, err := h.staffRepo.GetStaffMemberByUserID(h.ctx, targetID)
	require.NoError(t, err)

	// Helper to send requests through real router
	sendRequest := func(method, path string, token string, body []byte) (int, []byte) {
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req := httptest.NewRequest(method, path, reader)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)
		return rec.Code, rec.Body.Bytes()
	}

	// A. GET — active non-owner actor WITH direct staff.permissions.manage → 200
	t.Run("A_GET_ActiveNonOwnerWithManage_200", func(t *testing.T) {
		path := fmt.Sprintf("/api/admin/staff/members/%s/permissions", targetID)
		code, respBody := sendRequest(http.MethodGet, path, managerToken, nil)
		require.Equal(t, http.StatusOK, code)

		var resp struct {
			UserID      uuid.UUID `json:"userId"`
			Permissions []string  `json:"permissions"`
		}
		err := json.Unmarshal(respBody, &resp)
		require.NoError(t, err)
		require.Equal(t, targetID, resp.UserID)
		require.Equal(t, []string{"support.read"}, resp.Permissions)
	})

	// B. GET — owner role actor WITHOUT direct staff.permissions.manage → 403
	t.Run("B_GET_OwnerWithoutManage_403", func(t *testing.T) {
		path := fmt.Sprintf("/api/admin/staff/members/%s/permissions", targetID)
		code, _ := sendRequest(http.MethodGet, path, ownerNoManageToken, nil)
		require.Equal(t, http.StatusForbidden, code)
	})

	// C. PUT — active non-owner actor WITH direct staff.permissions.manage → 200 + DB PROOF (Section 7)
	t.Run("C_PUT_ActiveNonOwnerWithManage_200_AndDBProof", func(t *testing.T) {
		path := fmt.Sprintf("/api/admin/staff/members/%s/permissions", targetID)
		payload := `{"permissions": ["analytics.read", "orders.read"]}`
		code, respBody := sendRequest(http.MethodPut, path, managerToken, []byte(payload))
		require.Equal(t, http.StatusOK, code)

		var resp struct {
			UserID      uuid.UUID `json:"userId"`
			Permissions []string  `json:"permissions"`
		}
		err := json.Unmarshal(respBody, &resp)
		require.NoError(t, err)
		require.Equal(t, targetID, resp.UserID)
		require.Equal(t, []string{"analytics.read", "orders.read"}, resp.Permissions)

		// Post-mutation direct DB verification (Section 7)
		directPerms, err := h.staffRepo.GetMemberPermissions(h.ctx, targetID)
		require.NoError(t, err)
		require.Equal(t, []string{"analytics.read", "orders.read"}, directPerms, "DB permissions must equal exact submitted canonical set")

		memberAfter, _, err := h.staffRepo.GetStaffMemberByUserID(h.ctx, targetID)
		require.NoError(t, err)
		require.Equal(t, initialMember.StaffRoleID, memberAfter.StaffRoleID, "staff_role_id must be unchanged")
		require.Equal(t, initialRole.ID, memberAfter.StaffRoleID)
	})

	// D. PUT — actor WITHOUT direct manage → 403
	t.Run("D_PUT_ActorWithoutManage_403", func(t *testing.T) {
		path := fmt.Sprintf("/api/admin/staff/members/%s/permissions", targetID)
		payload := `{"permissions": ["orders.read"]}`
		code, _ := sendRequest(http.MethodPut, path, ordinaryAdminToken, []byte(payload))
		require.Equal(t, http.StatusForbidden, code)
	})

	// E. PUT unknown capability → 400 validation_error
	t.Run("E_PUT_UnknownCapability_400", func(t *testing.T) {
		path := fmt.Sprintf("/api/admin/staff/members/%s/permissions", targetID)
		payload := `{"permissions": ["analytics.read", "bogus.invalid.perm"]}`
		code, respBody := sendRequest(http.MethodPut, path, managerToken, []byte(payload))
		require.Equal(t, http.StatusBadRequest, code)

		var resp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(respBody, &resp)
		require.Equal(t, "validation_error", resp.Error)
	})

	// F. GET missing staff target → 404
	t.Run("F_GET_MissingStaffTarget_404", func(t *testing.T) {
		path := fmt.Sprintf("/api/admin/staff/members/%s/permissions", uuid.New())
		code, _ := sendRequest(http.MethodGet, path, managerToken, nil)
		require.Equal(t, http.StatusNotFound, code)
	})

	// G. PUT missing staff target → 404
	t.Run("G_PUT_MissingStaffTarget_404", func(t *testing.T) {
		path := fmt.Sprintf("/api/admin/staff/members/%s/permissions", uuid.New())
		payload := `{"permissions": ["orders.read"]}`
		code, _ := sendRequest(http.MethodPut, path, managerToken, []byte(payload))
		require.Equal(t, http.StatusNotFound, code)
	})

	// H. PUT removing last active manager → 409 last_permission_manager
	t.Run("H_PUT_RemovingLastActiveManager_409", func(t *testing.T) {
		// managerID is the single active manager
		path := fmt.Sprintf("/api/admin/staff/members/%s/permissions", managerID)
		payload := `{"permissions": []}`
		code, respBody := sendRequest(http.MethodPut, path, managerToken, []byte(payload))
		require.Equal(t, http.StatusConflict, code)

		var resp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(respBody, &resp)
		require.Equal(t, "last_permission_manager", resp.Error)
	})

	// I. permissions:null → 400 validation_error
	t.Run("I_PUT_PermissionsNull_400", func(t *testing.T) {
		path := fmt.Sprintf("/api/admin/staff/members/%s/permissions", targetID)
		payload := `{"permissions": null}`
		code, respBody := sendRequest(http.MethodPut, path, managerToken, []byte(payload))
		require.Equal(t, http.StatusBadRequest, code)

		var resp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(respBody, &resp)
		require.Equal(t, "validation_error", resp.Error)
	})

	// J. permissions field missing → 400 validation_error
	t.Run("J_PUT_PermissionsFieldMissing_400", func(t *testing.T) {
		path := fmt.Sprintf("/api/admin/staff/members/%s/permissions", targetID)
		payload := `{}`
		code, respBody := sendRequest(http.MethodPut, path, managerToken, []byte(payload))
		require.Equal(t, http.StatusBadRequest, code)

		var resp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(respBody, &resp)
		require.Equal(t, "validation_error", resp.Error)
	})
}
