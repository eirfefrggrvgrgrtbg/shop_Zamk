package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestParseInsertedIDs(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	cases := []struct {
		name  string
		sql   string
		args  []any
		table string
		ids   []string
	}{
		{"bound id", `INSERT INTO categories (id, name, slug) VALUES ($1, 'Cat', $2)`, []any{a, "s"}, "categories", []string{a.String()}},
		{"id not first", `INSERT INTO products (seller_id, id, title) VALUES ($1, $2, 'x')`, []any{b, a}, "products", []string{a.String()}},
		{"two rows", `INSERT INTO users (id, email) VALUES ($1, 'a'), ($2, 'b')`, []any{a, b}, "users", []string{a.String(), b.String()}},
		{"string arg", "INSERT INTO sellers (id, brand_name) VALUES ($1, 'B')", []any{a.String(), "x"}, "sellers", []string{a.String()}},
		{"literal uuid", `INSERT INTO brands (id, name) VALUES ('` + a.String() + `', 'B')`, nil, "brands", []string{a.String()}},
		{"function call in values", `INSERT INTO t (id, created_at) VALUES ($1, COALESCE(now(), now()))`, []any{a}, "t", []string{a.String()}},
		{"on conflict", `INSERT INTO size_systems (id, code) VALUES ($1, 'INT') ON CONFLICT (code) DO NOTHING`, []any{a}, "size_systems", []string{a.String()}},
		{"schema prefix", `insert into public.colors (id, code) values ($1, 'c')`, []any{a}, "colors", []string{a.String()}},
		{"table name contains VALUES", `INSERT INTO size_values (id, size_system_id, value, is_active) VALUES ($1, $2, $3, true)`, []any{a, b, "M"}, "size_values", []string{a.String()}},
		{"no id column", `INSERT INTO seller_users (seller_id, user_id) VALUES ($1, $2)`, []any{a, b}, "", nil},
		{"generated id", `INSERT INTO inventory_items (id, product_id) VALUES (gen_random_uuid(), $1)`, []any{a}, "inventory_items", nil},
		{"not an insert", `UPDATE products SET title = 'x' WHERE id = $1`, []any{a}, "", nil},
		{"insert select", `INSERT INTO t (id) SELECT id FROM u`, nil, "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			table, ids := parseInsertedIDs(c.sql, c.args)
			require.Equal(t, c.table, table)
			require.Equal(t, c.ids, ids)
		})
	}
}

// The tracker removes exactly what its test inserted, with dependents, even after the test closed its pool,
// and leaves rows it did not insert alone.
func TestFixtureTrackerPurgesOwnRowsOnly(t *testing.T) {
	ctx := context.Background()
	dsn := GetTestDatabaseURL()
	bystander := uuid.New()
	owned := struct{ cat, prod, seller, user uuid.UUID }{uuid.New(), uuid.New(), uuid.New(), uuid.New()}

	client, err := NewTrackedClient(t, ctx, dsn)
	require.NoError(t, err)
	AssertTestDatabase(t, client.Pool)
	other, err := NewTrackedClient(t, ctx, dsn) // an independent tracked client that inserts the bystander and removes it itself
	require.NoError(t, err)
	_, err = other.Pool.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'FT bystander', $2)`, bystander, "ft-by-"+bystander.String()[:8])
	require.NoError(t, err)

	t.Run("fixtures of one test", func(t *testing.T) {
		c, err := NewTrackedClient(t, ctx, dsn)
		require.NoError(t, err)
		tag := owned.cat.String()[:8]
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO categories (id, name, slug) VALUES ($1, 'FT cat', $2)`, []any{owned.cat, "ft-" + tag}},
			{`INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'seller', 'FT')`, []any{owned.user, fmt.Sprintf("ft-%s@test.com", tag)}},
			{`INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'FT', $2, 'ft@test.com', 'active')`, []any{owned.seller, "ft-" + tag}},
			{`INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, 'FT', $4, 'draft', 100, 'RUB')`, []any{owned.prod, owned.seller, owned.cat, "ft-p-" + tag}},
		} {
			_, err := c.Pool.Exec(ctx, q.sql, q.args...)
			require.NoError(t, err)
		}
		// a dependent row without a tracked insert (default id) and the test closing its pool early
		_, err = c.Pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES (gen_random_uuid(), $1, 'FT-SKU', 100, true)`, owned.prod)
		require.NoError(t, err)
		c.Close()
	})

	count := func(table string, id uuid.UUID) int {
		var n int
		require.NoError(t, client.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE id = $1`, table), id).Scan(&n))
		return n
	}
	require.Zero(t, count("products", owned.prod))
	require.Zero(t, count("sellers", owned.seller))
	require.Zero(t, count("users", owned.user))
	require.Zero(t, count("categories", owned.cat))
	var variants int
	require.NoError(t, client.Pool.QueryRow(ctx, `SELECT count(*) FROM product_variants WHERE product_id = $1`, owned.prod).Scan(&variants))
	require.Zero(t, variants, "dependents without a tracked id go with their parent")
	require.Equal(t, 1, count("categories", bystander), "rows of other owners are untouched")
}

func TestParseInsertedKeysNaturalKey(t *testing.T) {
	table, col, vals := parseInsertedKeys(`INSERT INTO product_media_cleanup_jobs (object_key, next_attempt_at, generation) VALUES ($1, NOW(), 1) ON CONFLICT (object_key) DO UPDATE SET generation = 2`, []any{"products/a/b.jpg"})
	require.Equal(t, "product_media_cleanup_jobs", table)
	require.Equal(t, "object_key", col)
	require.Equal(t, []string{"products/a/b.jpg"}, vals)

	table, _, _ = parseInsertedKeys(`INSERT INTO other_table (object_key) VALUES ($1)`, []any{"x"})
	require.Empty(t, table, "only listed natural-key tables are tracked")
}

