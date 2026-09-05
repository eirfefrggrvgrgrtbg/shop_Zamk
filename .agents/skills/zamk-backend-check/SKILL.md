---
name: zamk-backend-check
description: Run the standard ZAMK backend regression gate after backend changes, using only the exact safe test database and the required Go test/build sequence.
---

# ZAMK backend check

Run from `backend/` after the final relevant source change.

1. Confirm destructive or integration tests will use exactly:
   `postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable`
   Refuse any test command pointed at the development database `zamk`.
2. Run task-specific suites when required. Place them before or around the standard checks according to task risk.
3. Run each standard check as a separate terminal invocation, in order:
   - `TEST_DATABASE_URL="postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable" go test -p 1 ./internal/products/... -count=1`
   - `TEST_DATABASE_URL="postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable" go test -p 1 ./internal/http/router/... -count=1`
   - `go build -buildvcs=false ./cmd/...`
4. Stop on the first real mandatory failure. Do not run later mandatory checks, repair state with SQL, or use `migrate force`.

Do not combine terminal commands with `;`, `&&`, or `||`.

Return:

```text
TEST DATABASE: EXACT|REFUSED
TASK-SPECIFIC SUITES: PASS|FAIL|NOT REQUIRED|NOT RUN
PRODUCTS TESTS: PASS|FAIL|NOT RUN
ROUTER TESTS: PASS|FAIL|NOT RUN
BACKEND BUILD: PASS|FAIL|NOT RUN
BACKEND GATE: PASS|FAIL
```
