# Customer Checkout and Order Flow

## Overview
This document describes the customer cart, checkout, order lifecycle, and fulfillment mechanisms in Zamk.

## Roles
- **Customer**: Browses catalog, adds products to cart, and completes checkout to create an order. Can view their own orders.
- **Seller**: Receives orders for their products. Processes orders (mark assembling, packed) and ships them.
- **Admin**: Has global visibility over all orders across the platform.

## Public Product Visibility
Only products with `status = 'published'` are visible in the public catalog and available for purchase. Drafts, pending, or rejected products are excluded from `/api/public/products`.

## Cart Behavior
- The cart belongs to an authenticated `customer`.
- Adding to cart requires checking if the `variant` is active, the `product` is published, and `availableStock` is sufficient.
- The `availableStock` is computed as `totalStock - reservedStock`.
- Empty carts are gracefully handled in the UI.

## Checkout Behavior
- The customer provides their shipping details (name, email, phone, address).
- Backend creates an `Order`.
- The system automatically triggers an `InventoryReservation` for the ordered items.
- The reservation lasts until the payment completes or the order is cancelled. This prevents overselling.
- Once created, the order status is `awaiting_payment`.

## Payment MVP Behavior
- The system integrates a mocked TBANK provider using a `STUB` terminal key.
- Upon calling `POST /orders/{id}/payment`, the backend returns a mock payment URL (`https://stub.payment.url/pay/{pid}`).
- In a real environment, the payment gateway sends a webhook to mark the order as `paid`.

## Order Statuses
- `awaiting_payment`: Initial state.
- `paid`: Payment received.
- `cancelled`: Cancelled by user or admin, or payment timeout.
- `refunded`: Returned.

## Seller Order Flow (Fulfillments)
- Sellers do not interact with the *Order* entity directly, but rather through *Order Fulfillments*.
- A single order may contain items from multiple sellers. Each seller gets their own `Fulfillment`.
- Seller views their fulfillments and updates the status (`assembling`, `packed`, `shipped`).
- The overall order status automatically advances if all fulfillments are processed.

## Admin Order Flow
- Admins have read access to all orders and fulfillments.
- Admins can forcefully cancel or update order statuses if needed.

## Security & RBAC
- `customer` role is strictly required for cart and checkout.
- Customers cannot view orders not belonging to them.
- `seller` role is required for managing fulfillments. Sellers cannot see fulfillments belonging to other sellers.
- `admin` role is required for global order view.
