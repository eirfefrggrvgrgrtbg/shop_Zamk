package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func setupSessionIsolationEnv(t *testing.T) (context.Context, http.Handler, func(), *postgres.Client) {
	ctx := context.Background()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
		CORS: config.CORSConfig{
			AllowedOrigins: []string{
				"http://127.0.0.1:3000",
				"http://127.0.0.1:3001",
				"http://127.0.0.1:3002",
			},
		},
		Worker: config.WorkerConfig{MarketplaceCommissionBPS: 1500},
	}

	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)

	// Invariant check: only execute against zamk_test
	var dbName string
	err = pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "tests must strictly run against zamk_test")

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)

	cleanup := func() {
		cancel()
		redisClient.Close()
		pgClient.Close()
	}

	return ctx, r, cleanup, pgClient
}

func createIsolationUsers(t *testing.T, ctx context.Context, pgClient *postgres.Client) (customerEmail, sellerEmail, adminEmail string) {
	hash, err := auth.HashPassword("Password123!")
	require.NoError(t, err)

	createTestUser := func(role, email string) uuid.UUID {
		id := uuid.New()
		phone := fmt.Sprintf("+7999%07d", id.ID()%10000000)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'active', NOW(), NOW())
			ON CONFLICT (email) DO UPDATE SET password_hash = $5, role = $6, status = 'active'
			RETURNING id
		`, id, email, phone, "Test "+role, hash, role)
		require.NoError(t, err)
		return id
	}

	customerEmail = "isolation_customer@zamk.local"
	sellerEmail = "isolation_seller@zamk.local"
	adminEmail = "isolation_admin@zamk.local"

	_ = createTestUser("customer", customerEmail)
	_ = createTestUser("seller", sellerEmail)
	_ = createTestUser("admin", adminEmail)
	return customerEmail, sellerEmail, adminEmail
}

func sendLogin(r http.Handler, email, scope, origin string) *http.Response {
	body, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": "Password123!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if scope != "" {
		req.Header.Set("X-Zamk-App", scope)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Result()
}

func sendRefresh(r http.Handler, scope, origin string, cookies []*http.Cookie) (*http.Response, map[string]any) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	if scope != "" {
		req.Header.Set("X-Zamk-App", scope)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var respData map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&respData)
	return rec.Result(), respData
}

func sendLogout(r http.Handler, scope, origin string, cookies []*http.Cookie) *http.Response {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	if scope != "" {
		req.Header.Set("X-Zamk-App", scope)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Result()
}

func extractCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestSessionIsolationBetweenApps(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx, r, cleanup, pgClient := setupSessionIsolationEnv(t)
	defer cleanup()

	customerEmail, sellerEmail, adminEmail := createIsolationUsers(t, ctx, pgClient)

	// 1. Coexistence of Shop, Seller, Admin sessions on the same origin / localhost
	resCustomer := sendLogin(r, customerEmail, "shop", "http://127.0.0.1:3000")
	require.Equal(t, http.StatusOK, resCustomer.StatusCode)
	shopCookie := extractCookie(resCustomer, auth.CookieShopSession)
	require.NotNil(t, shopCookie, "Shop login must set zamk_shop_session cookie")
	assert.NotEmpty(t, shopCookie.Value)

	resAdmin := sendLogin(r, adminEmail, "admin", "http://127.0.0.1:3002")
	require.Equal(t, http.StatusOK, resAdmin.StatusCode)
	adminCookie := extractCookie(resAdmin, auth.CookieAdminSession)
	require.NotNil(t, adminCookie, "Admin login must set zamk_admin_session cookie")
	assert.NotEmpty(t, adminCookie.Value)

	resSeller := sendLogin(r, sellerEmail, "seller", "http://127.0.0.1:3001")
	require.Equal(t, http.StatusOK, resSeller.StatusCode)
	sellerCookie := extractCookie(resSeller, auth.CookieSellerSession)
	require.NotNil(t, sellerCookie, "Seller login must set zamk_seller_session cookie")
	assert.NotEmpty(t, sellerCookie.Value)

	// 2. Strict Role Guards on Login:
	// Admin cannot login via shop scope
	assert.Equal(t, http.StatusForbidden, sendLogin(r, adminEmail, "shop", "http://127.0.0.1:3000").StatusCode)
	// Seller cannot login via shop scope
	assert.Equal(t, http.StatusForbidden, sendLogin(r, sellerEmail, "shop", "http://127.0.0.1:3000").StatusCode)
	// Customer cannot login via admin scope
	assert.Equal(t, http.StatusForbidden, sendLogin(r, customerEmail, "admin", "http://127.0.0.1:3002").StatusCode)
	// Customer cannot login via seller scope
	assert.Equal(t, http.StatusForbidden, sendLogin(r, customerEmail, "seller", "http://127.0.0.1:3001").StatusCode)
	// Admin cannot login via seller scope
	assert.Equal(t, http.StatusForbidden, sendLogin(r, adminEmail, "seller", "http://127.0.0.1:3001").StatusCode)

	// 3. Independent Refresh:
	// Shared cookie jar containing all three session cookies
	jar := []*http.Cookie{shopCookie, adminCookie, sellerCookie}

	// 3A. Shop refresh only reads shop session
	resShopRef, dataShop := sendRefresh(r, "shop", "http://127.0.0.1:3000", jar)
	assert.Equal(t, http.StatusOK, resShopRef.StatusCode)
	userShop := dataShop["user"].(map[string]any)
	assert.Equal(t, "customer", userShop["role"])
	assert.Equal(t, customerEmail, userShop["email"])
	shopCookie = extractCookie(resShopRef, auth.CookieShopSession)
	require.NotNil(t, shopCookie)

	// 3B. Admin refresh only reads admin session
	resAdminRef, dataAdmin := sendRefresh(r, "admin", "http://127.0.0.1:3002", jar)
	assert.Equal(t, http.StatusOK, resAdminRef.StatusCode)
	userAdmin := dataAdmin["user"].(map[string]any)
	assert.Equal(t, "admin", userAdmin["role"])
	assert.Equal(t, adminEmail, userAdmin["email"])
	adminCookie = extractCookie(resAdminRef, auth.CookieAdminSession)
	require.NotNil(t, adminCookie)

	// 3C. Seller refresh only reads seller session
	resSellerRef, dataSeller := sendRefresh(r, "seller", "http://127.0.0.1:3001", jar)
	assert.Equal(t, http.StatusOK, resSellerRef.StatusCode)
	userSeller := dataSeller["user"].(map[string]any)
	assert.Equal(t, "seller", userSeller["role"])
	assert.Equal(t, sellerEmail, userSeller["email"])
	sellerCookie = extractCookie(resSellerRef, auth.CookieSellerSession)
	require.NotNil(t, sellerCookie)

	// Update jar with rotated cookies
	jar = []*http.Cookie{shopCookie, adminCookie, sellerCookie}

	// 4. Three-Way Independent Logout:

	// Case 4A: Logout Admin does not invalidate Shop or Seller
	resLogoutAdmin := sendLogout(r, "admin", "http://127.0.0.1:3002", jar)
	assert.Equal(t, http.StatusOK, resLogoutAdmin.StatusCode)
	clearedAdminCookie := extractCookie(resLogoutAdmin, auth.CookieAdminSession)
	require.NotNil(t, clearedAdminCookie)
	assert.Equal(t, -1, clearedAdminCookie.MaxAge)

	// Admin refresh is now unauthorized
	resAdminCheck, _ := sendRefresh(r, "admin", "http://127.0.0.1:3002", []*http.Cookie{adminCookie})
	assert.Equal(t, http.StatusUnauthorized, resAdminCheck.StatusCode)

	// Shop session remains valid and active
	resShopCheck, dataShopCheck := sendRefresh(r, "shop", "http://127.0.0.1:3000", jar)
	assert.Equal(t, http.StatusOK, resShopCheck.StatusCode)
	assert.Equal(t, "customer", dataShopCheck["user"].(map[string]any)["role"])
	shopCookie = extractCookie(resShopCheck, auth.CookieShopSession)

	// Seller session remains valid and active
	resSellerCheck, dataSellerCheck := sendRefresh(r, "seller", "http://127.0.0.1:3001", jar)
	assert.Equal(t, http.StatusOK, resSellerCheck.StatusCode)
	assert.Equal(t, "seller", dataSellerCheck["user"].(map[string]any)["role"])
	sellerCookie = extractCookie(resSellerCheck, auth.CookieSellerSession)

	// Re-authenticate Admin to restore 3-way coexistence
	resAdminRe := sendLogin(r, adminEmail, "admin", "http://127.0.0.1:3002")
	require.Equal(t, http.StatusOK, resAdminRe.StatusCode)
	adminCookie = extractCookie(resAdminRe, auth.CookieAdminSession)
	jar = []*http.Cookie{shopCookie, adminCookie, sellerCookie}

	// Case 4B: Logout Seller does not invalidate Shop or Admin
	resLogoutSeller := sendLogout(r, "seller", "http://127.0.0.1:3001", jar)
	assert.Equal(t, http.StatusOK, resLogoutSeller.StatusCode)
	clearedSellerCookie := extractCookie(resLogoutSeller, auth.CookieSellerSession)
	require.NotNil(t, clearedSellerCookie)
	assert.Equal(t, -1, clearedSellerCookie.MaxAge)

	// Seller refresh is now unauthorized
	resSellerCheck2, _ := sendRefresh(r, "seller", "http://127.0.0.1:3001", []*http.Cookie{sellerCookie})
	assert.Equal(t, http.StatusUnauthorized, resSellerCheck2.StatusCode)

	// Shop session remains valid and active
	resShopCheck2, dataShopCheck2 := sendRefresh(r, "shop", "http://127.0.0.1:3000", jar)
	assert.Equal(t, http.StatusOK, resShopCheck2.StatusCode)
	assert.Equal(t, "customer", dataShopCheck2["user"].(map[string]any)["role"])
	shopCookie = extractCookie(resShopCheck2, auth.CookieShopSession)

	// Admin session remains valid and active
	resAdminCheck2, dataAdminCheck2 := sendRefresh(r, "admin", "http://127.0.0.1:3002", jar)
	assert.Equal(t, http.StatusOK, resAdminCheck2.StatusCode)
	assert.Equal(t, "admin", dataAdminCheck2["user"].(map[string]any)["role"])
	adminCookie = extractCookie(resAdminCheck2, auth.CookieAdminSession)

	// Re-authenticate Seller to restore 3-way coexistence
	resSellerRe := sendLogin(r, sellerEmail, "seller", "http://127.0.0.1:3001")
	require.Equal(t, http.StatusOK, resSellerRe.StatusCode)
	sellerCookie = extractCookie(resSellerRe, auth.CookieSellerSession)
	jar = []*http.Cookie{shopCookie, adminCookie, sellerCookie}

	// Case 4C: Logout Shop does not invalidate Seller or Admin
	resLogoutShop := sendLogout(r, "shop", "http://127.0.0.1:3000", jar)
	assert.Equal(t, http.StatusOK, resLogoutShop.StatusCode)
	clearedShopCookie := extractCookie(resLogoutShop, auth.CookieShopSession)
	require.NotNil(t, clearedShopCookie)
	assert.Equal(t, -1, clearedShopCookie.MaxAge)

	// Shop refresh is now unauthorized
	resShopCheck3, _ := sendRefresh(r, "shop", "http://127.0.0.1:3000", []*http.Cookie{shopCookie})
	assert.Equal(t, http.StatusUnauthorized, resShopCheck3.StatusCode)

	// Seller session remains valid and active
	resSellerCheck3, dataSellerCheck3 := sendRefresh(r, "seller", "http://127.0.0.1:3001", jar)
	assert.Equal(t, http.StatusOK, resSellerCheck3.StatusCode)
	assert.Equal(t, "seller", dataSellerCheck3["user"].(map[string]any)["role"])

	// Admin session remains valid and active
	resAdminCheck3, dataAdminCheck3 := sendRefresh(r, "admin", "http://127.0.0.1:3002", jar)
	assert.Equal(t, http.StatusOK, resAdminCheck3.StatusCode)
	assert.Equal(t, "admin", dataAdminCheck3["user"].(map[string]any)["role"])
}

func TestSessionIsolation_InvalidAppScope(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx, r, cleanup, pgClient := setupSessionIsolationEnv(t)
	defer cleanup()

	customerEmail, _, _ := createIsolationUsers(t, ctx, pgClient)

	// 1. Invalid scope on Login returns 400 invalid_app_scope
	resLogin := sendLogin(r, customerEmail, "unknown_scope", "http://127.0.0.1:3000")
	assert.Equal(t, http.StatusBadRequest, resLogin.StatusCode)
	var errResp map[string]any
	_ = json.NewDecoder(resLogin.Body).Decode(&errResp)
	errMap, ok := errResp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "invalid_app_scope", errMap["code"])

	// 2. Invalid scope on Refresh returns 400 invalid_app_scope
	resRefresh, dataRefresh := sendRefresh(r, "invalid_app", "http://127.0.0.1:3000", nil)
	assert.Equal(t, http.StatusBadRequest, resRefresh.StatusCode)
	errMapRefresh, ok := dataRefresh["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "invalid_app_scope", errMapRefresh["code"])

	// 3. Invalid scope on Logout returns 400 invalid_app_scope
	resLogout := sendLogout(r, "bad_scope", "http://127.0.0.1:3000", nil)
	assert.Equal(t, http.StatusBadRequest, resLogout.StatusCode)
	var errLogout map[string]any
	_ = json.NewDecoder(resLogout.Body).Decode(&errLogout)
	errMapLogout, ok := errLogout["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "invalid_app_scope", errMapLogout["code"])

	// 4. Invalid scope on ChangePassword returns 400 invalid_app_scope
	resCustLogin := sendLogin(r, customerEmail, "shop", "http://127.0.0.1:3000")
	require.Equal(t, http.StatusOK, resCustLogin.StatusCode)
	var loginData map[string]any
	_ = json.NewDecoder(resCustLogin.Body).Decode(&loginData)
	accessToken, _ := loginData["accessToken"].(string)
	require.NotEmpty(t, accessToken)

	body, _ := json.Marshal(map[string]string{
		"current_password": "Password123!",
		"new_password":     "NewPassword123!",
	})
	reqChange := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", bytes.NewReader(body))
	reqChange.Header.Set("Content-Type", "application/json")
	reqChange.Header.Set("Authorization", "Bearer "+accessToken)
	reqChange.Header.Set("X-Zamk-App", "unsupported_scope")
	recChange := httptest.NewRecorder()
	r.ServeHTTP(recChange, reqChange)
	assert.Equal(t, http.StatusBadRequest, recChange.Code)
	var errChange map[string]any
	_ = json.NewDecoder(recChange.Body).Decode(&errChange)
	errMapChange, ok := errChange["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "invalid_app_scope", errMapChange["code"])
}

func TestSessionIsolation_NoOriginInference(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx, r, cleanup, pgClient := setupSessionIsolationEnv(t)
	defer cleanup()

	customerEmail, _, _ := createIsolationUsers(t, ctx, pgClient)

	// 1. Calling login with Origin / Referer but WITHOUT X-Zamk-App must NOT set zamk_shop_session
	// It must fall back to legacy zamk_refresh_token
	body, _ := json.Marshal(map[string]string{
		"email":    customerEmail,
		"password": "Password123!",
	})
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	reqLogin.Header.Set("Content-Type", "application/json")
	reqLogin.Header.Set("Origin", "http://127.0.0.1:3000")
	reqLogin.Header.Set("Referer", "http://127.0.0.1:3000/catalog")
	// No X-Zamk-App header!
	recLogin := httptest.NewRecorder()
	r.ServeHTTP(recLogin, reqLogin)

	resLogin := recLogin.Result()
	require.Equal(t, http.StatusOK, resLogin.StatusCode)

	// Verify zamk_shop_session was NOT set
	shopCookie := extractCookie(resLogin, auth.CookieShopSession)
	assert.Nil(t, shopCookie, "Must NOT set zamk_shop_session when X-Zamk-App is absent, even if Origin/Referer is localhost:3000")

	// Legacy cookie MUST be set
	legacyCookie := extractCookie(resLogin, auth.CookieLegacySession)
	require.NotNil(t, legacyCookie, "Legacy callers without X-Zamk-App must receive zamk_refresh_token")
	assert.NotEmpty(t, legacyCookie.Value)

	// 2. Refresh with Origin: 3000 but WITHOUT X-Zamk-App:
	// If only zamk_shop_session is passed in cookies, it must return 401 because it does NOT infer scope from Origin
	reqRefreshNoScope := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	reqRefreshNoScope.Header.Set("Origin", "http://127.0.0.1:3000")
	reqRefreshNoScope.AddCookie(&http.Cookie{
		Name:  auth.CookieShopSession,
		Value: "some-fake-or-real-shop-session",
	})
	recRefreshNoScope := httptest.NewRecorder()
	r.ServeHTTP(recRefreshNoScope, reqRefreshNoScope)
	// Since X-Zamk-App is absent, it only checks zamk_refresh_token, which is absent -> 401 Unauthorized
	assert.Equal(t, http.StatusUnauthorized, recRefreshNoScope.Code)

	// 3. Refresh with Origin: 3000 without X-Zamk-App using legacyCookie succeeds
	reqRefreshLegacy := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	reqRefreshLegacy.Header.Set("Origin", "http://127.0.0.1:3000")
	reqRefreshLegacy.AddCookie(legacyCookie)
	recRefreshLegacy := httptest.NewRecorder()
	r.ServeHTTP(recRefreshLegacy, reqRefreshLegacy)
	assert.Equal(t, http.StatusOK, recRefreshLegacy.Code)
	rotatedLegacy := extractCookie(recRefreshLegacy.Result(), auth.CookieLegacySession)
	require.NotNil(t, rotatedLegacy, "Legacy refresh must rotate zamk_refresh_token")
}
