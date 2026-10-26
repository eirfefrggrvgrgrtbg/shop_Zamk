package testutil

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
)

// TEST.REDIS.ISOLATION.1: integration tests never talk to the shared Redis (localhost:6379). They get a Redis of their
// own from backend/scripts/safe_go_test_redis.sh: a throw-away loopback instance whose address and password
// (= ownership token) arrive in these variables. There is no fallback: without them the test stops before any write.
const (
	TestRedisAddrEnv  = "ZAMK_TEST_REDIS_ADDR"
	TestRedisTokenEnv = "ZAMK_TEST_REDIS_TOKEN"
	sharedRedisPort   = "6379"
)

// ValidateIsolatedRedis refuses everything that is not a per-run loopback Redis with an ownership token.
func ValidateIsolatedRedis(addr, token string) error {
	if strings.TrimSpace(addr) == "" || strings.TrimSpace(token) == "" {
		return fmt.Errorf("REFUSING: no isolated test Redis (%s/%s unset); run the test through backend/scripts/safe_go_test_redis.sh — the shared Redis is never used", TestRedisAddrEnv, TestRedisTokenEnv)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("REFUSING: bad isolated Redis address %q: %w", addr, err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("REFUSING: isolated Redis must be loopback, not %q", host)
	}
	if port == sharedRedisPort || port == "" {
		return fmt.Errorf("REFUSING: port %q is the shared Redis default, not a per-run instance", port)
	}
	if len(token) != 32 {
		return fmt.Errorf("REFUSING: malformed ownership token")
	}
	return nil
}

// NewTestRedis returns a client for this run's isolated Redis. The address and token must come from the runner; the
// instance must accept the token as its password and be configured without persistence, which proves it is the
// throw-away one and not a developer's server. Any failure stops the test before it writes.
func NewTestRedis(t testing.TB, ctx context.Context) *redis.Client {
	t.Helper()
	addr, token := os.Getenv(TestRedisAddrEnv), os.Getenv(TestRedisTokenEnv)
	if err := ValidateIsolatedRedis(addr, token); err != nil {
		t.Fatalf("%v", err)
	}
	client, err := redis.NewClient(ctx, addr, token, 0)
	if err != nil {
		t.Fatalf("REFUSING: isolated test Redis %s does not accept this run's token: %v", addr, err)
	}
	if v, err := client.Client.ConfigGet(ctx, "appendonly").Result(); err != nil || v["appendonly"] != "no" {
		client.Close()
		t.Fatalf("REFUSING: Redis %s is not persistence-free (appendonly=%v, err=%v)", addr, v["appendonly"], err)
	}
	if v, err := client.Client.ConfigGet(ctx, "save").Result(); err != nil || v["save"] != "" {
		client.Close()
		t.Fatalf("REFUSING: Redis %s has snapshotting enabled (save=%q, err=%v)", addr, v["save"], err)
	}
	return client
}
