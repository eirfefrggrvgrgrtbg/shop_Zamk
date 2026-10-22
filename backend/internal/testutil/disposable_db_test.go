package testutil_test

import (
	"context"
	"errors"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TEST.DB.4: the guard in front of every destructive test statement.

func TestVerifyDisposableDatabase_RefusesEverySharedOrUnknownName(t *testing.T) {
	for _, name := range []string{"zamk_test", "zamk", "zamk_demo", "postgres", "template1", "", "zamk_disposable_", "zamk_disposable_tpl_0123456789ab",
		"zamk_disposable_0123456789AB", "zamk_disposable_0123456789ab_x", "zamk_test_disposable_0123456789ab", "x_zamk_disposable_0123456789ab"} {
		err := testutil.VerifyDisposableDatabase(context.Background(), &mockDB{dbName: name})
		require.Error(t, err, "%q must be refused", name)
		require.Contains(t, err.Error(), "REFUSING DESTRUCTIVE TEST")
	}
}

func TestVerifyDisposableDatabase_RefusesWhenTheDatabaseCannotBeIdentified(t *testing.T) {
	require.Error(t, testutil.VerifyDisposableDatabase(context.Background(), nil))
	err := testutil.VerifyDisposableDatabase(context.Background(), &mockDB{err: errors.New("connection reset")})
	require.ErrorContains(t, err, "cannot identify the database")
}

func TestVerifyDisposableDatabase_AcceptsOnlyAPerTestDisposableName(t *testing.T) {
	require.NoError(t, testutil.VerifyDisposableDatabase(context.Background(), &mockDB{dbName: "zamk_disposable_0123456789ab"}))
}

func TestVerifyTestDatabase_StillRefusesDevAndDemo(t *testing.T) {
	for _, name := range []string{"zamk", "zamk_demo", "zamk_disposable_tpl_0123456789ab"} {
		require.Error(t, testutil.VerifyTestDatabase(context.Background(), &mockDB{dbName: name}), name)
	}
	require.NoError(t, testutil.VerifyTestDatabase(context.Background(), &mockDB{dbName: "zamk_disposable_0123456789ab"}))
}

// Real connection to the shared zamk_test: read-only identification only, the guard must refuse.
func TestMustBeDisposableDatabase_RefusesTheRealSharedTestDatabase(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testutil.CanonicalTestDatabaseDSN)
	require.NoError(t, err)
	defer pool.Close()
	err = testutil.VerifyDisposableDatabase(ctx, pool)
	require.ErrorContains(t, err, `"zamk_test" is not a disposable test database`)
}

// A destructive scenario runs only on the disposable database, which is dropped afterwards; zamk_test is untouched.
func TestDisposableDatabase_IsTheOnlyPlaceForDestructiveStatements(t *testing.T) {
	ctx := context.Background()
	shared, err := pgxpool.New(ctx, testutil.CanonicalTestDatabaseDSN)
	require.NoError(t, err)
	defer shared.Close()
	countShared := func() (n int) {
		require.NoError(t, shared.QueryRow(ctx, `SELECT (SELECT count(*) FROM users) + (SELECT count(*) FROM products) + (SELECT count(*) FROM sellers)`).Scan(&n))
		return n
	}
	before := countShared()

	var name string
	t.Run("destructive statements on the disposable database", func(t *testing.T) {
		pool := testutil.NewDisposableDatabase(t)
		require.NoError(t, pool.QueryRow(ctx, `SELECT current_database()`).Scan(&name))
		require.Regexp(t, `^zamk_disposable_[0-9a-f]{12}$`, name)
		var version int64
		var dirty bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty))
		require.False(t, dirty)
		require.GreaterOrEqual(t, version, int64(119), "fully migrated")

		testutil.MustBeDisposableDatabase(t, pool)
		_, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, name) VALUES (gen_random_uuid(), 'disposable@test.local', 'h', 'customer', 'D')`)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `TRUNCATE TABLE users CASCADE`)
		require.NoError(t, err)
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n))
		require.Equal(t, 0, n)
	})

	var exists bool
	require.NoError(t, shared.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists))
	require.False(t, exists, "the disposable database is dropped when its test ends")
	require.Equal(t, before, countShared(), "no row of the shared zamk_test changed")
}
