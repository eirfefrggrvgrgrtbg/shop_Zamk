// Self-test of zamk_test_fixture_cleanup.cjs on a handful of ISOLATED fixture rows (no mass deletion, no existing data touched).
//   node backend/scripts/zamk_test_fixture_cleanup.selftest.cjs
const test = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('crypto');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');
const { Client } = require('pg');
const tool = require('./zamk_test_fixture_cleanup.cjs');

const DSN = process.env.TEST_DATABASE_URL || 'postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable';
const uuid = () => crypto.randomUUID();
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'zamk-cleanup-selftest-'));
const DICTS = ['categories', 'brands', 'colors', 'size_systems', 'size_values', 'category_size_systems', 'staff_roles', 'staff_role_permissions'];

let c; let fx; let manifestFile; let onlyUsers;
const tag = uuid().slice(0, 8);

async function counts(tables) { const o = {}; for (const t of tables) o[t] = (await c.query(`select count(*)::int n from "${t}"`)).rows[0].n; return o; }
const exists = async (table, id) => (await c.query(`select count(*)::int n from "${table}" where id = $1`, [id])).rows[0].n === 1;

async function makeFixtures() {
  const f = { cat: uuid(), u1: uuid(), s1: uuid(), su1: uuid(), p1: uuid(), p2: uuid(), v1: uuid(), u3: uuid(), s3: uuid(), su3: uuid(), p3: uuid() };
  await c.query(`INSERT INTO categories (id, name, slug) VALUES ($1, 'CL cat', $2)`, [f.cat, `cl-${tag}-${f.cat.slice(0, 6)}`]);
  for (const [u, s, su, p, n] of [[f.u1, f.s1, f.su1, f.p1, 1], [f.u3, f.s3, f.su3, f.p3, 3]]) {
    await c.query(`INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'seller', 'CL')`, [u, `cl-${n}-${u.slice(0, 8)}@test.com`]);
    await c.query(`INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'CL', $2, 'cl@test.com', 'active')`, [s, `cl-${s}`]);
    await c.query(`INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1, $2, $3, 'owner')`, [su, s, u]);
    await c.query(`INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, 'CL', $4, 'draft', 100, 'RUB')`, [p, s, f.cat, `cl-${p}`]);
  }
  await c.query(`INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, 'CL2', $4, 'draft', 100, 'RUB')`, [f.p2, f.s1, f.cat, `cl-${f.p2}`]);
  await c.query(`INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, $3, 100, true)`, [f.v1, f.p1, `CL-${f.v1.slice(0, 8)}`]);
  return f;
}
async function dropFixtures(f, extraProducts = []) {
  for (const p of extraProducts) await c.query(`DELETE FROM products WHERE id = $1`, [p]);
  await c.query(`DELETE FROM product_variants WHERE product_id = ANY($1::uuid[])`, [[f.p1, f.p2, f.p3]]);
  await c.query(`DELETE FROM products WHERE id = ANY($1::uuid[])`, [[f.p1, f.p2, f.p3]]);
  await c.query(`DELETE FROM seller_users WHERE id = ANY($1::uuid[])`, [[f.su1, f.su3]]);
  await c.query(`DELETE FROM sellers WHERE id = ANY($1::uuid[])`, [[f.s1, f.s3]]);
  await c.query(`DELETE FROM users WHERE id = ANY($1::uuid[])`, [[f.u1, f.u3]]);
  await c.query(`DELETE FROM categories WHERE id = $1`, [f.cat]);
}
const run = (over = {}) => tool.execute({ dsn: DSN, manifestPath: manifestFile, commit: false, onlyUsers, subsetOk: false, deps: {}, ...over });
const okDeps = (extra = {}) => ({ checkBackup: async () => ({}), authorize: async () => {}, ...extra });
const readManifest = () => JSON.parse(fs.readFileSync(manifestFile, 'utf8'));
const writeTampered = (m, name) => { const f = path.join(tmp, name); fs.writeFileSync(f, JSON.stringify(m)); return f; };
const rejects = (p, re) => assert.rejects(p, (e) => e instanceof tool.Refusal && re.test(e.message));

test.before(async () => {
  c = new Client({ connectionString: DSN }); await c.connect();
  assert.equal((await c.query('select current_database() d')).rows[0].d, 'zamk_test');
});
test.after(async () => { await c.end(); fs.rmSync(tmp, { recursive: true, force: true }); });

test.beforeEach(async () => {
  fx = await makeFixtures();
  onlyUsers = new Set([fx.u1]);
  manifestFile = path.join(tmp, `manifest-${uuid().slice(0, 6)}.json`);
  await tool.writeManifest(DSN, manifestFile, { onlyUsers });
});
test.afterEach(async () => { await dropFixtures(fx); });

