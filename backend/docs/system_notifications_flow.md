# System Notifications Flow (NOTIF-1)

## Overview
This document describes the end-to-end notification system implemented for the ZAMK marketplace (NOTIF-1). The flow covers event-driven alerts delivered in-app to Customers, Sellers, and Admins across key marketplace lifecycles.

## Notification Triggers

### 1. Moderation & Catalog
- **Product Review Submitted:** When a seller creates a product, admins are notified.
- **Product Approved/Rejected:** When an admin reviews a product, the seller is notified of the outcome.

### 2. Orders & Checkout
- **New Order:** When a customer completes checkout and the payment is authorized, the seller is notified of a new order to fulfill.

### 3. Fulfillment
- **Status Updates:** When a seller packs or ships a fulfillment, the customer is notified.
- **Delivery:** When a shipment is delivered, the customer receives a final delivery notification.

### 4. Returns & Refunds
- **Return Created:** When a customer initiates a return, the seller and admin are notified.
- **Return Approved/Rejected:** When an admin resolves the return, the seller is updated.
- **Refund Issued:** When an admin triggers a refund for an order, the customer is notified.

### 5. Payouts
- **Payout Requested:** When a seller requests a payout, the admin is notified.
- **Payout Processed:** When an admin processes or rejects a payout, the seller is updated.

## Implementation Details

- **Database Structure:** Notifications are stored in the `notifications` table, with dedicated indexes for fast retrieval. `recipient_kind` isolates Admin, Seller, and Customer notification visibility.
- **RBAC & Security:** Strict access controls ensure users can only query, read, and mark their own notifications. A seller cannot read customer notifications, preventing data leaks.
- **Idempotency & Deduplication:** System prevents duplicate notifications for the same entity and type, avoiding webhook retry flooding (e.g., duplicate T-Bank payment webhooks).
- **Transaction Atomicity:** Notifications are published within the same `pgx.Tx` as their parent business logic (orders, fulfillments, returns) ensuring that failure to commit business logic rolls back the notification.

## Status
- **NOTIF-1:** **CLOSED** - All E2E smoke tests and validation checks passed.
