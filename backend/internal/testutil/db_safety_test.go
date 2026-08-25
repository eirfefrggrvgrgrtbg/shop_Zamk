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

func TestVerifyTestDatabase_AllowedOnZamkTest(t *testing.T) {
	db := &mockDB{dbName: "zamk_test"}
	err := testutil.VerifyTestDatabase(context.Background(), db)
	if err != nil {
		t.Fatalf("expected nil error for zamk_test, got: %v", err)
	}
}

func TestVerifyTestDatabase_RejectsDevDatabase(t *testing.T) {
	db := &mockDB{dbName: "zamk"}
	err := testutil.VerifyTestDatabase(context.Background(), db)
	if err == nil {
		t.Fatalf("expected error for dev database 'zamk', got nil")
	}
	expectedSubstring := `REFUSING DESTRUCTIVE TEST SETUP: expected database "zamk_test", connected to "zamk"`
	if !strings.Contains(err.Error(), expectedSubstring) {
		t.Fatalf("expected error message %q, got: %v", expectedSubstring, err)
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

func TestGetTestDatabaseURL(t *testing.T) {
	// Unset TEST_DATABASE_URL
	orig := os.Getenv("TEST_DATABASE_URL")
	defer os.Setenv("TEST_DATABASE_URL", orig)

	os.Unsetenv("TEST_DATABASE_URL")
	url := testutil.GetTestDatabaseURL()
	if url != testutil.CanonicalTestDatabaseDSN {
		t.Fatalf("expected default %q, got %q", testutil.CanonicalTestDatabaseDSN, url)
	}
	if !strings.Contains(url, "/zamk_test") {
		t.Fatalf("expected default URL to target zamk_test, got %q", url)
	}

	// Custom URL
	os.Setenv("TEST_DATABASE_URL", "postgres://custom:pass@localhost:5433/zamk_test?sslmode=disable")
	url = testutil.GetTestDatabaseURL()
	if url != "postgres://custom:pass@localhost:5433/zamk_test?sslmode=disable" {
		t.Fatalf("expected custom URL, got %q", url)
	}
}
