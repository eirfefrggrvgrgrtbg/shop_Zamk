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

### Automatic Local Commits

- AI agents are authorized by the Product Owner to run `git add` and `git commit` automatically during normal development.
- No additional Product Owner acceptance is required for ordinary local commits.
- After completing an approved task and passing relevant checks, create a local Git commit automatically.
- Use clear, descriptive commit messages.
- Do not leave completed, verified work uncommitted without a concrete reason.
- Direct `git add` and `git commit` are allowed.
- `.agents/scripts/finalize.sh` is NOT required for ordinary local commits.
- `ZAMK_OWNER_ACCEPTED_FINALIZE=YES` is NOT required for ordinary local commits.

### Staging and Existing Work

- Stage only changes belonging to the completed task.
- Review `git status`, `git diff`, and staged changes before committing.
- Prefer explicit file paths or `git add -p` for mixed files.
- Never blindly run `git add .` or `git add -A`.
- Preserve unrelated uncommitted changes, untracked files, and Git stashes.
- Existing dirty files are normal in this repository and must not automatically block development.
- If changes from different tasks cannot be safely separated, report the specific conflict rather than committing unrelated work.

### Remote Operations and Finalization

- Local commits are automatic; pushing to remote is a separate operation.
- Do not run `git push` without explicit Product Owner authorization.
- Canonical finalization through `.agents/scripts/finalize.sh` requires `ZAMK_OWNER_ACCEPTED_FINALIZE=YES` in the latest user request.
- Never force-push, amend existing commits, rewrite history, or perform destructive Git operations without explicit Product Owner approval.
- Never use `git reset --hard` or `git clean` to discard existing work without explicit authorization.

### Standard Development Workflow

1. Implement the approved task.
2. Run relevant tests, build, and validation.
3. Inspect the changes.
4. Stage only task-related changes.
5. Create a local commit automatically.
6. Report the commit hash and test results.
7. Continue to the next task when instructed.


## Global Test / Acceptance Safety
- Stop on the first real mandatory acceptance failure; report later mandatory checks as not run.
- Never claim browser or manual PASS without Product Owner evidence.
- Do not use API mocks as business or E2E acceptance proof.

## Global Observability
For every new or changed meaningful business mutation, assess: LOG, TRACE, METRIC, DURABLE AUDIT, PRODUCT ANALYTICS.
Meaningful state transitions require semantic success observability. Emit success mutation events only after commit.
Never log secrets, tokens, authorization headers, cookies, passwords, customer PII, payment secrets, or SQL arguments.
