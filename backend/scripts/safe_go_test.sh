#!/usr/bin/env bash
# TEST.DB.4 — run backend integration tests for EXPLICIT packages only.
#   backend/scripts/safe_go_test.sh ./internal/products/ ./internal/http/router/ [-- extra go test flags]
# Refuses "./..." and wildcard package patterns, refuses any TEST_DATABASE_URL other than a local zamk_test,
# and first runs the static guard that fails if any test executes table-wide destructive SQL outside a disposable DB.
set -euo pipefail
cd "$(dirname "$0")/.."

pkgs=(); extra=()
while [ $# -gt 0 ]; do
  if [ "$1" = "--" ]; then shift; extra=("$@"); break; fi
  pkgs+=("$1"); shift
done
[ ${#pkgs[@]} -gt 0 ] || { echo "REFUSED: name the packages explicitly, e.g. ./internal/products/"; exit 2; }
for p in "${pkgs[@]}"; do
  case "$p" in
    *...*) echo "REFUSED: '$p' — wildcard package patterns (./...) are not allowed; list the packages"; exit 2 ;;
    ./internal/*|./cmd/*) ;;
    *) echo "REFUSED: '$p' is not an ./internal/... or ./cmd/... package path"; exit 2 ;;
  esac
done

dsn="${TEST_DATABASE_URL:-postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable}"
case "$dsn" in
  *@localhost:*/zamk_test\?*|*@127.0.0.1:*/zamk_test\?*|*@localhost:*/zamk_test|*@127.0.0.1:*/zamk_test) ;;
  *) echo "REFUSED: TEST_DATABASE_URL must point at a local zamk_test"; exit 2 ;;
esac
export TEST_DATABASE_URL="$dsn"

echo "== static guard: no table-wide destructive SQL outside disposable databases"
go test -count=1 ./internal/testutil/ -run 'TestDestructiveSQLOnlyOnDisposableDatabases|TestDestructiveLinesDetector'
echo "== go test -p 1 ${pkgs[*]} ${extra[*]:-}"
exec go test -p 1 -count=1 "${pkgs[@]}" ${extra[@]+"${extra[@]}"}
