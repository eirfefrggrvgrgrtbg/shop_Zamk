package testutil

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FixtureTracker makes an integration test responsible for every row it creates.
//
// It is attached to the test's connection pool as a pgx QueryTracer and remembers each successful
// `INSERT INTO <table> (id, ...) VALUES ($1, ...)` (the id is taken from the bound argument). When the test ends,
// even when it failed or already closed its pool, a fresh connection deletes exactly those rows together with the
// rows that depend on them through foreign keys. Rows the test did not insert are never touched, so shared data of
// other tests, seeds and manual fixtures are safe.
//
// Cleanup runs after the cleanups the test registered later (t.Cleanup is LIFO), so it is a safety net behind the
// test's own deletes; deleting an already deleted row is a no-op.
type FixtureTracker struct {
	dsn   string
	since time.Time // database clock at creation: rows older than this are never the test's own

	mu      sync.Mutex
	entries []trackedRow
	seen    map[trackedRow]bool
}

type trackedRow struct {
	table string
	col   string // "id" (uuid) or the natural key column of tables without a uuid id
	id    string
}

// naturalKeys lists tables whose rows are created by code under test without an id and identified by a unique key.
var naturalKeys = map[string]string{"product_media_cleanup_jobs": "object_key"}

type queryKey struct{}

type queryInfo struct {
	sql  string
	args []any
}

// NewTrackedClient opens a postgres.Client whose inserts are tracked and cleaned up with the test.
func NewTrackedClient(t testing.TB, ctx context.Context, dsn string) (*postgres.Client, error) {
	t.Helper()
	tr := newFixtureTracker(dsn)
	client, err := postgres.NewClient(ctx, dsn, postgres.WithTracer(tr))
	if err != nil {
		return nil, err
	}
	tr.markStart(ctx, client.Pool)
	tr.register(t, client.Close)
	return client, nil
}

// NewTrackedPool opens a pgx pool whose inserts are tracked and cleaned up with the test.
func NewTrackedPool(t testing.TB, ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	t.Helper()
	tr := newFixtureTracker(dsn)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.Tracer = tr
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	tr.markStart(ctx, pool)
	tr.register(t, pool.Close)
	return pool, nil
}

func newFixtureTracker(dsn string) *FixtureTracker {
	return &FixtureTracker{dsn: dsn, since: time.Now().Add(-time.Minute), seen: map[trackedRow]bool{}}
}

// markStart records the database clock; called once the first connection exists.
func (f *FixtureTracker) markStart(ctx context.Context, pool *pgxpool.Pool) {
	var now time.Time
	if err := pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err == nil {
		f.since = now
	}
}

// register schedules closing the pool and purging the tracked rows. The purge uses its own connection, so it works
// after the test closed (or broke) its pool.
func (f *FixtureTracker) register(t testing.TB, closePool func()) {
	t.Cleanup(func() {
		if err := f.Purge(context.Background()); err != nil {
			t.Logf("fixture cleanup: %v", err)
		}
	})
	t.Cleanup(closePool) // runs first (LIFO): the purge does not need the test's pool
}

// TraceQueryStart implements pgx.QueryTracer.
func (f *FixtureTracker) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryKey{}, &queryInfo{sql: data.SQL, args: data.Args})
}

// TraceQueryEnd implements pgx.QueryTracer.
func (f *FixtureTracker) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err != nil || data.CommandTag.RowsAffected() == 0 {
		return // failed, or ON CONFLICT DO NOTHING inserted nothing: the row is not ours
	}
	info, _ := ctx.Value(queryKey{}).(*queryInfo)
	if info == nil {
		return
	}
	table, col, ids := trackableInsert(info.sql, info.args, data.CommandTag.RowsAffected())
	if table == "" {
		return
	}
	f.mu.Lock()
	for _, id := range ids {
		row := trackedRow{table, col, id}
		if !f.seen[row] {
			f.seen[row] = true
			f.entries = append(f.entries, row)
		}
	}
	f.mu.Unlock()
}

