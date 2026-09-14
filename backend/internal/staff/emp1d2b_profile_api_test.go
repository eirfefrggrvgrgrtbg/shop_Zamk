package staff_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
)

func setupEMP1D2BHarness(t *testing.T) (*chi.Mux, *auth.TokenService, *postgres.Client) {
	t.Helper()
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pgClient.Close() })

	testutil.AssertTestDatabase(t, pgClient.Pool)

	// Hard database guard
	var currentDB string
	err = pgClient.Pool.QueryRow(ctx, `SELECT current_database()`).Scan(&currentDB)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", currentDB)

	cfg := &config.Config{}
	cfg.JWT.AccessTokenSecret = "test-jwt-secret"
	cfg.JWT.AccessTokenTTLMinutes = 60
	cfg.App.Env = "test"

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dummyRedis := &redis.Client{Client: goredis.NewClient(&goredis.Options{})}
	realRouter, cancel := app.BuildRouter(ctx, cfg, pgClient, dummyRedis, logger, nil)
	t.Cleanup(cancel)

	tokenService := auth.NewTokenService(cfg.JWT.AccessTokenSecret, cfg.JWT.RefreshTokenSecret, 60)

	return realRouter, tokenService, pgClient
}

func createStaff(t *testing.T, client *postgres.Client, roleCode string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New()
	email := fmt.Sprintf("d2b_%s@test.com", userID.String()[:8])
	var roleID uuid.UUID
	err := client.Pool.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = $1`, roleCode).Scan(&roleID)
	if err != nil {
		roleID = uuid.New()
		_, err = client.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name, description, created_at, updated_at) VALUES ($1, $2, $3, '', NOW(), NOW())`, roleID, roleCode, roleCode)
		require.NoError(t, err)
	}

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, 'Test', $2, 'dummy_hash', 'admin', 'active', false, NOW(), NOW())
	`, userID, email)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
	`, userID, roleID)
	require.NoError(t, err)

	return userID
}

func setPerms(t *testing.T, client *postgres.Client, userID uuid.UUID, perms ...string) {
	t.Helper()
	ctx := context.Background()
	_, err := client.Pool.Exec(ctx, `DELETE FROM staff_member_permissions WHERE user_id = $1`, userID)
	require.NoError(t, err)
	for _, p := range perms {
		_, err := client.Pool.Exec(ctx, `INSERT INTO staff_member_permissions (user_id, permission) VALUES ($1, $2)`, userID, p)
		require.NoError(t, err)
	}
}

func setStatus(t *testing.T, client *postgres.Client, userID uuid.UUID, status string) {
	t.Helper()
	ctx := context.Background()
	_, err := client.Pool.Exec(ctx, `UPDATE staff_members SET status = $1 WHERE user_id = $2`, status, userID)
	require.NoError(t, err)
}

