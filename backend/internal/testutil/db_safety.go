package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// RequiredTestDatabaseName is the exact and only database name allowed for destructive test execution.
const RequiredTestDatabaseName = "zamk_test"

// CanonicalTestDatabaseDSN is the standard connection string to the local test database.
const CanonicalTestDatabaseDSN = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"

// DBQueryRow is an interface for any database connection or pool capable of running QueryRow.
type DBQueryRow interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// DBExecutor is an interface for database connections capable of Exec and QueryRow.
type DBExecutor interface {
	DBQueryRow
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
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

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// EnsureCanonicalStarterTaxonomy guarantees that the canonical starter taxonomy categories
// and size chart fields (from migrations 60 and 61) are present in the test database.
// It strictly validates that the database is "zamk_test" before modifying any state.
func EnsureCanonicalStarterTaxonomy(ctx context.Context, db DBExecutor) error {
	if err := VerifyTestDatabase(ctx, db); err != nil {
		return err
	}

	var tankTopCount int
	err := db.QueryRow(ctx, "SELECT count(*) FROM categories WHERE slug = 'tank-tops'").Scan(&tankTopCount)
	if err == nil && tankTopCount > 0 {
		var fieldCount int
		err = db.QueryRow(ctx, "SELECT count(*) FROM category_size_chart_fields WHERE code = 'CHEST'").Scan(&fieldCount)
		if err == nil && fieldCount > 0 {
			return nil
		}
	}

	root := findRepoRoot()
	if root == "" {
		return fmt.Errorf("failed to locate repo root (go.mod)")
	}

	// Clean any partial starter taxonomy first to avoid primary key conflicts
	mig60DownPath := filepath.Join(root, "migrations", "60_add_starter_taxonomy.down.sql")
	if mig60DownBytes, err := os.ReadFile(mig60DownPath); err == nil {
		_, _ = db.Exec(ctx, string(mig60DownBytes))
	}

	// Apply migration 60 up
	mig60Path := filepath.Join(root, "migrations", "60_add_starter_taxonomy.up.sql")
	mig60Bytes, err := os.ReadFile(mig60Path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", mig60Path, err)
	}
	if _, err := db.Exec(ctx, string(mig60Bytes)); err != nil {
		return fmt.Errorf("failed to apply starter taxonomy (mig 60): %w", err)
	}

	// Apply migration 61 up
	mig61Path := filepath.Join(root, "migrations", "61_complete_category_size_chart_fields.up.sql")
	mig61Bytes, err := os.ReadFile(mig61Path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", mig61Path, err)
	}
	if _, err := db.Exec(ctx, string(mig61Bytes)); err != nil {
		return fmt.Errorf("failed to apply starter taxonomy fields (mig 61): %w", err)
	}

	return nil
}
