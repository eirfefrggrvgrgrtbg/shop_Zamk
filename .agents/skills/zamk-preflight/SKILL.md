---
name: zamk-preflight
description: Standard safe start before implementation. Use this to verify worktree state before beginning a task.
---

# zamk-preflight

Purpose: standard safe start before implementation.

Steps:

1. Run:
   `git branch --show-current`
   `git rev-parse HEAD`
   `git status --short`
   `git log -5 --oneline`

2. Compare HEAD with the expected HEAD stated in the current task, if any.

3. If worktree is dirty and the task did not explicitly expect those files:
   STOP and report exact files.

4. Do not edit anything during preflight.

Return only:
branch
HEAD
worktree clean YES/NO
expected HEAD match YES/NO/NOT SPECIFIED
