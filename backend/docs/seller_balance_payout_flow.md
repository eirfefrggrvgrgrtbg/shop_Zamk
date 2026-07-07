# Seller Balance, Commission and Payout Flow (FIN-1)

## Overview

This document describes the complete financial lifecycle for marketplace sellers in ZAMK: from a sale being made, through commission calculation, pending/available balance transitions, payout requests, and admin payout moderation.

---

## Commission Calculation

### Configuration

| Parameter | Source | Value |
|---|---|---|
| `MARKETPLACE_COMMISSION_BPS` | `.env` | `1500` (15%) |
| Config default fallback | `config.go` | `900` (9%) — overridden by env |
| Effective in orders | `orders/service.go` | reads `cfg.Worker.MarketplaceCommissionBPS` |
| Effective in payouts | `payouts/service.go` | reads `cfg.Worker.MarketplaceCommissionBPS` |

### Formula

```
gross = subtotal_cents (order item total, in kopecks)
commission = gross * commission_bps / 10000
net (seller amount) = gross - commission
```

### Example (1500 BPS = 15%)

```
Item price: 1000 ₽ (100000 kopecks)
Quantity: 1
Gross:      100000 kopecks (1000 ₽)
Commission: 100000 * 1500 / 10000 = 15000 kopecks (150 ₽)
Seller net: 100000 - 15000 = 85000 kopecks (850 ₽)
```

### Rounding

Integer truncation (no rounding). Any fractional kopeck is lost. This is the standard Go integer division behaviour.

### Currency

All amounts are stored as kopecks (int64). Currency is `RUB`.

### What goes into commission?

- Commission is computed on `subtotal_cents` per seller per order (sum of all items from that seller).
- Delivery fee: **not implemented** — delivery is not a separate line item in current architecture.
- Discounts/promo codes: **not implemented** — commission is applied to full item subtotal.
- These are documented as future work (FIN-2).

---

## Seller Balance Lifecycle

### Ledger Model

All balance changes are recorded in `seller_balance_ledger` (append-only, immutable entries):

| Type | Sign | When |
|---|---|---|
| `sale_pending` | + positive net amount | When order is marked `delivered` (shipment delivered hook) |
| `sale_available` | + positive net amount | After `return_window_days` (default 14 days), via worker job or `/trigger-availability` |
| `refund_deduction` | - negative | When a return/refund is processed (future FIN-2) |
| `manual_adjustment` | ± | Admin manual correction |
| `payout_requested` | - negative hold | When seller creates a payout request |
| `payout_rejected` | + positive release | When admin rejects payout (hold released) |
| `payout_cancelled` | + positive release | When payout is cancelled (hold released) |
| `payout_paid` | 0 (audit marker) | When payout marked as paid — balance deduction already done via hold |

### Balance Fields Derivation

```
pending_balance    = SUM(sale_pending)
available_balance  = SUM(sale_available)
                   + SUM(manual_adjustment)
                   + SUM(refund_deduction)      -- negative if refunded
                   + SUM(payout_requested)      -- negative (hold)
                   + SUM(payout_rejected)       -- positive (release)
                   + SUM(payout_cancelled)      -- positive (release)
```

### Balance Transition Timeline

```
1. Order created by customer          → no balance change
2. Payment confirmed (TBank webhook)  → no balance change
3. Seller marks assembling/packed     → no balance change
4. Shipment delivered                 → sale_pending entry created
                                        (net = gross * (1 - bps/10000))
5. After ReturnWindowDays (14 days)   → sale_available entry created
   [worker: MakeSellerFundsAvailable]    (same net amount)
   OR: POST /api/admin/payouts/trigger-availability (E2E testing)
6. Seller creates payout request      → payout_requested entry (negative hold)
7. Admin approves payout              → payout status → approved
8. Admin marks paid                   → payout status → paid, payout_paid audit entry
   OR: Admin rejects                  → payout_rejected entry (hold released)
```

### When Funds Become Available

**Funds become PENDING** immediately when an order's shipment is marked `delivered`.

**Funds become AVAILABLE** after `RETURN_WINDOW_DAYS` (default: 14 days), when the background worker `MakeSellerFundsAvailable` processes the pending entry. For testing, the endpoint `POST /api/admin/payouts/trigger-availability` with `{"daysToSimulate": 15}` simulates the time passage.

**Current behavior**: If an order item has an open return, it will not be converted to available (it stays as pending indefinitely until the return is resolved).

---

## Payout Statuses

| Status | Meaning |
|---|---|
| `requested` | Seller created a payout request, hold applied |
| `approved` | Admin approved, waiting for actual payment transfer |
| `paid` | Admin confirmed payment was made (manual, outside system) |
| `rejected` | Admin rejected with reason, hold released |
| `cancelled` | Cancelled (by admin), hold released |

### Allowed Transitions

```
requested → approved
requested → rejected
approved  → paid
approved  → cancelled
```

Any other transition returns `400 invalid_status_transition`.

---

## Role-Based Flows & RBAC

### Seller

**Can:**
- `GET /api/seller/balance` — view own balance only
- `GET /api/seller/payouts` — view own payout history
- `POST /api/seller/payouts/request` — create a payout request

