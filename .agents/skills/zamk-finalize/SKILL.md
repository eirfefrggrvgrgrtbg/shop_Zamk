---
name: zamk-finalize
description: Standard git closure after all technical checks already PASS. Use this to commit work.
---

# zamk-finalize

Purpose: standard git closure after all technical checks already PASS.

Steps:

1. Run:
   `git diff --check`
   `git status --short`
   `git diff --stat`

2. Inspect changed files.

3. Stage ONLY exact task files explicitly.
   NEVER: `git add .` or `git add -A`

4. Use the exact commit message specified in the current task.
   If the current task does not specify an exact commit message:
   STOP before commit and report:
   COMMIT MESSAGE REQUIRED

5. Never amend.

6. Commit.

7. Run:
   `git rev-parse HEAD`
   `git status --short`

Required final worktree: clean.

Return:

DIFF CHECK:
PASS/FAIL

COMMIT:
<full hash or NOT CREATED>

WORKTREE CLEAN:
YES/NO
