#!/usr/bin/env bash
set -e

DRY_RUN=false
PUSH=false

while [ "$#" -gt 0 ]; do
    case "$1" in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --push)
            PUSH=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--dry-run] [--push] '<commit message>' <file1> <file2> ..."
            echo "Mandatory requirements:"
            echo "  - Branch must be main"
            echo "  - Files must be explicit paths (never '.', '-A', or '--all')"
            echo "  - Commit message must be provided"
            echo "  - Push is disabled by default; requires --push and PO authorization"
            exit 1
            ;;
        *)
            break
            ;;
    esac
done

if [ "$#" -lt 2 ]; then
    echo "Usage: $0 [--dry-run] [--push] '<commit message>' <file1> <file2> ..."
    echo "Mandatory requirements:"
    echo "  - Branch must be main"
    echo "  - Files must be explicit paths (never '.', '-A', or '--all')"
    echo "  - Commit message must be provided"
    echo "  - Push is disabled by default; requires --push and PO authorization"
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
COMMIT_SHA=$(git rev-parse HEAD)

if [ "$PUSH" = true ]; then
    echo "Pushing main to origin (PO authorized)..."
    git push origin main
    echo "Finalize with push complete: $COMMIT_SHA"
else
    echo "Local commit created successfully: $COMMIT_SHA (Push omitted by default)"
fi
