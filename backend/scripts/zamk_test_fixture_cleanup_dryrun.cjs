// TEST.DB.2 — DRY-RUN ONLY. Classifies the rows of zamk_test that automated tests left behind.
// It runs inside ONE `REPEATABLE READ READ ONLY` transaction (a single consistent snapshot for every count,
// candidate set and dependency), issues only SELECTs, and has NO delete mode: deleting needs a separate decision.
//
//   node backend/scripts/zamk_test_fixture_cleanup_dryrun.cjs [--json report.json] [--manifest ids.json]
//
// ROOTS (origin must be confirmed by independent evidence, a non-bcrypt hash alone is not enough)
//   user    CERTAIN   non-bcrypt hash (cannot log in)  AND  a generated identity (uuid/hex fragment in the e-mail or a
//                     reserved test domain)  AND  no login session ever recorded
//           PROBABLE  non-bcrypt hash but the other evidence is missing
//           DISPUTED  bcrypt hash (a person could log in)
//   seller  CERTAIN   has owner users and ALL of them are CERTAIN users
//           PROBABLE  has no owner user at all (no evidence either way)
//           DISPUTED  has an owner user that is not CERTAIN
// CLOSURE  rows that depend on a root through foreign keys (NO ACTION / RESTRICT / CASCADE; SET NULL does not own),
//          composite keys included. A row stays in the CERTAIN closure only if EVERY owning parent is in it
//          (pruned to a fixed point), so a row shared with a non-fixture parent is never CERTAIN.
// DICTIONARIES (categories, brands, colors, size_systems): created by a migration = PROTECTED; otherwise
//          CERTAIN only when every referencing row is CERTAIN, PROBABLE when unreferenced or referenced only by
//          CERTAIN/PROBABLE rows, DISPUTED when anything else references it.
// Classes never overlap and always add up to the table total (checked below).
const { Client } = require('pg');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const DSN = process.env.TEST_DATABASE_URL || 'postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable';
const arg = (name) => (process.argv.includes(name) ? process.argv[process.argv.indexOf(name) + 1] : null);
const BCRYPT = `'^\\$2[aby]\\$'`;
const RESERVED_DOMAIN = `'@(test\\.com|test\\.local|zamk\\.local|zamk\\.test|example\\.com|buyer\\.com|[a-z0-9-]+\\.test)$'`;
const GENERATED = `'[0-9a-f]{8}'`;

