#!/usr/bin/env bash
set -e

DRY_RUN=false
if [ "$1" = "--dry-run" ]; then
    DRY_RUN=true
    shift
fi

if [ "$1" = "--help" ] || [ "$#" -lt 2 ]; then
    echo "Usage: $0 [--dry-run] '<commit message>' <file1> <file2> ..."
    echo "Mandatory requirements:"
    echo "  - Branch must be main"
    echo "  - Files must be explicit paths (never '.', '-A', or '--all')"
    echo "  - Commit message must be provided"
    exit 1
fi

MSG="$1"
shift
FILES=("$@")

# Reject generic wildcard or dot staging
for f in "${FILES[@]}"; do
    if [ "$f" = "." ] || [ "$f" = "-A" ] || [ "$f" = "--all" ] || [ "$f" = "*" ]; then
        echo "ERROR: Wildcard/all staging ('$f') is strictly forbidden. Specify exact file paths."
        exit 1
    fi
done

echo "=== ZAMK Finalize ==="
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo "ERROR: Current branch is '$CURRENT_BRANCH'. Must be on canonical branch 'main' to finalize."
    exit 1
fi

echo "Staging approved exact files:"
for f in "${FILES[@]}"; do
    echo "  + $f"
done
git add -- "${FILES[@]}"

echo "Checking staged diff for whitespace / conflict errors..."
git diff --cached --check
git diff --cached --stat

if [ "$DRY_RUN" = true ]; then
    echo "[DRY-RUN] Unstaging files to preserve worktree..."
    git reset HEAD -- "${FILES[@]}" >/dev/null 2>&1 || true
    echo "[DRY-RUN] PASS: Dry-run staging and validation succeeded. No commit or push performed."
    exit 0
fi

echo "Creating commit..."
git commit -m "$MSG"

echo "Pushing main to origin..."
git push origin main

echo "Finalize complete!"
