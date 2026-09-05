---
name: zamk-finalize
description: Finalize an already Product Owner-accepted ZAMK milestone by rerunning requested checks, exact-staging approved files, committing with a supplied short Russian message, and pushing main safely.
---

# ZAMK finalize

Use only after Product Owner acceptance and explicit task authorization to commit and push.

1. Verify the current branch is `main`, capture HEAD, `git status --short`, and `git stash list`, and preserve the stash list for the final comparison.
2. Rerun every final test requested for the milestone as separate commands. Stop on the first real mandatory failure.
3. Inspect `git diff --name-status`, `git diff --stat`, and `git diff --check`. Stop if the diff includes unapproved or unexpected paths.
4. Stage each approved file with exact pathspecs, for example `git add -- <approved-file-1> <approved-file-2>`. Do not stage a directory or any unapproved path.
5. Inspect all staged content and run separately:
   - `git diff --cached --name-status`
   - `git diff --cached --stat`
   - `git diff --cached --check`
6. Stop if staged scope or content is not exactly approved.
7. Require a supplied short, human Russian commit message. If absent, STOP with `COMMIT MESSAGE REQUIRED`.
8. Commit once, then push immediately with `git push origin main`.
9. Confirm HEAD equals `origin/main`, the worktree is clean, and `git stash list` is byte-for-byte unchanged.

Never run `git add .`, `git add -A`, force-push, or amend without explicit Product Owner approval. Never modify, apply, drop, or clear unrelated stashes. Do not combine terminal commands with `;`, `&&`, or `||`.

Return:

```text
FINAL TESTS: PASS|FAIL
SCOPE CHECK: PASS|FAIL
STAGED FILES: <exact paths|none>
CACHED DIFF CHECK: PASS|FAIL|NOT RUN
COMMIT: <full hash|NOT CREATED>
PUSH ORIGIN/MAIN: PASS|FAIL|NOT RUN
WORKTREE CLEAN: YES|NO
STASHES UNTOUCHED: YES|NO
```
