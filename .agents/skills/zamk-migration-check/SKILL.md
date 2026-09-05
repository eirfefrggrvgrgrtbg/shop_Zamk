---
name: zamk-migration-check
description: Verify changed ZAMK PostgreSQL migrations for numbering, forward-only history, up/down consistency, compatibility, destructive behavior, and exact test-database safety.
---

# ZAMK migration check

1. List changed migration files and inspect the repository naming/numbering convention. Verify unique, ordered numbering with no collision or accidental gap relative to existing migrations.
2. Determine whether each edited migration already exists on `origin/main`. Treat every pushed migration as immutable and forward-only; require a new migration for corrections.
3. Review up/down pairs where the repository uses them. Verify object names, order, dependencies, and reversibility are consistent.
4. Explicitly identify every destructive or irreversible action, including data loss, narrowing conversions, unbackfilled `NOT NULL`, table/column drops, and non-restoring down migrations. Do not silently label such behavior safe.
5. Reject unrelated broad mutations, direct `schema_migrations` edits, or `migrate force` instructions.
6. Run schema, migration, build, and relevant test compatibility checks using repository commands. Invoke `zamk-backend-check` for the standard backend gate when applicable.
7. Run all destructive or integration verification only against exactly:
   `postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable`
   Never truncate, drop, or mutate the development database `zamk` from tests. Use read-only `SELECT` against dev only when needed for sanity.
8. Run down/up verification only when applicable and safe for the exact test database. Stop on the first real mandatory failure.

Run terminal commands separately; do not combine them with `;`, `&&`, or `||`.

Return:

```text
MIGRATIONS: <exact files>
NUMBERING: PASS|FAIL
FORWARD-ONLY: PASS|FAIL
UP/DOWN: PASS|FAIL|NOT APPLICABLE|NOT RUN
SCHEMA/BUILD/TEST COMPATIBILITY: PASS|FAIL|NOT RUN
TEST DATABASE: EXACT|REFUSED|NOT REQUIRED
DESTRUCTIVE/IRREVERSIBLE: NONE|<explicit behavior and mitigation>
MIGRATION GATE: PASS|FAIL
```
