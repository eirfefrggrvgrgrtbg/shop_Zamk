---
name: zamk-finalize
description: Standard git closure after all technical checks already PASS. Use this to commit work.
---

# zamk-finalize

Purpose: standard git closure after all technical checks already PASS.

Steps:

1. run `git diff --check`

   then separately:
   run `git status --short`

   then separately:
   run `git diff --stat`

2. Inspect changed files.

3. Stage ONLY exact task files explicitly.
   NEVER: `git add .` or `git add -A`

4. Use the exact commit message specified in the current task.
   If the current task does not specify an exact commit message:
   STOP before commit and report:
   COMMIT MESSAGE REQUIRED

5. Never amend.

6. Commit.

7. run `git rev-parse HEAD`

   then separately:
   run `git status --short`

Required final worktree: clean.

Whenever a workflow runs terminal commands, execute EACH command as a separate terminal tool invocation. Do NOT combine commands using: ;, &&, ||.

Return:

DIFF CHECK:
PASS/FAIL

COMMIT:
<full hash or NOT CREATED>

WORKTREE CLEAN:
YES/NO