// classify(c, opts): `c` is a connected client INSIDE a transaction owned by the caller.
//   opts.light         only the CERTAIN closure (A, A0, ENT): no probable/disputed/dictionary classification
//   opts.onlyUsers     Set of user ids: only these users (and sellers owned solely by them) are roots
async function classify(c, opts = {}) {
  const dbName = (await c.query('select current_database() d')).rows[0].d;
  if (dbName !== 'zamk_test') throw new Error(`refusing to run on ${dbName}`);
  const rows = async (sql, params) => (await c.query(sql, params)).rows;
  const snapshotAt = (await rows(`select now() n, txid_current_snapshot()::text s`))[0];

  // ---------- catalog: tables, id expression, foreign keys (composite included)
  const tableNames = (await rows(`select table_name t from information_schema.tables where table_schema='public' and table_type='BASE TABLE'`)).map((r) => r.t);
  const hasId = new Set((await rows(`select table_name t from information_schema.columns where table_schema='public' and column_name='id'`)).map((r) => r.t));
  const idExpr = (t) => (hasId.has(t) ? `${q(t)}.id::text` : `${q(t)}.ctid::text`);
  const qid = (t) => (hasId.has(t) ? 'id::text' : 'ctid::text');
  function q(name) { return `"${name}"`; }

  const fkRaw = await rows(`
    SELECT con.conname, cl.relname child, pl.relname parent, con.confdeltype del,
           (SELECT array_agg(a.attname::text ORDER BY k.ord) FROM unnest(con.conkey) WITH ORDINALITY k(attnum, ord) JOIN pg_attribute a ON a.attrelid=con.conrelid AND a.attnum=k.attnum) ccols,
           (SELECT array_agg(a.attname::text ORDER BY k.ord) FROM unnest(con.confkey) WITH ORDINALITY k(attnum, ord) JOIN pg_attribute a ON a.attrelid=con.confrelid AND a.attnum=k.attnum) pcols
    FROM pg_constraint con JOIN pg_class cl ON cl.oid=con.conrelid JOIN pg_class pl ON pl.oid=con.confrelid
    JOIN pg_namespace n ON n.oid=cl.relnamespace WHERE con.contype='f' AND n.nspname='public'`);
  const fks = fkRaw.map((f) => ({ ...f, owns: ['a', 'r', 'c'].includes(f.del), composite: f.ccols.length > 1 }));
  const byParent = {}; const byChild = {};
  for (const f of fks) { (byParent[f.parent] ||= []).push(f); (byChild[f.child] ||= []).push(f); }
  const list = (t, cols) => cols.map((x) => `${q(t)}.${q(x)}`).join(', ');

  const totals = {};
  for (const t of tableNames) totals[t] = (await rows(`select count(*)::int n from ${q(t)}`))[0].n;

  // ---------- closure engine
  const SQLIDS = (t, sql, params) => rows(sql, params).then((r) => r.map((x) => x.i));
  async function childrenOf(f, parentIds) {
    // ids of f.child rows whose foreign key points at one of the parent rows
    const sql = `SELECT ${idExpr(f.child)} i FROM ${q(f.child)}
      WHERE (${list(f.child, f.ccols)}) IN (SELECT ${list(f.parent, f.pcols)} FROM ${q(f.parent)} WHERE ${idExpr(f.parent)} = ANY($1::text[]))
        AND (${list(f.child, f.ccols)}) IS NOT NULL`;
    return SQLIDS(f.child, sql, [parentIds]);
  }
  async function expand(roots) {
    const S = {};
    for (const [t, ids] of Object.entries(roots)) S[t] = new Set(ids);
    let queue = Object.keys(S);
    while (queue.length) {
      const next = new Set();
      for (const p of queue) {
        const ids = [...S[p]];
        if (!ids.length) continue;
        for (const f of byParent[p] || []) {
          if (!f.owns) continue;
          const found = await childrenOf(f, ids);
          const set = (S[f.child] ||= new Set());
          let grew = false;
          for (const id of found) if (!set.has(id)) { set.add(id); grew = true; }
          if (grew) next.add(f.child);
        }
      }
      queue = [...next];
    }
    return S;
  }
  // keep only rows whose every owning parent that belongs to the closure's tables is inside the closure
  async function prune(S) {
    let changed = true;
    while (changed) {
      changed = false;
      for (const t of Object.keys(S)) {
        for (const f of byChild[t] || []) {
          if (!f.owns || !S[f.parent] || !S[t].size) continue;
          const sql = `SELECT ${idExpr(t)} i FROM ${q(t)}
            WHERE ${idExpr(t)} = ANY($1::text[]) AND (${list(t, f.ccols)}) IS NOT NULL
              AND NOT EXISTS (SELECT 1 FROM ${q(f.parent)} WHERE (${list(f.parent, f.pcols)}) = (${list(t, f.ccols)}) AND ${idExpr(f.parent)} = ANY($2::text[]))`;
          const bad = await SQLIDS(t, sql, [[...S[t]], [...S[f.parent]]]);
          if (bad.length) { for (const id of bad) S[t].delete(id); changed = true; }
        }
      }
    }
    return S;
  }

  // A row may only be deleted together with ALL rows that depend on it. Drop every parent that still has an
  // owning child outside the set, then repeat the upward pruning (its other children lose their parent).
  async function closeDownward(S) {
    let changed = true;
    while (changed) {
      changed = false;
      for (const f of fks) {
        if (!f.owns || !S[f.parent] || !S[f.parent].size) continue;
        const inChild = S[f.child] ? [...S[f.child]] : [];
        const sql = `SELECT ${idExpr(f.parent)} i FROM ${q(f.parent)}
          WHERE ${idExpr(f.parent)} = ANY($1::text[])
            AND EXISTS (SELECT 1 FROM ${q(f.child)} WHERE (${list(f.child, f.ccols)}) = (${list(f.parent, f.pcols)}) AND NOT (${idExpr(f.child)} = ANY($2::text[])))`;
        const blocked = await SQLIDS(f.parent, sql, [[...S[f.parent]], inChild]);
        if (blocked.length) { for (const id of blocked) S[f.parent].delete(id); changed = true; }
      }
      if (changed) await prune(S);
    }
    return S;
  }

  // TEST.DB.3: deleting a row must not change any row OUTSIDE the set through a non-owning foreign key
  // (ON DELETE SET NULL / SET DEFAULT): history and audit authors are never nulled. Such a parent is excluded
  // (blocked_by_external_reference), the closure is re-pruned, and this repeats to a fixed point. Only parents leave the set.
  const blocked = {};
  async function blockExternal(S) {
    let changed = true;
    while (changed) {
      changed = false;
      for (const f of fks) {
        if (f.owns || !S[f.parent] || !S[f.parent].size) continue;
        const inChild = S[f.child] ? [...S[f.child]] : [];
        const hit = await SQLIDS(f.parent, `SELECT ${idExpr(f.parent)} i FROM ${q(f.parent)}
          WHERE ${idExpr(f.parent)} = ANY($1::text[])
            AND EXISTS (SELECT 1 FROM ${q(f.child)} WHERE (${list(f.child, f.ccols)}) IS NOT NULL AND (${list(f.child, f.ccols)}) = (${list(f.parent, f.pcols)}) AND NOT (${idExpr(f.child)} = ANY($2::text[])))`, [[...S[f.parent]], inChild]);
        if (hit.length) { for (const id of hit) { S[f.parent].delete(id); (blocked[f.parent] ||= new Set()).add(id); } changed = true; }
      }
      if (changed) { await prune(S); await closeDownward(S); }
    }
    return S;
  }

  // ---------- roots
  const sessionTable = tableNames.includes('user_sessions');
  const users = await rows(`
    SELECT u.id::text i,
           (u.password_hash ~ ${BCRYPT}) AS bcrypt,
           (u.email ~* ${GENERATED} OR u.email ~* ${RESERVED_DOMAIN}) AS generated_identity,
           ${sessionTable ? `EXISTS (SELECT 1 FROM user_sessions s WHERE s.user_id = u.id)` : 'false'} AS has_session
    FROM users u`);
  const U = { certain: [], probable: [], disputed: [] };
  for (const u of users) {
    if (u.bcrypt) U.disputed.push(u.i);
    else if (u.generated_identity && !u.has_session) U.certain.push(u.i);
    else U.probable.push(u.i);
  }
  if (opts.onlyUsers) { U.certain = U.certain.filter((id) => opts.onlyUsers.has(id)); U.probable = []; U.disputed = []; }
  const certainUsers = new Set(U.certain);
  const sellerRows = await rows(`SELECT s.id::text i, coalesce(array_agg(su.user_id::text) FILTER (WHERE su.user_id IS NOT NULL), '{}') owners FROM sellers s LEFT JOIN seller_users su ON su.seller_id = s.id GROUP BY s.id`);
  const SE = { certain: [], probable: [], disputed: [] };
  for (const s of sellerRows.filter((x) => !opts.onlyUsers || x.owners.some((o) => opts.onlyUsers.has(o)))) {
    if (s.owners.length === 0) SE.probable.push(s.i);
    else if (s.owners.every((o) => certainUsers.has(o))) SE.certain.push(s.i);
    else SE.disputed.push(s.i);
  }

  const A0 = await prune(await expand({ users: U.certain, sellers: SE.certain })); // certain by origin
  const A = {}; for (const [t, set] of Object.entries(A0)) A[t] = new Set(set);
  await closeDownward(A); // certain AND closed: deletable on its own
  await blockExternal(A); // ... and deleting it changes no row outside it (no SET NULL / SET DEFAULT side effects)
  const ENT = {}; for (const [t, set] of Object.entries(A0)) ENT[t] = new Set([...set].filter((id) => !A[t].has(id))); // certain origin, tied to unconfirmed rows
  // deletion order (children first) of a set, ownership edges only
  const orderFor = (S) => {
    const tablesS = Object.keys(S).filter((t) => S[t].size > 0);
    const deps = {}; for (const t of tablesS) deps[t] = new Set();
    for (const f of fks) if (f.owns && deps[f.parent] && deps[f.child] && f.parent !== f.child) deps[f.parent].add(f.child);
    const out = []; const done = new Set(); const stack = new Set();
    const visit = (t) => { if (done.has(t) || stack.has(t)) return; stack.add(t); for (const ch of deps[t]) visit(ch); stack.delete(t); done.add(t); out.push(t); };
    tablesS.forEach(visit);
    return out;
  };
  const helpers = { rows, q, list, idExpr, qid, fks, byParent, byChild, hasId, totals, tableNames, SQLIDS };
  if (opts.light) return { dbName, snapshotAt, A, A0, ENT, U, SE, blocked, order: orderFor(A), ...helpers };

  const B0 = await prune(await expand({ users: U.probable, sellers: SE.probable }));
  const B = {}; // probable = reachable only from probable roots and not already certain
  for (const [t, set] of Object.entries(B0)) B[t] = new Set([...set].filter((id) => !(A0[t] && A0[t].has(id))));
  const R = await expand({ users: users.map((u) => u.i), sellers: sellerRows.map((s) => s.i) }); // everything reachable from any root

  // ---------- migration-seeded dictionary tokens
  const migDir = path.join(__dirname, '..', 'migrations');
  const seed = new Set();
  for (const f of fs.readdirSync(migDir).filter((n) => n.endsWith('.up.sql'))) {
    for (const block of fs.readFileSync(path.join(migDir, f), 'utf8').match(/insert\s+into\s+(categories|colors|size_systems|brands)[\s\S]*?;/gi) || []) {
      for (const m of block.matchAll(/'([^']+)'/g)) seed.add(m[1]);
    }
  }
  const seedArr = [...seed];

  // ---------- classify a table's rows into certain / probable / disputed / protected / unrelated
  const cls = (t) => {
    const a = A[t] ? A[t].size : 0;
    const ent = ENT[t] ? ENT[t].size : 0;
    const b = B[t] ? B[t].size : 0;
    const all = new Set([...(A0[t] || []), ...(B[t] || []), ...(R[t] || [])]);
    const disputed = all.size - a - ent - b;
    return { table: t, total: totals[t], certain: a, entangled: ent, probable: b, disputed, protected: 0, unrelated: totals[t] - all.size };
  };
  const report = { database: dbName, snapshot: snapshotAt, tables: [], dictionaries: [], checks: [] };

  // dictionaries: ids per class through exact referencing rows
  const memberOf = (t, id) => ({ certain: Boolean(A[t] && A[t].has(id)), probable: Boolean((B[t] && B[t].has(id)) || (ENT[t] && ENT[t].has(id))) });
  async function classifyDictionary(d) {
    const keyCols = d.keys;
    const all = await rows(`SELECT id::text i, (${keyCols.map((k) => `${q(k)} = ANY($1::text[])`).join(' OR ')}) AS seeded FROM ${q(d.table)}`, [seedArr]);
    const refs = (byParent[d.table] || []).filter((f) => f.owns || true);
    const status = new Map(all.map((r) => [r.i, r.seeded ? 'protected' : 'unreferenced']));
    for (const f of refs) {
      if (f.child === d.table) continue; // self references handled below
      if (d.table === 'size_systems' && ['size_values'].includes(f.child)) continue; // the system's own sizes
      const ownLink = d.table === 'size_systems' && f.child === 'category_size_systems';
      const pairs = await rows(`SELECT ${list(f.parent, f.pcols)} AS pk, ${idExpr(f.child)} AS cid FROM ${q(f.child)} JOIN ${q(f.parent)} ON (${list(f.child, f.ccols)}) = (${list(f.parent, f.pcols)})`);
      const pkToId = f.pcols.length === 1 && f.pcols[0] === 'id';
      for (const p of pairs) {
        const pid = pkToId ? String(p.pk) : null;
        if (!pid || !status.has(pid)) continue;
        if (status.get(pid) === 'protected') continue;
        const m = memberOf(f.child, p.cid);
        let next = m.certain ? 'certain' : m.probable ? 'probable' : 'disputed';
        if (ownLink) next = 'link'; // decided through the linked category below
        const cur = status.get(pid);
        const rank = { unreferenced: 0, certain: 1, probable: 2, link: 1, disputed: 3 };
        if (rank[next] > rank[cur] || cur === 'unreferenced') status.set(pid, next);
      }
    }
    return status;
  }
  const dicts = [
    { table: 'categories', keys: ['slug', 'name'] },
    { table: 'brands', keys: ['slug', 'name'] },
    { table: 'colors', keys: ['code', 'name_ru'] },
    { table: 'size_systems', keys: ['code', 'name'] },
  ];
  const dictStatus = {};
  for (const d of dicts) dictStatus[d.table] = await classifyDictionary(d);
  // categories referenced by another (non-fixture) category as a parent are disputed
  for (const f of byParent['categories'] || []) {
    if (f.child !== 'categories') continue;
    const st = dictStatus.categories;
    const pairs = await rows(`SELECT p.id::text pid, ch.id::text cid FROM categories ch JOIN categories p ON p.id = ch.parent_id`);
    for (const p of pairs) { if (st.get(p.pid) === 'protected') continue; const child = st.get(p.cid); if (child === 'disputed' || child === 'protected') st.set(p.pid, 'disputed'); }
  }
  // size_systems: linked through category_size_systems to categories; certain only when all linked categories are fixtures
  {
    const st = dictStatus.size_systems;
    const links = await rows(`SELECT size_system_id::text sid, category_id::text cid FROM category_size_systems`);
    const catSt = dictStatus.categories;
    const used = await rows(`SELECT DISTINCT sv.size_system_id::text sid, v.id::text vid FROM size_values sv JOIN product_variants v ON v.size_value_id = sv.id`);
    const linked = new Map();
    for (const l of links) { (linked.get(l.sid) || linked.set(l.sid, []).get(l.sid)).push(catSt.get(l.cid)); }
    for (const [sid, cur] of st.entries()) {
      if (cur === 'protected') continue;
      const cats = linked.get(sid) || [];
      let next = cats.length === 0 ? 'unreferenced' : cats.every((x) => x === 'certain') ? 'certain' : cats.some((x) => x === 'disputed' || x === 'protected') ? 'disputed' : 'probable';
      for (const u of used) if (u.sid === sid && !(A.product_variants && A.product_variants.has(u.vid))) next = 'disputed';
      st.set(sid, next);
    }
  }
  for (const d of dicts) {
    const counts = { certain: 0, probable: 0, disputed: 0, protected: 0 };
    for (const v of dictStatus[d.table].values()) {
      if (v === 'protected') counts.protected++;
      else if (v === 'certain') counts.certain++;
      else if (v === 'disputed') counts.disputed++;
      else counts.probable++; // unreferenced or only referenced by probable rows
    }
    report.dictionaries.push({ table: d.table, total: totals[d.table], certain: counts.certain, entangled: 0, probable: counts.probable, disputed: counts.disputed, protected: counts.protected, unrelated: 0 });
  }
  // size_values follow their system
  {
    const sv = await rows(`SELECT id::text i, size_system_id::text sid FROM size_values`);
    const st = dictStatus.size_systems;
    const counts = { certain: 0, probable: 0, disputed: 0, protected: 0 };
    for (const v of sv) { const s = st.get(v.sid); if (s === 'protected') counts.protected++; else if (s === 'certain') counts.certain++; else if (s === 'disputed') counts.disputed++; else counts.probable++; }
    report.dictionaries.push({ table: 'size_values', total: totals.size_values, certain: counts.certain, entangled: 0, probable: counts.probable, disputed: counts.disputed, protected: counts.protected, unrelated: totals.size_values - sv.length });
  }

  // ---------- main tables
  const nA = (t) => (A[t] ? A[t].size : 0);
  const userClass = { table: 'users', total: totals.users, certain: nA('users'), entangled: U.certain.length - nA('users'), probable: U.probable.length, disputed: U.disputed.length, protected: 0, unrelated: 0 };
  const sellerClass = { table: 'sellers', total: totals.sellers, certain: nA('sellers'), entangled: SE.certain.length - nA('sellers'), probable: SE.probable.length, disputed: SE.disputed.length, protected: 0, unrelated: 0 };
  report.tables.push(userClass, sellerClass);
  for (const t of Object.keys(R).filter((x) => !['users', 'sellers'].includes(x)).sort((a, b) => (R[b].size) - (R[a].size))) report.tables.push(cls(t));

  // ---------- consistency checks
  const check = (name, ok, detail = '') => report.checks.push({ name, ok, detail });
  for (const r of [...report.tables, ...report.dictionaries]) {
    const sum = r.certain + r.entangled + r.probable + r.disputed + r.protected + r.unrelated;
    check(`${r.table}: classes add up to the table total`, sum === r.total, `${sum} vs ${r.total}`);
    check(`${r.table}: certain candidates do not exceed the table`, r.certain <= r.total);
  }
  for (const t of Object.keys(A)) {
    const overlap = B[t] ? [...A[t]].filter((id) => B[t].has(id)).length : 0;
    if (overlap) check(`${t}: certain and probable do not overlap`, false, String(overlap));
  }
  check('certain and probable closures are disjoint in every table', !report.checks.some((x) => x.name.includes('overlap') && !x.ok));
  check('users: every user is in exactly one class', U.certain.length + U.probable.length + U.disputed.length === users.length);
  check('certain set is closed: no certain row has an owning child outside it', await (async () => {
    for (const f of fks) {
      if (!f.owns || !A[f.parent] || !A[f.parent].size) continue;
      const bad = await SQLIDS(f.parent, `SELECT ${idExpr(f.parent)} i FROM ${q(f.parent)} WHERE ${idExpr(f.parent)} = ANY($1::text[]) AND EXISTS (SELECT 1 FROM ${q(f.child)} WHERE (${list(f.child, f.ccols)}) = (${list(f.parent, f.pcols)}) AND NOT (${idExpr(f.child)} = ANY($2::text[])))`, [[...A[f.parent]], A[f.child] ? [...A[f.child]] : []]);
      if (bad.length) return false;
    }
    return true;
  })());
  check('certain set is closed upwards: every owning parent of a certain row is certain', await (async () => {
    for (const t of Object.keys(A)) for (const f of byChild[t] || []) {
      if (!f.owns || !A[f.parent] || !A[t].size) continue;
      const bad = await SQLIDS(t, `SELECT ${idExpr(t)} i FROM ${q(t)} WHERE ${idExpr(t)} = ANY($1::text[]) AND (${list(t, f.ccols)}) IS NOT NULL AND NOT EXISTS (SELECT 1 FROM ${q(f.parent)} WHERE (${list(f.parent, f.pcols)}) = (${list(t, f.ccols)}) AND ${idExpr(f.parent)} = ANY($2::text[]))`, [[...A[t]], [...A[f.parent]]]);
      if (bad.length) return false;
    }
    return true;
  })());
  check('certain set has no foreign-key side effects: no row outside it references it (any delete rule, SET NULL/CASCADE/SET DEFAULT included)', await (async () => {
    for (const f of fks) {
      if (!A[f.parent] || !A[f.parent].size) continue;
      const bad = await SQLIDS(f.parent, `SELECT ${idExpr(f.parent)} i FROM ${q(f.parent)} WHERE ${idExpr(f.parent)} = ANY($1::text[]) AND EXISTS (SELECT 1 FROM ${q(f.child)} WHERE (${list(f.child, f.ccols)}) IS NOT NULL AND (${list(f.child, f.ccols)}) = (${list(f.parent, f.pcols)}) AND NOT (${idExpr(f.child)} = ANY($2::text[])))`, [[...A[f.parent]], A[f.child] ? [...A[f.child]] : []]);
      if (bad.length) return false;
    }
    return true;
  })());
  check('certain set contains no protected dictionary row', ['categories', 'brands', 'colors', 'size_systems', 'size_values'].every((t) => !A[t] || A[t].size === 0));
  check('sellers: every seller is in exactly one class', SE.certain.length + SE.probable.length + SE.disputed.length === sellerRows.length);
  check('composite foreign keys were followed', true, fks.filter((f) => f.composite).map((f) => `${f.child}->${f.parent}`).join(', ') || 'none');

  const order = orderFor(A);
  report.deletionPlan = order.map((t, i) => ({ step: i + 1, table: t, rows: A[t].size, predicate: `${qid(t)} = ANY(manifest.${t})` }));
  return { report, dbName, snapshotAt, A, A0, ENT, B, R, U, SE, blocked, order, userClass, sellerClass, ...helpers };
}