// parseInsertedIDs extracts the table and the uuid ids of an `INSERT INTO t (id, ...) VALUES (...)[, (...)]`
// statement whose id values are bound parameters or uuid literals. Anything else yields no ids.
var (
	onConflictRe     = regexp.MustCompile(`(?i)\bon\s+conflict\b`)
	doUpdateRe       = regexp.MustCompile(`(?i)\bdo\s+update\b`)
	multiStatementRe = regexp.MustCompile(`;\s*\S`)
)

// trackableInsert decides whether a successfully executed statement created rows that belong to the test, and
// which ones. It errs on the side of NOT tracking: an untracked row can only leak, a wrongly tracked one would
// delete somebody else's data.
//   - several statements in one call: the row count belongs to the last one, so nothing is tracked
//   - ON CONFLICT ... DO UPDATE: the affected row may be a pre-existing one, so nothing is tracked
//   - ON CONFLICT DO NOTHING: tracked only when every listed row was inserted (ids == rows affected)
func trackableInsert(sql string, args []any, rowsAffected int64) (table, col string, vals []string) {
	if multiStatementRe.MatchString(strings.TrimSpace(sql)) {
		return "", "", nil
	}
	table, col, vals = parseInsertedKeys(sql, args)
	if table == "" || len(vals) == 0 {
		return "", "", nil
	}
	if onConflictRe.MatchString(sql) {
		if doUpdateRe.MatchString(sql) || int64(len(vals)) != rowsAffected {
			return "", "", nil
		}
	}
	return table, col, vals
}

// parseInsertedKeys is parseInsertedIDs plus the natural-key tables (see naturalKeys).
func parseInsertedKeys(sql string, args []any) (table, col string, vals []string) {
	if t, ids := parseInsertedIDs(sql, args); t != "" && len(ids) > 0 {
		return t, "id", ids
	}
	m := insertHead.FindStringSubmatchIndex(sql)
	if m == nil {
		return "", "", nil
	}
	t := strings.ToLower(strings.Trim(sql[m[2]:m[3]], `"`))
	keyCol, ok := naturalKeys[t]
	if !ok {
		return "", "", nil
	}
	cols := splitTopLevel(sql[m[4]:m[5]])
	for i, c := range cols {
		if strings.EqualFold(strings.TrimSpace(c), keyCol) {
			rest := sql[m[1]:]
			open := strings.Index(rest, "(")
			if open < 0 {
				return "", "", nil
			}
			closeIdx := strings.Index(rest[open:], ")")
			tuple := splitTopLevel(rest[open+1 : open+closeIdx])
			if i < len(tuple) {
				if v := strings.TrimSpace(tuple[i]); strings.HasPrefix(v, "$") {
					if n, err := strconv.Atoi(v[1:]); err == nil && n >= 1 && n <= len(args) {
						if str, ok := args[n-1].(string); ok {
							return t, keyCol, []string{str}
						}
					}
				}
			}
		}
	}
	return "", "", nil
}

var insertHead = regexp.MustCompile(`(?is)^\s*insert\s+into\s+("?[a-z_][a-z0-9_.]*"?)\s*\(([^)]*)\)\s*values\b`)

// parseInsertedIDs extracts the table and the uuid ids of an `INSERT INTO t (id, ...) VALUES (...)[, (...)]`
// statement whose id values are bound parameters or uuid literals. Anything else yields no ids.
func parseInsertedIDs(sql string, args []any) (string, []string) {
	m := insertHead.FindStringSubmatchIndex(sql)
	if m == nil {
		return "", nil
	}
	table := strings.ToLower(strings.Trim(sql[m[2]:m[3]], `"`))
	table = strings.TrimPrefix(table, "public.")
	if strings.Contains(table, ".") {
		return "", nil
	}
	cols := splitTopLevel(sql[m[4]:m[5]])
	idCol := -1
	for i, c := range cols {
		if strings.EqualFold(strings.Trim(strings.TrimSpace(c), `"`), "id") {
			idCol = i
			break
		}
	}
	if idCol < 0 {
		return "", nil
	}

	var ids []string
	rest := sql[m[1]:]
	depth, start := 0, -1
scan:
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '(':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ')':
			depth--
			if depth == 0 && start >= 0 {
				vals := splitTopLevel(rest[start:i])
				if idCol < len(vals) {
					if id := resolveUUID(strings.TrimSpace(vals[idCol]), args); id != "" {
						ids = append(ids, id)
					}
				}
				start = -1
			}
		case '\'':
			for i++; i < len(rest) && rest[i] != '\''; i++ {
			}
		case ';':
			break scan
		default:
			if depth == 0 {
				tail := strings.ToUpper(strings.TrimSpace(rest[i:]))
				if strings.HasPrefix(tail, "ON CONFLICT") || strings.HasPrefix(tail, "RETURNING") {
					break scan
				}
			}
		}
	}
	return table, ids
}

