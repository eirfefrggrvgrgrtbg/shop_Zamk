package testutil

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SharedTestDatabaseName is the shared, long-lived test database. TEST.DB.5: no test may read or write it any more;
// every integration test runs on its own disposable database (zamk_disposable_<12 hex>) created by
// backend/scripts/safe_go_test.sh. The constant exists only so that guards and messages can name what is refused.
const SharedTestDatabaseName = "zamk_test"

// CanonicalTestDatabaseDSN is the DSN of the shared zamk_test. It is kept only to locate the local PostgreSQL server
// (host, port, credentials) when TEST_DATABASE_URL is not set; no test may connect to the database it names.
const CanonicalTestDatabaseDSN = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"

// BlockedTestDatabaseDSN is what GetTestDatabaseURL returns when the test process was not started through the
// isolating runner: a DSN that cannot connect anywhere and names the cure in the database part.
const BlockedTestDatabaseDSN = "postgres://blocked@127.0.0.1:1/BLOCKED_run_integration_tests_via_backend_scripts_safe_go_test_sh?sslmode=disable&connect_timeout=1"

// DBQueryRow is an interface for any database connection or pool capable of running QueryRow.
type DBQueryRow interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// DBExecutor is an interface for database connections capable of Exec and QueryRow.
type DBExecutor interface {
	DBQueryRow
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// VerifyTestDatabase verifies that the database connected to is a per-run disposable test database
// (zamk_disposable_<12 hex>). TEST.DB.5: the shared zamk_test (and zamk, zamk_demo, postgres, templates) is refused,
// so a test that was pointed at a shared database stops before its first write.
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
	if !disposableNameRE.MatchString(currentDB) {
		return fmt.Errorf("REFUSING DESTRUCTIVE TEST SETUP: expected a disposable database (%s<12 hex>), connected to %q; run integration tests through backend/scripts/safe_go_test.sh", DisposableDatabasePrefix, currentDB)
	}

	return nil
}

// AssertTestDatabase asserts via testing.TB that the connected database is a disposable test database (never zamk_test).
// If the database is anything else or the query fails, it fails the test immediately (t.Fatalf) before any destructive cleanup.
func AssertTestDatabase(t testing.TB, db DBQueryRow) {
	t.Helper()
	if err := VerifyTestDatabase(context.Background(), db); err != nil {
		t.Fatalf("%v", err)
	}
	if exec, ok := db.(DBExecutor); ok {
		if err := EnsureMarketingMigrations(context.Background(), exec); err != nil {
			t.Fatalf("failed to ensure marketing migrations: %v", err)
		}
	}
}

// RunnerDatabaseURL returns TEST_DATABASE_URL only when it names a disposable database (what safe_go_test.sh
// exports); otherwise "". Tests that skip when no database is configured use it instead of reading the variable
// themselves, so a hand-set shared DSN makes them skip instead of write.
func RunnerDatabaseURL() string {
	val := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if val == "" {
		return ""
	}
	u, err := url.Parse(val)
	if err != nil || !disposableNameRE.MatchString(strings.TrimPrefix(u.Path, "/")) {
		return ""
	}
	return val
}

// GetTestDatabaseURL returns the DSN of the disposable database the test run was given. TEST.DB.5, fail-closed:
// it never returns zamk_test, zamk or zamk_demo, whatever TEST_DATABASE_URL says and whether or not it is set. When the
// process was not started through backend/scripts/safe_go_test.sh it returns BlockedTestDatabaseDSN, so the test
// fails to connect before it can write.
func GetTestDatabaseURL() string {
	if val := RunnerDatabaseURL(); val != "" {
		return val
	}
	return BlockedTestDatabaseDSN
}

