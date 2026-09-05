---
name: zamk-seller-check
description: Run ZAMK Seller frontend tests, lint/build checks, and a business-boundary regression review using only scripts currently declared in the repository package files.
---

# ZAMK Seller check

1. Inspect root `package.json` and `apps/seller/package.json` before running commands. Use only relevant Seller scripts actually declared there; never invent a command.
2. Run declared Seller unit/component tests when present, then the declared lint and build scripts. In the current package structure, the declared checks are `npm run lint --workspace=seller` and `npm run build:seller`; rediscover them before each run.
3. Run each command separately and stop on the first real mandatory failure.
4. Review the changed Seller UI, routes, API calls, permissions, and relevant tests against the responsibility contract in root `AGENTS.md`.
5. Fail the boundary check if Seller gains any physical receive, pick, pack, ship, reconcile, write-off, ZMU mutation, or physical customer-return processing action. Preparing supplies and reading operational state remain allowed. Treat read access as distinct from action authority.
6. Report unit/component mocks only as unit evidence, never as business or E2E acceptance.

Return:

```text
SELLER SCRIPTS DISCOVERED: <exact script names>
SELLER TESTS: PASS|FAIL|NOT AVAILABLE|NOT RUN
SELLER LINT: PASS|FAIL|NOT AVAILABLE|NOT RUN
SELLER BUILD: PASS|FAIL|NOT AVAILABLE|NOT RUN
RESPONSIBILITY BOUNDARY: PASS|FAIL
BOUNDARY EVIDENCE: <changed actions/routes/tests inspected>
SELLER GATE: PASS|FAIL
```