test('the manifest of the isolated fixtures contains exactly the fixture graph', () => {
  const m = readManifest();
  assert.deepEqual(Object.keys(m.certain).sort(), ['product_variants', 'products', 'seller_users', 'sellers', 'users']);
  assert.deepEqual(m.certain.users, [fx.u1]);
  assert.deepEqual(m.certain.sellers, [fx.s1]);
  assert.deepEqual(m.certain.products.sort(), [fx.p1, fx.p2].sort());
  assert.equal(m.version, 2);
});

test('refuses any database other than zamk_test (before connecting) and non-local hosts', async () => {
  assert.throws(() => tool.assertTestDsn('postgres://zamk:x@localhost:5433/zamk?sslmode=disable'), tool.Refusal);
  assert.throws(() => tool.assertTestDsn('postgres://zamk:x@localhost:5433/postgres'), tool.Refusal);
  assert.throws(() => tool.assertTestDsn('postgres://zamk:x@db.example.com:5432/zamk_test'), tool.Refusal);
  tool.assertTestDsn(DSN);
  await rejects(run({ dsn: 'postgres://zamk:x@127.0.0.1:1/zamk' }), /not zamk_test/);
});

test('dry-run verifies everything and changes nothing', async () => {
  const before = await counts(['users', 'sellers', 'seller_users', 'products', 'product_variants', ...DICTS]);
  const r = await run();
  assert.equal(r.mode, 'dry-run');
  assert.deepEqual(r.problems, []);
  assert.equal(r.totalRows, 6);
  const order = r.plan.map((p) => p.table);
  assert.ok(order.indexOf('product_variants') < order.indexOf('products') && order.indexOf('products') < order.indexOf('sellers'), 'children are deleted before their parents');
  assert.deepEqual(await counts(Object.keys(before)), before);
  assert.equal(r.committed, false);
});

test('a modified manifest is refused', async () => {
  const m = readManifest();
  // 1. an id swapped: the recorded sha256 no longer matches
  const swapped = JSON.parse(JSON.stringify(m)); swapped.certain.products[0] = uuid();
  await rejects(run({ manifestPath: writeTampered(swapped, 'swapped.json'), commit: true, deps: okDeps() }), /modified/);
  // 2. forged: ids removed AND hash recomputed -> the live CONFIRMED set differs
  const { manifestHash } = require('./zamk_test_fixture_cleanup_dryrun.cjs');
  const forged = JSON.parse(JSON.stringify(m)); forged.certain.products = [fx.p1]; forged.counts.products = 1; forged.sha256 = manifestHash(forged.certain);
  await rejects(run({ manifestPath: writeTampered(forged, 'forged.json'), commit: true, expectSha: forged.sha256, deps: okDeps() }), /differ|manifest/);
  assert.ok(await exists('products', fx.p2) && await exists('products', fx.p1));
  // 3. a different table list
  const extra = JSON.parse(JSON.stringify(m)); extra.certain.categories = [fx.cat]; extra.counts.categories = 1; extra.rowHashes.categories = 'x';
  await rejects(run({ manifestPath: writeTampered(extra, 'extra.json'), deps: okDeps() }), /protected reference table/);
  // 4. wrong database named inside, malformed ids
  await rejects(run({ manifestPath: writeTampered({ ...m, database: 'zamk' }, 'db.json') }), /not zamk_test/);
  await rejects(run({ manifestPath: writeTampered({ ...m, certain: { users: [''] } }, 'bad.json') }), /malformed/);
  await rejects(run({ manifestPath: writeTampered({ ...m, certain: { 'users; drop table x': ['a'] } }, 'inj.json') }), /plain table name/);
  // 5. expected sha not matching
  await rejects(run({ expectSha: 'deadbeef' }), /not the expected/);
});

test('a dependent row outside the manifest blocks the run (live set drift, and FK check on its own)', async () => {
  const p4 = uuid();
  await c.query(`INSERT INTO products (id, seller_id, category_id, title, slug, status, price_cents, currency) VALUES ($1, $2, $3, 'late', $4, 'draft', 100, 'RUB')`, [p4, fx.s1, fx.cat, `cl-${p4}`]);
  try {
    await rejects(run({ commit: true, expectSha: readManifest().sha256, deps: okDeps() }), /differ/);
    // with --subset-ok the CONFIRMED-set comparison is relaxed, the foreign-key check still refuses
    await rejects(run({ commit: true, subsetOk: true, expectSha: readManifest().sha256, deps: okDeps() }), /outside the manifest reference/);
    for (const [t, id] of [['users', fx.u1], ['sellers', fx.s1], ['products', fx.p1], ['products', p4]]) assert.ok(await exists(t, id), `${t} row survived`);
  } finally { await c.query(`DELETE FROM products WHERE id = $1`, [p4]); }
});

