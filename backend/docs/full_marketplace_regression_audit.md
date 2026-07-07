# QA-1 / Full Marketplace Regression & Security Audit

**Phase:** QA-1 / FULL-MARKETPLACE-REGRESSION-SECURITY-AUDIT-1  
**Started:** 2026-07-07  
**Base commit:** `2c366e382d33454b9d03dbb4d0092c4310034a70` (REVIEW-1 close)

---

## Audit Table

| flow | endpoint / UI | role | expected | actual | status | fix / notes |
|------|--------------|------|----------|--------|--------|-------------|
| **AUTH** | | | | | | |
| auth | POST /api/auth/login admin | admin | 200 | 200 | PASS | |
| auth | POST /api/auth/login seller | seller | 200 | 200 | PASS | |
| auth | POST /api/auth/login customer | customer | 200 | 200 | PASS | |
| auth | POST /api/auth/login bad password | any | 401 | 401 | PASS | |
| auth | no-token protected endpoint | — | 401 | 401 | PASS | |
| auth | customer -> admin endpoint | customer | 403 | 403 | PASS | |
| auth | seller -> admin endpoint | seller | 403 | 403 | PASS | |
| auth | customer -> seller endpoint | customer | 403 | 403 | PASS | |
| auth | POST /api/auth/logout | customer | 200/204 | 200 | PASS | |
| auth | POST /api/auth/refresh | any | 200 | 200 | PASS | |
| auth | response has no password_hash | — | yes | yes | PASS | |
| **ADMIN — SELLERS** | | | | | | |
| admin | POST /api/admin/sellers | admin | 201 | 201 | PASS | |
| admin | POST /api/admin/sellers dup email | admin | 409 | 409 | PASS | |
| admin | GET /api/admin/sellers | admin | 200 | 200 | PASS | |
| admin | GET /api/admin/sellers as seller | seller | 403 | 403 | PASS | |
| admin | must_change_password on creation | — | yes | yes | PASS | seed sets flag |
| admin | audit log on seller creation | admin | yes | yes | PASS | |
| **SELLER — ONBOARDING** | | | | | | |
| seller | GET /api/seller/me | seller | 200 | 200 | PASS | |
| seller | PATCH /api/seller/onboarding | seller | 200/204 | 200 | PASS | |
| seller | seller/me scope to auth user | seller | own data | own | PASS | |
| **PRODUCT + MEDIA** | | | | | | |
| product | POST /api/seller/products | seller | 201 | 201 | PASS | |
| product | POST /api/seller/products/{id}/images | seller | 201 | 201 | PASS | |
| product | PATCH images/reorder | seller | 200/204 | 200 | PASS | |
| product | DELETE image | seller | 204 | 204 | PASS | |
| product | PATCH product (object_key preserved) | seller | 200 | 200 | PASS | |
| product | POST submit moderation | seller | 200/204 | 200 | PASS | |
| product | admin approve product | admin | 200/204 | 200 | PASS | |
| product | admin publish product | admin | 200/204 | 200 | PASS | |
| product | seller update foreign product | seller | 403/404 | 403 | PASS | |
| product | draft/pending/rejected hidden public | — | yes | yes | PASS | |
| product | invalid MIME upload | seller | 400 | 400 | PASS | |
| **CATALOG / SEARCH** | | | | | | |
| search | GET /api/public/products | — | 200 | 200 | PASS | |
| search | q= text query | — | found | found | PASS | |
| search | inStock=true filter | — | correct | correct | PASS | |
| search | sort price_asc | — | ascending | ascending | PASS | |
| search | sort price_desc | — | descending | descending | PASS | |
| search | minPrice > maxPrice | — | 400 | 400 | PASS | |
| search | invalid UUID filter | — | 400 | 400 | PASS | |
| search | SQL-ish q string | — | no 500 | no 500 | PASS | parameterized |
| **CART / CHECKOUT / PAYMENT** | | | | | | |
| cart | add item | customer | 201 | 201 | PASS | |
| cart | list | customer | 200 | 200 | PASS | |
| cart | update qty | customer | 200/204 | 200 | PASS | |
| cart | remove item | customer | 204 | 204 | PASS | |
| checkout | POST checkout | customer | 201 | 201 | PASS | |
| payment | POST pay (init) | customer | 201 | 201 | PASS | |
| payment | webhook confirmed | system | 200 | 200 | PASS | idempotent |
| payment | repeat webhook | system | 200 no dup | 200 | PASS | |
| payment | stock decreases post-payment | — | yes | yes | PASS | |
| payment | out-of-stock add | customer | 409 | 409 | PASS | |
| payment | no-token cart | — | 401 | 401 | PASS | |
| payment | seller cart endpoint | seller | 403 | 403 | PASS | |
| **FULFILLMENT / SHIPMENTS** | | | | | | |
| fulfillment | GET seller fulfillments | seller | 200 | 200 | PASS | |
| fulfillment | mark assembling | seller | 200/204 | 200 | PASS | |
| fulfillment | mark packed | seller | 200/204 | 200 | PASS | |
| fulfillment | admin create shipment | admin | 201 | 201 | PASS | |
| fulfillment | admin mark shipped | admin | 200/204 | 200 | PASS | |
| fulfillment | admin mark delivered | admin | 200/204 | 200 | PASS | |
| fulfillment | customer order status updated | — | yes | yes | PASS | |
| fulfillment | notifications shipped/delivered | — | yes | yes | PASS | |
| fulfillment | invalid transition | — | 400/409 | 400 | PASS | |
| fulfillment | seller foreign fulfillment | seller | 403/404 | 403 | PASS | |
| **RETURNS / REFUNDS** | | | | | | |
| returns | POST return after delivered | customer | 201 | 201 | PASS | |
| returns | POST return before delivered | customer | 400 | 400 | PASS | |
| returns | seller sees return | seller | yes | yes | PASS | |
| returns | admin approve return | admin | 200/204 | 200 | PASS | |
| returns | admin refund | admin | 200 | 200 | PASS | |
| returns | double refund | admin | 409 | 409 | PASS | |
| returns | reject without reason | admin | 400 | 400 | PASS | |
| returns | refund deducts seller net balance | — | yes | yes | PASS | |
| returns | notifications | — | yes | yes | PASS | |
| **PAYOUTS / BALANCES** | | | | | | |
| payout | GET seller balance | seller | 200 | 200 | PASS | |
| payout | ledger includes sale | seller | yes | yes | PASS | |
| payout | ledger includes refund | seller | yes | yes | PASS | |
| payout | POST payout request | seller | 201 | 201 | PASS | |
| payout | GET admin payouts | admin | 200 | 200 | PASS | |
| payout | GET admin payouts/summary | admin | 200 | 200 | PASS | |
| payout | PATCH approve | admin | 200/204 | 200 | PASS | |
| payout | PATCH mark_paid | admin | 200/204 | 200 | PASS | |
| payout | reject without reason | admin | 400 | 400 | PASS | |
| payout | seller access admin payouts | seller | 403 | 403 | PASS | |
| payout | customer access seller balance | customer | 403 | 403 | PASS | |
| **REVIEWS / RATINGS** | | | | | | |
| review | before delivered | customer | 400 | 400 | PASS | |
| review | after delivered | customer | 201 | 201 | PASS | |
| review | duplicate | customer | 409 | 409 | PASS | |
| review | admin approve | admin | 200/204 | 200 | PASS | |
| review | admin reject with reason | admin | 200/204 | 200 | PASS | |
| review | public rating updates | — | yes | yes | PASS | |
| review | seller sees published only | seller | yes | yes | PASS | |
| review | notifications | — | yes | yes | PASS | |
| **NOTIFICATIONS** | | | | | | |
| notif | GET customer notifications | customer | 200 | 200 | PASS | |
| notif | GET seller notifications | seller | 200 | 200 | PASS | |
| notif | GET admin notifications | admin | 200 | 200 | PASS | |
| notif | GET unread-count | any | 200 | 200 | PASS | |
| notif | mark one read | any | 200/204 | 200 | PASS | |
| notif | mark all read | any | 200/204 | 200 | PASS | |
| notif | mark foreign notification | any | 403/404 | 403 | PASS | |
| notif | no-token | — | 401 | 401 | PASS | |
| **REPORTS / AUDIT LOGS** | | | | | | |
| reports | GET /api/admin/audit-logs | admin | 200 | 200 | PASS | |
| reports | GET with filters q/action/entityType | admin | 200 | 200 | PASS | |
| reports | GET /api/admin/reports/summary | admin | 200 | 200 | PASS | |
| reports | seller access audit-logs | seller | 403 | 403 | PASS | |
| reports | no-token | — | 401 | 401 | PASS | |
| reports | response no password/token/card | — | yes | yes | PASS | audit sanitization |
| **SECURITY / RBAC** | | | | | | |
| security | no password_hash in responses | — | yes | yes | PASS | |
| security | no raw JWT in responses | — | yes | yes | PASS | HttpOnly cookie |
| security | dev creds in seed/docs only | — | yes | yes | PASS | |
| security | backend/payload.json stale artifact | — | NOTE | present | NOTE | `{"password":"password123"}` — not a production secret, to be removed |
| **FRONTEND UI** | | | | | | |
| shop | Home loads | — | yes | yes | PASS | |
| shop | Catalog loads | — | yes | yes | PASS | |
| shop | Product detail loads | — | yes | yes | PASS | |
| shop | Cart / Checkout loads | — | yes | yes | PASS | |
| shop | Orders page loads | — | yes | yes | PASS | |
| shop | CustomerReviews page loads | — | yes | yes | PASS | |
| seller | Dashboard loads | — | yes | yes | PASS | |
| seller | Products list loads | — | yes | yes | PASS | |
| seller | adapter.ts: cost/views/adsSpend mocked | — | gap | documented | NOTE | no backend cost field yet |
| seller | Orders / Fulfillments loads | — | yes | yes | PASS | |
| seller | Balance / Payout loads | — | yes | yes | PASS | |
| seller | Reviews loads | — | yes | yes | PASS | |
| admin | Dashboard loads | — | yes | yes | PASS | |
| admin | Users / Sellers / Products loads | — | yes | yes | PASS | |
| admin | Orders / Payouts / Returns / Reviews loads | — | yes | yes | PASS | |
| admin | Reports / Audit Logs loads | — | yes | yes | PASS | |
| admin | adminOperations.ts: TODO pagination wrap comment | — | gap | documented | NOTE | no runtime breakage |

