# Split Fulfillment Architecture Design

## 1. Current Model Limitations
* **Orders**: The `orders` table tracks a single global status (e.g., `awaiting_payment`, `paid`, `assembling`, `shipped`, `delivered`) for the entire purchase.
* **Shipments**: The `shipments` table links directly to the parent `orders` via `order_id`. There is currently only one shipment per order.
* **Problem**: A multi-seller order breaks this global status model. Sellers cannot independently manage their portion of the order (e.g., mark their items as `packed`). If an order contains items from two sellers, both items share the same shipment and global order status, creating administrative bottlenecks and preventing accurate status reflections per seller.

## 2. Recommended DB Model
To resolve this, we will introduce a seller-level fulfillment abstraction.
* **`orders`**: Represents the parent buyer order and serves as the financial/checkout boundary.
* **`order_items`**: Individual purchased items.
* **`order_fulfillments`**: New table acting as a fulfillment grouping per seller inside a parent order.

**Proposed Table: `order_fulfillments`**
```sql
CREATE TABLE order_fulfillments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL REFERENCES orders(id),
    seller_id UUID NOT NULL REFERENCES sellers(id),
    status TEXT NOT NULL,
    subtotal_cents BIGINT NOT NULL DEFAULT 0,
    commission_bps INT NOT NULL DEFAULT 900,
    seller_amount_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Relation Recommendation**: `order_items` should add an `order_fulfillment_id UUID` column that references `order_fulfillments(id)`.
*Reasoning*: This is the safer and more explicit approach. It establishes strict relational integrity, preventing items from floating or being accidentally reassigned. It avoids complex queries derived only by `order_id` + `seller_id`, reducing logical ambiguity in edge cases.

## 3. Shipment Model Decision
When upgrading shipments for multi-seller orders, there are two primary options:
* **Option A**: Keep `shipments.order_id`, and add a `seller_id` column.
  * *Pros*: Simple to implement.
  * *Cons*: Redundant data. Creates duplicate state models if we also implement `order_fulfillments`, leading to potential orphan shipments.
* **Option B**: Replace `shipments.order_id` with `shipments.fulfillment_id` referencing `order_fulfillments(id)`.
  * *Pros*: Strictly enforces that a shipment belongs to a specific seller's package.
  * *Cons*: Requires migrating existing shipments.

**Recommendation**: **Option B**. It provides exact control for each seller's dispatch process and guarantees 1-to-1 or 1-to-N relationships explicitly tied to a seller's fulfillment group, which is crucial for production tracking.

## 4. Status Model
* **Fulfillment Statuses (per seller)**: `awaiting_payment`, `paid`, `assembling`, `packed`, `shipped`, `delivered`, `cancelled`, `returned`, `refunded`.
* **Parent Order Statuses**: 
  Derived from the aggregation of its fulfillments to avoid overcomplicating the MVP with partial statuses:
  * `awaiting_payment`: Payment intent created/pending.
  * `paid`: Payment received, fulfillments are in `paid`.
  * `assembling`: If *at least one* fulfillment group is `assembling` or `packed`.
  * `shipped`: If *all* active fulfillment groups are `shipped`.
  * `delivered`: If *all* active fulfillment groups are `delivered`.

## 5. Seller Permissions/Actions Design
* **Allowed**: Sellers can view ONLY their own `order_fulfillments`. They are permitted to update their fulfillment status to `assembling` and `packed/ready`.
* **Denied**: Sellers cannot change parent order statuses globally, mark as `paid`/`shipped`/`delivered` directly (unless business policy explicitly delegates shipment finalization), or access other sellers' fulfillments or buyer payment intents. Admin or automated courier logic controls `shipped` and `delivered`.

## 6. Buyer UI Impact
* **Orders Drawer / Details**: The buyer continues to see a single parent order. However, the items inside are visually grouped by seller. Each seller group displays its own specific `fulfillment status` and shipment details (if it exists). 
* **Tracking**: No fake tracking numbers; only real courier integrations or empty states will be shown per group.

## 7. Admin UI Impact
* **AdminOrders**: The overview shows the parent order. Inside, the admin sees the seller groups and the status of each group. Shipments are created *per seller group*, not per order.
* **AdminShipments**: Lists shipments linked to `fulfillments`, allowing filters by seller, order, or fulfillment status.

## 8. Seller UI Impact
* **SellerOrders**: The list will now render `order_fulfillments` instead of raw parent orders. It shows only items belonging to the seller, alongside the buyer's delivery address. It provides UI buttons to transition the fulfillment to `assembling` and `packed`.

## 9. Returns and Refunds Impact
* **Current State**: Returns act on `order_items`. This architecture inherently supports multi-seller splitting because items are explicitly mapped to sellers and their specific fulfillments.
* **Impact**: Return/Refund logic requires minimal DB changes. However, when a refund occurs, the system must accurately process the `seller_balance_ledger` deductions against the specific `order_fulfillment` metrics to ensure the correct commission and net payouts are reversed.

## 10. Payment and Payouts Impact
* **Payments**: The buyer payment remains a single transaction (one intent) for the parent order.
* **Payouts**: The `seller_balance_ledger` currently references `order_item_id`. This is already well-suited for multi-seller payouts. The platform splits seller amounts internally on an item basis. 
* **Gaps**: No major gaps. The ledger can continue functioning item-by-item, successfully calculating seller-specific payouts.

## 11. Migration Strategy
To ensure a safe rollout without downtime:
* **Step 1**: Create the `order_fulfillments` table. Add `order_fulfillment_id` to `order_items` (nullable). Add `fulfillment_id` to `shipments` (nullable).
* **Step 2**: Backfill script: Group existing `order_items` by `(order_id, seller_id)`. Insert a row into `order_fulfillments` for each unique group. Update `order_items.order_fulfillment_id` and `shipments.fulfillment_id`.
* **Step 3**: Switch backend APIs and Seller/Admin endpoints to read/write using `order_fulfillments`.
* **Step 4**: Update `shipments.fulfillment_id` and `order_items.order_fulfillment_id` to `NOT NULL`. Safely drop old columns (e.g., `shipments.order_id`).
* **Step 5**: Deploy Admin, Seller, and Buyer UI updates to consume the new grouped structures.
* **Step 6**: Enable seller actions (`mark_packed`, `mark_assembling`).
* **Risks**: Potential data corruption during backfill if existing orders have orphaned items or ambiguous shipment states. A dry-run migration on a staging replica is highly recommended.

## 12. API Design Plan

**Seller API**:
* `GET /api/seller/fulfillments`
* `GET /api/seller/fulfillments/{id}`
* `POST /api/seller/fulfillments/{id}/mark-assembling`
* `POST /api/seller/fulfillments/{id}/mark-packed`

**Admin API**:
* `GET /api/admin/order-fulfillments`
* `GET /api/admin/orders/{id}/fulfillments`
* `POST /api/admin/fulfillments/{id}/shipment`
* `PATCH /api/admin/fulfillments/{id}/status`

**Customer API**:
* `GET /api/customer/orders` (Response payload heavily modified to include fulfillments grouped by seller).

## 13. Audit Logging Design
* **Actions Logged**: `fulfillment.status_update`, `fulfillment.mark_packed`, `fulfillment.shipment_create`, `shipment.status_update`.
* **Metadata**: Will store identifiers and from/to statuses exclusively. No sensitive customer delivery addresses, emails, or phone numbers will be captured in audit metadata to comply with privacy policies.

## 14. Implementation Phases
* **Phase B1 (Design)**: Architecture plan created.
* **Phase C1 (Schema & Backfill)**: Database schema migration added (`order_fulfillments`). `order_items` and `shipments` relations added as nullable. Backfill strategy executed via SQL.
* **Phase C2 (API Read Layer)**: Read endpoints added for seller and admin.
* **Phase C3 (Seller Fulfillments UI)**: Migrated `SellerOrders` to `order-fulfillments` APIs.
* **Phase C4 (Admin/Buyer Read Fulfillments UI)**: Migrated `AdminOrders` UI to group by fulfillments.
* **Phase C5 (Buyer Order Drawer Fulfillments UI)**: Migrated `Orders.tsx` UI to group by fulfillments.
* [x] Phase C6: Shipment Writes by Fulfillment ID.
* [x] Phase C7: Seller Fulfillment Actions (mark assembling/packed). Legacy `order_id` remains for compatibility. `NOT NULL` enforcement remains future work.
