# Admin Panel Next Steps

The MVP for the admin panel routing and structure is in place, but several sections require polish, unified UX, and deeper feature integration.

## ADMIN-2: Dashboard Metrics & Overview
**Goal**: Replace client-side aggregation with aggregated backend endpoints.
- **Pages**: `AdminDashboard.tsx`
- **Backend Endpoints**: `GET /api/admin/metrics/dashboard` (needs implementation)
- **Tasks**: Show real stats (revenue, active auctions, moderation queue size) securely without pulling all lists into memory.

## ADMIN-3: Users, Staff, and RBAC
**Goal**: Make the user management and staff/role assignments robust.
- **Pages**: `AdminUsers.tsx`, `AdminStaff.tsx`, `AdminRoles.tsx`
- **Tasks**: Connect `AdminUsers.tsx` to a real backend endpoint (`GET /api/admin/users`). Ensure help tooltips explain RBAC concepts.

## ADMIN-4: Seller Management Completion
**Goal**: Finalize seller workflows, metrics, and isolation.
- **Pages**: `AdminSellers.tsx`
- **Tasks**: Ensure platform seller ("ZAMK") cannot be blocked or archived accidentally. Fix text encodings.

## ADMIN-5: Catalog & Product Moderation
**Goal**: Complete the moderation flow and category/brand management.
- **Pages**: `AdminProducts.tsx`, `AdminModeration.tsx`, `AdminCatalog.tsx`
- **Tasks**: Implement bulk actions for moderation. Add help tooltips ("?") explaining product status ("pending", "active", "rejected").

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

- ADMIN-2/ADMIN-2B: Dashboard real metrics and RBAC verification completed. Next: ADMIN-3.
