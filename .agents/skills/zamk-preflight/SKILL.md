---
name: zamk-preflight
description: Run the ZAMK safe-start Git preflight before development or repository edits; verify branch, revisions, status, history, stashes, and any task-supplied baseline without altering existing work.
---

# ZAMK preflight

Run this workflow before making changes.

1. Run each command as a separate terminal invocation:
   - `git branch --show-current`
   - `git rev-parse HEAD`
   - `git status --short`
   - `git log -5 --oneline`
   - `git stash list`
2. Resolve `origin/main` with `git rev-parse origin/main` when the task depends on the canonical baseline. If remote freshness is material and network access is available, run `git fetch origin main` separately first.
3. Compare the branch, HEAD, and relevant `origin/main` value with every expected baseline stated in the task.
4. Classify every dirty path as expected or unexpected from the task context.
5. If any baseline mismatches or any dirty path is unexpected, STOP. Report the evidence and do not edit.

Never reset, stash, apply, drop, discard, clean, overwrite, or otherwise alter existing work or stashes during preflight. Do not combine terminal commands with `;`, `&&`, or `||`.

Return this compact result:

```text
PREFLIGHT
BRANCH: <name>
HEAD: <full hash>
ORIGIN/MAIN: <full hash|NOT CHECKED>
EXPECTED BASELINE: MATCH|MISMATCH|NOT SPECIFIED
WORKTREE CLEAN: YES|NO
UNEXPECTED DIRTY FILES: <none|exact paths>
RECENT LOG: <five compact entries>
STASHES: <none|unchanged entries>
RESULT: PASS|STOP
```
