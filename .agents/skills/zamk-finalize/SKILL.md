---
name: zamk-finalize
description: Finalize an already Product Owner-accepted ZAMK milestone by exact-staging approved files and committing/pushing safely.
---

# ZAMK Finalize

Canonical workflow for staging, committing, and optionally pushing approved files.

## Workflow Modes

1. **Local Commit (Default):**
   ```bash
   bash .agents/scripts/finalize.sh "<commit message>" <file1> <file2> ...
   ```
   - Verifies canonical branch `main`.
   - Strictly enforces explicit file paths (wildcards like `.` and `-A` are forbidden).
   - Stages only the approved files.
   - Runs diff checks for whitespace and merge conflict markers.
   - Creates the local Git commit.
   - **Omit push by default** to preserve safety.

2. **Remote Push (`--push`):**
   ```bash
   bash .agents/scripts/finalize.sh --push "<commit message>" <file1> <file2> ...
   ```
   - Requires explicit Product Owner authorization line in the latest user turn:
     `ZAMK_OWNER_ACCEPTED_FINALIZE=YES`
   - The pre-tool guard automatically verifies this authorization before permitting `--push`.

3. **Dry-Run Validation (`--dry-run`):**
   ```bash
   bash .agents/scripts/finalize.sh --dry-run "<commit message>" <file1> <file2> ...
   ```
   - Stages files, validates diff, and unstages without committing or pushing.
