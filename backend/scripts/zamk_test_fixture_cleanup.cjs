#!/usr/bin/env node
// TEST.DB.3 — removal of CONFIRMED test fixtures from zamk_test, from a manifest written by the dry-run.
//
//   node backend/scripts/zamk_test_fixture_cleanup.cjs --manifest m.json                  # DRY-RUN (default): nothing is modified
//   node backend/scripts/zamk_test_fixture_cleanup.cjs --write-manifest out.json          # snapshot the CONFIRMED set (v2: counts + row hashes)
//   node backend/scripts/zamk_test_fixture_cleanup.cjs --manifest m.json --commit \
//        --expect-sha <sha> --backup <pg_dump file> --authorization "<phrase printed by the dry-run>"   # real deletion
//
// Nothing here is ever run by an agent without the Product Owner's separate, explicit authorization.
//
// Guards (every one of them aborts before anything is deleted):
//   1  the connection string must name database zamk_test on a local host; current_database() is checked again
//   2  the manifest must be well formed, name zamk_test, contain only plain table names and unique ids
//   3  the CONFIRMED set is derived again from the live database and must equal the manifest (or, with --subset-ok,
//      the manifest must be a subset of it); ids, counts and a canonical sha256 are compared
//   4  v2 manifests carry a hash of every row's content: rows changed since the dry-run are refused
//   5  every foreign key (composite included, ANY delete rule) that points into the manifest must come from a row that is
//      itself in the manifest, and every owning parent of a manifest row must be in the manifest. The one exception is an
//      history/audit authors are never nulled: there is no --allow-set-null, a user referenced from outside is excluded by
//      the classifier (blocked_by_external_reference) and a manifest that still contains one is refused
//   6  reference/dictionary tables are never deletable
//   7  rows are deleted child-first with an exact per-table row count; ON DELETE CASCADE is not relied upon: a cascade
//      would make the count differ and abort
//   8  one PostgreSQL transaction; the involved tables are locked (SHARE ROW EXCLUSIVE) so no dependency can appear
//      between verification and deletion; any error, mismatch or refusal rolls everything back
//   9  --commit needs --expect-sha, the exact authorization phrase AND a typed confirmation on a terminal, and a fresh
//      pg_dump backup of zamk_test taken after the manifest snapshot
//  10 object storage (S3/MinIO) is never touched
const { Client } = require('pg');
const fs = require('fs');
const readline = require('readline');
const { classify, manifestHash, rowHash: rowHashOf } = require('./zamk_test_fixture_cleanup_dryrun.cjs');

const DB = 'zamk_test';
const LOCAL_HOSTS = new Set(['localhost', '127.0.0.1', '::1', '[::1]']);
// dictionaries / reference data created by migrations, and migration bookkeeping: never deletable by this tool
const PROTECTED_TABLES = new Set([
  'categories', 'brands', 'colors', 'size_systems', 'size_values', 'category_size_systems', 'category_size_chart_fields',
  'category_attribute_definitions', 'attribute_definitions', 'attribute_dictionaries', 'attribute_dictionary_values',
  'materials', 'delivery_methods', 'staff_roles', 'staff_role_permissions', 'schema_migrations',
]);
const IDENT = /^[a-z_][a-z0-9_]*$/;
const q = (name) => `"${name}"`;

class Refusal extends Error {}
const refuse = (msg) => { throw new Refusal(msg); };

