package router_test

// MARKETING.M1.2 — server-side lifecycle of advertising campaigns through the real router on zamk_test.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
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

func TestAdminMarketingCampaignStatus_Lifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	ctx := context.Background()
	cfg := &config.Config{
		JWT:  config.JWTConfig{AccessTokenSecret: "test-secret", RefreshTokenSecret: "test-secret-refresh", AccessTokenTTLMinutes: 60, RefreshTokenTTLDays: 7},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
	}
	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)
	defer pgClient.Close()
	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	require.NoError(t, err)
	defer redisClient.Close()
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)
	newAdmin := func(perms []string) string {
		id := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'M12', 'hash', 'admin', 'active', NOW(), NOW())`, id, id.String()+"@test.com", "7999"+id.String()[:7])
		require.NoError(t, err)
		_, err = setupRouterStaffWithPermissions(ctx, pgClient.Pool, id, "M12Role", perms)
		require.NoError(t, err)
		tok, err := tokenService.GenerateAccessToken(id, id.String()+"@test.com", "admin")
		require.NoError(t, err)
		return tok
	}
	writer := newAdmin([]string{"marketing.campaigns.read", "marketing.campaigns.write"})
	reader := newAdmin([]string{"marketing.campaigns.read"})

	call := func(token, method, path string, body any) (int, map[string]any) {
		var buf bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&buf).Encode(body))
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		out := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	create := func() string {
		code, body := call(writer, http.MethodPost, "/api/admin/marketing/campaigns", map[string]any{
			"purpose": "advertising", "title": "M12 " + uuid.NewString()[:8], "fundingMode": "zamk", "discountType": "percent",
		})
		require.Equal(t, http.StatusCreated, code)
		assert.Equal(t, "approved", body["status"], "admin campaigns start approved")
		return body["id"].(string)
	}
	patch := func(token, id string, body map[string]any) (int, map[string]any) {
		return call(token, http.MethodPatch, "/api/admin/marketing/campaigns/"+id, body)
	}
	statusOf := func(id string) string {
		var s string
		require.NoError(t, pgClient.Pool.QueryRow(ctx, `SELECT status FROM marketing_campaigns WHERE id = $1`, id).Scan(&s))
		return s
	}

	t.Run("allowed transitions", func(t *testing.T) {
		for _, path := range [][]string{
			{"active", "ended"},
			{"ended"},
			{"cancelled"},
			{"active", "cancelled"},
		} {
			id := create()
			for _, to := range path {
				code, body := patch(writer, id, map[string]any{"status": to})
				require.Equal(t, http.StatusOK, code, "%v: %v", path, body)
				assert.Equal(t, to, statusOf(id))
			}
		}
	})

	t.Run("same status and field edits stay allowed", func(t *testing.T) {
		id := create()
		code, _ := patch(writer, id, map[string]any{"status": "approved", "title": "Renamed"})
		assert.Equal(t, http.StatusOK, code)
		code, _ = patch(writer, id, map[string]any{"status": "active"})
		require.Equal(t, http.StatusOK, code)
		code, _ = patch(writer, id, map[string]any{"status": "active", "plannedBudgetCents": 1000})
		assert.Equal(t, http.StatusOK, code)
	})

	t.Run("forbidden transitions are rejected with 409 and change nothing", func(t *testing.T) {
		id := create()
		for _, to := range []string{"draft", "submitted", "counter_offered", "rejected"} {
			code, body := patch(writer, id, map[string]any{"status": to})
			assert.Equal(t, http.StatusConflict, code, to)
			assert.Equal(t, "invalid_status_transition", body["code"])
			assert.Equal(t, "approved", statusOf(id))
		}
		code, _ := patch(writer, id, map[string]any{"status": "active"})
		require.Equal(t, http.StatusOK, code)
		// legacy "pause" marker: allowed for compatibility, but it changes nothing for tracking
		code, _ = patch(writer, id, map[string]any{"status": "approved"})
		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, "approved", statusOf(id))
	})

	t.Run("ended and cancelled campaigns cannot be reactivated or revived", func(t *testing.T) {
		for _, terminal := range []string{"ended", "cancelled"} {
			id := create()
			code, _ := patch(writer, id, map[string]any{"status": terminal})
			require.Equal(t, http.StatusOK, code)
			for _, to := range []string{"active", "approved", "ended", "cancelled", "draft"} {
				if to == terminal {
					continue
				}
				code, body := patch(writer, id, map[string]any{"status": to})
				assert.Equal(t, http.StatusConflict, code, "%s -> %s", terminal, to)
				assert.Equal(t, "invalid_status_transition", body["code"])
			}
			assert.Equal(t, terminal, statusOf(id))
			// non-status edits of a finished campaign are still possible (title/description correction)
			code, _ = patch(writer, id, map[string]any{"title": "Corrected"})
			assert.Equal(t, http.StatusOK, code)
		}
	})

	t.Run("a rejected status change does not apply other fields of the same request", func(t *testing.T) {
		id := create()
		code, _ := patch(writer, id, map[string]any{"status": "ended"})
		require.Equal(t, http.StatusOK, code)
		code, _ = patch(writer, id, map[string]any{"status": "active", "title": "Should not be saved"})
		assert.Equal(t, http.StatusConflict, code)
		var title string
		require.NoError(t, pgClient.Pool.QueryRow(ctx, `SELECT title FROM marketing_campaigns WHERE id = $1`, id).Scan(&title))
		assert.NotEqual(t, "Should not be saved", title)
	})

	t.Run("write permission is required", func(t *testing.T) {
		id := create()
		code, _ := patch(reader, id, map[string]any{"status": "active"})
		assert.Equal(t, http.StatusForbidden, code)
		code, _ = call(reader, http.MethodPost, "/api/admin/marketing/campaigns", map[string]any{
			"purpose": "advertising", "title": "x", "fundingMode": "zamk", "discountType": "percent"})
		assert.Equal(t, http.StatusForbidden, code)
		assert.Equal(t, "approved", statusOf(id))
		code, _ = call(reader, http.MethodGet, "/api/admin/marketing/campaigns/"+id, nil)
		assert.Equal(t, http.StatusOK, code, "read permission still allows reading")
	})

	t.Run("unknown campaign id and malformed id", func(t *testing.T) {
		code, _ := patch(writer, uuid.NewString(), map[string]any{"status": "ended"})
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = patch(writer, "not-a-uuid", map[string]any{"status": "ended"})
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("a status outside the advertising lifecycle (paused) is rejected", func(t *testing.T) {
		id := create()
		code, body := patch(writer, id, map[string]any{"status": "paused"})
		assert.Equal(t, http.StatusConflict, code, "unlisted target is rejected; body=%v", body)
		assert.Equal(t, "approved", statusOf(id))
	})

	t.Run("concurrent activate and end leave one consistent final state", func(t *testing.T) {
		for i := 0; i < 12; i++ {
			id := create()
			var wg sync.WaitGroup
			codes := make([]int, 2)
			for n, to := range []string{"active", "ended"} {
				wg.Add(1)
				go func(n int, to string) {
					defer wg.Done()
					codes[n], _ = patch(writer, id, map[string]any{"status": to})
				}(n, to)
			}
			wg.Wait()
			// "ended" is reachable from approved and from active, so it always wins; "active" either ran
			// first (200) or lost the race against the terminal state (409). Never a 5xx, never "active" at the end.
			assert.Equal(t, http.StatusOK, codes[1])
			assert.Contains(t, []int{http.StatusOK, http.StatusConflict}, codes[0])
			assert.Equal(t, "ended", statusOf(id))
		}
	})

	t.Run("concurrent end and cancel: exactly one terminal transition wins", func(t *testing.T) {
		for i := 0; i < 12; i++ {
			id := create()
			var wg sync.WaitGroup
			codes := make([]int, 2)
			targets := []string{"ended", "cancelled"}
			for n := range targets {
				wg.Add(1)
				go func(n int) {
					defer wg.Done()
					codes[n], _ = patch(writer, id, map[string]any{"status": targets[n]})
				}(n)
			}
			wg.Wait()
			ok := 0
			for _, c := range codes {
				if c == http.StatusOK {
					ok++
				} else {
					assert.Equal(t, http.StatusConflict, c)
				}
			}
			assert.Equal(t, 1, ok, "codes=%v", codes)
			final := statusOf(id)
			winner := targets[0]
			if codes[1] == http.StatusOK {
				winner = targets[1]
			}
			assert.Equal(t, winner, final)
		}
	})

	t.Run("concurrent title edit and status change do not lose each other", func(t *testing.T) {
		for i := 0; i < 12; i++ {
			id := create()
			title := "T-" + uuid.NewString()[:6]
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); patch(writer, id, map[string]any{"title": title}) }()
			go func() { defer wg.Done(); patch(writer, id, map[string]any{"status": "active"}) }()
			wg.Wait()
			var gotTitle, gotStatus string
			require.NoError(t, pgClient.Pool.QueryRow(ctx, `SELECT title, status FROM marketing_campaigns WHERE id = $1`, id).Scan(&gotTitle, &gotStatus))
			assert.Equal(t, title, gotTitle)
			assert.Equal(t, "active", gotStatus)
		}
	})
}