func TestTrackableInsertIsConservative(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	cases := []struct {
		name string
		sql  string
		args []any
		rows int64
		want int // number of ids tracked
	}{
		{"plain insert", `INSERT INTO categories (id, name) VALUES ($1, 'x')`, []any{a}, 1, 1},
		{"do nothing, inserted", `INSERT INTO categories (id, name) VALUES ($1, 'x') ON CONFLICT (id) DO NOTHING`, []any{a}, 1, 1},
		{"do nothing, partially conflicting two rows", `INSERT INTO categories (id, name) VALUES ($1, 'x'), ($2, 'y') ON CONFLICT DO NOTHING`, []any{a, b}, 1, 0},
		{"do nothing, both inserted", `INSERT INTO categories (id, name) VALUES ($1, 'x'), ($2, 'y') ON CONFLICT DO NOTHING`, []any{a, b}, 2, 2},
		{"upsert may touch an existing row", `INSERT INTO categories (id, name) VALUES ($1, 'x') ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`, []any{a}, 1, 0},
		{"several statements in one call", `DELETE FROM categories WHERE false; INSERT INTO categories (id, name) VALUES ($1, 'x')`, []any{a}, 1, 0},
		{"insert then another statement", `INSERT INTO categories (id, name) VALUES ($1, 'x'); UPDATE categories SET name = 'y' WHERE id = $1`, []any{a}, 1, 0},
		{"trailing semicolon only", `INSERT INTO categories (id, name) VALUES ($1, 'x');`, []any{a}, 1, 1},
		{"insert select is unsupported", `INSERT INTO categories (id, name) SELECT id, name FROM other`, nil, 1, 0},
		{"cte insert is unsupported", `WITH x AS (SELECT 1) INSERT INTO categories (id, name) VALUES ($1, 'x')`, []any{a}, 1, 0},
		{"id from an expression is unsupported", `INSERT INTO categories (id, name) VALUES (uuid_generate_v4(), 'x')`, nil, 1, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, vals := trackableInsert(c.sql, c.args, c.rows)
			require.Len(t, vals, c.want)
		})
	}
}

func TestFixtureTrackerNeverTouchesRowsItDidNotCreate(t *testing.T) {
	ctx := context.Background()
	dsn := GetTestDatabaseURL()
	owner, err := NewTrackedClient(t, ctx, dsn) // creates the pre-existing rows and removes them itself at the end
	require.NoError(t, err)
	AssertTestDatabase(t, owner.Pool)
	exists := func(id uuid.UUID) bool {
		var n int
		require.NoError(t, owner.Pool.QueryRow(ctx, `SELECT count(*) FROM categories WHERE id = $1`, id).Scan(&n))
		return n == 1
	}
	insert := func(pool interface {
		Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	}, id uuid.UUID, tail string) error {
		_, err := pool.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'FT pre', $2)`+tail, id, "ft-pre-"+id.String()[:8])
		return err
	}

	pre := uuid.New()
	require.NoError(t, insert(owner.Pool, pre, ""))

	t.Run("a failed insert is not registered as a created fixture", func(t *testing.T) {
		c, err := NewTrackedClient(t, ctx, dsn)
		require.NoError(t, err)
		require.Error(t, insert(c.Pool, pre, "")) // duplicate primary key
	})
	require.True(t, exists(pre), "the pre-existing row survived the failed insert's cleanup")

	t.Run("ON CONFLICT DO NOTHING does not claim the existing row", func(t *testing.T) {
		c, err := NewTrackedClient(t, ctx, dsn)
		require.NoError(t, err)
		require.NoError(t, insert(c.Pool, pre, " ON CONFLICT (id) DO NOTHING"))
	})
	require.True(t, exists(pre))

	t.Run("an upsert of an existing row does not claim it", func(t *testing.T) {
		c, err := NewTrackedClient(t, ctx, dsn)
		require.NoError(t, err)
		require.NoError(t, insert(c.Pool, pre, " ON CONFLICT (id) DO UPDATE SET name = 'FT touched'"))
	})
	require.True(t, exists(pre))

	t.Run("a dependent row that existed before the test is never deleted, even if its parent was tracked by mistake", func(t *testing.T) {
		parent, child := uuid.New(), uuid.New()
		require.NoError(t, insert(owner.Pool, parent, ""))
		_, err := owner.Pool.Exec(ctx, `INSERT INTO categories (id, parent_id, name, slug) VALUES ($1, $2, 'FT child', $3)`, child, parent, "ft-child-"+child.String()[:8])
		require.NoError(t, err)

		f := newFixtureTracker(dsn) // the test "starts" now: both rows are older
		f.markStart(ctx, owner.Pool)
		f.entries = append(f.entries, trackedRow{"categories", "id", parent.String()})
		err = f.Purge(ctx)
		require.Error(t, err, "the purge refuses instead of deleting a foreign dependent")
		require.Contains(t, err.Error(), "existed before the test started")
		require.True(t, exists(parent))
		require.True(t, exists(child))
	})

	t.Run("purging twice is harmless", func(t *testing.T) {
		f := newFixtureTracker(dsn)
		f.markStart(ctx, owner.Pool)
		id := uuid.New()
		require.NoError(t, insert(owner.Pool, id, ""))
		f.entries = append(f.entries, trackedRow{"categories", "id", id.String()})
		require.NoError(t, f.Purge(ctx))
		require.False(t, exists(id))
		require.NoError(t, f.Purge(ctx))
	})
}
