---
name: zamk-backend-check
description: Standard backend regression check after final backend source changes. Use this to verify tests and build.
---

# zamk-backend-check

Purpose: standard backend regression check after final backend source changes.

Steps:

From backend:
`TEST_DATABASE_URL="postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable" go test -p 1 ./internal/products/... -count=1`

Then:
`TEST_DATABASE_URL="postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable" go test -p 1 ./internal/http/router/... -count=1`

Then:
`go build -buildvcs=false ./cmd/...`

Rules:
- Run commands separately.
- Stop on first failure.
- Do not run later mandatory commands after failure.
- Do not repair test state with SQL.
- Do not use migrate force.

Return:

PRODUCTS TESTS:
PASS/FAIL

ROUTER TESTS:
PASS/FAIL/NOT RUN

BACKEND BUILD:
PASS/FAIL/NOT RUN
