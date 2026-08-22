---
name: zamk-migration-check
description: Safe migration verification when backend/migrations changed. Use this when migrations are altered.
---

# zamk-migration-check

Purpose: safe migration verification when backend/migrations changed.

Steps:

1. Inspect changed migration files first.
2. Reject immediately if they contain unsafe broad operations such as: blanket category/table mutation, TRUNCATE, DROP of unrelated structures, direct schema_migrations edits, migrate force instructions.
3. Never reset/drop/truncate the whole zamk_test database.
4. Never use `migrate force` for acceptance.
5. Prefer the repository migration check only if it is proven compatible with ZAMK DB safety rules.
6. If a clean scratch database is required: use an isolated temporary scratch DB, never zamk or zamk_test, and remove only that scratch DB afterward.
7. Verify normal up chain. If the task specifically requires down/up validation, do it only on scratch.
8. Whenever a workflow runs terminal commands, execute EACH command as a separate terminal tool invocation. Do NOT combine commands using: ;, &&, ||.

Return:

MIGRATION SAFETY:
PASS/FAIL

CLEAN UP:
PASS/FAIL/NOT RUN

DOWN/UP:
PASS/FAIL/NOT REQUIRED

FORCE USED:
NO/YES
