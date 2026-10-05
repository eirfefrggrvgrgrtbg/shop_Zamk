---
name: zamk-migration-check
description: Run the canonical ZAMK database migration test against an isolated test database (zamk_test) to verify clean up-chain, status, and sequence constraints.
---

# ZAMK Migration Check

Run `npm run test:migrations` from the repository root.
This encapsulates:
1. Recreating the clean isolated test database `zamk_test`.
2. Running all migrations (`1..N`) up via `golang-migrate`.
3. Verifying non-dirty migration status in `schema_migrations`.
4. Validating sequence and entity constraints (e.g., `PAY-` sequence generation).

If any step fails, stop immediately. Never run against the development database `zamk`.