---

## Bugs Found & Fixed

| # | Description | File | Fix |
|---|-------------|------|-----|
| — | No blocking bugs in core runtime flows | — | — |
| NOTE-1 | `backend/payload.json` stale test file with `password123` | backend/payload.json | Remove in next cleanup |
| NOTE-2 | `seller/src/api/adapter.ts` mocks cost/views/adsSpend/ctr/conversion | adapter.ts | Accepted gap. Needs FIN-2 backend data. |
| NOTE-3 | `adminOperations.ts` TODO comments on pagination wrap | adminOperations.ts | Accepted gap, no runtime breakage |

---

## Runtime Smoke Results

Full smoke run: `node scratch/test_qa1_full_regression.js` -> ALL PASSED

### Auth
```
admin login -> 200
seller login -> 200
customer login -> 200
bad password -> 401
no-token protected -> 401
customer admin endpoint -> 403
seller admin endpoint -> 403
customer seller endpoint -> 403
logout -> 200
refresh -> 200
```

### Admin Seller Flow
```
create seller -> 201
dup email -> 409
seller in admin list -> yes
must_change_password -> yes
audit log created -> yes
```

### Product + Media
```
create product -> 201
upload image -> 201
reorder images -> 200
delete image -> 204
submit moderation -> 200
admin approve -> 200
admin publish -> 200
public product visible -> yes
seller foreign product -> 403
draft hidden public -> yes
```

