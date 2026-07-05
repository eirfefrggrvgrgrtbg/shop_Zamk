# Admin Panel Audit (ADMIN-1)

This document maps out the current state of the admin panel as of Phase ADMIN-1. It highlights missing features, UX gaps, and the readiness of each admin section.

## 1. Admin Route Map
- `/dashboard`: Overview metrics (Sellers, Products, Moderation, Orders).
- `/users`: Global users list.
- `/staff`: RBAC management (Staff users).
- `/roles`: RBAC management (Roles/Permissions).
- `/sellers`: Seller management, statuses, warnings, violations.
- `/catalog`: Management for Categories and Brands.
- `/products`: Product list.
- `/moderation`: Product moderation queue.
- `/orders`: Order management, status updates, fulfillment creation.
- `/shipments`: Shipment tracking and manual creation.
- `/inventory`: Inventory tracking, receipts, and write-offs.
- `/returns`: Return requests.
- `/refunds`: Refund tracking.
- `/payouts`: Seller payouts tracking.
- `/auctions`: Auctions management.
- `/reviews`: Review moderation.
- `/audit-logs`: Audit logs of admin actions.
- `/settings`: Current admin profile view.

## 2. Section-by-Section Status
| Section | Status | Notes |
|---|---|---|
| Dashboard | Ready | Uses client-side aggregation. Needs backend aggregated endpoints for scaling. |
| Users | Ready | Displays paginated real user list, connected to backend. |
| Staff/RBAC | Ready | Functional RBAC management with users.read permission. |
| Sellers | Ready | Good functionality but needs fixing for Russian encoding glitches in text. |
| Products | Ready | Standard CRUD. |
| Product Moderation | Ready | Fully connected to backend. |
| Orders | Ready | Handles normal and auction orders. Needs HelpTooltips. |
| Shipments | Partially ready | Creation is currently a manual workaround form. Should be embedded in the Order panel. |
| Inventory | Ready | Allows receipt, adjustment, and write-off. Connected to backend. |
| Payouts | Ready | Fully connected. Needs HelpTooltips for commissions. |
| Notifications | Missing UI | `notifications` components exist, but no dedicated page or route. |
| Logs/Audit | Ready | Fully connected. |
| Auctions | Ready | Fully implemented MVP. |
| Direct-sale (ZAMK) | Ready | Handled organically via products/catalog. |
| Settings | Partially ready | Only displays profile. Settings logic is not yet implemented. |

## 3. Fake/Demo Data Inventory
- **No hardcoded mock data arrays found.** 
- The application natively connects to backend APIs (e.g. `getAdminOrders`, `getAdminSellers`). 
- A temporary workaround exists in `AdminShipments.tsx` (a manual form to create a shipment outside the order flow).

## 4. Missing Backend Endpoints
- `GET /api/admin/metrics/dashboard` (Aggregation endpoint)
- Proper system Settings endpoint (commissions, maintenance intervals)

## 5. Missing Frontend Pages
- Dedicated **Notifications** management page.
- Seller "Cabinet" (Out of scope for ADMIN-1).

## 6. RBAC Gaps
- `AdminUsers.tsx` lacks a permission guard on the internal UI (route is not explicitly guarded in `App.tsx`, it only relies on `<AdminProtectedRoute>`).
- Platform seller "ZAMK" is not explicitly protected from blocking/archiving in the frontend seller list.

## 7. UX Gaps
- **Encoding Issues**: `AdminSellers.tsx` has broken Cyrillic strings (e.g., `'?жидае? ак?ива?ии'`).
- **Loading/Error States**: Consistently implemented in `AdminOrders`, `AdminInventory`, etc., but lack universal wrapper components. 
- **Shipments UX**: "Создание отгрузки вручную — временное решение."
- **HelpTooltips**: Completely missing across the board (except from Auction routes).

## 8. Help Tooltip Requirements
Every unclear term must have a small `?` icon (e.g. using `HelpTooltip.tsx`).
Terms needing tooltips:
- RBAC, роль, разрешение
- модерация, статус товара
- резерв, остаток, списание
- выплата, комиссия, возврат
- fulfillment, shipment
- audit log, payout
- ручное решение, прямая продажа

## 9. Storage/Inventory Audit
- **Backend**: Implements `inventory_items`, `stock_movements`.
- **Admin UI**: `AdminInventory.tsx` allows admins to see stock, add receipts, adjust, write off, and view history.
- **Risks**: No explicit warehouse location logic. Mixing platform direct-sale and third-party seller items in the same physical view without distinct filters.
- **Next Phase**: ADMIN-7 should introduce warehouse locations if required, and add clear filters.

## 10. Orders/Fulfillment Audit
- Admins can manage catalog orders and auction orders. 
- Status updates respect safe targets.
- Fulfillment logic maps correctly to items.
- **Risk**: Creating shipments manually by ID in `AdminShipments.tsx` is prone to human error. It must be moved to `AdminOrders.tsx` detail panel.

## 11. Seller Management Audit
- **Works**: Listing, status updates, warnings, violations.
- **Missing**: Platform seller "ZAMK" is treated as a normal seller and could accidentally be blocked by a careless admin. Needs strict frontend guard.
- **Broken**: Text encoding in `AdminSellers.tsx`.

## 12. Auction/Direct-sale Admin Status
- Completed MVP. Verified through AUC-1 to AUC-7.

## 13. Recommended Implementation Phases
See `admin_next_steps.md` for the detailed phase breakdown (ADMIN-2 to ADMIN-10).

- ADMIN-3/ADMIN-3B: Users, Staff, and RBAC runtime verification completed. Next: ADMIN-4 Seller Management.