func splitTopLevel(s string) []string {
	var out []string
	depth, last := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case '\'':
			for i++; i < len(s) && s[i] != '\''; i++ {
			}
		case ',':
			if depth == 0 {
				out = append(out, s[last:i])
				last = i + 1
			}
		}
	}
	return append(out, s[last:])
}

func resolveUUID(expr string, args []any) string {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "$") {
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(expr, "$"), "::uuid"))
		if err != nil || n < 1 || n > len(args) {
			return ""
		}
		return asUUID(args[n-1])
	}
	if strings.HasPrefix(expr, "'") {
		if end := strings.Index(expr[1:], "'"); end >= 0 {
			return asUUID(expr[1 : 1+end])
		}
	}
	return ""
}

func asUUID(v any) string {
	switch x := v.(type) {
	case uuid.UUID:
		return x.String()
	case *uuid.UUID:
		if x != nil {
			return x.String()
		}
	case string:
		if u, err := uuid.Parse(x); err == nil {
			return u.String()
		}
	case fmt.Stringer:
		if u, err := uuid.Parse(x.String()); err == nil {
			return u.String()
		}
	}
	return ""
}

// Tracked returns the rows recorded so far (table:id), for tests of the tracker itself.
func (f *FixtureTracker) Tracked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.entries))
	for i, e := range f.entries {
		out[i] = e.table + ":" + e.id
	}
	return out
}

type fkEdge struct {
	child, childCol, parent string
}

