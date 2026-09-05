# ZAMK Codex Project Instructions

## Project

ZAMK / ZAMOK — FBO fashion marketplace.

## Instruction priority

Current user task instructions override reusable workflow defaults unless they would violate an explicit repository safety rule in this file.

## Canonical branch

`main`.

## Git safety

- After each completed and Product Owner-accepted logical milestone, create a meaningful commit and push it immediately.
- Stage only exact approved paths.
- Never run `git add .`.
- Never run `git add -A`.
- Never force-push.
- Never amend without explicit Product Owner approval.
- Treat pushed migrations as forward-only; correct them with a new migration.
- Preserve unrelated stashes.
- Stop and report exact paths if unexpected dirty files are found. Do not reset, stash, discard, or overwrite them.

## Database safety

- Run destructive or integration tests only against exactly:
  `postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable`
- Never truncate, drop, or mutate the development database `zamk` from tests.
- Read-only `SELECT` sanity checks against the development database are allowed when needed.

## Test safety

- Do not use API mocks as business or E2E acceptance proof.
- Frontend unit/component mocks are allowed only as unit tests.
- Stop on the first real mandatory acceptance failure; report later mandatory checks as not run.
- Never claim browser or manual PASS without Product Owner evidence.

## Responsibility contract

- Seller is the commercial owner. Seller may prepare supplies and read operational state. Seller cannot physically receive, pick, pack, ship, reconcile, write off, mutate ZMU, or physically process customer returns.
- Admin/ZAMK owns physical warehouse/platform operations and moderation.
- Customer owns buying, payment initiation, cancellation where allowed, and return initiation.
- System owns reservations, allocations, callbacks, expiry, derived state, and automatic side effects.
- READ != ACTION.

## Observability rule

For every new or changed meaningful business mutation, assess:

- LOG
- TRACE
- METRIC
- DURABLE AUDIT
- PRODUCT ANALYTICS

Meaningful state transitions require semantic success observability or an explicit justification. Emit success mutation events only after commit. Represent expected important rejection with stable reason codes.

Never log secrets, tokens, authorization headers, cookies, passwords, customer PII, payment secrets, or SQL arguments.
