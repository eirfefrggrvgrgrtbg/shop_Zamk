---
name: zamk-preflight
description: Run the ZAMK safe-start Git preflight to verify branch, HEAD, clean worktree, and stashes before allowing edits.
---

# ZAMK Preflight

Run `bash .agents/scripts/preflight.sh` to capture the Git baseline.
Verify there are no unexpected dirty files. Preserve all existing stashes.
