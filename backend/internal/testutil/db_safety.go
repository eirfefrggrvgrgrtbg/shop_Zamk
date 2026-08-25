package testutil

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// RequiredTestDatabaseName is the exact and only database name allowed for destructive test execution.
const RequiredTestDatabaseName = "zamk_test"

// CanonicalTestDatabaseDSN is the standard connection string to the local test database.
const CanonicalTestDatabaseDSN = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"

// DBQueryRow is an interface for any database connection or pool capable of running QueryRow.
type DBQueryRow interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// VerifyTestDatabase verifies that the database connected to is strictly "zamk_test".
// It returns a descriptive error if the connected database is anything else, preventing destructive test cleanup.
func VerifyTestDatabase(ctx context.Context, db DBQueryRow) error {
	if db == nil {
		return fmt.Errorf("REFUSING DESTRUCTIVE TEST SETUP: database connection is nil")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var currentDB string
	err := db.QueryRow(queryCtx, "SELECT current_database()").Scan(&currentDB)
	if err != nil {
		return fmt.Errorf("REFUSING DESTRUCTIVE TEST SETUP: failed to query current_database(): %w", err)
	}

	currentDB = strings.TrimSpace(currentDB)
	if currentDB != RequiredTestDatabaseName {
		return fmt.Errorf("REFUSING DESTRUCTIVE TEST SETUP: expected database %q, connected to %q", RequiredTestDatabaseName, currentDB)
	}

	return nil
}

// AssertTestDatabase asserts via testing.TB that the connected database is strictly "zamk_test".
// If the database is anything else or the query fails, it fails the test immediately (t.Fatalf) before any destructive cleanup.
func AssertTestDatabase(t testing.TB, db DBQueryRow) {
	t.Helper()
	if err := VerifyTestDatabase(context.Background(), db); err != nil {
		t.Fatalf("%v", err)
	}
}

// GetTestDatabaseURL retrieves the test database URL from TEST_DATABASE_URL or returns the canonical test DSN.
// It never falls back to the development database ("zamk").
func GetTestDatabaseURL() string {
	if val := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")); val != "" {
		return val
	}
	return CanonicalTestDatabaseDSN
}
