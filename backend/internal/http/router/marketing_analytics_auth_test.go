package router_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

func TestAdminMarketingAnalyticsRouter_RBAC(t *testing.T) {
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
	require.NoError(t, err)
	defer pgClient.Close()

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	insertUser := func(role string) uuid.UUID {
		id := uuid.New()
		phone := "7999" + id.String()[:7]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test User', 'hash', $4, 'active', NOW(), NOW())
		`, id, id.String()+"@test.com", phone, role)
		require.NoError(t, err)
		return id
	}

	insertAdminWithPerms := func(userID uuid.UUID, perms []string) {
		_, err := setupRouterStaffWithPermissions(ctx, pgClient.Pool, userID, "MarketingRole", perms)
		require.NoError(t, err)
	}

	makeToken := func(userID uuid.UUID, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		require.NoError(t, err)
		return tok
	}

	adminWithRead := insertUser("admin")
	insertAdminWithPerms(adminWithRead, []string{"marketing.campaigns.read"})
	adminWithReadToken := makeToken(adminWithRead, "admin")

	adminNoPerm := insertUser("admin")
	insertAdminWithPerms(adminNoPerm, []string{"inventory.read"})
	adminNoPermToken := makeToken(adminNoPerm, "admin")

	customerUser := insertUser("customer")
	customerToken := makeToken(customerUser, "customer")

	fromStr := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	toStr := time.Now().UTC().Format(time.RFC3339)
	endpoint := "/api/admin/marketing/analytics/overview?from=" + fromStr + "&to=" + toStr

	// Matrix AD: Read permission allowed
	t.Run("MatrixAD_ReadPermission_Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+adminWithReadToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Admin with marketing.campaigns.read should be allowed")
	})

	// Matrix AE: No permission denied
	t.Run("MatrixAE_NoPermission_Denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+adminNoPermToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code, "Admin without marketing.campaigns.read must be 403 Forbidden")

		// Customer denied
		reqCustomer := httptest.NewRequest(http.MethodGet, endpoint, nil)
		reqCustomer.Header.Set("Authorization", "Bearer "+customerToken)
		wCustomer := httptest.NewRecorder()
		r.ServeHTTP(wCustomer, reqCustomer)
		assert.Equal(t, http.StatusForbidden, wCustomer.Code, "Customer must be 403 Forbidden")

		// Unauthenticated denied
		reqUnauth := httptest.NewRequest(http.MethodGet, endpoint, nil)
		wUnauth := httptest.NewRecorder()
		r.ServeHTTP(wUnauth, reqUnauth)
		assert.Equal(t, http.StatusUnauthorized, wUnauth.Code, "Unauthenticated request must be 401 Unauthorized")
	})

	t.Run("SavedQueries_MatrixAC_Customer_Denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/marketing/saved-queries", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code, "Customer should be forbidden")
	})

	t.Run("SavedQueries_MatrixAD_ReadPermission_Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/marketing/saved-queries", nil)
		req.Header.Set("Authorization", "Bearer "+adminWithReadToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Admin with marketing.campaigns.read should be allowed")
	})

	t.Run("SavedQueries_MatrixAE_NoPermission_Denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/marketing/saved-queries", nil)
		req.Header.Set("Authorization", "Bearer "+adminNoPermToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code, "Admin without read perms should be forbidden")
	})

	t.Run("SavedQueries_MatrixAF_Unauthenticated_Denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/marketing/saved-queries", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "Unauthenticated should be unauthorized")
	})
}

func TestAdminMarketingAnalyticsRouter_SourcesRBACAndValidation(t *testing.T) {
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
	require.NoError(t, err)
	defer pgClient.Close()

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	insertUser := func(role string) uuid.UUID {
		id := uuid.New()
		phone := "7999" + id.String()[:7]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test User', 'hash', $4, 'active', NOW(), NOW())
		`, id, id.String()+"@test.com", phone, role)
		require.NoError(t, err)
		return id
	}

	insertAdminWithPerms := func(userID uuid.UUID, perms []string) {
		_, err := setupRouterStaffWithPermissions(ctx, pgClient.Pool, userID, "MarketingSourcesRole", perms)
		require.NoError(t, err)
	}

	makeToken := func(userID uuid.UUID, role string) string {
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		require.NoError(t, err)
		return tok
	}

	// 1. Admin with marketing.campaigns.read
	adminWithCampaignRead := insertUser("admin")
	insertAdminWithPerms(adminWithCampaignRead, []string{"marketing.campaigns.read"})
	campaignReadToken := makeToken(adminWithCampaignRead, "admin")

	// 2. Admin with analytics.read
	adminWithAnalyticsRead := insertUser("admin")
	insertAdminWithPerms(adminWithAnalyticsRead, []string{"analytics.read"})
	analyticsReadToken := makeToken(adminWithAnalyticsRead, "admin")

	// 3. Admin with only unprivileged permission (inventory.read)
	adminNoPerm := insertUser("admin")
	insertAdminWithPerms(adminNoPerm, []string{"inventory.read"})
	noPermToken := makeToken(adminNoPerm, "admin")

	// 4. Customer
	customerUser := insertUser("customer")
	customerToken := makeToken(customerUser, "customer")

	fromStr := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	toStr := time.Now().UTC().Format(time.RFC3339)

	sourcesEndpoint := "/api/admin/marketing/analytics/sources?from=" + fromStr + "&to=" + toStr
	detailEndpoint := "/api/admin/marketing/analytics/sources/vk?from=" + fromStr + "&to=" + toStr
	unattrDetailEndpoint := "/api/admin/marketing/analytics/sources/_unattributed?from=" + fromStr + "&to=" + toStr

	// A. RBAC for /sources list
	t.Run("Sources_RBAC", func(t *testing.T) {
		// marketing.campaigns.read => 200
		req1 := httptest.NewRequest(http.MethodGet, sourcesEndpoint, nil)
		req1.Header.Set("Authorization", "Bearer "+campaignReadToken)
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code, "Admin with marketing.campaigns.read allowed")

		// analytics.read => 200
		req2 := httptest.NewRequest(http.MethodGet, sourcesEndpoint, nil)
		req2.Header.Set("Authorization", "Bearer "+analyticsReadToken)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code, "Admin with analytics.read allowed")

		// Unprivileged staff => 403
		req3 := httptest.NewRequest(http.MethodGet, sourcesEndpoint, nil)
		req3.Header.Set("Authorization", "Bearer "+noPermToken)
		w3 := httptest.NewRecorder()
		r.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusForbidden, w3.Code, "Unprivileged staff denied (403)")

		// Customer => 403
		reqCust := httptest.NewRequest(http.MethodGet, sourcesEndpoint, nil)
		reqCust.Header.Set("Authorization", "Bearer "+customerToken)
		wCust := httptest.NewRecorder()
		r.ServeHTTP(wCust, reqCust)
		assert.Equal(t, http.StatusForbidden, wCust.Code, "Customer denied (403)")

		// Unauthenticated => 401
		reqUnauth := httptest.NewRequest(http.MethodGet, sourcesEndpoint, nil)
		wUnauth := httptest.NewRecorder()
		r.ServeHTTP(wUnauth, reqUnauth)
		assert.Equal(t, http.StatusUnauthorized, wUnauth.Code, "Unauthenticated denied (401)")
	})

	// B. RBAC for /sources/{source} detail
	t.Run("SourceDetail_RBAC", func(t *testing.T) {
		// marketing.campaigns.read => 200
		req1 := httptest.NewRequest(http.MethodGet, detailEndpoint, nil)
		req1.Header.Set("Authorization", "Bearer "+campaignReadToken)
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code, "Admin with marketing.campaigns.read allowed for detail")

		// Sentinel _unattributed => 200
		reqUnattr := httptest.NewRequest(http.MethodGet, unattrDetailEndpoint, nil)
		reqUnattr.Header.Set("Authorization", "Bearer "+analyticsReadToken)
		wUnattr := httptest.NewRecorder()
		r.ServeHTTP(wUnattr, reqUnattr)
		assert.Equal(t, http.StatusOK, wUnattr.Code, "Admin with analytics.read allowed for _unattributed detail")

		// Unprivileged staff => 403
		req3 := httptest.NewRequest(http.MethodGet, detailEndpoint, nil)
		req3.Header.Set("Authorization", "Bearer "+noPermToken)
		w3 := httptest.NewRecorder()
		r.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusForbidden, w3.Code, "Unprivileged staff denied (403)")

		// Customer => 403
		reqCust := httptest.NewRequest(http.MethodGet, detailEndpoint, nil)
		reqCust.Header.Set("Authorization", "Bearer "+customerToken)
		wCust := httptest.NewRecorder()
		r.ServeHTTP(wCust, reqCust)
		assert.Equal(t, http.StatusForbidden, wCust.Code, "Customer denied (403)")

		// Unauthenticated => 401
		reqUnauth := httptest.NewRequest(http.MethodGet, detailEndpoint, nil)
		wUnauth := httptest.NewRecorder()
		r.ServeHTTP(wUnauth, reqUnauth)
		assert.Equal(t, http.StatusUnauthorized, wUnauth.Code, "Unauthenticated denied (401)")
	})

	// C. Date validation & Invalid sort
	t.Run("Validation_DatesAndSort", func(t *testing.T) {
		// Invalid from date
		reqInvFrom := httptest.NewRequest(http.MethodGet, "/api/admin/marketing/analytics/sources?from=bad-date&to="+toStr, nil)
		reqInvFrom.Header.Set("Authorization", "Bearer "+campaignReadToken)
		wInvFrom := httptest.NewRecorder()
		r.ServeHTTP(wInvFrom, reqInvFrom)
		assert.Equal(t, http.StatusBadRequest, wInvFrom.Code, "Invalid 'from' date must be 400 Bad Request")

		// Invalid to date
		reqInvTo := httptest.NewRequest(http.MethodGet, "/api/admin/marketing/analytics/sources?from="+fromStr+"&to=bad-date", nil)
		reqInvTo.Header.Set("Authorization", "Bearer "+campaignReadToken)
		wInvTo := httptest.NewRecorder()
		r.ServeHTTP(wInvTo, reqInvTo)
		assert.Equal(t, http.StatusBadRequest, wInvTo.Code, "Invalid 'to' date must be 400 Bad Request")

		// from >= to
		reqInverted := httptest.NewRequest(http.MethodGet, "/api/admin/marketing/analytics/sources?from="+toStr+"&to="+fromStr, nil)
		reqInverted.Header.Set("Authorization", "Bearer "+campaignReadToken)
		wInverted := httptest.NewRecorder()
		r.ServeHTTP(wInverted, reqInverted)
		assert.Equal(t, http.StatusBadRequest, wInverted.Code, "'to' date before 'from' date must be 400 Bad Request")

		// Invalid sort
		reqInvSort := httptest.NewRequest(http.MethodGet, sourcesEndpoint+"&sort=invalid_sort_param", nil)
		reqInvSort.Header.Set("Authorization", "Bearer "+campaignReadToken)
		wInvSort := httptest.NewRecorder()
		r.ServeHTTP(wInvSort, reqInvSort)
		assert.Equal(t, http.StatusBadRequest, wInvSort.Code, "Invalid sort parameter must be 400 Bad Request")

		// Detail: inverted dates
		reqDetailInv := httptest.NewRequest(http.MethodGet, "/api/admin/marketing/analytics/sources/vk?from="+toStr+"&to="+fromStr, nil)
		reqDetailInv.Header.Set("Authorization", "Bearer "+campaignReadToken)
		wDetailInv := httptest.NewRecorder()
		r.ServeHTTP(wDetailInv, reqDetailInv)
		assert.Equal(t, http.StatusBadRequest, wDetailInv.Code, "Detail with inverted dates must be 400 Bad Request")
	})

	// Matrix 58 & 59: Query Engine POST RBAC
	t.Run("Matrix58_59_QueryEngine_RBAC", func(t *testing.T) {
		queryEndpoint := "/api/admin/marketing/analytics/query"
		bodyJSON := `{"version": 1, "period": {"from": "` + fromStr + `", "to": "` + toStr + `"}, "dimensions": ["source"], "metrics": ["orders"]}`

		// 1. Matrix 58: Allowed with marketing.campaigns.read
		reqCampRead := httptest.NewRequest(http.MethodPost, queryEndpoint, strings.NewReader(bodyJSON))
		reqCampRead.Header.Set("Authorization", "Bearer "+campaignReadToken)
		reqCampRead.Header.Set("Content-Type", "application/json")
		wCampRead := httptest.NewRecorder()
		r.ServeHTTP(wCampRead, reqCampRead)
		assert.Equal(t, http.StatusOK, wCampRead.Code, "Admin with marketing.campaigns.read allowed (Matrix 58)")

		// 2. Matrix 58: Allowed with analytics.read
		reqAnalyticsRead := httptest.NewRequest(http.MethodPost, queryEndpoint, strings.NewReader(bodyJSON))
		reqAnalyticsRead.Header.Set("Authorization", "Bearer "+analyticsReadToken)
		reqAnalyticsRead.Header.Set("Content-Type", "application/json")
		wAnalyticsRead := httptest.NewRecorder()
		r.ServeHTTP(wAnalyticsRead, reqAnalyticsRead)
		assert.Equal(t, http.StatusOK, wAnalyticsRead.Code, "Admin with analytics.read allowed (Matrix 58)")

		// 3. Matrix 59: Denied for unprivileged staff (403)
		reqNoPerm := httptest.NewRequest(http.MethodPost, queryEndpoint, strings.NewReader(bodyJSON))
		reqNoPerm.Header.Set("Authorization", "Bearer "+noPermToken)
		reqNoPerm.Header.Set("Content-Type", "application/json")
		wNoPerm := httptest.NewRecorder()
		r.ServeHTTP(wNoPerm, reqNoPerm)
		assert.Equal(t, http.StatusForbidden, wNoPerm.Code, "Unprivileged staff denied 403 (Matrix 59)")

		// 4. Matrix 59: Denied for customer (403)
		reqCust := httptest.NewRequest(http.MethodPost, queryEndpoint, strings.NewReader(bodyJSON))
		reqCust.Header.Set("Authorization", "Bearer "+customerToken)
		reqCust.Header.Set("Content-Type", "application/json")
		wCust := httptest.NewRecorder()
		r.ServeHTTP(wCust, reqCust)
		assert.Equal(t, http.StatusForbidden, wCust.Code, "Customer denied 403 (Matrix 59)")

		// 5. Matrix 59: Denied for unauthenticated (401)
		reqUnauth := httptest.NewRequest(http.MethodPost, queryEndpoint, strings.NewReader(bodyJSON))
		reqUnauth.Header.Set("Content-Type", "application/json")
		wUnauth := httptest.NewRecorder()
		r.ServeHTTP(wUnauth, reqUnauth)
		assert.Equal(t, http.StatusUnauthorized, wUnauth.Code, "Unauthenticated denied 401 (Matrix 59)")
	})

	// Matrix 60: Export POST RBAC
	t.Run("Matrix60_Export_RBAC", func(t *testing.T) {
		exportEndpoint := "/api/admin/marketing/analytics/export"
		bodyJSON := `{"format": "csv", "query": {"version": 1, "period": {"from": "` + fromStr + `", "to": "` + toStr + `"}, "dimensions": ["source"], "metrics": ["orders"]}}`

		// 1. Allowed with marketing.campaigns.read
		reqCampRead := httptest.NewRequest(http.MethodPost, exportEndpoint, strings.NewReader(bodyJSON))
		reqCampRead.Header.Set("Authorization", "Bearer "+campaignReadToken)
		reqCampRead.Header.Set("Content-Type", "application/json")
		wCampRead := httptest.NewRecorder()
		r.ServeHTTP(wCampRead, reqCampRead)
		assert.Equal(t, http.StatusOK, wCampRead.Code, "Admin with marketing.campaigns.read allowed")
		assert.Equal(t, "text/csv; charset=utf-8", wCampRead.Header().Get("Content-Type"))

		// 2. Allowed with analytics.read
		reqAnalyticsRead := httptest.NewRequest(http.MethodPost, exportEndpoint, strings.NewReader(bodyJSON))
		reqAnalyticsRead.Header.Set("Authorization", "Bearer "+analyticsReadToken)
		reqAnalyticsRead.Header.Set("Content-Type", "application/json")
		wAnalyticsRead := httptest.NewRecorder()
		r.ServeHTTP(wAnalyticsRead, reqAnalyticsRead)
		assert.Equal(t, http.StatusOK, wAnalyticsRead.Code, "Admin with analytics.read allowed")

		// 3. Denied for unprivileged staff (403)
		reqNoPerm := httptest.NewRequest(http.MethodPost, exportEndpoint, strings.NewReader(bodyJSON))
		reqNoPerm.Header.Set("Authorization", "Bearer "+noPermToken)
		reqNoPerm.Header.Set("Content-Type", "application/json")
		wNoPerm := httptest.NewRecorder()
		r.ServeHTTP(wNoPerm, reqNoPerm)
		assert.Equal(t, http.StatusForbidden, wNoPerm.Code, "Unprivileged staff denied 403")

		// 4. Denied for customer (403)
		reqCust := httptest.NewRequest(http.MethodPost, exportEndpoint, strings.NewReader(bodyJSON))
		reqCust.Header.Set("Authorization", "Bearer "+customerToken)
		reqCust.Header.Set("Content-Type", "application/json")
		wCust := httptest.NewRecorder()
		r.ServeHTTP(wCust, reqCust)
		assert.Equal(t, http.StatusForbidden, wCust.Code, "Customer denied 403")

		// 5. Denied for unauthenticated (401)
		reqUnauth := httptest.NewRequest(http.MethodPost, exportEndpoint, strings.NewReader(bodyJSON))
		reqUnauth.Header.Set("Content-Type", "application/json")
		wUnauth := httptest.NewRecorder()
		r.ServeHTTP(wUnauth, reqUnauth)
		assert.Equal(t, http.StatusUnauthorized, wUnauth.Code, "Unauthenticated denied 401")
	})
}