test('a user referenced from outside is blocked (never nulled): excluded from the set, history and audit untouched', async () => {
  const audit = uuid();
  const staleManifest = readManifest(); // written before the audit row existed: still lists the user
  await c.query(`INSERT INTO audit_logs (id, actor_user_id, action) VALUES ($1, $2, 'cleanup-selftest')`, [audit, fx.u1]);
  try {
    // the stale manifest is refused (the live set no longer contains the user; the FK check also refuses it) ...
    await rejects(run({ commit: true, expectSha: staleManifest.sha256, deps: okDeps() }), /differ|reference/);
    await rejects(run({ commit: true, subsetOk: true, expectSha: staleManifest.sha256, deps: okDeps() }), /outside the manifest reference/);
    // ... there is no escape hatch
    const cli = spawnSync('node', [path.join(__dirname, 'zamk_test_fixture_cleanup.cjs'), '--manifest', manifestFile, '--allow-set-null', 'audit_logs.actor_user_id'], { encoding: 'utf8', env: { ...process.env, TEST_DATABASE_URL: DSN } });
    assert.equal(cli.status, 3);
    assert.match(cli.stderr, /no longer exists/);
    // a fresh manifest excludes the user and everything that depends on it
    const fresh = path.join(tmp, 'fresh.json');
    await tool.writeManifest(DSN, fresh, { onlyUsers });
    assert.deepEqual(JSON.parse(fs.readFileSync(fresh, 'utf8')).certain, {});
    const cl = new Client({ connectionString: DSN }); await cl.connect();
    try {
      await cl.query('BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY');
      const res = await require('./zamk_test_fixture_cleanup_dryrun.cjs').classify(cl, { light: true, onlyUsers });
      assert.deepEqual([...res.blocked.users], [fx.u1]);
      await cl.query('ROLLBACK');
    } finally { await cl.end(); }
    const row = (await c.query(`SELECT actor_user_id, action FROM audit_logs WHERE id = $1`, [audit])).rows[0];
    assert.deepEqual(row, { actor_user_id: fx.u1, action: 'cleanup-selftest' });
    for (const [t, id] of [['users', fx.u1], ['sellers', fx.s1], ['products', fx.p1]]) assert.ok(await exists(t, id), `${t} row survived`);
  } finally { await c.query(`DELETE FROM audit_logs WHERE id = $1`, [audit]); }
});

test('a manifest that still lists a blocked user is refused by the FK check even when forged consistently', async () => {
  const audit = uuid();
  await c.query(`INSERT INTO audit_logs (id, actor_user_id, action) VALUES ($1, $2, 'cleanup-selftest')`, [audit, fx.u1]);
  try {
    await rejects(run({ subsetOk: true }), /outside the manifest reference users/);
    assert.equal((await c.query(`SELECT actor_user_id FROM audit_logs WHERE id = $1`, [audit])).rows[0].actor_user_id, fx.u1);
  } finally { await c.query(`DELETE FROM audit_logs WHERE id = $1`, [audit]); }
});

test('a row that changed after the manifest was written is refused', async () => {
  await c.query(`UPDATE products SET title = 'changed' WHERE id = $1`, [fx.p1]);
  await rejects(run({ commit: true, expectSha: readManifest().sha256, deps: okDeps() }), /row contents changed/);
  assert.ok(await exists('products', fx.p1));
});

test('rows outside the manifest and dictionaries are never touched, and the commit deletes exactly the manifest', async () => {
  const watch = ['users', 'sellers', 'seller_users', 'products', 'product_variants', ...DICTS];
  const before = await counts(watch);
  const m = readManifest();
  const r = await run({ commit: true, expectSha: m.sha256, deps: okDeps() });
  assert.equal(r.committed, true);
  assert.deepEqual(r.deleted, { product_variants: 1, products: 2, seller_users: 1, sellers: 1, users: 1 });
  const after = await counts(watch);
  assert.equal(after.users, before.users - 1);
  assert.equal(after.sellers, before.sellers - 1);
  assert.equal(after.seller_users, before.seller_users - 1);
  assert.equal(after.products, before.products - 2);
  assert.equal(after.product_variants, before.product_variants - 1);
  for (const d of DICTS) assert.equal(after[d], before[d], `${d} untouched`);
  for (const [t, id] of [['users', fx.u3], ['sellers', fx.s3], ['products', fx.p3], ['categories', fx.cat]]) assert.ok(await exists(t, id), `${t} bystander survived`);
  for (const [t, id] of [['users', fx.u1], ['sellers', fx.s1], ['products', fx.p1], ['products', fx.p2], ['product_variants', fx.v1]]) assert.ok(!(await exists(t, id)), `${t} row deleted`);
});

