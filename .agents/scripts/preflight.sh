#!/usr/bin/env bash
set -e

echo "=== ZAMK Preflight ==="
echo "Branch: $(git branch --show-current)"
echo "HEAD: $(git rev-parse HEAD)"

echo "--- Git Status ---"
git status --short

echo "--- Stash List ---"
git stash list

echo "Preflight complete. Ensure no unexpected dirty files are present."