**Cannot:**
- View another seller's balance
- Approve/reject/mark-paid any payout
- Modify balance directly

**Constraints:**
- Amount must be > 0
- Amount must not exceed `available_balance`
- Returns `409 insufficient_balance` if insufficient

### Admin

**Can:**
- `GET /api/admin/payouts` — view all payouts (with filters: q, sellerId, status)
- `GET /api/admin/payouts/{id}` — view payout detail
- `PATCH /api/admin/payouts/{id}/status` — approve/reject/paid/cancelled
- `GET /api/admin/payouts/summary` — financial summary
- `GET /api/admin/seller-balances` — all seller balance overview
- `POST /api/admin/payouts/trigger-availability` — advance time simulation (testing only)

**Permissions required:**
- `payouts.approve` for approve transitions
- `payouts.reject` for reject transitions
- `payouts.mark_paid` for paid transitions
- `payouts.read` for listing

**Reject rule:** requires non-empty comment (400 if omitted).

### Customer / No-token

- All seller and admin payout endpoints → `401` (unauthenticated) or `403` (wrong role)

---

## RBAC Matrix

| Endpoint | no-token | customer | seller | admin |
|---|---|---|---|---|
| GET /api/seller/balance | 401 | 403 | ✅ own only | ❌ |
| GET /api/seller/payouts | 401 | 403 | ✅ own only | ❌ |
| POST /api/seller/payouts/request | 401 | 403 | ✅ | ❌ |
| GET /api/admin/payouts | 401 | 403 | 403 | ✅ all |
| PATCH /api/admin/payouts/{id}/status | 401 | 403 | 403 | ✅ with perm |
| GET /api/admin/seller-balances | 401 | 403 | 403 | ✅ |

---

## Ledger / Transaction Safety

- All balance and payout operations use `RunInTx` (PostgreSQL transactions)
- Ledger entries are append-only (no UPDATE/DELETE on ledger rows)
- Payout hold: when `payout_requested` is created (negative ledger), the available balance is reduced atomically
- Double-spend: `RequestPayout` checks `available_balance >= requested_amount` **inside** the same transaction as creating the hold
- Idempotency: `HasSalePendingForOrderItem` prevents duplicate `sale_pending` entries for the same order item

---

## Refunds / Returns — Future Work

Refund and return settlement is **not** part of FIN-1.

- If an order is cancelled before delivery, the `sale_pending` entry is never created
- If an order item is returned, `MakeSellerFundsAvailable` skips the conversion to `sale_available`
- Full refund deduction via `refund_deduction` ledger type is implemented in service code (`ProcessRefundDeduction`), but the returns flow is a separate stage
- This is documented for FIN-2 / RET-1

---

## API Endpoints Summary

| Method | Path | Role | Description |
|---|---|---|---|
| GET | `/api/seller/balance` | seller | Get own balance |
| GET | `/api/seller/payouts` | seller | List own payouts |
| POST | `/api/seller/payouts/request` | seller | Create payout request |
| GET | `/api/admin/payouts` | admin | List all payouts (filtered) |
| GET | `/api/admin/payouts/summary` | admin | Financial summary |
| GET | `/api/admin/payouts/{id}` | admin | Get payout detail |
| PATCH | `/api/admin/payouts/{id}/status` | admin | Update payout status |
| GET | `/api/admin/seller-balances` | admin | All seller balances |
| POST | `/api/admin/payouts/trigger-availability` | admin | Advance time (testing) |

---

## Runtime Smoke Summary (FIN-1 Verified)

Full E2E verified in `scratch/test_fin1_flow.js`.

| Step | HTTP |
|---|---|
| customer checkout | 201 |
| payment webhook | 200 |
| seller mark-assembling | 200 |
| seller mark-packed | 200 |
| admin create shipment | 201 |
| admin mark shipped | 200 |
| admin mark delivered | 200 |
| GET /api/seller/balance (before sale_available) | 200 |
| POST trigger-availability | 200 |
| GET /api/seller/balance (after) | 200 — available > 0 |
| POST /api/seller/payouts/request | 201 |
| POST request > available | 409 |
| POST request amount=0 | 400 |
| GET /api/admin/payouts | 200 |
| PATCH reject without comment | 400 |
| PATCH reject with comment | 204 |
| GET seller balance after reject | 200 — available restored |
| POST second payout request | 201 |
| PATCH approve | 204 |
| PATCH paid | 204 |
| PATCH paid again (double) | 400 (invalid transition) |
| GET seller balance after paid | 200 — available decreased |
| RBAC: no-token seller balance | 401 |
| RBAC: customer seller balance | 403 |
| RBAC: seller admin payouts | 403 |

---

## FIN-1 Completion Criteria (All Met)

- [x] Seller balance verified after sale (pending → available via trigger)
- [x] Commission 15% (1500 BPS) verified in runtime
- [x] Payout request works (amount, insufficient balance check, zero check)
- [x] Admin can approve/reject/mark-paid
- [x] Balance changes correctly after approve/reject/paid
- [x] RBAC matrix verified
- [x] go test ./... passes
- [x] go build ./cmd/... passes
- [x] All npm builds pass
- [x] git clean, commit, push
