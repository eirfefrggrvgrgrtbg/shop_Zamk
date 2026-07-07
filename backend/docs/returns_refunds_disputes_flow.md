# Returns, Refunds, and Disputes Flow

## 1. Overview
The RET-1 phase implements the Returns and Refunds flow, allowing customers to return delivered items, and admins to process and refund those items while correctly adjusting the seller's balance (ledger) and optionally restocking inventory.

## 2. Roles & Responsibilities

### Customer
- Can create a return request (`requested`) for items in a `delivered` order within the return window.
- Can view their own returns and statuses.
- Can see if a return was approved, rejected, or refunded.
- Cannot view returns belonging to other customers.

### Seller
- Has read-only access to returns for their own products.
- Can see the status and admin decisions (e.g., refund reasons or rejection comments).
- Cannot directly approve returns, perform refunds, or manually alter ledger balances.
- Cannot view returns belonging to other sellers.

### Admin
- Has full view of all returns across the marketplace.
- Processes the return state machine: `requested` -> `approved` -> `item_received` -> `refunded` -> `completed`.
- Can reject returns with a mandatory reason.
- Creates the refund, which automatically interacts with the seller ledger and inventory restock.

## 3. Status Models

### Return Statuses
1. `requested` - Customer initiated the return.
2. `approved` - Admin approved the request. Customer should send the item back.
3. `rejected` - Admin rejected the request. Terminal state. Requires a comment.
4. `item_received` - Admin confirmed receipt of the physical item.
5. `refunded` - Refund has been issued.
6. `cancelled` - Customer or Admin cancelled the request.
7. `completed` - The return cycle is fully finished.

### Refund Statuses
- `succeeded` - The refund was successfully processed (in the current MVP, it jumps to succeeded as a stubbed provider).

## 4. Allowable Transitions (Admin)
- `requested` -> `approved`, `rejected`, `cancelled`
- `approved` -> `item_received`, `cancelled`
- `item_received` -> `refunded`, `completed`
- `refunded` -> `completed`

## 5. Balance & Ledger Behavior
When an order is delivered, the seller receives a `sale_pending` ledger entry for the **NET** amount (Gross - Commission).
When a return is approved and refunded:
- A `refund_deduction` ledger entry is created for the seller.
- **CRITICAL FIX**: The deduction must equal the **NET** amount the seller received, not the **GROSS** amount. Otherwise, the seller loses the commission out of pocket.
- Double refunds are blocked by the system checking total refunded amounts.

## 6. Inventory Behavior
- During the `UpdateReturnStatus` or `CreateRefund`, if `Restock: true` is provided by the admin, the inventory stock is incremented.
- A new stock movement of type `return` is recorded.
- Damaged/rejected returns do not automatically restock.

## 7. What constitutes completion of RET-1
- Fix `ProcessRefundDeduction` to subtract only the `net` amount from the seller's ledger.
- A fully passing E2E smoke test verifying the return lifecycle, RBAC checks, seller ledger adjustments, and inventory restock.
- Frontend UI cleanup (no "TODO", no broken layouts).
- Clean `git status`, successful `go build`/`npm run build`, and committed to `main`.
