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

func TestReferenceRoutesAuth(t *testing.T) {
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
		phone := "7999" + id.String()[:7]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test', 'hash', $4, 'active', NOW(), NOW())
		`, id, id.String()+"@test.com", phone, role)
		if err != nil {
			t.Fatalf("insertUser: %v", err)
		}
		return id
	}

	insertSeller := func(t *testing.T, ownerUserID uuid.UUID) uuid.UUID {
		t.Helper()
		sid := uuid.New()
		slug := "test-slug-" + sid.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, 'Test Brand', $2, $3, 'active', NOW(), NOW())
		`, sid, slug, sid.String()+"@seller.com")
		if err != nil {
			t.Fatalf("insertSeller: %v", err)
		}
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
			VALUES ($1, $2, $3, 'owner', NOW())
		`, uuid.New(), sid, ownerUserID)
		if err != nil {
			t.Fatalf("insertSellerUser: %v", err)
		}
		return sid
	}

	makeToken := func(t *testing.T, userID uuid.UUID, role string) string {
		t.Helper()
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		if err != nil {
			t.Fatalf("makeToken: %v", err)
		}
		return tok
	}

	uid := insertUser(t, "seller")
	insertSeller(t, uid)
	tok := makeToken(t, uid, "seller")

	catID := uuid.New()
	sysID := uuid.New()
	dictID := uuid.New()

	routes := []string{
		"/api/seller/reference/categories",
		"/api/seller/reference/categories/" + catID.String() + "/schema",
		"/api/seller/reference/colors",
		"/api/seller/reference/materials",
		"/api/seller/reference/size-systems",
		"/api/seller/reference/size-systems/" + sysID.String() + "/values",
		"/api/seller/reference/dictionaries/" + dictID.String() + "/values",
	}

	for _, route := range routes {
		t.Run("Auth GET "+route, func(t *testing.T) {
			// Unauthenticated -> 401
			req := httptest.NewRequest("GET", route, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", rr.Code)
			}

			// Authenticated Seller -> NOT 401/403
			req = httptest.NewRequest("GET", route, nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			rr = httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			if rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden {
				t.Errorf("expected access, got %d", rr.Code)
			}
		})
	}
}
