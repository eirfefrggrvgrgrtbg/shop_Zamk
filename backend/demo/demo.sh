#!/usr/bin/env bash
# DEMO.D1 launcher. Usage: backend/demo/demo.sh {seed|fill|images|backend|shop|seller|admin|check}
# Everything runs against zamk_demo / zamk-demo / :8090; dev and test configs are not touched.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
[ -f "$HERE/.env" ] || { echo "missing $HERE/.env — copy demo.env.example and fill it in"; exit 1; }
set -a; . "$HERE/.env"; set +a
case "$POSTGRES_DSN$DEMO_DATABASE_URL" in *"/zamk_demo?"*"/zamk_demo?"*) ;; *) echo "REFUSED: DSN is not zamk_demo"; exit 1;; esac
API="http://127.0.0.1:${APP_PORT}/api"
case "${1:-}" in
  seed)    cd "$ROOT/backend" && go run ./cmd/demo-seed ;;
  fill)    cd "$ROOT/backend" && go run ./cmd/demo-catalog-fill ;; # DEMO.D1.1: composition + size charts, idempotent
  images)  shift; cd "$ROOT/backend" && go run ./cmd/demo-image-import --manifest "$HERE/images/manifest.json" "$@" ;; # DEMO.D1.2 (--dry-run, --allow-partial, --manifest <file>)
  backend) mkdir -p "$HERE/bin" && (cd "$ROOT/backend" && go build -o "$HERE/bin/api" ./cmd/api) && cd "$HERE" && exec ./bin/api ;; # cwd = demo dir, so godotenv reads demo/.env only
  shop)    cd "$ROOT" && VITE_API_URL="$API" npm run dev --workspace=shop -- --port 3100 --strictPort ;;
  seller)  cd "$ROOT" && VITE_API_URL="$API" npm run dev --workspace=seller -- --port 3101 --strictPort ;;
  admin)   cd "$ROOT" && VITE_API_URL="$API" npm run dev --workspace=admin -- --port 3102 --strictPort ;;
  check)   curl -fsS "$API/health" ;;
  *) echo "usage: $0 {seed|fill|images|backend|shop|seller|admin|check}"; exit 1 ;;
esac
