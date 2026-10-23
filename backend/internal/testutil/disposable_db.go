package testutil

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TEST.DB.4: destructive integration tests (TRUNCATE, DELETE without a scope) never run on a shared database.
// They get their own throw-away database, cloned from a migrated template on the local test server, and every
// destructive statement is preceded by MustBeDisposableDatabase. zamk, zamk_test and zamk_demo can never match.

// DisposableDatabasePrefix is the only name prefix a destructive test may run on.
const DisposableDatabasePrefix = "zamk_disposable_"

var (
	disposableNameRE = regexp.MustCompile(`^zamk_disposable_[0-9a-f]{12}$`)
	templateNameRE   = regexp.MustCompile(`^zamk_disposable_tpl_[0-9a-f]{12}$`)
	localHosts       = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}
)

// VerifyDisposableDatabase returns nil only when the connected database is a per-test disposable database.
// Any other name (zamk, zamk_test, zamk_demo, the template, …) or a failure to identify it is a refusal.
func VerifyDisposableDatabase(ctx context.Context, db DBQueryRow) error {
	if db == nil {
		return fmt.Errorf("REFUSING DESTRUCTIVE TEST: database connection is nil")
	}
	qctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var name string
	if err := db.QueryRow(qctx, "SELECT current_database()").Scan(&name); err != nil {
		return fmt.Errorf("REFUSING DESTRUCTIVE TEST: cannot identify the database: %w", err)
	}
	if !disposableNameRE.MatchString(name) {
		return fmt.Errorf("REFUSING DESTRUCTIVE TEST: %q is not a disposable test database (%s<12 hex>); shared databases are never truncated", name, DisposableDatabasePrefix)
	}
	return nil
}

// MustBeDisposableDatabase stops the test before any destructive statement unless it runs on a disposable database.
func MustBeDisposableDatabase(t testing.TB, db DBQueryRow) {
	t.Helper()
	if err := VerifyDisposableDatabase(context.Background(), db); err != nil {
		t.Fatalf("%v", err)
	}
}

// disposableServerDSN derives the server connection from TEST_DATABASE_URL (never from the dev database settings):
// it must point at a local PostgreSQL; the database part is replaced.
func disposableServerDSN(dbName string) (string, error) {
	u, err := url.Parse(sharedServerDSN())
	if err != nil {
		return "", fmt.Errorf("invalid TEST_DATABASE_URL: %w", err)
	}
	if !localHosts[u.Hostname()] {
		return "", fmt.Errorf("REFUSING: disposable databases are only created on a local server, not %q", u.Hostname())
	}
	u.Path = "/" + dbName
	return u.String(), nil
}

func migrationsDir() (string, error) {
	root := findRepoRoot()
	if root == "" {
		return "", fmt.Errorf("cannot find the backend module root")
	}
	return filepath.Join(root, "migrations"), nil
}

type migrationFile struct {
	version uint64
	path    string
}

// upMigrations lists *.up.sql in numeric version order (the order golang-migrate applies them) and a content hash.
func upMigrations() ([]migrationFile, string, error) {
	dir, err := migrationsDir()
	if err != nil {
		return nil, "", err
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, "", err
	}
	var files []migrationFile
	for _, p := range paths {
		base := filepath.Base(p)
		v, err := strconv.ParseUint(base[:strings.Index(base, "_")], 10, 64)
		if err != nil {
			return nil, "", fmt.Errorf("unexpected migration name %s", base)
		}
		files = append(files, migrationFile{v, p})
	}
	sort.Slice(files, func(a, b int) bool { return files[a].version < files[b].version })
	h := sha256.New()
	for _, f := range files {
		b, err := os.ReadFile(f.path)
		if err != nil {
			return nil, "", err
		}
		fmt.Fprintf(h, "%d:%s\n", f.version, filepath.Base(f.path))
		h.Write(b)
	}
	return files, hex.EncodeToString(h.Sum(nil))[:12], nil
}

