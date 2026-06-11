#!/usr/bin/env bash
set -euo pipefail

API="http://127.0.0.1:8080/api"
SHOP="http://127.0.0.1:3000"
PRODUCT_ID="88888888-8888-4888-8888-888888888888"
SELLER_ID="44444444-4444-4444-8444-444444444444"
CUSTOMER_EMAIL="customer@zamk.local"
CUSTOMER_PASS="Customer12345!"
ADMIN_EMAIL="admin@zamk.local"
ADMIN_PASS="Admin12345!"
DSN="postgres://zamk:zamk_password@localhost:5433/zamk?sslmode=disable"

PASS=0
FAIL=0
note() { echo "[NOTE] $*"; }
ok() { echo "[PASS] $*"; PASS=$((PASS+1)); }
bad() { echo "[FAIL] $*"; FAIL=$((FAIL+1)); }

login() {
  local email="$1" pass="$2"
  curl -sS -c "/tmp/zamk_cookies_$$.txt" -X POST "$API/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$email\",\"password\":\"$pass\"}"
}

token_from_login() {
  login "$1" "$2" | python -c "import sys,json; print(json.load(sys.stdin)['accessToken'])"
}

# --- API checks ---
note "API: unauthenticated GET favorites"
CODE=$(curl -sS -o /tmp/fav_unauth.json -w "%{http_code}" "$API/customer/favorites")
if [[ "$CODE" == "401" ]]; then ok "GET /customer/favorites unauthenticated -> $CODE"; else bad "expected 401, got $CODE"; fi
if grep -q '"error"' /tmp/fav_unauth.json 2>/dev/null || grep -q 'unauthorized' /tmp/fav_unauth.json 2>/dev/null; then
  ok "unauthenticated response is JSON error, not raw stack trace"
else
  note "unauth body: $(cat /tmp/fav_unauth.json)"
fi

CUSTOMER_TOKEN=$(token_from_login "$CUSTOMER_EMAIL" "$CUSTOMER_PASS")
ADMIN_TOKEN=$(token_from_login "$ADMIN_EMAIL" "$ADMIN_PASS")