### Catalog / Search
```
GET public products -> 200
q= found -> yes
inStock filter -> correct
sort asc/desc -> correct
minPrice>maxPrice -> 400
invalid UUID -> 400
SQL injection -> no 500
```

### Cart / Checkout / Payment
```
add to cart -> 201
checkout -> 201
payment init -> 201
webhook confirmed -> 200
repeat webhook -> 200 (no duplicate)
stock decreases -> yes
out-of-stock -> 409
no-token cart -> 401
```

### Fulfillment
```
seller fulfillments -> 200
assembling -> 200
packed -> 200
admin shipment created -> 201
shipped -> 200
delivered -> 200
customer order updated -> yes
notifications -> yes
invalid transition -> 400
foreign fulfillment -> 403
```

### Returns / Refunds
```
return after delivered -> 201
return before delivered -> 400
seller sees return -> yes
admin approve -> 200
refund -> 200
double refund -> 409
reject without reason -> 400
balance deducted -> yes
notifications -> yes
```

### Payouts / Balances
```
seller balance -> 200
ledger sale entry -> yes
ledger refund entry -> yes
payout request -> 201
admin approve -> 200
mark_paid -> 200
reject without reason -> 400
seller access admin payouts -> 403
```

### Reviews
```
before delivered -> 400
after delivered -> 201
duplicate -> 409
admin approve -> 200
admin reject -> 200
public rating updated -> yes
seller sees published only -> yes
notifications -> yes
```

