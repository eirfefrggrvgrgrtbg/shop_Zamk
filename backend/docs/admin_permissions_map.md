# Admin Routes Permission Map

Every `/api/admin` route requires `AuthMiddleware + RequireRole("admin")` as the baseline.
`RequirePermission` is applied **on top** — it never replaces `RequireRole`.

| Route | Method | Current Guard | New Permission |
|---|---|---|---|
| /api/admin/me | GET | auth + role | (no fine-grained, role=admin only) |
| /api/admin/sellers | GET | auth + role | sellers.read |
| /api/admin/sellers | POST | auth + role | sellers.create_access |
| /api/admin/sellers/{id}/status | PATCH | auth + role | sellers.update_status |
| /api/admin/categories | GET | auth + role | categories.read |
| /api/admin/categories | POST | auth + role | categories.create |
| /api/admin/categories/{id} | PUT/PATCH | — | **route not found** |
| /api/admin/categories/{id} | DELETE | — | **route not found** |
| /api/admin/brands | GET | auth + role | brands.read |
| /api/admin/brands | POST | auth + role | brands.create |
| /api/admin/brands/{id} | PUT/PATCH | — | **route not found** |
| /api/admin/brands/{id} | DELETE | — | **route not found** |
| /api/admin/brands/{id}/logo/upload | POST | auth + role | brands.update |
| /api/admin/products | GET | auth + role | products.read |
| /api/admin/products/{id}/images/upload | POST | auth + role | products.moderate |
| /api/admin/moderation/products | GET | auth + role | products.moderate |
| /api/admin/moderation/products/{id}/approve | POST | auth + role | products.approve |
| /api/admin/moderation/products/{id}/reject | POST | auth + role | products.reject |
| /api/admin/moderation/products/{id}/publish | POST | auth + role | products.publish |
| /api/admin/moderation/products/{id}/hide | POST | auth + role | products.hide |
| /api/admin/moderation/products/{id}/block | POST | auth + role | products.block |
| /api/admin/inventory | GET | auth + role | inventory.read |
| /api/admin/inventory/{id} | GET | auth + role | inventory.read |
| /api/admin/inventory/{id}/movements | GET | auth + role | inventory.movements.read |
| /api/admin/inventory/receipts | POST | auth + role | inventory.receipt |
| /api/admin/inventory/adjustments | POST | auth + role | inventory.adjust |
| /api/admin/inventory/write-offs | POST | auth + role | inventory.write_off |
| /api/admin/orders | GET | auth + role | orders.read |
| /api/admin/orders/{id} | GET | auth + role | orders.read |
| /api/admin/orders/{id}/status | PATCH | auth + role | orders.update_status |
| /api/admin/orders/{id}/shipment | POST | auth + role | shipments.create |
| /api/admin/payments | GET | auth + role | payments.read |
| /api/admin/payments/{id} | GET | auth + role | payments.read |
| /api/admin/shipments | GET | auth + role | shipments.read |
| /api/admin/shipments/{id} | GET | auth + role | shipments.read |
| /api/admin/shipments/{id}/status | PATCH | auth + role | shipments.update_status |
| /api/admin/returns | GET | auth + role | returns.read |
| /api/admin/returns/{id} | GET | auth + role | returns.read |
| /api/admin/returns/{id}/status | PATCH | auth + role | returns.update_status |
| /api/admin/returns/{id}/refund | POST | auth + role | refunds.create |
| /api/admin/refunds | GET | auth + role | refunds.read |
| /api/admin/refunds/{id} | GET | auth + role | refunds.read |
| /api/admin/payouts | GET | auth + role | payouts.read |
| /api/admin/payouts/{id} | GET | auth + role | payouts.read |
| /api/admin/payouts/{id}/status | PATCH | auth + role | **handler-level** (approve/reject/mark_paid based on target status) |
| /api/admin/payouts/trigger-availability | POST | auth + role | payouts.read |
| /api/admin/reviews | GET | auth + role | reviews.read |
| /api/admin/reviews/{id} | GET | auth + role | reviews.read |
| /api/admin/reviews/{id}/{action} | POST | auth + role | **handler-level** (reviews.approve/reject/hide/block based on action) |
| /api/admin/staff/roles | GET | auth + role | roles.read |
| /api/admin/staff/members | GET | auth + role | staff.read |
| /api/admin/storefront/blocks | POST/PUT/DELETE | auth + role | storefront.manage |

### Аукционы (Auctions)
| Маршрут (Route) | Метод | Тип доступа | Требуемое право (Permission) |
| --- | --- | --- | --- |
| /api/admin/auctions | GET | auth + role | auctions.read |
| /api/admin/auctions/{id} | GET | auth + role | auctions.read |
| /api/admin/auctions | POST | auth + role | auctions.create |
| /api/admin/auctions/{id} | PATCH | auth + role | auctions.update |
| /api/admin/auctions/{id}/publish | POST | auth + role | auctions.publish |
| /api/admin/auctions/{id}/pause | POST | auth + role | auctions.pause |
| /api/admin/auctions/{id}/resume | POST | auth + role | auctions.resume |
| /api/admin/auctions/{id}/cancel | POST | auth + role | auctions.cancel |
| /api/admin/auctions/{id}/finalize | POST | auth + role | auctions.finalize |
| /api/admin/auctions/{id}/lots | POST | auth + role | auctions.update |
| /api/admin/auction-lots/{id} | PATCH | auth + role | auctions.update |
| /api/admin/auction-lots/{id}/bids | GET | auth + role | auctions.read |
| /api/admin/auction-lots/{id}/mark-unpaid-review | POST | auth + role | auctions.update |
| /api/admin/auction-lots/{id}/move-to-direct-sale | POST | auth + role | auctions.move_to_direct_sale |
| /api/admin/staff/members/{userId}/role | PATCH | auth + role | staff.update |
| /api/admin/staff/members/{userId}/status | PATCH | auth + role | staff.block |
| /api/admin/staff/members/{userId}/reset-password | POST | auth + role | staff.update |
| /api/admin/audit-logs | GET | auth + role | audit.read |

## Handler-level dynamic permission routes

### PATCH /api/admin/payouts/{id}/status
Reads `status` from request body and maps:
- `"approved"` → requires `payouts.approve`
- `"rejected"` → requires `payouts.reject`
- `"paid"` / `"mark_paid"` → requires `payouts.mark_paid`
- other → fallback `payouts.approve`

### POST /api/admin/reviews/{id}/{action}
Reads `action` from URL param and maps:
- `"approve"` → requires `reviews.approve`
- `"reject"` → requires `reviews.reject`
- `"hide"` → requires `reviews.hide`
- `"block"` → requires `reviews.block`

## Routes not found in router (skipped)
- PUT/PATCH `/api/admin/categories/{id}` — route not registered
- DELETE `/api/admin/categories/{id}` — route not registered
- PUT/PATCH `/api/admin/brands/{id}` — route not registered
- DELETE `/api/admin/brands/{id}` — route not registered