function assertTestDsn(dsn) {
  let u;
  try { u = new URL(dsn); } catch { refuse('the connection string is not a valid URL'); }
  const name = decodeURIComponent(u.pathname.replace(/^\//, ''));
  if (name !== DB) refuse(`database "${name}" is not ${DB}: refusing to run`);
  if (!LOCAL_HOSTS.has(u.hostname)) refuse(`host "${u.hostname}" is not local: refusing to run`);
}

function loadManifest(file, { requireV2 }) {
  let m;
  try { m = JSON.parse(fs.readFileSync(file, 'utf8')); } catch (e) { refuse(`cannot read the manifest: ${e.message}`); }
  if (!m || typeof m !== 'object' || typeof m.certain !== 'object' || m.certain === null || Array.isArray(m.certain)) refuse('malformed manifest: "certain" must be an object of id lists');
  if (m.database !== DB) refuse(`the manifest was made for "${m.database}", not ${DB}`);
  for (const [t, ids] of Object.entries(m.certain)) {
    if (!IDENT.test(t)) refuse(`malformed manifest: "${t}" is not a plain table name`);
    if (PROTECTED_TABLES.has(t)) refuse(`table ${t} is a protected reference table and cannot be in a manifest`);
    if (!Array.isArray(ids) || ids.length === 0 || ids.some((x) => typeof x !== 'string' || x === '')) refuse(`malformed manifest: ids of ${t} must be a non-empty list of strings`);
    if (new Set(ids).size !== ids.length) refuse(`malformed manifest: duplicate ids in ${t}`);
  }
  if (requireV2) {
    if (m.version !== 2 || !m.sha256 || !m.rowHashes || !m.counts) refuse('a v2 manifest (counts + row hashes) is required to commit: write one with --write-manifest');
    for (const t of Object.keys(m.certain)) {
      if (m.counts[t] !== m.certain[t].length) refuse(`manifest count of ${t} does not match its id list`);
      if (!m.rowHashes[t]) refuse(`manifest has no row hash for ${t}`);
    }
    if (manifestHash(m.certain) !== m.sha256) refuse('the manifest content does not match its recorded sha256 (it was modified)');
  }
  return m;
}

const idCol = (hasId, alias) => (hasId ? `${alias}.id::text` : `${alias}.ctid::text`);

const rowHash = rowHashOf;

// every table that must be locked: the manifest tables and every table with a foreign key into one of them
async function tablesToLock(c, manifestTables) {
  const rows = (await c.query(`
    SELECT DISTINCT cl.relname t FROM pg_constraint con
    JOIN pg_class cl ON cl.oid = con.conrelid JOIN pg_class pl ON pl.oid = con.confrelid
    JOIN pg_namespace n ON n.oid = cl.relnamespace
    WHERE con.contype = 'f' AND n.nspname = 'public' AND pl.relname = ANY($1::text[])`, [manifestTables])).rows.map((r) => r.t);
  return [...new Set([...manifestTables, ...rows])].sort();
}

// everything that must hold before the first DELETE; returns the plan
async function verify(c, manifest, o) {
  const res = await classify(c, { light: true, onlyUsers: o.onlyUsers });
  const { A, fks, hasId } = res;
  const mTables = Object.keys(manifest.certain);
  const mSets = Object.fromEntries(mTables.map((t) => [t, new Set(manifest.certain[t])]));
  const report = { tables: [], problems: [] };
  const problem = (msg) => report.problems.push(msg);

  // 3. the manifest against the live CONFIRMED set
  const derivedTables = Object.keys(A).filter((t) => A[t].size > 0);
  for (const t of new Set([...mTables, ...derivedTables])) {
    const m = mSets[t] || new Set();
    const d = A[t] || new Set();
    const missing = [...m].filter((id) => !d.has(id)).length; // in the manifest, no longer confirmed
    const extra = o.subsetOk ? 0 : [...d].filter((id) => !m.has(id)).length; // confirmed now, not in the manifest
    if (missing || extra) problem(`${t}: manifest and live CONFIRMED set differ (${missing} not confirmed any more, ${extra} new)`);
  }

  // 5. dependencies, from the live catalog and independent of the classifier
  for (const f of fks) {
    const list = (t, cols) => cols.map((x) => `${q(t)}.${q(x)}`).join(', ');
    const pSet = mSets[f.parent];
    if (pSet) { // rows pointing INTO the manifest must be in the manifest (any delete rule, so SET NULL cannot touch outsiders either)
      const inChild = mSets[f.child] ? [...mSets[f.child]] : [];
      const bad = (await c.query(`SELECT count(*)::int n FROM ${q(f.child)} WHERE (${list(f.child, f.ccols)}) IS NOT NULL
        AND (${list(f.child, f.ccols)}) IN (SELECT ${list(f.parent, f.pcols)} FROM ${q(f.parent)} WHERE ${idCol(hasId.has(f.parent), q(f.parent))} = ANY($1::text[]))
        AND NOT (${idCol(hasId.has(f.child), q(f.child))} = ANY($2::text[]))`, [[...pSet], inChild])).rows[0].n;
      if (bad) problem(`${bad} row(s) of ${f.child} outside the manifest reference ${f.parent} rows of the manifest (${f.conname})`);
    }
    const cSet = mSets[f.child];
    if (cSet && f.owns && mSets[f.parent] !== undefined) { // owning parents of manifest rows must be in the manifest
      const bad = (await c.query(`SELECT count(*)::int n FROM ${q(f.child)} WHERE ${idCol(hasId.has(f.child), q(f.child))} = ANY($1::text[])
        AND (${list(f.child, f.ccols)}) IS NOT NULL AND NOT EXISTS (SELECT 1 FROM ${q(f.parent)} WHERE (${list(f.parent, f.pcols)}) = (${list(f.child, f.ccols)})
          AND ${idCol(hasId.has(f.parent), q(f.parent))} = ANY($2::text[]))`, [[...cSet], [...mSets[f.parent]]])).rows[0].n;
      if (bad) problem(`${bad} manifest row(s) of ${f.child} belong to a ${f.parent} row that is not in the manifest (${f.conname})`);
    }
  }

  // 4. content hashes of the rows themselves
  for (const t of mTables) {
    const live = await rowHash(c, t, manifest.certain[t], hasId.has(t));
    const entry = { table: t, rows: manifest.certain[t].length, found: live.count, hashOk: null };
    if (live.count !== manifest.certain[t].length) problem(`${t}: ${live.count} of ${manifest.certain[t].length} manifest rows exist`);
    if (manifest.rowHashes) {
      entry.hashOk = manifest.rowHashes[t] === live.hash;
      if (!entry.hashOk) problem(`${t}: row contents changed since the manifest was written`);
    }
    report.tables.push(entry);
  }

  // deletion order, children first, over the manifest tables
  const deps = Object.fromEntries(mTables.map((t) => [t, new Set()]));
  for (const f of fks) if (f.owns && deps[f.parent] && deps[f.child] && f.parent !== f.child) deps[f.parent].add(f.child);
  const order = []; const done = new Set(); const stack = new Set();
  const visit = (t) => { if (done.has(t) || stack.has(t)) return; stack.add(t); for (const ch of deps[t]) visit(ch); stack.delete(t); done.add(t); order.push(t); };
  mTables.forEach(visit);
  report.order = order;
  report.hasId = hasId;
  return report;
}

async function countsOf(c, tables) {
  const out = {};
  for (const t of tables) out[t] = (await c.query(`SELECT count(*)::int n FROM ${q(t)}`)).rows[0].n;
  return out;
}

async function execute(o) {
  const deps = { checkBackup: checkBackupFile, authorize: async () => refuse('no authorization handler'), afterTable: async () => {}, beforeCommit: async () => {}, ...o.deps };
  assertTestDsn(o.dsn);
  const manifest = loadManifest(o.manifestPath, { requireV2: o.commit });
  const sha = manifestHash(manifest.certain);
  if (o.expectSha && o.expectSha !== sha) refuse(`manifest sha256 ${sha} is not the expected ${o.expectSha}`);
  const totalRows = Object.values(manifest.certain).reduce((a, ids) => a + ids.length, 0);
  const phrase = authorizationPhrase(totalRows, sha);
  if (o.commit) {
    if (!o.expectSha) refuse('--expect-sha is required to commit');
    await deps.checkBackup(o.backup, manifest);
    await deps.authorize({ phrase, rows: totalRows, sha });
  }

  const c = new Client({ connectionString: o.dsn });
  await c.connect();
  let finished = false;
  try {
    const name = (await c.query('select current_database() d')).rows[0].d;
    if (name !== DB) refuse(`connected to "${name}", not ${DB}`);
    await c.query(o.commit ? 'BEGIN' : 'BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY');
    const mTables = Object.keys(manifest.certain);
    let locked = [];
    if (o.commit) {
      await c.query(`SET LOCAL lock_timeout = '20s'`);
      await c.query(`SET LOCAL statement_timeout = '1h'`);
      await c.query(`SET LOCAL idle_in_transaction_session_timeout = '30min'`);
      locked = await tablesToLock(c, mTables);
      await c.query(`LOCK TABLE ${locked.map(q).join(', ')} IN SHARE ROW EXCLUSIVE MODE`);
    }
    const plan = await verify(c, manifest, o);
    const result = { mode: o.commit ? 'commit' : 'dry-run', database: name, manifestSha256: sha, totalRows, phrase, plan: plan.order.map((t, i) => ({ step: i + 1, table: t, rows: manifest.certain[t].length })), problems: plan.problems, tables: plan.tables, deleted: {}, committed: false };
    if (plan.problems.length) refuse(`verification failed:\n  - ${plan.problems.join('\n  - ')}`);
    if (!o.commit) { await c.query('ROLLBACK'); finished = true; return result; }

    const before = await countsOf(c, locked);
    for (const t of plan.order) {
      const r = await c.query(`DELETE FROM ${q(t)} t WHERE ${idCol(plan.hasId.has(t), 't')} = ANY($1::text[])`, [manifest.certain[t]]);
      result.deleted[t] = r.rowCount;
      if (r.rowCount !== manifest.certain[t].length) refuse(`${t}: deleted ${r.rowCount} rows, the manifest lists ${manifest.certain[t].length}`);
      await deps.afterTable(t);
    }
    const after = await countsOf(c, locked);
    for (const t of locked) {
      const expected = before[t] - (result.deleted[t] || 0);
      if (after[t] !== expected) refuse(`${t}: ${after[t]} rows remain, expected ${expected} (something outside the manifest changed)`);
    }
    for (const t of plan.order) {
      const left = (await c.query(`SELECT count(*)::int n FROM ${q(t)} t WHERE ${idCol(plan.hasId.has(t), 't')} = ANY($1::text[])`, [manifest.certain[t]])).rows[0].n;
      if (left) refuse(`${t}: ${left} manifest rows are still present`);
    }
    result.before = before; result.after = after;
    await deps.beforeCommit(result);
    await c.query('COMMIT');
    finished = true;
    result.committed = true;
    return result;
  } catch (e) {
    if (!finished) { try { await c.query('ROLLBACK'); } catch { /* connection already gone: the server rolls back */ } }
    throw e;
  } finally {
    await c.end().catch(() => {});
  }
}

const authorizationPhrase = (rows, sha) => `DELETE ${rows} FIXTURE ROWS FROM ${DB} MANIFEST ${sha.slice(0, 16)}`;

// a fresh, consistent pg_dump of zamk_test taken after the manifest snapshot
async function checkBackupFile(file, manifest, opts = {}) {
  if (!file) refuse('--backup <pg_dump file of zamk_test> is required to commit');
  let st;
  try { st = fs.statSync(file); } catch { refuse(`backup file ${file} does not exist`); }
  if (!st.isFile() || st.size < 1024) refuse('the backup file is empty or too small to be a database dump');
  const maxAgeH = opts.maxAgeHours ?? 6;
  const now = opts.now ?? Date.now();
  if (now - st.mtimeMs > maxAgeH * 3600 * 1000) refuse(`the backup is older than ${maxAgeH} hours`);
  const snapshotAt = manifest && manifest.snapshot && manifest.snapshot.n ? new Date(manifest.snapshot.n).getTime() : 0;
  if (snapshotAt && st.mtimeMs < snapshotAt) refuse('the backup is older than the manifest snapshot: take the backup after writing the manifest');
  const fd = fs.openSync(file, 'r');
  const head = Buffer.alloc(8192);
  const n = fs.readSync(fd, head, 0, head.length, 0);
  fs.closeSync(fd);
  const text = head.subarray(0, n).toString('latin1');
  const custom = text.startsWith('PGDMP');
  const plain = /-- PostgreSQL database dump/.test(text);
  if (!custom && !plain) refuse('the file is not a pg_dump archive (custom "PGDMP" or plain SQL)');
  if (!text.includes(DB)) refuse(`the dump does not name database ${DB}`);
  return { file, bytes: st.size, ageHours: (now - st.mtimeMs) / 3600000 };
}

function cliAuthorize(provided) {
  return async ({ phrase }) => {
    if (provided !== phrase) refuse(`--authorization must be exactly: ${phrase}`);
    if (!process.stdin.isTTY) refuse('an interactive terminal is required: the Product Owner must type the confirmation');
    const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
    const typed = await new Promise((resolve) => rl.question(`Type the phrase to confirm the irreversible deletion:\n${phrase}\n> `, resolve));
    rl.close();
    if (typed.trim() !== phrase) refuse('the typed confirmation does not match');
  };
}

async function writeManifest(dsn, out, opts = {}) {
  assertTestDsn(dsn);
  const c = new Client({ connectionString: dsn });
  await c.connect();
  try {
    if ((await c.query('select current_database() d')).rows[0].d !== DB) refuse(`not ${DB}`);
    await c.query('BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY');
    const res = await classify(c, { light: true, onlyUsers: opts.onlyUsers });
    const certain = {}; const counts = {}; const rowHashes = {};
    for (const t of res.order) {
      certain[t] = [...res.A[t]];
      counts[t] = certain[t].length;
      rowHashes[t] = (await rowHash(c, t, certain[t], res.hasId.has(t))).hash;
    }
    await c.query('ROLLBACK');
    const manifest = { version: 2, database: DB, snapshot: res.snapshotAt, certain, counts, rowHashes, sha256: manifestHash(certain) };
    fs.writeFileSync(out, JSON.stringify(manifest));
    return manifest;
  } finally { await c.end(); }
}

function printResult(r) {
  console.log(`${r.mode.toUpperCase()} on ${r.database}${r.mode === 'dry-run' ? ' — nothing is modified' : ''}`);
  console.log(`manifest sha256 ${r.manifestSha256}, ${r.totalRows} rows in ${r.plan.length} tables`);
  console.table(r.plan.map((p) => ({ step: p.step, table: p.table, rows: p.rows, deleted: r.deleted[p.table] ?? '', before: r.before ? r.before[p.table] : '', after: r.after ? r.after[p.table] : '' })));
  console.log(r.problems.length ? `PROBLEMS:\n- ${r.problems.join('\n- ')}` : 'All verifications passed.');
  if (r.mode === 'dry-run') {
    console.log('\nTo delete (Product Owner only, separate explicit authorization; needs a pg_dump of zamk_test taken after the manifest):');
    console.log(`  --commit --expect-sha ${r.manifestSha256} --backup <dump file> --authorization "${r.phrase}"`);
  } else {
    console.log(r.committed ? 'COMMITTED.' : 'NOT COMMITTED (rolled back).');
  }
}

async function main() {
  const args = process.argv.slice(2);
  const get = (n) => (args.includes(n) ? args[args.indexOf(n) + 1] : undefined);
  const dsn = process.env.TEST_DATABASE_URL || 'postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable';
  if (get('--write-manifest')) {
    if (args.includes('--commit')) refuse('--write-manifest cannot be combined with --commit');
    const m = await writeManifest(dsn, get('--write-manifest'));
    console.log(`manifest v2 written: ${Object.values(m.counts).reduce((a, b) => a + b, 0)} rows in ${Object.keys(m.counts).length} tables, sha256 ${m.sha256}`);
    return;
  }
  if (!get('--manifest')) refuse('--manifest <file> is required');
  const commit = args.includes('--commit');
  if (args.includes('--allow-set-null')) refuse('--allow-set-null no longer exists: history and audit authors are never nulled');
  const result = await execute({
    dsn, manifestPath: get('--manifest'), commit, expectSha: get('--expect-sha'), backup: get('--backup'),
    subsetOk: args.includes('--subset-ok'),
    deps: { authorize: cliAuthorize(get('--authorization')), checkBackup: (f, m) => checkBackupFile(f, m, { maxAgeHours: Number(get('--backup-max-age-hours') || 6) }) },
  });
  printResult(result);
}

module.exports = { execute, loadManifest, checkBackupFile, writeManifest, assertTestDsn, authorizationPhrase, Refusal, PROTECTED_TABLES };
if (require.main === module) main().catch((e) => { console.error(e instanceof Refusal ? `REFUSED: ${e.message}` : `FAILED: ${e.message}`); process.exit(e instanceof Refusal ? 3 : 1); });
