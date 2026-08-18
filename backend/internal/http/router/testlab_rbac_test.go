package router_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func TestTestLabRBAC(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 60,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
	}
	pgClient, err := postgres.NewClient(ctx, testDBURL)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pgClient.Close()

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("failed to connect to redis: %v", err)
	}
	defer redisClient.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	insertUser := func(t *testing.T, role string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		phone := "7888" + id.String()[:7]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test', 'hash', $4, 'active', NOW(), NOW())
		`, id, id.String()+"@test.com", phone, role)
		if err != nil {
			t.Fatalf("insertUser: %v", err)
		}
		return id
	}

	insertAdminWithPerms := func(t *testing.T, userID uuid.UUID, perms []string) {
		t.Helper()
		roleID := uuid.New()
		code := roleID.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'TestRole')`, roleID, code)
		if err != nil {
			t.Fatalf("insertAdminRole: %v", err)
		}
		for _, p := range perms {
			_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, $2)`, roleID, p)
			if err != nil {
				t.Fatalf("insertPerm %s: %v", p, err)
			}
		}
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, userID, roleID)
		if err != nil {
			t.Fatalf("insertStaffMember: %v", err)
		}
	}

	makeToken := func(t *testing.T, userID uuid.UUID, role string) string {
		t.Helper()
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		if err != nil {
			t.Fatalf("makeToken: %v", err)
		}
		return tok
	}

	t.Run("owner with testing.manage -> allowed", func(t *testing.T) {
		uid := insertUser(t, "admin")
		insertAdminWithPerms(t, uid, []string{"testing.manage"})
		tok := makeToken(t, uid, "admin") // the token role field is 'admin' for staff
		req := httptest.NewRequest("GET", "/api/admin/testing/analytics/scenarios", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code == http.StatusForbidden {
			t.Errorf("expected allowed, got 403")
		}
	})

	t.Run("admin without testing.manage -> forbidden", func(t *testing.T) {
		uid := insertUser(t, "admin")
		insertAdminWithPerms(t, uid, []string{"inventory.read"})
		tok := makeToken(t, uid, "admin")
		req := httptest.NewRequest("GET", "/api/admin/testing/analytics/scenarios", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})
}

func TestTestLabProductionGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 60,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "production"},
	}
	pgClient, err := postgres.NewClient(ctx, testDBURL)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pgClient.Close()

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("failed to connect to redis: %v", err)
	}
	defer redisClient.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	insertUser := func(t *testing.T, role string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		phone := "7999" + id.String()[:7]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test Prod', 'hash', $4, 'active', NOW(), NOW())
		`, id, id.String()+"@prod.com", phone, role)
		if err != nil {
			t.Fatalf("insertUser: %v", err)
		}
		return id
	}

	insertAdminWithPerms := func(t *testing.T, userID uuid.UUID, perms []string) {
		t.Helper()
		roleID := uuid.New()
		code := roleID.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'TestRoleProd')`, roleID, code)
		if err != nil {
			t.Fatalf("insertAdminRole: %v", err)
		}
		for _, p := range perms {
			_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, $2)`, roleID, p)
			if err != nil {
				t.Fatalf("insertPerm %s: %v", p, err)
			}
		}
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, userID, roleID)
		if err != nil {
			t.Fatalf("insertStaffMember: %v", err)
		}
	}

	makeToken := func(t *testing.T, userID uuid.UUID, role string) string {
		t.Helper()
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@prod.com", role)
		if err != nil {
			t.Fatalf("makeToken: %v", err)
		}
		return tok
	}

	t.Run("authorized owner in production -> 404", func(t *testing.T) {
		uid := insertUser(t, "admin")
		insertAdminWithPerms(t, uid, []string{"testing.manage"})
		tok := makeToken(t, uid, "admin")
		req := httptest.NewRequest("GET", "/api/admin/testing/analytics/scenarios", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found in production, got %d", rr.Code)
		}
	})
}
