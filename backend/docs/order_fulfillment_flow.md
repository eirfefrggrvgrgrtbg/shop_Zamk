# Order & Fulfillment Flow (UX-4)

## Overview

This document describes the lifecycle of an order after checkout, including FBO fulfillment, shipment, and delivery. It covers the roles of Customer, Seller, and Admin, along with the allowed status transitions, API interactions, and verified inventory behavior.

## Status Definitions

### Order Statuses (Parent)
- **created**: Initial state immediately after checkout.
- **pending_payment**: Order is awaiting payment processing (or is unpaid).
- **paid**: Payment is successful.
- **processing**: Order is currently being handled by sellers.
- **shipped**: All items in the order have been shipped.
- **delivered**: All items in the order have been delivered to the customer.
- **cancelled**: Order was cancelled (by customer, admin, or automatically).

### Fulfillment Statuses (Seller-Scoped FBO Group)
- **awaiting_payment**: Initial state, waiting for the parent order to be paid.
- **paid**: Payment successful; the fulfillment is ready for ZAMK warehouse processing.
- **assembling**: ZAMK has started picking/assembling the items.
- **packed**: ZAMK has packed the items and is ready for shipment.
- **ready_to_ship**: Admin/Logistics has acknowledged the package is ready.
- **shipped**: The package has been handed over to the delivery carrier.
- **delivered**: The package reached the customer.
- **cancelled**: The fulfillment was cancelled.

### Shipment Statuses (Logistics)
- **pending**: Shipment created, waiting for dispatch.
- **assembling**: Shipment is being gathered.
- **packed**: Shipment is packed.
- **shipped**: Shipment is in transit.
- **delivered**: Shipment has been delivered to the customer.
- **cancelled**: Shipment was cancelled.

## Role-Based Flows & RBAC

### 1. Customer
**Permissions:** Can view their own orders and cancel them in early stages.
**Flow:**
- Views order history in `My Orders` (`apps/shop/src/pages/Orders.tsx`).
- Uses `GET /api/customer/orders` and `GET /api/customer/orders/{id}`.
- Can see granular fulfillment statuses per seller using `GET /api/customer/orders/{orderId}/fulfillments`.

### 2. Seller
**Permissions:** Can read their own order and fulfillment progress where exposed. `users.RoleSeller` is required.
**Flow:**
- Uses the Seller order read APIs currently exposed under `/api/seller/orders`.
- Does not pick, pack, create shipments, dispatch, or mutate fulfillment status.
- A dedicated Seller fulfillment read projection may be added later without granting fulfillment commands.

### 3. Admin
**Permissions:** Performs ZAMK warehouse fulfillment and logistics operations.
**Flow:**
- Uses `GET /api/admin/orders` and `GET /api/admin/order-fulfillments`.
- Performs receiving, picking, packing, and dispatch through the semantic Admin fulfillment routes.
- Creates a shipment for a fulfillment:
  - `POST /api/admin/fulfillments/{id}/shipment`
- Updates the shipment status as it moves through the delivery network:
  - `PATCH /api/admin/shipments/{id}/status` -> `shipped`, `delivered`
- The Shipment status updates cascade back to the `Fulfillment` status, which in turn cascades up to the parent `Order` status via `recalculateParentOrderStatusTx`.

## Inventory Behavior Verified in UX-4

1. **Cart:**
   - Adding item to cart does not change stock.

2. **Order creation / checkout initiation:**
   - Stock is not reserved immediately.
   - `reserved_stock` remains unchanged.

3. **Payment confirmation webhook:**
   - After successful TBank/STUB webhook, backend records a stock movement with type `sale`.
   - `total_stock` decreases by ordered quantity.
   - `available_stock` decreases by ordered quantity.
   - `reserved_stock` remains unchanged.

4. **Fulfillment statuses:**
   - `assembling`, `packed`, `shipped`, `delivered` do not change stock again.
   - Stock was already consumed at successful payment.

5. **Verified SQL example:**
   - before: total=10, reserved=0, available=10
   - after payment: total=9, reserved=0, available=9
   - after delivered: total=9, reserved=0, available=9

6. **Future work:**
   - If business decides to support long reservation before payment, implement a separate INV phase.
   - Current behavior is immediate sale consumption after payment success.

## Notifications

- **ZAMK packs a fulfillment**: Notification sent to Customer (`CustomerFulfillmentPacked`).
- Seller does not emit assembling or packed fulfillment mutation events.
- **Status Updates**: Customer is notified on `shipped` and `delivered` events via notification system.
