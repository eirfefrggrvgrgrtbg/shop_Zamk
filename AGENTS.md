# ZAMK Codex Project Instructions

## Project
ZAMK / ZAMOK — FBO fashion marketplace.

## Instruction priority
Current user task instructions override reusable workflow defaults unless they would violate an explicit repository safety rule in this file.

## Canonical branch
`main`.

## FBO Responsibility Contract
- **Seller** is the commercial owner. Seller may prepare supplies and read operational state. Seller cannot physically receive, pick, pack, ship, reconcile, write off, mutate ZMU, or physically process customer returns.
- **Admin/ZAMK** owns physical warehouse/platform operations and moderation.
- **Customer** owns buying, payment initiation, cancellation where allowed, and return initiation.
- **System** owns reservations, allocations, callbacks, expiry, derived state, and automatic side effects.
- **READ != ACTION**.

## Global Git Safety
- Never run direct `git add`, `git stage`, `git commit`, or `git push` before explicit Product Owner acceptance. All staging, committing, and pushing must occur strictly through canonical `zamk-finalize` (`.agents/scripts/finalize.sh`) after owner review.
- Canonical finalization via `.agents/scripts/finalize.sh` requires explicit Product Owner authorization line `ZAMK_OWNER_ACCEPTED_FINALIZE=YES` in the latest user request.
- Stage only exact approved paths. Never run `git add .` or `git add -A`.
- Never force-push or amend without explicit Product Owner approval.
- Preserve unrelated stashes and dirty files. Stop and report if unexpected dirty files are found.

## Global Test / Acceptance Safety
- Stop on the first real mandatory acceptance failure; report later mandatory checks as not run.
- Never claim browser or manual PASS without Product Owner evidence.
- Do not use API mocks as business or E2E acceptance proof.

## Global Observability
For every new or changed meaningful business mutation, assess: LOG, TRACE, METRIC, DURABLE AUDIT, PRODUCT ANALYTICS.
Meaningful state transitions require semantic success observability. Emit success mutation events only after commit.
Never log secrets, tokens, authorization headers, cookies, passwords, customer PII, payment secrets, or SQL arguments.
