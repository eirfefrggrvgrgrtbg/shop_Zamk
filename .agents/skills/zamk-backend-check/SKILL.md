---
name: zamk-backend-check
description: Run the standard ZAMK backend test and compilation check against zamk_test database after backend changes.
---

# ZAMK Backend Check

Run `npm run test:backend` from the repository root.
This encapsulates:
1. Running isolated backend regression tests (`./internal/products/...`, `./internal/http/router/...`) strictly against `zamk_test`.
2. Verifying compilation of all backend binaries (`go build ./cmd/...`).

For standalone compilation checking without running tests, run `npm run build:backend`.
If any check fails, stop immediately.
