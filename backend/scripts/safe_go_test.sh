#!/usr/bin/env bash
# TEST.DB.5 — run backend tests for EXPLICIT packages, each run on its OWN throw-away PostgreSQL database.
#   backend/scripts/safe_go_test.sh ./internal/fulfillment/ [./internal/orders/ ...] [-- extra go test flags, e.g. -run '^TestX$' -v]
#
# 1. Refuses "./..." and wildcard package patterns.
# 2. Creates a fresh zamk_disposable_<12 random hex> database (cloned from the migrated template) and proves, by
#    connecting to it, that it is that database. If that is impossible nothing is run.
# 3. Hands the tests ONLY that database: TEST_DATABASE_URL, DATABASE_URL and POSTGRES_* all point at it; other PG* /
#    DEMO_DATABASE_URL variables are removed. zamk, zamk_test and zamk_demo are never given to a test, and
#    testutil.GetTestDatabaseURL / AssertTestDatabase refuse them anyway.
# 4. Runs the static guards first (destructive SQL, no shared-database access), then the tests.
# 5. Drops exactly the database it created (name + ownership token), on success, test failure, INT/TERM/HUP.
#    It never touches other zamk_disposable_* databases (no pattern cleanup). SIGKILL cannot be trapped and would
#    leave this run's database behind.
# The exit code is go test's (or 2 for a refusal, 1 when the isolated database cannot be created).
set -uo pipefail
cd "$(dirname "$0")/.."

pkgs=(); extra=()
while [ $# -gt 0 ]; do
  if [ "$1" = "--" ]; then shift; extra=("$@"); break; fi
  pkgs+=("$1"); shift
done
usage() {
  echo "  backend/scripts/safe_go_test.sh ./internal/<pkg>/ [./internal/<pkg2>/ ...] [-- -run '^TestX\$' -v]"
  echo "  npm run test:backend -- ./internal/<pkg>/ [-- -run '^TestX\$' -v]"
}
[ ${#pkgs[@]} -gt 0 ] || { echo "REFUSED: name the packages explicitly (no default suite):"; usage; exit 2; }
for p in "${pkgs[@]}"; do
  case "$p" in
    *...*) echo "REFUSED: '$p' — wildcard package patterns (./...) are not allowed; list the packages"; exit 2 ;;
    ./internal/*|./cmd/*) ;;
    *) echo "REFUSED: '$p' is not an ./internal/... or ./cmd/... package path"; exit 2 ;;
  esac
done

# Where the local PostgreSQL server is (host, port, credentials). The database part of this URL is never used.
server="${TEST_DATABASE_URL:-postgres://zamk:zamk_password@localhost:5433/postgres?sslmode=disable}"
case "$server" in
  postgres://*@localhost:*|postgres://*@127.0.0.1:*|postgresql://*@localhost:*|postgresql://*@127.0.0.1:*) ;;
  *) echo "REFUSED: TEST_DATABASE_URL must point at a LOCAL PostgreSQL server"; exit 2 ;;
esac

workdir="$(mktemp -d)"
db_name=""; db_token=""; child=""

cleanup() {
  local rc=$?
  trap - EXIT INT TERM HUP
  if [ -n "$child" ]; then kill -TERM "$child" 2>/dev/null; wait "$child" 2>/dev/null; fi
  if [ -n "$db_name" ] && [ -n "$db_token" ] && [ -x "$workdir/zamk-testdb" ]; then
    if TEST_DATABASE_URL="$server" "$workdir/zamk-testdb" drop "$db_name" "$db_token"; then
      echo "== dropped this run's database $db_name"
    else
      echo "WARNING: could not drop $db_name — it was created by this run and is safe to remove by hand"
    fi
  fi
  rm -rf "$workdir"
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

go build -o "$workdir/zamk-testdb" ./cmd/zamk-testdb || { echo "REFUSED: cannot build the test database tool"; exit 1; }

created="$(TEST_DATABASE_URL="$server" "$workdir/zamk-testdb" create)" || { echo "REFUSED: no isolated database — tests were not started"; exit 1; }
field() { printf '%s\n' "$created" | sed -n "s/^$1=//p" | head -n1; }
db_name="$(field NAME)"; db_token="$(field TOKEN)"
dsn="$(field DSN)"; host="$(field HOST)"; port="$(field PORT)"; user="$(field USER)"; password="$(field PASSWORD)"

# Belt and braces: whatever the tool printed, the name must be a per-run disposable database.
if ! printf '%s' "$db_name" | grep -Eq '^zamk_disposable_[0-9a-f]{12}$'; then
  echo "REFUSED: '$db_name' is not a per-run disposable database — tests were not started"; db_name=""; db_token=""; exit 1
fi
case "$db_name" in zamk|zamk_test|zamk_demo) echo "REFUSED: shared database name"; db_name=""; db_token=""; exit 1 ;; esac
[ -n "$db_token" ] && [ -n "$dsn" ] || { echo "REFUSED: incomplete isolated database description"; exit 1; }
echo "== isolated database for this run: $db_name"

unset DEMO_DATABASE_URL PGHOST PGHOSTADDR PGPORT PGDATABASE PGUSER PGPASSWORD PGSERVICE PGSERVICEFILE PGPASSFILE PGOPTIONS POSTGRES_DSN
export TEST_DATABASE_URL="$dsn" DATABASE_URL="$dsn"
export POSTGRES_HOST="$host" POSTGRES_PORT="$port" POSTGRES_USER="$user" POSTGRES_PASSWORD="$password" POSTGRES_DB="$db_name"
export ZAMK_TEST_RUNNER=1 ZAMK_TEST_RUNNER_DB="$db_name"

echo "== static guards: destructive SQL only on disposable databases; no code reaches zamk_test by itself"
go test -count=1 ./internal/testutil/ -run 'TestDestructiveSQLOnlyOnDisposableDatabases|TestDestructiveLinesDetector|TestNoSharedTestDatabaseAccessOutsideTestutil|TestSharedAccessDetector' || exit $?

echo "== go test -p 1 ${pkgs[*]} ${extra[*]:-}"
go test -p 1 -count=1 "${pkgs[@]}" ${extra[@]+"${extra[@]}"} &
child=$!
wait "$child"
rc=$?
child=""
exit "$rc"
