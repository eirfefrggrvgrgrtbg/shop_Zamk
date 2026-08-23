---
name: zamk-core
description: ZAMK Core Engineering Rules
trigger: always_on
---

# ZAMK Core Engineering Rules

- Work only inside the current ZAMK workspace unless explicitly authorized.
- Never claim implementation, test, build, migration, runtime, or UX PASS without direct evidence.
- Stop on the first failed mandatory acceptance command. Later mandatory checks are NOT RUN.
- Never fabricate screenshots or visual acceptance.
- Visual/browser Product Owner acceptance is manual by the user.
- Automated technical verification is allowed for tests/build/API/backend.
- No API mocks or route.fulfill for acceptance.
- Never use native alert()/confirm() as accepted production UX.
- Never hide errors merely to obtain green tests.
- Never introduce t.Skip, remove assertions, or weaken regression tests just to pass.
- Existing accepted foundations must not be rewritten without a concrete regression.
- Narrow bug -> narrow fix.
- Cross-domain architecture/security/migrations -> inspect first, then implement.
- Before coding: git status + HEAD.
- Accepted milestone -> meaningful commit -> clean worktree.
- Never use git add . or git add -A.
- Never amend accepted commits unless explicitly requested.
- Always push changes upstream (`git push` or `git push -u origin <branch>`) after successful technical checks and commits.
- Do not start the next milestone automatically.
