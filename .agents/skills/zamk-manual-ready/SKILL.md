---
name: zamk-manual-ready
description: Prepare a ZAMK milestone for Product Owner browser acceptance, automate only readiness checks, and return an exact manual verification script without claiming visual or manual PASS.
---

# ZAMK manual ready

1. Verify HEAD, worktree state, and required automated checks.
2. Discover service commands and URLs from repository configuration; do not invent them.
3. Check required services such as PostgreSQL, Redis, backend, the relevant frontend, and MinIO/storage when media is in scope. Start only missing services using existing project conventions and only when in task scope.
4. Verify readiness with non-destructive automated checks. Do not reset databases or create/modify business state merely to claim readiness.
5. Stop on the first real mandatory failure and mark later mandatory checks not run.
6. Prepare exact Product Owner browser steps by URL, role, and action, with the expected result for each step.

Never perform visual acceptance or claim browser/manual PASS. Only Product Owner evidence can establish it. Run terminal commands separately; do not combine them with `;`, `&&`, or `||`.

Return only:

```text
AUTOMATED:
- <check and evidence>

MANUAL VERIFICATION:
- URL: <exact URL>
  ROLE: <exact role>
  ACTION: <exact actions>

EXPECTED RESULT:
- <result corresponding to each manual action>

STOP RULE:
- Stop on the first real failure; later mandatory checks are NOT RUN.
```
