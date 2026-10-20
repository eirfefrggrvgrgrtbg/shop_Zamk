package router_test

// MARKETING.M1.2/M1.3 — server-side lifecycle of advertising campaigns through the real router on zamk_test.
//
//	draft -> approved -> active <-> paused; active|paused -> ended (COMPLETED); draft|approved|active|paused -> cancelled

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
			VALUES ($1, $2, $3, 'M13', 'hash', 'admin', 'active', NOW(), NOW())`, id, id.String()+"@test.com", "7999"+id.String()[:7])
		require.NoError(t, err)
		_, err = setupRouterStaffWithPermissions(ctx, pgClient.Pool, id, "M13Role", perms)
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
	ready := map[string]any{"campaignChannel": "telegram", "campaignType": "drop", "startsAt": "2026-01-01T00:00:00+03:00"}
	createDraft := func(extra map[string]any) string {
		body := map[string]any{"purpose": "advertising", "title": "M13 " + uuid.NewString()[:8], "fundingMode": "zamk", "discountType": "percent"}
		for k, v := range extra {
			body[k] = v
		}
		code, out := call(writer, http.MethodPost, "/api/admin/marketing/campaigns", body)
		require.Equal(t, http.StatusCreated, code, "%v", out)
		require.Equal(t, "draft", out["status"], "A: new advertising campaigns are drafts")
		return out["id"].(string)
	}
	patch := func(token, id string, body map[string]any) (int, map[string]any) {
		return call(token, http.MethodPatch, "/api/admin/marketing/campaigns/"+id, body)
	}
	statusOf := func(id string) string {
		var s string
		require.NoError(t, pgClient.Pool.QueryRow(ctx, `SELECT status FROM marketing_campaigns WHERE id = $1`, id).Scan(&s))
		return s
	}
	moveTo := func(id string, path ...string) {
		for _, to := range path {
			code, out := patch(writer, id, map[string]any{"status": to})
			require.Equal(t, http.StatusOK, code, "-> %s: %v", to, out)
		}
	}
	readyCampaign := func(path ...string) string {
		id := createDraft(ready)
		moveTo(id, path...)
		return id
	}

	t.Run("A/B: an incomplete draft is created and saved", func(t *testing.T) {
		id := createDraft(nil)
		code, out := patch(writer, id, map[string]any{"title": "Still a draft", "plannedBudgetCents": 500000})
		require.Equal(t, http.StatusOK, code, "%v", out)
		assert.Equal(t, "draft", out["status"])
		assert.Equal(t, "Still a draft", out["title"])
		assert.Equal(t, false, out["legacyApprovedTracking"])
	})

	t.Run("C: approval requires a complete campaign", func(t *testing.T) {
		id := createDraft(nil)
		code, out := patch(writer, id, map[string]any{"status": "approved"})
		assert.Equal(t, http.StatusUnprocessableEntity, code)
		assert.Equal(t, "campaign_not_ready", out["code"])
		assert.Contains(t, out["message"], "campaignChannel")
		assert.Contains(t, out["message"], "startsAt")
		assert.Equal(t, "draft", statusOf(id))

		code, out = patch(writer, id, map[string]any{"status": "approved", "campaignChannel": "vk", "campaignType": "drop",
			"startsAt": "2026-03-01T00:00:00+03:00", "endsAt": "2026-02-01T00:00:00+03:00"})
		assert.Equal(t, http.StatusBadRequest, code, "end before start: %v", out)
		assert.Equal(t, "draft", statusOf(id))

		ready := map[string]any{"status": "approved", "campaignChannel": "vk", "campaignType": "drop",
			"startsAt": "2026-03-01T00:00:00+03:00", "endsAt": "2026-03-02T00:00:00+03:00"}
		code, out = patch(writer, id, ready)
		require.Equal(t, http.StatusOK, code, "%v", out)
		assert.Equal(t, "approved", out["status"])
	})

	t.Run("D-H: allowed lifecycle paths", func(t *testing.T) {
		for _, path := range [][]string{
			{"approved", "active"},
			{"approved", "active", "paused", "active", "paused", "ended"},
			{"approved", "active", "ended"},
			{"cancelled"},
			{"approved", "cancelled"},
			{"approved", "active", "cancelled"},
			{"approved", "active", "paused", "cancelled"},
		} {
			id := createDraft(ready)
			for _, to := range path {
				code, out := patch(writer, id, map[string]any{"status": to})
				require.Equal(t, http.StatusOK, code, "%v at %s: %v", path, to, out)
				assert.Equal(t, to, statusOf(id))
			}
		}
	})

	t.Run("field edits work in draft, approved, active and paused", func(t *testing.T) {
		id := createDraft(ready)
		for _, st := range []string{"", "approved", "active", "paused"} {
			if st != "" {
				moveTo(id, st)
			}
			code, out := patch(writer, id, map[string]any{"title": "Edited in " + st, "plannedBudgetCents": 1000})
			require.Equal(t, http.StatusOK, code, "%s: %v", st, out)
		}
	})

	t.Run("forbidden transitions are rejected with 409 and change nothing", func(t *testing.T) {
		cases := []struct {
			path []string
			to   string
		}{
			{nil, "active"}, {nil, "paused"}, {nil, "ended"},
			{[]string{"approved"}, "paused"}, {[]string{"approved"}, "ended"}, {[]string{"approved"}, "draft"},
			{[]string{"approved", "active"}, "approved"}, // not a pause
			{[]string{"approved", "active"}, "draft"},
			{[]string{"approved", "active", "paused"}, "approved"},
		}
		for _, c := range cases {
			id := readyCampaign(c.path...)
			before := statusOf(id)
			for _, to := range []string{c.to, "submitted", "counter_offered", "rejected", "completed"} {
				code, out := patch(writer, id, map[string]any{"status": to, "title": "must not be saved"})
				assert.Contains(t, []int{http.StatusConflict, http.StatusBadRequest}, code, "%v -> %s: %v", c.path, to, out)
			}
			assert.Equal(t, before, statusOf(id))
			var title string
			require.NoError(t, pgClient.Pool.QueryRow(ctx, `SELECT title FROM marketing_campaigns WHERE id = $1`, id).Scan(&title))
			assert.NotEqual(t, "must not be saved", title)
		}
	})

	t.Run("G/I: completed and cancelled campaigns are closed for any change", func(t *testing.T) {
		for _, path := range [][]string{{"approved", "active", "ended"}, {"approved", "active", "paused", "ended"}, {"cancelled"}, {"approved", "active", "cancelled"}} {
			id := readyCampaign(path...)
			terminal := path[len(path)-1]
			for _, to := range []string{"draft", "approved", "active", "paused", "ended", "cancelled"} {
				if to == terminal {
					continue
				}
				code, out := patch(writer, id, map[string]any{"status": to})
				assert.Equal(t, http.StatusConflict, code, "%s -> %s", terminal, to)
				assert.Equal(t, "campaign_closed", out["code"])
			}
			code, out := patch(writer, id, map[string]any{"title": "Corrected"})
			assert.Equal(t, http.StatusConflict, code, "no edits of a closed campaign")
			assert.Equal(t, "campaign_closed", out["code"])
			code, _ = patch(writer, id, map[string]any{"status": terminal})
			assert.Equal(t, http.StatusOK, code, "idempotent repeat of the terminal status")
			assert.Equal(t, terminal, statusOf(id))
		}
	})

	t.Run("M: write permission is required for every lifecycle action", func(t *testing.T) {
		id := createDraft(ready)
		for _, to := range []string{"approved", "cancelled"} {
			code, _ := patch(reader, id, map[string]any{"status": to})
			assert.Equal(t, http.StatusForbidden, code)
		}
		code, _ := call(reader, http.MethodPost, "/api/admin/marketing/campaigns", map[string]any{
			"purpose": "advertising", "title": "x", "fundingMode": "zamk", "discountType": "percent"})
		assert.Equal(t, http.StatusForbidden, code)
		code, _ = call(reader, http.MethodPost, "/api/admin/marketing/campaigns/"+id+"/tracking-links", map[string]any{"targetType": "landing", "landingPath": "/catalog"})
		assert.Equal(t, http.StatusForbidden, code)
		assert.Equal(t, "draft", statusOf(id))
		code, _ = call(reader, http.MethodGet, "/api/admin/marketing/campaigns/"+id, nil)
		assert.Equal(t, http.StatusOK, code, "read permission still allows reading")
	})

	t.Run("tracking links: allowed before launch, refused for closed campaigns", func(t *testing.T) {
		link := map[string]any{"targetType": "landing", "landingPath": "/catalog"}
		for _, path := range [][]string{nil, {"approved"}, {"approved", "active"}, {"approved", "active", "paused"}} {
			id := readyCampaign(path...)
			code, out := call(writer, http.MethodPost, "/api/admin/marketing/campaigns/"+id+"/tracking-links", link)
			assert.Equal(t, http.StatusCreated, code, "%v: %v", path, out)
		}
		for _, path := range [][]string{{"approved", "active", "ended"}, {"cancelled"}} {
			id := readyCampaign(path...)
			code, out := call(writer, http.MethodPost, "/api/admin/marketing/campaigns/"+id+"/tracking-links", link)
			assert.Equal(t, http.StatusConflict, code, "%v", path)
			assert.Equal(t, "campaign_closed", out["code"])
		}
	})

	t.Run("unknown and malformed ids", func(t *testing.T) {
		code, _ := patch(writer, uuid.NewString(), map[string]any{"status": "ended"})
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = patch(writer, "not-a-uuid", map[string]any{"status": "ended"})
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("L: concurrent pause and complete leave one consistent state", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			id := readyCampaign("approved", "active")
			var wg sync.WaitGroup
			codes := make([]int, 2)
			targets := []string{"paused", "ended"}
			for n := range targets {
				wg.Add(1)
				go func(n int) { defer wg.Done(); codes[n], _ = patch(writer, id, map[string]any{"status": targets[n]}) }(n)
			}
			wg.Wait()
			// ended is reachable from active and from paused, so it always succeeds; paused succeeds only
			// if it ran first. The final state is always the terminal one.
			assert.Equal(t, http.StatusOK, codes[1])
			assert.Contains(t, []int{http.StatusOK, http.StatusConflict}, codes[0])
			assert.Equal(t, "ended", statusOf(id))
		}
	})

	t.Run("L: concurrent complete and cancel — exactly one wins", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			id := readyCampaign("approved", "active")
			var wg sync.WaitGroup
			codes := make([]int, 2)
			targets := []string{"ended", "cancelled"}
			for n := range targets {
				wg.Add(1)
				go func(n int) { defer wg.Done(); codes[n], _ = patch(writer, id, map[string]any{"status": targets[n]}) }(n)
			}
			wg.Wait()
			ok := 0
			winner := ""
			for n, c := range codes {
				if c == http.StatusOK {
					ok++
					winner = targets[n]
				} else {
					assert.Equal(t, http.StatusConflict, c)
				}
			}
			assert.Equal(t, 1, ok, "codes=%v", codes)
			assert.Equal(t, winner, statusOf(id))
		}
	})

	t.Run("L: concurrent title edit and pause do not lose each other", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			id := readyCampaign("approved", "active")
			title := "T-" + uuid.NewString()[:6]
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); patch(writer, id, map[string]any{"title": title}) }()
			go func() { defer wg.Done(); patch(writer, id, map[string]any{"status": "paused"}) }()
			wg.Wait()
			var gotTitle, gotStatus string
			require.NoError(t, pgClient.Pool.QueryRow(ctx, `SELECT title, status FROM marketing_campaigns WHERE id = $1`, id).Scan(&gotTitle, &gotStatus))
			assert.Equal(t, title, gotTitle)
			assert.Equal(t, "paused", gotStatus)
		}
	})

	t.Run("N: a pre-M1.3 approved campaign keeps tracking until it is moved on", func(t *testing.T) {
		id := readyCampaign("approved")
		// simulate a row that existed before migration 000115 (column default TRUE)
		_, err := pgClient.Pool.Exec(ctx, `UPDATE marketing_campaigns SET legacy_approved_tracking = true WHERE id = $1`, id)
		require.NoError(t, err)
		code, out := patch(writer, id, map[string]any{"title": "Legacy edit"})
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, true, out["legacyApprovedTracking"], "field edits keep the compatibility rule")
		moveTo(id, "active")
		_, out = call(writer, http.MethodGet, "/api/admin/marketing/campaigns/"+id, nil)
		assert.Equal(t, false, out["legacyApprovedTracking"], "any lifecycle move clears it")
	})
}
