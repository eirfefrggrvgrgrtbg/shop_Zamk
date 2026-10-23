---
name: zamk-backend
description: ZAMK Backend / Database Rules
trigger: glob
glob: "backend/**"
---

# ZAMK Backend / Database Rules

## Database Safety
- TEST.DB.5: the only supported way to run integration tests is `backend/scripts/safe_go_test.sh <explicit packages> [-- -run '^TestX$' -v]` (or `npm run test:backend -- <packages> ...`). Each run gets its own disposable database `zamk_disposable_<12 hex>`, which the runner drops afterwards (name + ownership token).
- Direct `go test` for integration tests and `./...` are forbidden. If the runner cannot create the disposable database, stop; never fall back to a shared database or override `TEST_DATABASE_URL` / `DATABASE_URL` / `POSTGRES_*` / `PG*`.
- The shared databases `zamk`, `zamk_test` and `zamk_demo` are never written by tests; only read-only `SELECT` checks are allowed there.
- Tests obtain a database only through `internal/testutil` (`GetTestDatabaseURL`, `RunnerDatabaseURL`, `DisposableDatabaseURL`, `NewDisposableDatabase`); no hard-coded DSNs, no direct reads of `TEST_DATABASE_URL`.
- Never clean up or restore test fixtures with manual SQL (`DELETE`/`UPDATE`/`INSERT`/`TRUNCATE`), never use the old TEST.DB.3 cleanup, and never mass-delete leftover `zamk_disposable_*` databases or the template.
- Never truncate, drop, or mutate the development database `zamk` from tests.
- Read-only `SELECT` sanity checks against the development database are allowed when needed.
- Never use the main development DB for automated integration tests.
- Never TRUNCATE, DROP, reset, or recreate the whole zamk_test database.
- Never mutate normal business state by direct SQL after a browser/business flow starts.
- Setup fixtures before a test are allowed only when isolated and disclosed.

## Migrations
- Treat pushed migrations as forward-only; correct them with a new migration (Committed migrations не редактировать).
- Never edit schema_migrations directly.
- `migrate force` is forbidden during normal work and acceptance.
- If a migration DB is dirty, stop and report before recovery.
- Migration recovery requires explicit authorization.
- Migrations must be forward-safe and environment-independent.

## Identifiers & Architecture
- Never copy environment-specific UUIDs into canonical migrations when a stable code lookup exists.
- Preserve foreign-key/business-history identities.
- Product Variant ID is canonical commerce identity.
- Product creation never creates sellable inventory.
- Warehouse stock changes only through real receiving acceptance.
- Backend validation is authoritative; frontend validation is convenience only.
