# Admin Panel Next Steps

The MVP for the admin panel routing and structure is in place, but several sections require polish, unified UX, and deeper feature integration.

## ✅ ADMIN-2: Dashboard Metrics & Overview — COMPLETED
Real aggregate metrics via `GET /api/admin/dashboard/summary`. PII-safe. Role-protected.

## ✅ ADMIN-3: Users, Staff, and RBAC — COMPLETED
`GET /api/admin/users` with pagination/search/filter. `users.read` permission, role-mapped. `AdminUsers.tsx` uses real API.

## ✅ ADMIN-4: Seller Management Completion — COMPLETED
Seller list/search/filter/status. Platform seller protected from block/archive. Reset owner password flow. `sellers.manage` permission.

## ✅ ADMIN-5: Catalog & Product Moderation — COMPLETED (ADMIN-5, 5B, 5C)

**ADMIN-5**: Product list with filters (q, status, source), pagination, ZAMK badge (`auction_direct_sale`).

**ADMIN-5B**: Source consistency verified (`seller` and `auction_direct_sale` are the only valid values). RBAC confirmed in DB. All 3 builds pass.

**ADMIN-5C**: Product detail drawer added.
- Backend: `GET /api/admin/products/{id}` — returns full product (title, slug, status, source, price, variants, images, dates, moderation comment). Permission: `products.read`. No PII.
- Backend: `GET /api/admin/products/{id}/moderation-logs` — returns `[]` safely when empty.
- Frontend: Product detail drawer opens from table row click.
- Frontend: Moderation logs timeline shown in drawer.
- Frontend: Approve / Reject (with required reason) / Publish / Hide / Block action buttons — context-aware.
- Frontend: All UI strings in Russian. No English labels.
- Product sources: `seller` (default), `auction_direct_sale` (ZAMK platform products).
- Moderation flow: draft → pending_moderation → approved → published. Reject → rejected. Hide/Block available.
- Category/brand admin mutations: deferred to future work (read-only access exists).
- No password hash, tokens, or customer PII returned in any product admin endpoint.

## ✅ ADMIN-6: Orders, Fulfillment, and Shipments — COMPLETED (ADMIN-6, 6B, 6C, 6D)

**ADMIN-6 & 6B**: Admin orders list with server-side pagination, debounced search, and advanced filters (status, fulfillmentStatus, sourceType). Source types accurately inferred (`auction`, `direct_sale`, `normal`). Runtime verified.

**ADMIN-6C & 6D**: Admin mutation endpoints for `fulfillment-status`. Manual shipment creation moved into Order Detail side-panel.

## ✅ ADMIN-7: Inventory and Storage — COMPLETED (ADMIN-7, 7B, 7C)
**Goal**: Make inventory management precise and location-aware.
- **Completed**: Inventory list with advanced search/source filters. Detailed movements ledger. Unified stock adjustments endpoint (`receipt`, `adjustment`, `write_off`). Basic reservations listing endpoint. Admin/seller/public stock access correctly bounded. `PlatformSellerIDStr` UUID properly established for ZAMK platform tracking.

## ✅ ADMIN-8: Payouts and Commissions — COMPLETED (ADMIN-8, 8B)
**Goal**: Automate and track payouts to sellers.
- **Completed**: Payout summary works. Seller balances work. Payout filters work. Payout detail/action behavior strictly enforced (reject reason required, valid transitions). Direct sale ZAMK payout behavior validated. Seller endpoint state secured. No sensitive bank/payment data exposed.

## ADMIN-9: Audit Logs and Security Monitoring
**Goal**: Ensure all admin actions are logged and auditable.
- **Pages**: `AdminAuditLogs.tsx`
- **Tasks**: Connect UI to backend audit logs. Parse metadata visually.

## ADMIN-10: UX Polish & Error States
**Goal**: Ensure every page has strict loading/error/empty states and HelpTooltips.
- **Pages**: All.
- **Tasks**: Standardize the use of `PermissionGuard` and replace raw English errors with clean Russian equivalents. Add short `?` help tooltips everywhere.