note "API: authenticated GET favorites"
CODE=$(curl -sS -o /tmp/fav_auth.json -w "%{http_code}" -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/favorites")
if [[ "$CODE" == "200" ]]; then ok "GET /customer/favorites authenticated -> 200"; else bad "expected 200, got $CODE"; fi

note "API: POST favorite (published active product)"
CODE=$(curl -sS -o /tmp/fav_add.json -w "%{http_code}" -X POST -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/favorites/$PRODUCT_ID")
if [[ "$CODE" == "201" || "$CODE" == "200" ]]; then ok "POST favorite first time -> $CODE"; else bad "POST favorite failed: $CODE $(cat /tmp/fav_add.json)"; fi

note "API: POST favorite idempotent"
CODE=$(curl -sS -o /tmp/fav_add2.json -w "%{http_code}" -X POST -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/favorites/$PRODUCT_ID")
if [[ "$CODE" == "201" || "$CODE" == "200" ]]; then ok "POST favorite duplicate -> $CODE (idempotent)"; else bad "duplicate POST failed: $CODE"; fi

COUNT=$(curl -sS -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/favorites" | python -c "import sys,json; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else 0)")
if [[ "$COUNT" == "1" ]]; then ok "favorites list has exactly 1 item after duplicate add"; else bad "expected 1 favorite, got $COUNT"; fi

note "API: DELETE favorite"
CODE=$(curl -sS -o /tmp/fav_del.json -w "%{http_code}" -X DELETE -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/favorites/$PRODUCT_ID")
if [[ "$CODE" == "200" ]]; then ok "DELETE favorite -> 200"; else bad "DELETE failed: $CODE"; fi

note "API: DELETE favorite idempotent"
CODE=$(curl -sS -o /tmp/fav_del2.json -w "%{http_code}" -X DELETE -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/favorites/$PRODUCT_ID")
if [[ "$CODE" == "200" ]]; then ok "DELETE favorite again -> 200 (idempotent)"; else bad "idempotent DELETE failed: $CODE"; fi

note "API: POST draft product rejected"
docker exec zamk_postgres psql -U zamk -d zamk -c "UPDATE products SET status='draft' WHERE id='$PRODUCT_ID';" >/dev/null
CODE=$(curl -sS -o /tmp/fav_draft.json -w "%{http_code}" -X POST -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/favorites/$PRODUCT_ID")
docker exec zamk_postgres psql -U zamk -d zamk -c "UPDATE products SET status='published' WHERE id='$PRODUCT_ID';" >/dev/null
if [[ "$CODE" == "404" ]]; then ok "POST draft product -> 404"; else bad "expected 404 for draft, got $CODE"; fi

note "API: POST with blocked seller rejected"
docker exec zamk_postgres psql -U zamk -d zamk -c "UPDATE sellers SET status='blocked' WHERE id='$SELLER_ID';" >/dev/null
CODE=$(curl -sS -o /tmp/fav_blocked.json -w "%{http_code}" -X POST -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/favorites/$PRODUCT_ID")
docker exec zamk_postgres psql -U zamk -d zamk -c "UPDATE sellers SET status='active' WHERE id='$SELLER_ID';" >/dev/null
if [[ "$CODE" == "404" ]]; then ok "POST with blocked seller -> 404"; else bad "expected 404 for blocked seller, got $CODE"; fi

# Re-add for visibility test
curl -sS -X POST -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/favorites/$PRODUCT_ID" >/dev/null

note "API: hide product removes from favorites list"
curl -sS -X POST -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
  "$API/admin/moderation/products/$PRODUCT_ID/hide" -d '{}' >/dev/null
COUNT=$(curl -sS -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/favorites" | python -c "import sys,json; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else 0)")
curl -sS -X POST -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
  "$API/admin/moderation/products/$PRODUCT_ID/publish" -d '{}' >/dev/null
if [[ "$COUNT" == "0" ]]; then ok "hidden product not in favorites list"; else bad "hidden product still listed, count=$COUNT"; fi

note "API: orders and reviews still work"
CODE=$(curl -sS -o /tmp/orders.json -w "%{http_code}" -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/orders")
if [[ "$CODE" == "200" ]]; then ok "GET /customer/orders -> 200"; else bad "orders failed: $CODE"; fi
CODE=$(curl -sS -o /tmp/reviews.json -w "%{http_code}" -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/customer/reviews")
if [[ "$CODE" == "200" ]]; then ok "GET /customer/reviews -> 200"; else bad "reviews failed: $CODE"; fi

note "API: /auth/me profile data"
ME=$(curl -sS -H "Authorization: Bearer $CUSTOMER_TOKEN" "$API/auth/me")
echo "$ME" | python -c "import sys,json; u=json.load(sys.stdin)['user']; assert u['email']=='customer@zamk.local'; assert u['name']; print('profile:', u['name'], u['email'], u.get('status',''))"
ok "/auth/me returns real customer profile"

# --- Shop page smoke ---
note "Shop: pages load"
for path in /account /favorites /orders /reviews "/product/$PRODUCT_ID"; do
  CODE=$(curl -sS -o /dev/null -w "%{http_code}" "$SHOP$path")
  if [[ "$CODE" == "200" ]]; then ok "GET $path -> 200"; else bad "GET $path -> $CODE"; fi
done

# --- Index check ---
note "DB: idx_customer_favorites_user_id exists"
IDX=$(docker exec zamk_postgres psql -U zamk -d zamk -tAc "SELECT indexname FROM pg_indexes WHERE tablename='customer_favorites' AND indexname='idx_customer_favorites_user_id';")
if [[ "$IDX" == "idx_customer_favorites_user_id" ]]; then ok "index idx_customer_favorites_user_id present"; else bad "index missing: '$IDX'"; fi

echo ""
echo "=== SUMMARY: $PASS passed, $FAIL failed ==="
exit $FAIL
