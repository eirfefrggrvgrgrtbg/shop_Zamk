---
name: zamk-backend
description: ZAMK Backend / Database Rules
trigger: glob
glob: "backend/**"
---

# ZAMK Backend / Database Rules

## Database Safety
- Run destructive or integration tests only against exactly: `postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable`
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
