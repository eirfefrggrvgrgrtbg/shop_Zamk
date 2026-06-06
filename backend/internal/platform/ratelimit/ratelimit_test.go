package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type fakeRedis struct {
	counts map[string]int64
	err    error
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{counts: map[string]int64{}}
}

func (f *fakeRedis) Incr(ctx context.Context, key string) *goredis.IntCmd {
	cmd := goredis.NewIntCmd(ctx)
	if f.err != nil {
		cmd.SetErr(f.err)
		return cmd
	}
	f.counts[key]++
	cmd.SetVal(f.counts[key])
	return cmd
}

func (f *fakeRedis) Expire(ctx context.Context, key string, expiration time.Duration) *goredis.BoolCmd {
	cmd := goredis.NewBoolCmd(ctx)
	if f.err != nil {
		cmd.SetErr(f.err)
		return cmd
	}
	cmd.SetVal(true)
	return cmd
}

func (f *fakeRedis) TTL(ctx context.Context, key string) *goredis.DurationCmd {
	cmd := goredis.NewDurationCmd(ctx, time.Second)
	if f.err != nil {
		cmd.SetErr(f.err)
		return cmd
	}
	cmd.SetVal(time.Minute)
	return cmd
}

func TestLimiterAllowsBelowLimitAndBlocksAbove(t *testing.T) {
	limiter := New(newFakeRedis())
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		result, err := limiter.Allow(ctx, "rl:test", 2, time.Minute)
		if err != nil {
			t.Fatalf("expected request %d below limit: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}

	result, err := limiter.Allow(ctx, "rl:test", 2, time.Minute)
	if !errors.Is(err, ErrLimited) {
		t.Fatalf("expected ErrLimited, got %v", err)
	}
	if result.Allowed {
		t.Fatal("expected request over limit to be blocked")
	}
	if result.RetryAfter <= 0 {
		t.Fatal("expected retry-after duration")
	}
}

func TestMiddlewareReturns429(t *testing.T) {
	mw := NewMiddleware(New(newFakeRedis()), true, false, nil)
	handler := mw.Limit(Rule{
		Group:  "test",
		Limit:  1,
		Window: time.Minute,
		Key:    IPKey("test"),
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"rate_limited"`) {
		t.Fatalf("expected rate_limited response, got %s", rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestMiddlewareDisabledBypasses(t *testing.T) {
	store := newFakeRedis()
	mw := NewMiddleware(New(store), false, false, nil)
	called := 0
	handler := mw.Limit(Rule{Group: "test", Limit: 1, Window: time.Minute, Key: IPKey("test")})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if called != 2 {
		t.Fatalf("expected middleware bypass, called %d times", called)
	}
	if len(store.counts) != 0 {
		t.Fatal("disabled middleware should not touch redis")
	}
}

func TestMiddlewareFailOpenAndFailClosed(t *testing.T) {
	store := newFakeRedis()
	store.err = errors.New("redis unavailable")

	failOpen := NewMiddleware(New(store), true, true, nil)
	called := false
	failOpenHandler := failOpen.Limit(Rule{Group: "test", Limit: 1, Window: time.Minute, Key: IPKey("test")})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	failOpenHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/test", nil))
	if !called {
		t.Fatal("expected fail-open middleware to call next")
	}

	failClosed := NewMiddleware(New(store), true, false, nil)
	rec := httptest.NewRecorder()
	failClosedHandler := failClosed.Limit(Rule{Group: "test", Limit: 1, Window: time.Minute, Key: IPKey("test")})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	failClosedHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/test", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected fail-closed 429, got %d", rec.Code)
	}
}

func TestKeysDoNotIncludeRawSecrets(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"User@Example.com","password":"super-secret-password"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.10:9999"

	key := LoginKey(req)
	if strings.Contains(key, "User@Example.com") || strings.Contains(key, "user@example.com") {
		t.Fatalf("login key leaked email: %s", key)
	}
	if strings.Contains(key, "super-secret-password") {
		t.Fatalf("login key leaked password: %s", key)
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	refreshReq.RemoteAddr = "192.0.2.10:9999"
	refreshReq.AddCookie(&http.Cookie{Name: "zamk_refresh_token", Value: "raw-refresh-token"})
	refreshKey := RefreshKey(refreshReq)
	if strings.Contains(refreshKey, "raw-refresh-token") {
		t.Fatalf("refresh key leaked token: %s", refreshKey)
	}
}
