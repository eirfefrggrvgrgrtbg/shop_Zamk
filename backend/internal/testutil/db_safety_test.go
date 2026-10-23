package testutil_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/jackc/pgx/v5"
)

type mockRow struct {
	val string
	err error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	if len(dest) > 0 {
		if ptr, ok := dest[0].(*string); ok {
			*ptr = m.val
		}
	}
	return nil
}

type mockDB struct {
	dbName string
	err    error
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &mockRow{val: m.dbName, err: m.err}
}

// TEST.DB.5: the shared zamk_test is refused like every other shared database; only a disposable one is accepted.
func TestVerifyTestDatabase_RefusesTheSharedZamkTest(t *testing.T) {
	for _, name := range []string{"zamk_test", "zamk", "zamk_demo"} {
		err := testutil.VerifyTestDatabase(context.Background(), &mockDB{dbName: name})
		if err == nil {
			t.Fatalf("expected %q to be refused", name)
		}
		if !strings.Contains(err.Error(), "REFUSING DESTRUCTIVE TEST SETUP") || !strings.Contains(err.Error(), "safe_go_test.sh") {
			t.Fatalf("unexpected message for %q: %v", name, err)
		}
	}
}

func TestVerifyTestDatabase_RejectsPostgresAndOthers(t *testing.T) {
	invalidNames := []string{"postgres", "template1", "", "unknown", "prod_db", "zamk_production"}
	for _, name := range invalidNames {
		db := &mockDB{dbName: name}
		err := testutil.VerifyTestDatabase(context.Background(), db)
		if err == nil {
			t.Errorf("expected error for database %q, got nil", name)
		}
		if !strings.Contains(err.Error(), "REFUSING DESTRUCTIVE TEST SETUP") {
			t.Errorf("expected error to contain safety prefix, got: %v", err)
		}
	}
}

func TestVerifyTestDatabase_HandlesNilOrQueryError(t *testing.T) {
	// Nil db
	err := testutil.VerifyTestDatabase(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "database connection is nil") {
		t.Fatalf("expected nil db error, got: %v", err)
	}

	// Query error
	db := &mockDB{err: errors.New("connection closed")}
	err = testutil.VerifyTestDatabase(context.Background(), db)
	if err == nil || !strings.Contains(err.Error(), "failed to query current_database()") {
		t.Fatalf("expected query error, got: %v", err)
	}
}

// TEST.DB.5: GetTestDatabaseURL is fail-closed. It hands out only a disposable DSN and never zamk_test.
func TestGetTestDatabaseURL_IsFailClosed(t *testing.T) {
	const disposable = "postgres://u:p@localhost:5433/zamk_disposable_0123456789ab?sslmode=disable"
	for name, env := range map[string]string{
		"unset":                 "",
		"the shared zamk_test":  testutil.CanonicalTestDatabaseDSN,
		"a custom zamk_test":    "postgres://custom:pass@localhost:5433/zamk_test?sslmode=disable",
		"the dev database":      "postgres://zamk:x@localhost:5433/zamk?sslmode=disable",
		"the demo database":     "postgres://zamk:x@localhost:5433/zamk_demo?sslmode=disable",
		"the template database": "postgres://zamk:x@localhost:5433/zamk_disposable_tpl_0123456789ab?sslmode=disable",
		"not a URL":             "::::",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("TEST_DATABASE_URL", env)
			if got := testutil.GetTestDatabaseURL(); got != testutil.BlockedTestDatabaseDSN {
				t.Fatalf("expected the blocked DSN, got %q", got)
			}
			if got := testutil.RunnerDatabaseURL(); got != "" {
				t.Fatalf("RunnerDatabaseURL must be empty, got %q", got)
			}
		})
	}
	t.Run("a disposable database is passed through", func(t *testing.T) {
		t.Setenv("TEST_DATABASE_URL", disposable)
		if got := testutil.GetTestDatabaseURL(); got != disposable {
			t.Fatalf("expected %q, got %q", disposable, got)
		}
		if got := testutil.RunnerDatabaseURL(); got != disposable {
			t.Fatalf("expected %q, got %q", disposable, got)
		}
	})
	t.Run("the blocked DSN cannot name a real database", func(t *testing.T) {
		if strings.Contains(testutil.BlockedTestDatabaseDSN, "zamk_test") || !strings.Contains(testutil.BlockedTestDatabaseDSN, "127.0.0.1:1") {
			t.Fatal("the blocked DSN must not point at any real database")
		}
	})
}

// TEST.DB.5, verification A: only meaningful under backend/scripts/safe_go_test.sh, which exports the run's own database.
func TestRunnerProvidesItsOwnDisposableDatabase(t *testing.T) {
	want := os.Getenv("ZAMK_TEST_RUNNER_DB")
	if os.Getenv("ZAMK_TEST_RUNNER") != "1" || want == "" {
		t.Skip("not started through backend/scripts/safe_go_test.sh")
	}
	dsn := testutil.GetTestDatabaseURL()
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var got string
	if err := conn.QueryRow(context.Background(), "SELECT current_database()").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want || !strings.HasPrefix(got, "zamk_disposable_") || got == "zamk_test" {
		t.Fatalf("connected to %q, runner created %q", got, want)
	}
	if err := testutil.VerifyTestDatabase(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"DATABASE_URL", "POSTGRES_DB"} {
		if e := os.Getenv(v); e != "" && !strings.Contains(e, want) {
			t.Fatalf("%s=%q does not point at the runner database", v, e)
		}
	}
}
