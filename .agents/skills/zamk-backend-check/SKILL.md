---
name: zamk-backend-check
description: Run the ZAMK backend integration tests for the affected packages in a disposable database (TEST.DB.5) and the compilation check after backend changes.
---

# ZAMK Backend Check

Run `npm run test:backend -- ./internal/<pkg>/ [-- -run '^TestX$' -v]` from the repository root, naming the affected packages explicitly (there is no default suite).
This runs `backend/scripts/safe_go_test.sh`, which:
1. Creates a disposable database for this run (never `zamk_test`), runs the static DB guards, then the named packages, and drops the database.
2. Refuses to run when packages are missing or the disposable database cannot be created.

Verify compilation of all backend binaries with `npm run build:backend`.

For standalone compilation checking without running tests, run `npm run build:backend`.
If any check fails, stop immediately.