test('any error during the deletion rolls everything back', async () => {
  const watch = ['users', 'sellers', 'seller_users', 'products', 'product_variants'];
  const before = await counts(watch);
  const m = readManifest();
  let seen = 0;
  await assert.rejects(run({ commit: true, expectSha: m.sha256, deps: okDeps({ afterTable: async () => { if (++seen === 3) throw new Error('injected failure after the third table'); } }) }), /injected failure/);
  assert.equal(seen, 3, 'three tables had already been deleted inside the transaction');
  assert.deepEqual(await counts(watch), before);
  for (const [t, id] of [['users', fx.u1], ['sellers', fx.s1], ['seller_users', fx.su1], ['products', fx.p1], ['products', fx.p2], ['product_variants', fx.v1]]) assert.ok(await exists(t, id), `${t} restored`);
  // and a failure at the very end, after every table was deleted and verified
  await assert.rejects(run({ commit: true, expectSha: m.sha256, deps: okDeps({ beforeCommit: async () => { throw new Error('injected failure before commit'); } }) }), /before commit/);
  assert.deepEqual(await counts(watch), before);
});

test('--commit needs the backup, the authorization phrase and a terminal', async () => {
  const m = readManifest();
  await rejects(run({ commit: true, expectSha: m.sha256, deps: { authorize: async () => {} } }), /backup/);
  await rejects(run({ commit: true, expectSha: m.sha256, backup: '/nonexistent', deps: { authorize: async () => {} } }), /does not exist/);
  await rejects(run({ commit: true, deps: okDeps() }), /--expect-sha is required/);
  await rejects(run({ commit: true, expectSha: m.sha256, deps: { checkBackup: async () => ({}) } }), /no authorization/);
  assert.ok(await exists('users', fx.u1), 'nothing was deleted by any refused attempt');
  // the CLI: dry-run is the default; --commit without the exact phrase / a terminal is refused
  const cli = (args) => spawnSync('node', [path.join(__dirname, 'zamk_test_fixture_cleanup.cjs'), ...args], { encoding: 'utf8', env: { ...process.env, TEST_DATABASE_URL: DSN }, input: '' });
  const noPhrase = cli(['--manifest', manifestFile, '--commit', '--expect-sha', m.sha256, '--backup', '/nonexistent']);
  assert.equal(noPhrase.status, 3);
  assert.match(noPhrase.stderr, /REFUSED/);
  const wrongDb = spawnSync('node', [path.join(__dirname, 'zamk_test_fixture_cleanup.cjs'), '--manifest', manifestFile], { encoding: 'utf8', env: { ...process.env, TEST_DATABASE_URL: 'postgres://zamk:x@localhost:5433/zamk' } });
  assert.equal(wrongDb.status, 3);
  assert.match(wrongDb.stderr, /not zamk_test/);
  assert.ok(await exists('users', fx.u1));
});

test('the backup check', async () => {
  const mk = (name, body, ageHours = 0) => { const f = path.join(tmp, name); fs.writeFileSync(f, body); const t = new Date(Date.now() - ageHours * 3600000); fs.utimesSync(f, t, t); return f; };
  const good = 'PGDMP' + '\0'.repeat(10) + ' zamk_test ' + 'x'.repeat(2000);
  const manifest = { snapshot: { n: new Date(Date.now() - 1000).toISOString() } };
  assert.ok((await tool.checkBackupFile(mk('good.dump', good), manifest)).bytes > 1024);
  await rejects(tool.checkBackupFile(undefined, manifest), /required/);
  await rejects(tool.checkBackupFile(path.join(tmp, 'nope'), manifest), /does not exist/);
  await rejects(tool.checkBackupFile(mk('tiny.dump', 'PGDMP zamk_test'), manifest), /too small/);
  await rejects(tool.checkBackupFile(mk('stale.dump', good, 8), manifest), /older than 6 hours/);
  await rejects(tool.checkBackupFile(mk('other.dump', 'PGDMP' + ' zamk '.repeat(1) + 'x'.repeat(2000)), manifest), /does not name database/);
  await rejects(tool.checkBackupFile(mk('junk.dump', 'hello zamk_test '.repeat(200)), manifest), /not a pg_dump/);
  const older = { snapshot: { n: new Date(Date.now() + 60000).toISOString() } };
  await rejects(tool.checkBackupFile(mk('before.dump', good), older), /older than the manifest snapshot/);
  assert.ok((await tool.checkBackupFile(mk('plain.sql', '-- PostgreSQL database dump\n-- Dumped from database zamk_test\n' + 'x'.repeat(2000)), manifest)).bytes > 1024);
});
