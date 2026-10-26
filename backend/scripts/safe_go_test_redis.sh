#!/usr/bin/env bash
# TEST.REDIS.ISOLATION.1 — safe_go_test.sh plus a throw-away Redis of its own.
#   backend/scripts/safe_go_test_redis.sh ./internal/http/router/ [-- -run '^TestX$' -v]
#
# 1. Starts ONE docker container from the already-present redis:7-alpine image (--pull never: nothing is downloaded):
#    published only on 127.0.0.1 on a free random port, no persistence (--save "" --appendonly no), protected by a
#    random per-run password that doubles as the ownership token (label zamk-test-redis-owner).
# 2. Proves ownership (label == token, 127.0.0.1 binding, authenticated PING) before any test starts; otherwise nothing runs.
# 3. Hands the tests ZAMK_TEST_REDIS_ADDR / ZAMK_TEST_REDIS_TOKEN only; testutil.NewTestRedis refuses anything else
#    (6379, a non-loopback host, a missing token). There is NO fallback to localhost:6379.
# 4. Delegates to safe_go_test.sh unchanged (own disposable PostgreSQL, static guards, ownership-checked drop).
# 5. Removes exactly the container it created (full id + label token check) on success, failure, INT/TERM/HUP.
#    It never stops a container by port or by name pattern and never touches the developer's Redis.
set -uo pipefail
cd "$(dirname "$0")/.."

IMAGE="redis:7-alpine"
LABEL="zamk-test-redis-owner"
cid=""; token=""; child=""

command -v docker >/dev/null 2>&1 || { echo "REFUSED: docker is required for the isolated test Redis"; exit 1; }
docker image inspect "$IMAGE" >/dev/null 2>&1 || { echo "REFUSED: image $IMAGE is not present locally (it is never pulled); no isolated Redis — tests were not started"; exit 1; }

cleanup() {
  local rc=$?
  trap - EXIT INT TERM HUP
  if [ -n "$child" ]; then kill -TERM "$child" 2>/dev/null; wait "$child" 2>/dev/null; fi
  if [ -n "$cid" ] && [ -n "$token" ]; then
    owner="$(docker inspect -f "{{index .Config.Labels \"$LABEL\"}}" "$cid" 2>/dev/null || true)"
    if [ "$owner" = "$token" ]; then
      docker rm -f "$cid" >/dev/null 2>&1 && echo "== removed this run's Redis container ${cid:0:12}" || echo "WARNING: could not remove container $cid (created by this run)"
    else
      echo "WARNING: container $cid is not owned by this run (or already gone); left alone"
    fi
  fi
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

token="$(openssl rand -hex 16)"
[ ${#token} -eq 32 ] || { echo "REFUSED: cannot create an ownership token"; exit 1; }

cid="$(docker run -d --rm --pull never --label "$LABEL=$token" -p 127.0.0.1::6379 "$IMAGE" \
  redis-server --requirepass "$token" --save "" --appendonly no 2>/dev/null)" || { cid=""; echo "REFUSED: could not start the isolated Redis — tests were not started"; exit 1; }
[ ${#cid} -eq 64 ] || { echo "REFUSED: unexpected container id; tests were not started"; cid=""; exit 1; }

owner="$(docker inspect -f "{{index .Config.Labels \"$LABEL\"}}" "$cid" 2>/dev/null || true)"
[ "$owner" = "$token" ] || { echo "REFUSED: container ownership could not be proven"; exit 1; }
bound="$(docker port "$cid" 6379/tcp 2>/dev/null | head -n1)"
case "$bound" in
  127.0.0.1:*) ;;
  *) echo "REFUSED: isolated Redis is not bound to 127.0.0.1 only ('$bound')"; exit 1 ;;
esac
port="${bound##*:}"
case "$port" in ''|*[!0-9]*|6379) echo "REFUSED: bad isolated Redis port '$port'"; exit 1 ;; esac

ready=""
for _ in $(seq 1 50); do
  if [ "$(docker exec "$cid" redis-cli -a "$token" --no-auth-warning ping 2>/dev/null)" = "PONG" ]; then ready=1; break; fi
  sleep 0.2
done
[ -n "$ready" ] || { echo "REFUSED: isolated Redis did not become ready"; exit 1; }
echo "== isolated Redis for this run: container ${cid:0:12} on 127.0.0.1:$port (no persistence)"

export ZAMK_TEST_REDIS_ADDR="127.0.0.1:$port" ZAMK_TEST_REDIS_TOKEN="$token"
scripts/safe_go_test.sh "$@" &
child=$!
wait "$child"
rc=$?
child=""
exit "$rc"