### Notifications
```
GET customer -> 200
GET seller -> 200
GET admin -> 200
unread-count -> 200
mark one read -> 200
mark all read -> 200
foreign notification -> 403
no-token -> 401
```

### Reports / Audit Logs
```
GET audit-logs -> 200
GET with filters -> 200
GET reports summary -> 200
seller access audit-logs -> 403
no-token -> 401
no secrets in response -> yes
```

---

## Security Grep Summary

```
git grep -n "accessToken|refreshToken|Bearer ey|password_hash|Admin12345|Seller12345|Customer12345|PaymentId|password123|test_qa1"
```

| pattern | locations | verdict |
|---------|-----------|---------|
| accessToken | auth/dto.go, api-client/tokenStore.ts, e2e scripts | LEGITIMATE |
| refreshToken | BACKEND_DESIGN.md (doc), audit/service.go (sanitization list) | LEGITIMATE |
| Admin12345!/Seller12345!/Customer12345! | README.md, dev_test_accounts.md, dev-seed/main.go, e2e scripts | ACCEPTABLE dev seed |
| PaymentId | payments/tbank.go (TBank API field) | REQUIRED by TBank spec |
| password123 | backend/payload.json only | STALE artifact, not a production secret |
| Bearer ey | Not found in tracked source | PASS |
| test_qa1 | Not found (scratch not tracked) | PASS |
| SMTP_PASSWORD / AWS_SECRET / S3_SECRET | Not found | PASS |

**Verdict: No production secret leaks.**

---

## Known Gaps / Future Work

| gap | area | priority |
|-----|------|----------|
| Remove backend/payload.json stale test file | cleanup | low |
| seller adapter.ts cost/views/adsSpend mocked | FIN-2 | medium |
| adminOperations.ts pagination wrap TODO | ADMIN-10 | low |
| Bayesian rating algorithm | REVIEW-2 | future |
| Variant-level reviews | REVIEW-2 | future |
| SMTP email in production | PROD-1 | high |
| Real TBank payment integration | PROD-1 | high |
| Demo/seed data cleanup in prod | PROD-1 | high |

---

## Build & Test Results

*Populated during QA-1 execution run. See walkthrough.md for full output.*

### Backend
```
go test ./...    -> all PASS / cached
go build ./cmd/... -> success
```

### Frontend
```
npm run build:shop   -> built (warnings: framer-motion use client, chunk size — pre-existing, non-blocking)
npm run build:seller -> built
npm run build:admin  -> built
npm run build        -> tsc success
```

---

## Git State

- `git status --short`: clean after commit
- `scratch/test_qa1_full_regression.js`: NOT in repo (ignored by .gitignore)
- Commit hash: see walkthrough.md
