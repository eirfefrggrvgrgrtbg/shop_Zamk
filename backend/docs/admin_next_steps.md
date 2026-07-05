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

## ADMIN-6: Orders, Fulfillment, and Shipments
**Goal**: Combine fulfillment creation and shipment tracking.
- **Pages**: `AdminOrders.tsx`, `AdminShipments.tsx`
- **Tasks**: Move manual shipment creation into the Order Detail side-panel. Remove the temporary manual `createOrderId` form from `AdminShipments.tsx`.

## ADMIN-7: Inventory and Storage
**Goal**: Make inventory management precise and location-aware.
- **Pages**: `AdminInventory.tsx`
- **Tasks**: Determine if warehouse locations are needed. Make sure direct-sale items are distinctly separated from third-party seller items.

## ADMIN-8: Payouts and Commissions
**Goal**: Automate and track payouts to sellers.
- **Pages**: `AdminPayouts.tsx`
- **Tasks**: Integrate actual payout gateways (or tracking). Ensure commission calculations are visible and explainable via Tooltips.

## ADMIN-9: Audit Logs and Security Monitoring
**Goal**: Ensure all admin actions are logged and auditable.
- **Pages**: `AdminAuditLogs.tsx`
- **Tasks**: Connect UI to backend audit logs. Parse metadata visually.

## ADMIN-10: UX Polish & Error States
**Goal**: Ensure every page has strict loading/error/empty states and HelpTooltips.
- **Pages**: All.
- **Tasks**: Standardize the use of `PermissionGuard` and replace raw English errors with clean Russian equivalents. Add short `?` help tooltips everywhere.