// sharedServerDSN locates the local PostgreSQL server (never a database to test on): TEST_DATABASE_URL when set, else
// the canonical local DSN. Only host, port and credentials are used; the database part is always replaced.
func sharedServerDSN() string {
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
// It strictly validates that the database is a disposable test database before modifying any state.
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

// EnsureMarketingMigrations guarantees that the marketing foundation migrations (000095-000098)
// are applied to the test database.
// It strictly validates that the database is a disposable test database before modifying any state.
func EnsureMarketingMigrations(ctx context.Context, db DBExecutor) error {
	if err := VerifyTestDatabase(ctx, db); err != nil {
		return err
	}

	var hasIndex, hasClaimsTable, hasInitOutcomeCol, hasReconCol, hasProviderPIDIndex, hasTargetsTable, hasCategoryTargetsTable bool
	_ = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM pg_indexes
			WHERE indexname = 'uq_first_order_active_per_customer'
		)
	`).Scan(&hasIndex)
	_ = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'customer_first_payment_claims'
		)
	`).Scan(&hasClaimsTable)
	_ = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns
			WHERE table_name = 'payments' AND column_name = 'init_outcome'
		)
	`).Scan(&hasInitOutcomeCol)
	_ = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns
			WHERE table_name = 'payments' AND column_name = 'reconciliation_attempted_at'
		)
	`).Scan(&hasReconCol)
	_ = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM pg_indexes
			WHERE indexname = 'uq_payments_provider_payment_id'
		)
	`).Scan(&hasProviderPIDIndex)
	_ = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'promo_code_product_targets'
		)
	`).Scan(&hasTargetsTable)
	_ = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'promo_code_category_targets'
		)
	`).Scan(&hasCategoryTargetsTable)
	var hasMinQuantityCol bool
	_ = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns
			WHERE table_name = 'promo_codes' AND column_name = 'min_eligible_quantity'
		)
	`).Scan(&hasMinQuantityCol)
	var hasAudienceTypeCol bool
	_ = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns
			WHERE table_name = 'promo_codes' AND column_name = 'audience_type'
		)
	`).Scan(&hasAudienceTypeCol)
	var hasMinDistinctCol bool
	_ = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns
			WHERE table_name = 'promo_codes' AND column_name = 'min_distinct_products'
		)
	`).Scan(&hasMinDistinctCol)
	var hasSavedQueriesTable bool
	_ = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'marketing_saved_queries'
		)
	`).Scan(&hasSavedQueriesTable)
	if hasIndex && hasClaimsTable && hasInitOutcomeCol && hasReconCol && hasProviderPIDIndex && hasTargetsTable && hasCategoryTargetsTable && hasMinQuantityCol && hasAudienceTypeCol && hasMinDistinctCol && hasSavedQueriesTable {
		return nil
	}

	root := findRepoRoot()
	if root == "" {
		return fmt.Errorf("failed to locate repo root (go.mod)")
	}

	migs := []string{
		"000095_create_marketing_campaigns_and_promotions.up.sql",
		"000096_create_product_price_history.up.sql",
		"000097_create_order_item_promotions.up.sql",
		"000098_harden_promotion_allocations_and_first_order.up.sql",
		"000099_add_promotion_product_scope_and_max_discount.up.sql",
		"000100_add_promotion_category_targeting.up.sql",
		"000101_add_min_eligible_quantity_to_promo_codes.up.sql",
		"000102_evolve_promo_customer_audience.up.sql",
		"000103_add_min_distinct_products_to_promo_codes.up.sql",
		"000111_create_marketing_saved_queries.up.sql",
	}

	for _, mig := range migs {
		migPath := filepath.Join(root, "migrations", mig)
		content, err := os.ReadFile(migPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", migPath, err)
		}
		if _, err := db.Exec(ctx, string(content)); err != nil {
			// Ignore if already applied or table exists
			errStr := err.Error()
			if !strings.Contains(errStr, "already exists") && !strings.Contains(errStr, "duplicate key") {
				return fmt.Errorf("failed to apply %s: %w", mig, err)
			}
		}
	}

	return nil
}

// EnsureSupportMigrations ensures migration 000104 is applied safely on test database.
func EnsureSupportMigrations(ctx context.Context, db DBExecutor) error {
	if err := VerifyTestDatabase(ctx, db); err != nil {
		return err
	}

	root := findRepoRoot()
	if root == "" {
		return fmt.Errorf("failed to locate repo root (go.mod)")
	}
	migPath := filepath.Join(root, "migrations", "000104_create_support_conversations.up.sql")
	content, err := os.ReadFile(migPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", migPath, err)
	}

	statements := strings.Split(string(content), ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(ctx, stmt); err != nil {
			errStr := err.Error()
			if !strings.Contains(errStr, "already exists") && !strings.Contains(errStr, "duplicate key") {
				return fmt.Errorf("failed to apply statement in 000104: %w (stmt: %s)", err, stmt)
			}
		}
	}

	return nil
}