func makeRequest(t *testing.T, router *chi.Mux, tokenSvc *auth.TokenService, actorID uuid.UUID, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(b)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")

	if actorID != uuid.Nil {
		token, err := tokenSvc.GenerateAccessToken(actorID, "test@test.com", "admin")
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestEMP1D2B_CompleteAcceptanceMatrix(t *testing.T) {
	router, tokenSvc, client := setupEMP1D2BHarness(t)
	ctx := context.Background()

	readerID := createStaff(t, client, "manager")
	setPerms(t, client, readerID, staff.PermissionStaffRead)

	updaterID := createStaff(t, client, "manager")
	setPerms(t, client, updaterID, staff.PermissionStaffRead, staff.PermissionStaffUpdate)

	noneID := createStaff(t, client, "manager")
	setPerms(t, client, noneID) // no perms

	targetID := createStaff(t, client, "manager")

	// A. GET with staff.read = 200
	t.Run("A. GET with staff.read = 200", func(t *testing.T) {
		rec := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+targetID.String(), nil)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	// B. GET without staff.read = 403
	t.Run("B. GET without staff.read = 403", func(t *testing.T) {
		rec := makeRequest(t, router, tokenSvc, noneID, "GET", "/api/admin/staff/members/"+targetID.String(), nil)
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	// C. GET unknown target = 404
	t.Run("C. GET unknown target = 404", func(t *testing.T) {
		rec := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+uuid.NewString(), nil)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	// D. GET returns responsibilities/workNote
	t.Run("D. GET returns responsibilities/workNote", func(t *testing.T) {
		rec := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+targetID.String(), nil)
		require.Equal(t, http.StatusOK, rec.Code)
		var res map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &res)
		require.NoError(t, err)
		require.Contains(t, res, "responsibilities")
		require.Contains(t, res, "workNote")
	})

	// E. PATCH responsibilities persists
	t.Run("E. PATCH responsibilities persists", func(t *testing.T) {
		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": "Supervising receiving operations",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		rec2 := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+targetID.String(), nil)
		var res map[string]any
		json.Unmarshal(rec2.Body.Bytes(), &res)
		require.Equal(t, "Supervising receiving operations", res["responsibilities"])
	})

	// F. PATCH workNote persists
	t.Run("F. PATCH workNote persists", func(t *testing.T) {
		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"workNote": "Works Monday to Thursday",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		rec2 := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+targetID.String(), nil)
		var res map[string]any
		json.Unmarshal(rec2.Body.Bytes(), &res)
		require.Equal(t, "Works Monday to Thursday", res["workNote"])
	})

	// G. PATCH without staff.update = 403
	t.Run("G. PATCH without staff.update = 403", func(t *testing.T) {
		rec := makeRequest(t, router, tokenSvc, readerID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": "Unauthorized update",
		})
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	// H. trim works
	t.Run("H. trim works", func(t *testing.T) {
		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": "   Trimmed Duty   ",
			"workNote":         "   Trimmed Note   ",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		rec2 := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+targetID.String(), nil)
		var res map[string]any
		json.Unmarshal(rec2.Body.Bytes(), &res)
		require.Equal(t, "Trimmed Duty", res["responsibilities"])
		require.Equal(t, "Trimmed Note", res["workNote"])
	})

	// I. whitespace-only string -> NULL
	t.Run("I. whitespace-only string -> NULL", func(t *testing.T) {
		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": "     ",
			"workNote":         "\t  \n  ",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		rec2 := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+targetID.String(), nil)
		var res map[string]any
		json.Unmarshal(rec2.Body.Bytes(), &res)
		require.Nil(t, res["responsibilities"])
		require.Nil(t, res["workNote"])
	})

	// J. explicit null -> NULL
	t.Run("J. explicit null -> NULL", func(t *testing.T) {
		// First set values
		makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": "Initial",
			"workNote":         "Initial",
		})

		// Explicit null
		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": nil,
			"workNote":         nil,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		rec2 := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+targetID.String(), nil)
		var res map[string]any
		json.Unmarshal(rec2.Body.Bytes(), &res)
		require.Nil(t, res["responsibilities"])
		require.Nil(t, res["workNote"])
	})

	// K. omitted responsibilities preserves old responsibilities
	t.Run("K. omitted responsibilities preserves old responsibilities", func(t *testing.T) {
		makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": "Preserved Responsibility",
			"workNote":         "Original Note",
		})

		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"workNote": "Updated Note Only",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		rec2 := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+targetID.String(), nil)
		var res map[string]any
		json.Unmarshal(rec2.Body.Bytes(), &res)
		require.Equal(t, "Preserved Responsibility", res["responsibilities"])
		require.Equal(t, "Updated Note Only", res["workNote"])
	})

	// L. omitted workNote preserves old workNote
	t.Run("L. omitted workNote preserves old workNote", func(t *testing.T) {
		makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": "Original Resp",
			"workNote":         "Preserved Work Note",
		})

		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": "Updated Resp Only",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		rec2 := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+targetID.String(), nil)
		var res map[string]any
		json.Unmarshal(rec2.Body.Bytes(), &res)
		require.Equal(t, "Updated Resp Only", res["responsibilities"])
		require.Equal(t, "Preserved Work Note", res["workNote"])
	})

	// M. {} -> 400 validation_error
	t.Run("M. {} -> 400 validation_error", func(t *testing.T) {
		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{})
		require.Equal(t, http.StatusBadRequest, rec.Code)
		var res map[string]string
		json.Unmarshal(rec.Body.Bytes(), &res)
		require.Equal(t, "validation_error", res["error"])
	})

	// N. unknown-only payload -> 400 invalid_request
	t.Run("N. unknown-only payload -> 400 invalid_request", func(t *testing.T) {
		// unknown-only
		rec1 := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"foo": "bar",
		})
		require.Equal(t, http.StatusBadRequest, rec1.Code)
		var res1 map[string]string
		json.Unmarshal(rec1.Body.Bytes(), &res1)
		require.Equal(t, "invalid_request", res1["error"])

		// mixed unknown with valid field
		rec2 := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": "Should not pass",
			"unexpectedField":  123,
		})
		require.Equal(t, http.StatusBadRequest, rec2.Code)
		var res2 map[string]string
		json.Unmarshal(rec2.Body.Bytes(), &res2)
		require.Equal(t, "invalid_request", res2["error"])
	})

	// O. exactly 4000 Unicode characters accepted (multibyte Cyrillic)
	t.Run("O. exactly 4000 Unicode characters accepted", func(t *testing.T) {
		// 'Ж' is 2 bytes in UTF-8. 4000 runes = 8000 bytes.
		multibyte4000 := strings.Repeat("Ж", 4000)
		require.Equal(t, 4000, len([]rune(multibyte4000)))
		require.Equal(t, 8000, len(multibyte4000))

		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": multibyte4000,
			"workNote":         multibyte4000,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		rec2 := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+targetID.String(), nil)
		var res map[string]any
		json.Unmarshal(rec2.Body.Bytes(), &res)
		require.Equal(t, multibyte4000, res["responsibilities"])
		require.Equal(t, multibyte4000, res["workNote"])
	})

	// P. 4001 Unicode characters rejected
	t.Run("P. 4001 Unicode characters rejected", func(t *testing.T) {
		multibyte4001 := strings.Repeat("Ж", 4001)
		require.Equal(t, 4001, len([]rune(multibyte4001)))

		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"responsibilities": multibyte4001,
		})
		require.Equal(t, http.StatusBadRequest, rec.Code)
		var res map[string]string
		json.Unmarshal(rec.Body.Bytes(), &res)
		require.Equal(t, "validation_error", res["error"])

		recWorkNote := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+targetID.String()+"/profile", map[string]any{
			"workNote": multibyte4001,
		})
		require.Equal(t, http.StatusBadRequest, recWorkNote.Code)
	})

	// Q. PATCH unknown target = 404
	t.Run("Q. PATCH unknown target = 404", func(t *testing.T) {
		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+uuid.NewString()+"/profile", map[string]any{
			"responsibilities": "Valid text",
		})
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	// R. blocked target editable
	t.Run("R. blocked target editable", func(t *testing.T) {
		blockedID := createStaff(t, client, "manager")
		setStatus(t, client, blockedID, "blocked")

		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+blockedID.String()+"/profile", map[string]any{
			"responsibilities": "Blocked profile duty",
			"workNote":         "Account blocked pending review",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		rec2 := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+blockedID.String(), nil)
		var res map[string]any
		json.Unmarshal(rec2.Body.Bytes(), &res)
		require.Equal(t, "Blocked profile duty", res["responsibilities"])
		require.Equal(t, "Account blocked pending review", res["workNote"])
		require.Equal(t, "blocked", res["staffStatus"])
	})

	// S. archived target editable
	t.Run("S. archived target editable", func(t *testing.T) {
		archivedID := createStaff(t, client, "manager")
		setStatus(t, client, archivedID, "archived")

		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+archivedID.String()+"/profile", map[string]any{
			"responsibilities": "Archived historical duties",
			"workNote":         "Former employee 2025",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		rec2 := makeRequest(t, router, tokenSvc, readerID, "GET", "/api/admin/staff/members/"+archivedID.String(), nil)
		var res map[string]any
		json.Unmarshal(rec2.Body.Bytes(), &res)
		require.Equal(t, "Archived historical duties", res["responsibilities"])
		require.Equal(t, "Former employee 2025", res["workNote"])
		require.Equal(t, "archived", res["staffStatus"])
	})

	// T - X. Isolation proof: role, status, direct permissions, password_hash, must_change_password remain unchanged
	t.Run("T - X. Isolation proof: role, status, perms, password unchanged", func(t *testing.T) {
		isoTargetID := createStaff(t, client, "manager")
		setPerms(t, client, isoTargetID, "orders.read", "inventory.read")

		// Read baseline before update
		var roleIDBefore uuid.UUID
		var staffStatusBefore string
		err := client.Pool.QueryRow(ctx, `SELECT staff_role_id, status FROM staff_members WHERE user_id = $1`, isoTargetID).Scan(&roleIDBefore, &staffStatusBefore)
		require.NoError(t, err)

		var passHashBefore string
		var mustChangeBefore bool
		err = client.Pool.QueryRow(ctx, `SELECT password_hash, must_change_password FROM users WHERE id = $1`, isoTargetID).Scan(&passHashBefore, &mustChangeBefore)
		require.NoError(t, err)

		var permsBefore []string
		rows, err := client.Pool.Query(ctx, `SELECT permission FROM staff_member_permissions WHERE user_id = $1 ORDER BY permission`, isoTargetID)
		require.NoError(t, err)
		for rows.Next() {
			var p string
			require.NoError(t, rows.Scan(&p))
			permsBefore = append(permsBefore, p)
		}
		rows.Close()
		require.ElementsMatch(t, []string{"inventory.read", "orders.read"}, permsBefore)

		// Perform profile update
		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+isoTargetID.String()+"/profile", map[string]any{
			"responsibilities": "Isolated update responsibilities",
			"workNote":         "Isolated update work note",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		// Read after update and assert exact identity
		var roleIDAfter uuid.UUID
		var staffStatusAfter string
		err = client.Pool.QueryRow(ctx, `SELECT staff_role_id, status FROM staff_members WHERE user_id = $1`, isoTargetID).Scan(&roleIDAfter, &staffStatusAfter)
		require.NoError(t, err)

		var passHashAfter string
		var mustChangeAfter bool
		err = client.Pool.QueryRow(ctx, `SELECT password_hash, must_change_password FROM users WHERE id = $1`, isoTargetID).Scan(&passHashAfter, &mustChangeAfter)
		require.NoError(t, err)

		var permsAfter []string
		rows, err = client.Pool.Query(ctx, `SELECT permission FROM staff_member_permissions WHERE user_id = $1 ORDER BY permission`, isoTargetID)
		require.NoError(t, err)
		for rows.Next() {
			var p string
			require.NoError(t, rows.Scan(&p))
			permsAfter = append(permsAfter, p)
		}
		rows.Close()

		// T. role unchanged
		require.Equal(t, roleIDBefore, roleIDAfter)
		// U. staff status unchanged
		require.Equal(t, staffStatusBefore, staffStatusAfter)
		// V. direct permissions unchanged
		require.Equal(t, permsBefore, permsAfter)
		// W. password_hash unchanged
		require.Equal(t, passHashBefore, passHashAfter)
		// X. must_change_password unchanged
		require.Equal(t, mustChangeBefore, mustChangeAfter)
	})

	// Y. successful update writes staff.profile_update audit
	t.Run("Y. successful update writes staff.profile_update audit", func(t *testing.T) {
		auditTargetID := createStaff(t, client, "manager")
		_, err := client.Pool.Exec(ctx, `DELETE FROM audit_logs WHERE entity_id = $1`, auditTargetID.String())
		require.NoError(t, err)

		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+auditTargetID.String()+"/profile", map[string]any{
			"responsibilities": "New duty",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		var count int
		err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE entity_id = $1 AND action = 'staff.profile_update'`, auditTargetID.String()).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	// Z - AA. audit metadata contains ONLY bounded changed flags, no bodies
	t.Run("Z - AA. audit metadata contains ONLY bounded changed flags, no bodies", func(t *testing.T) {
		zaTargetID := createStaff(t, client, "manager")
		_, err := client.Pool.Exec(ctx, `DELETE FROM audit_logs WHERE entity_id = $1`, zaTargetID.String())
		require.NoError(t, err)

		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+zaTargetID.String()+"/profile", map[string]any{
			"responsibilities": "Audit privacy test duty",
			"workNote":         "Audit privacy test note",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		var metadata map[string]any
		err = client.Pool.QueryRow(ctx, `SELECT metadata FROM audit_logs WHERE entity_id = $1 AND action = 'staff.profile_update' ORDER BY created_at DESC LIMIT 1`, zaTargetID.String()).Scan(&metadata)
		require.NoError(t, err)

		// Z. Contains only bounded flags
		require.Equal(t, true, metadata["responsibilitiesChanged"])
		require.Equal(t, true, metadata["workNoteChanged"])

		// AA. Contains no text bodies
		require.NotContains(t, metadata, "responsibilities")
		require.NotContains(t, metadata, "workNote")
		require.NotContains(t, metadata, "oldResponsibilities")
		require.NotContains(t, metadata, "newResponsibilities")
		require.NotContains(t, metadata, "oldWorkNote")
		require.NotContains(t, metadata, "newWorkNote")
	})

	// AB. validation failure produces NO staff.profile_update success audit
	t.Run("AB. validation failure produces NO staff.profile_update success audit", func(t *testing.T) {
		abTargetID := createStaff(t, client, "manager")
		_, err := client.Pool.Exec(ctx, `DELETE FROM audit_logs WHERE entity_id = $1`, abTargetID.String())
		require.NoError(t, err)

		// Validation error: 4001 characters
		longStr := strings.Repeat("я", 4001)
		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+abTargetID.String()+"/profile", map[string]any{
			"responsibilities": longStr,
		})
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var count int
		err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE entity_id = $1 AND action = 'staff.profile_update'`, abTargetID.String()).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	// AC. unknown-field failure produces NO staff.profile_update audit
	t.Run("AC. unknown-field failure produces NO staff.profile_update audit", func(t *testing.T) {
		acTargetID := createStaff(t, client, "manager")
		_, err := client.Pool.Exec(ctx, `DELETE FROM audit_logs WHERE entity_id = $1`, acTargetID.String())
		require.NoError(t, err)

		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+acTargetID.String()+"/profile", map[string]any{
			"unknownKey": "forbidden",
		})
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var count int
		err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE entity_id = $1 AND action = 'staff.profile_update'`, acTargetID.String()).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	// AD. forced audit INSERT failure rolls back profile UPDATE
	t.Run("AD. forced audit INSERT failure rolls back profile UPDATE", func(t *testing.T) {
		adTargetID := createStaff(t, client, "manager")

		// Hard guard on database name
		var curDB string
		err := client.Pool.QueryRow(ctx, `SELECT current_database()`).Scan(&curDB)
		require.NoError(t, err)
		require.Equal(t, "zamk_test", curDB)

		// Create isolated trigger function targeting ONLY adTargetID
		triggerSQL := fmt.Sprintf(`
			CREATE OR REPLACE FUNCTION fail_audit_ad() RETURNS TRIGGER AS $$
			BEGIN
				IF NEW.entity_id = '%s'::uuid THEN
					RAISE EXCEPTION 'forced_audit_insert_failure';
				END IF;
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql;

			DROP TRIGGER IF EXISTS test_fail_audit_ad ON audit_logs;
			CREATE TRIGGER test_fail_audit_ad BEFORE INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION fail_audit_ad();
		`, adTargetID.String())

		_, err = client.Pool.Exec(ctx, triggerSQL)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.Pool.Exec(ctx, `DROP TRIGGER IF EXISTS test_fail_audit_ad ON audit_logs; DROP FUNCTION IF EXISTS fail_audit_ad();`)
		})

		// Perform update which should fail during audit insert
		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+adTargetID.String()+"/profile", map[string]any{
			"responsibilities": "Will be rolled back",
			"workNote":         "Will be rolled back",
		})
		require.Equal(t, http.StatusInternalServerError, rec.Code)

		// Verify rollback on staff_members: fields remain NULL
		var dbResp *string
		var dbNote *string
		err = client.Pool.QueryRow(ctx, `SELECT responsibilities, work_note FROM staff_members WHERE user_id = $1`, adTargetID).Scan(&dbResp, &dbNote)
		require.NoError(t, err)
		require.Nil(t, dbResp)
		require.Nil(t, dbNote)

		// Verify no audit row persisted
		var auditCount int
		err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE entity_id = $1`, adTargetID.String()).Scan(&auditCount)
		require.NoError(t, err)
		require.Equal(t, 0, auditCount)
	})

	// AE. successful transaction persists BOTH: profile change and matching audit row
	t.Run("AE. successful transaction persists BOTH: profile change and matching audit row", func(t *testing.T) {
		aeTargetID := createStaff(t, client, "manager")
		_, err := client.Pool.Exec(ctx, `DELETE FROM audit_logs WHERE entity_id = $1`, aeTargetID.String())
		require.NoError(t, err)

		rec := makeRequest(t, router, tokenSvc, updaterID, "PATCH", "/api/admin/staff/members/"+aeTargetID.String()+"/profile", map[string]any{
			"responsibilities": "Persisted Duty AE",
			"workNote":         "Persisted Note AE",
		})
		require.Equal(t, http.StatusOK, rec.Code)

		// 1. Check profile change in database
		var dbResp *string
		var dbNote *string
		err = client.Pool.QueryRow(ctx, `SELECT responsibilities, work_note FROM staff_members WHERE user_id = $1`, aeTargetID).Scan(&dbResp, &dbNote)
		require.NoError(t, err)
		require.NotNil(t, dbResp)
		require.NotNil(t, dbNote)
		require.Equal(t, "Persisted Duty AE", *dbResp)
		require.Equal(t, "Persisted Note AE", *dbNote)

		// 2. Check matching audit log row in database
		var actorUserID uuid.UUID
		var action string
		var entityID *uuid.UUID
		var meta map[string]any
		err = client.Pool.QueryRow(ctx, `
			SELECT actor_user_id, action, entity_id, metadata
			FROM audit_logs
			WHERE entity_id = $1 AND action = 'staff.profile_update'
		`, aeTargetID.String()).Scan(&actorUserID, &action, &entityID, &meta)
		require.NoError(t, err)
		require.Equal(t, updaterID, actorUserID)
		require.Equal(t, "staff.profile_update", action)
		require.NotNil(t, entityID)
		require.Equal(t, aeTargetID, *entityID)
		require.Equal(t, true, meta["responsibilitiesChanged"])
		require.Equal(t, true, meta["workNoteChanged"])
	})
}