// Purge deletes the tracked rows (newest first) and everything that depends on them. It only runs against a disposable test database.
func (f *FixtureTracker) Purge(ctx context.Context) error {
	f.mu.Lock()
	entries := append([]trackedRow(nil), f.entries...)
	f.entries = nil // a second Purge has nothing left to do
	f.seen = map[trackedRow]bool{}
	f.mu.Unlock()
	if len(entries) == 0 {
		return nil
	}
	pool, err := pgxpool.New(ctx, f.dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := VerifyTestDatabase(ctx, pool); err != nil {
		return err
	}
	trackedSet := make(map[string]bool, len(entries))
	for _, e := range entries {
		trackedSet[e.table+":"+e.id] = true
	}
	p, err := newPurger(ctx, pool, f.since, trackedSet)
	if err != nil {
		return err
	}

	var errs []string
	for i := len(entries) - 1; i >= 0; {
		j := i
		var ids []string
		for j >= 0 && entries[j].table == entries[i].table && entries[j].col == entries[i].col {
			ids = append(ids, entries[j].id)
			j--
		}
		var err error
		if entries[i].col == "id" {
			err = p.deleteRows(ctx, entries[i].table, ids, 0)
		} else {
			_, err = pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE %s = ANY($1::text[])`, pgx.Identifier{entries[i].table}.Sanitize(), pgx.Identifier{entries[i].col}.Sanitize()), ids)
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", entries[i].table, err))
		}
		i = j
	}
	if len(errs) > 0 {
		return fmt.Errorf("could not remove all test fixtures: %s", strings.Join(errs, "; "))
	}
	return nil
}

type purger struct {
	pool       *pgxpool.Pool
	byParent   map[string][]fkEdge
	hasID      map[string]bool
	hasCreated map[string]bool
	visited    map[string]bool
	tracked    map[string]bool // "table:id" of rows the test inserted itself
	since      time.Time       // database clock when the test's pool was opened
}

func newPurger(ctx context.Context, pool *pgxpool.Pool, since time.Time, tracked map[string]bool) (*purger, error) {
	p := &purger{pool: pool, byParent: map[string][]fkEdge{}, hasID: map[string]bool{}, hasCreated: map[string]bool{}, visited: map[string]bool{}, tracked: tracked, since: since}
	rows, err := pool.Query(ctx, `
		SELECT cl.relname, (SELECT attname FROM pg_attribute WHERE attrelid=con.conrelid AND attnum=con.conkey[1]), pl.relname
		FROM pg_constraint con
		JOIN pg_class cl ON cl.oid = con.conrelid
		JOIN pg_class pl ON pl.oid = con.confrelid
		JOIN pg_namespace n ON n.oid = cl.relnamespace
		WHERE con.contype = 'f' AND n.nspname = 'public' AND array_length(con.conkey, 1) = 1
		  AND (SELECT attname FROM pg_attribute WHERE attrelid=con.confrelid AND attnum=con.confkey[1]) = 'id'`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e fkEdge
		if err := rows.Scan(&e.child, &e.childCol, &e.parent); err != nil {
			rows.Close()
			return nil, err
		}
		p.byParent[e.parent] = append(p.byParent[e.parent], e)
	}
	rows.Close()
	colRows, err := pool.Query(ctx, `SELECT table_name, column_name FROM information_schema.columns
		WHERE table_schema='public' AND column_name IN ('id','created_at')`)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()
	for colRows.Next() {
		var table, column string
		if err := colRows.Scan(&table, &column); err != nil {
			return nil, err
		}
		if column == "id" {
			p.hasID[table] = true
		} else {
			p.hasCreated[table] = true
		}
	}
	return p, colRows.Err()
}

// deleteRows removes rows of `table` with the given ids after removing every row that references them.
func (p *purger) deleteRows(ctx context.Context, table string, ids []string, depth int) error {
	if len(ids) == 0 {
		return nil
	}
	if depth > 10 {
		return fmt.Errorf("foreign-key chain too deep at %s", table)
	}
	var fresh []string
	for _, id := range ids {
		if key := table + ":" + id; !p.visited[key] {
			p.visited[key] = true
			fresh = append(fresh, id)
		}
	}
	if len(fresh) == 0 {
		return nil
	}
	for _, e := range p.byParent[table] {
		if e.child == table && e.childCol == "" {
			continue
		}
		if p.hasID[e.child] {
			createdExpr := "NULL::timestamptz"
			if p.hasCreated[e.child] {
				createdExpr = "created_at::timestamptz"
			}
			rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT id::text, %s FROM %s WHERE %s = ANY($1::uuid[])`, createdExpr, pgx.Identifier{e.child}.Sanitize(), pgx.Identifier{e.childCol}.Sanitize()), fresh)
			if err != nil {
				return err
			}
			var childIDs []string
			var foreign []string
			for rows.Next() {
				var id string
				var created *time.Time
				if err := rows.Scan(&id, &created); err != nil {
					rows.Close()
					return err
				}
				// A dependent that existed before the test started cannot be the test's own: never delete it
				// (the parent's delete then fails on the foreign key and everything stays intact).
				if created != nil && created.Before(p.since) && !p.tracked[e.child+":"+id] {
					foreign = append(foreign, id)
					continue
				}
				childIDs = append(childIDs, id)
			}
			rows.Close()
			if len(foreign) > 0 {
				return fmt.Errorf("refusing to delete %s row(s) %v that existed before the test started and reference %s", e.child, foreign, table)
			}
			if err := p.deleteRows(ctx, e.child, childIDs, depth+1); err != nil {
				return err
			}
		} else if _, err := p.pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE %s = ANY($1::uuid[])`, pgx.Identifier{e.child}.Sanitize(), pgx.Identifier{e.childCol}.Sanitize()), fresh); err != nil {
			return err
		}
	}
	_, err := p.pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ANY($1::uuid[])`, pgx.Identifier{table}.Sanitize()), fresh)
	return err
}