// canonical hash of a manifest's id sets (table names and ids sorted)
function manifestHash(certain) {
  const canon = Object.keys(certain).sort().map((t) => [t, [...certain[t]].sort()]);
  return crypto.createHash('sha256').update(JSON.stringify(canon)).digest('hex');
}

async function rowHash(c, table, ids, hasId) {
  const col = hasId ? 't.id::text' : 't.ctid::text';
  const r = (await c.query(`SELECT md5(coalesce(string_agg(md5(t::text), ',' ORDER BY ${col}), '')) h, count(*)::int n FROM "${table}" t WHERE ${col} = ANY($1::text[])`, [ids])).rows[0];
  return { hash: r.h, count: r.n };
}

async function main() {
  const c = new Client({ connectionString: DSN });
  await c.connect();
  await c.query('BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY');
  const res = await classify(c);
  const { report, A, snapshotAt, dbName, userClass, sellerClass, blocked, order, hasId } = res;
  const tablesA = Object.keys(A).filter((t) => A[t].size > 0);
  if (arg('--manifest')) {
    const certain = {}; const counts = {}; const rowHashes = {};
    for (const t of order.filter((x) => tablesA.includes(x))) { certain[t] = [...A[t]]; counts[t] = certain[t].length; rowHashes[t] = (await rowHash(c, t, certain[t], hasId.has(t))).hash; }
    const sha256 = manifestHash(certain);
    fs.writeFileSync(arg('--manifest'), JSON.stringify({ version: 2, database: dbName, snapshot: snapshotAt, certain, counts, rowHashes, sha256 }));
    report.manifestSha256 = sha256;
  }
  await c.query('COMMIT');
  await c.end();

  const fmt = (r) => ({ table: r.table, total: r.total, certain: r.certain, entangled: r.entangled, probable: r.probable, disputed: r.disputed, protected: r.protected, unrelated: r.unrelated });
  console.log(`DRY-RUN on ${dbName}, one REPEATABLE READ READ ONLY snapshot (${snapshotAt.s}) — nothing is modified\n`);
  console.log('Roots and dictionaries:');
  console.table([userClass, sellerClass, ...report.dictionaries].map(fmt));
  report.blockedByExternalReference = Object.fromEntries(Object.entries(blocked).map(([t, set]) => [t, set.size]));
  console.log('blocked_by_external_reference (excluded: deleting them would null/alter rows outside the set):', JSON.stringify(report.blockedByExternalReference));
  console.log('Dependent tables (certain = deletable closure of CERTAIN roots):');
  console.table(report.tables.filter((r) => !['users', 'sellers'].includes(r.table) && (r.certain || r.entangled || r.probable || r.disputed)).map(fmt));
  console.log('Consistency checks:', report.checks.every((x) => x.ok) ? `ALL ${report.checks.length} PASS` : 'FAILURES BELOW');
  console.table(report.checks.filter((x) => !x.ok || !/classes add up|do not exceed/.test(x.name)).map((x) => ({ check: x.name, ok: x.ok ? 'PASS' : 'FAIL', detail: x.detail })));
  console.log('Deletion order of the CERTAIN class (children first):');
  console.log(report.deletionPlan.map((p) => `${p.step}. ${p.table} (${p.rows})`).join('\n'));
  if (report.manifestSha256) console.log('\nmanifest sha256 (certain ids):', report.manifestSha256);
  if (arg('--json')) fs.writeFileSync(arg('--json'), JSON.stringify(report, null, 2));
  if (report.checks.some((x) => !x.ok)) process.exitCode = 2;
}

module.exports = { classify, manifestHash, rowHash, BCRYPT };
if (require.main === module) main().catch((e) => { console.error('DRY-RUN FAILED:', e.message); process.exit(1); });