// ensureTemplate creates (once per migration set) a migrated template database; concurrent test binaries serialize
// on an advisory lock. The template is never used directly by a test and never dropped by this helper.
func ensureTemplate(ctx context.Context, admin *pgx.Conn) (string, error) {
	files, hash, err := upMigrations()
	if err != nil {
		return "", err
	}
	tpl := DisposableDatabasePrefix + "tpl_" + hash
	if !templateNameRE.MatchString(tpl) {
		return "", fmt.Errorf("bad template name %q", tpl)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(hashtext('zamk_disposable_template'))`); err != nil {
		return "", err
	}
	defer admin.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext('zamk_disposable_template'))`)
	var exists bool
	if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, tpl).Scan(&exists); err != nil {
		return "", err
	}
	if exists {
		return tpl, nil
	}
	building := tpl + "_build"
	_, _ = admin.Exec(ctx, `DROP DATABASE IF EXISTS `+pgx.Identifier{building}.Sanitize()+` WITH (FORCE)`) // only a half-built template of ours
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{building}.Sanitize()+` TEMPLATE template0`); err != nil {
		return "", fmt.Errorf("create template: %w", err)
	}
	dsn, err := disposableServerDSN(building)
	if err != nil {
		return "", err
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		sqlBytes, err := os.ReadFile(f.path)
		if err != nil {
			conn.Close(ctx)
			return "", err
		}
		if _, err := conn.Exec(ctx, string(sqlBytes)); err != nil {
			conn.Close(ctx)
			return "", fmt.Errorf("migration %s: %w", filepath.Base(f.path), err)
		}
	}
	last := files[len(files)-1].version
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL)`); err != nil {
		conn.Close(ctx)
		return "", err
	}
	if _, err := conn.Exec(ctx, `INSERT INTO schema_migrations (version, dirty) VALUES ($1, false)`, int64(last)); err != nil {
		conn.Close(ctx)
		return "", err
	}
	conn.Close(ctx)
	if _, err := admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{building}.Sanitize()+` RENAME TO `+pgx.Identifier{tpl}.Sanitize()); err != nil {
		return "", err
	}
	return tpl, nil
}

// createDisposable clones the migrated template into a new zamk_disposable_<12 hex> database and returns its DSN.
func createDisposable(ctx context.Context) (string, string, error) {
	adminDSN, err := disposableServerDSN("postgres")
	if err != nil {
		return "", "", err
	}
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return "", "", fmt.Errorf("connect to the local test server: %w", err)
	}
	defer admin.Close(ctx)
	tpl, err := ensureTemplate(ctx, admin)
	if err != nil {
		return "", "", fmt.Errorf("prepare the disposable template: %w", err)
	}
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	name := DisposableDatabasePrefix + hex.EncodeToString(b)
	if !disposableNameRE.MatchString(name) {
		return "", "", fmt.Errorf("bad disposable name %q", name)
	}
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()+` TEMPLATE `+pgx.Identifier{tpl}.Sanitize()); err != nil {
		return "", "", fmt.Errorf("create disposable database: %w", err)
	}
	dsn, err := disposableServerDSN(name)
	if err != nil {
		dropDisposable(name)
		return "", "", err
	}
	return name, dsn, nil
}

// NewDisposableDatabase gives the test its own fully migrated database (zamk_disposable_<12 hex>) on the local test
// server and drops it when the test ends. Destructive tests use only this pool; they never connect to zamk_test.
func NewDisposableDatabase(t testing.TB) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	name, dsn, err := createDisposable(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to %s: %v", name, err)
	}
	MustBeDisposableDatabase(t, pool)
	t.Cleanup(func() {
		pool.Close()
		dropDisposable(name)
	})
	return pool
}

// DisposableDatabaseURL is NewDisposableDatabase for tests that open their own connections from a DSN.
func DisposableDatabaseURL(t testing.TB) string {
	t.Helper()
	pool := NewDisposableDatabase(t)
	var name string
	if err := pool.QueryRow(context.Background(), "SELECT current_database()").Scan(&name); err != nil {
		t.Fatalf("%v", err)
	}
	dsn, err := disposableServerDSN(name)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return dsn
}

// dropDisposable removes only a per-test disposable database (never the template, never a shared database).
func dropDisposable(name string) {
	if !disposableNameRE.MatchString(name) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn, err := disposableServerDSN("postgres")
	if err != nil {
		return
	}
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return
	}
	defer admin.Close(ctx)
	_, _ = admin.Exec(ctx, `DROP DATABASE IF EXISTS `+pgx.Identifier{name}.Sanitize()+` WITH (FORCE)`)
}

// UseDisposableDatabaseForPackage is for a TestMain of a package whose tests wipe whole tables: the package runs on
// its own disposable database. TEST_DATABASE_URL is pointed at it for this test process only (so every helper of the
// package, GetTestDatabaseURL included, uses it); the returned cleanup drops it. Call cleanup before os.Exit.
func UseDisposableDatabaseForPackage() (string, func(), error) {
	ctx := context.Background()
	name, dsn, err := createDisposable(ctx)
	if err != nil {
		return "", func() {}, err
	}
	prev, had := os.LookupEnv("TEST_DATABASE_URL")
	if err := os.Setenv("TEST_DATABASE_URL", dsn); err != nil {
		dropDisposable(name)
		return "", func() {}, err
	}
	return dsn, func() {
		if had {
			_ = os.Setenv("TEST_DATABASE_URL", prev)
		} else {
			_ = os.Unsetenv("TEST_DATABASE_URL")
		}
		dropDisposable(name)
	}, nil
}

// RunnerDatabase describes one throw-away database created for a whole test run by backend/scripts/safe_go_test.sh.
type RunnerDatabase struct {
	Name, DSN, Token, Host, Port, User, Password string
}

// CreateRunnerDatabase clones the migrated template into a new zamk_disposable_<12 hex> database, stamps it with a
// random ownership token (database comment) and re-connects to prove the connected database is that disposable one.
// On any failure nothing it created is left behind.
func CreateRunnerDatabase(ctx context.Context) (*RunnerDatabase, error) {
	name, dsn, err := createDisposable(ctx)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*RunnerDatabase, error) {
		dropDisposable(name)
		return nil, err
	}
	tb := make([]byte, 16)
	if _, err := rand.Read(tb); err != nil {
		return fail(err)
	}
	token := hex.EncodeToString(tb)
	adminDSN, err := disposableServerDSN("postgres")
	if err != nil {
		return fail(err)
	}
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return fail(err)
	}
	_, err = admin.Exec(ctx, `COMMENT ON DATABASE `+pgx.Identifier{name}.Sanitize()+` IS 'zamk-test-runner:`+token+`'`)
	admin.Close(ctx)
	if err != nil {
		return fail(fmt.Errorf("stamp the disposable database: %w", err))
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fail(err)
	}
	err = VerifyDisposableDatabase(ctx, conn)
	conn.Close(ctx)
	if err != nil {
		return fail(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return fail(err)
	}
	pw, _ := u.User.Password()
	return &RunnerDatabase{Name: name, DSN: dsn, Token: token, Host: u.Hostname(), Port: u.Port(), User: u.User.Username(), Password: pw}, nil
}

// DropRunnerDatabase drops exactly the database the runner created: the name must be a per-run disposable name (never
// the template, never a shared database) AND its ownership stamp must match the token handed out at creation.
func DropRunnerDatabase(ctx context.Context, name, token string) error {
	if !disposableNameRE.MatchString(name) {
		return fmt.Errorf("REFUSING TO DROP %q: not a per-run disposable database name", name)
	}
	if len(token) != 32 {
		return fmt.Errorf("REFUSING TO DROP %q: missing ownership token", name)
	}
	adminDSN, err := disposableServerDSN("postgres")
	if err != nil {
		return err
	}
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return err
	}
	defer admin.Close(ctx)
	var stamp *string
	if err := admin.QueryRow(ctx, `SELECT shobj_description(oid, 'pg_database') FROM pg_database WHERE datname = $1`, name).Scan(&stamp); err != nil {
		if err == pgx.ErrNoRows {
			return nil // already gone
		}
		return err
	}
	if stamp == nil || *stamp != "zamk-test-runner:"+token {
		return fmt.Errorf("REFUSING TO DROP %q: it is not owned by this runner", name)
	}
	_, err = admin.Exec(ctx, `DROP DATABASE IF EXISTS `+pgx.Identifier{name}.Sanitize()+` WITH (FORCE)`)
	return err
}
