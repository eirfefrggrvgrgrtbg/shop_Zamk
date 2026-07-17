# Product Variants & Stock Flow

## Variant Merge Behavior
- When updating a product, `MergeProductVariants` checks existing variants.
- New variant IDs are inserted.
- Matching variant IDs are updated.
- Missing variant IDs (that previously existed) are soft-deleted (`is_active = false`).

## Ownership Protection
- `MergeProductVariants` ensures `UPDATE` and `DELETE` queries strictly scope to the current `product_id`.
- The product itself is verified for ownership in the service layer (`UpdateProductForSeller`).
- Cross-product variant injection is blocked (a seller cannot update another seller's variant ID by passing it in the array, as the `WHERE id = $X AND product_id = $Y` condition will fail to update anything).

## Duplicate Behavior
- Soft-delete handles removals.
- If a seller uploads identical SKU or size/color combos, it is handled via the front-end or accepted as distinct variants (currently, uniqueness is loosely enforced based on client-side logic, though `id` conflicts are rejected by Postgres).

## Attribute Support
- **Size**: Supported.
- **Color**: Supported (Known Gap: Complex color groupings/filtering might require dedicated entity mapping later).
- **SKU**: Supported (Known Gap: Not strictly verified for global uniqueness, left to the seller's domain).

## Moderation Reset
- Editing published or approved products (including variants, stock, sizes, colors, SKUs) immediately reverts the product status to `pending_moderation`.
- A single moderation log entry is created.
- No-op updates (updates that do not change variants or product fields in a way that requires re-moderation) are currently not strictly differentiated at the repository level, but the service explicitly applies the reset when `StatusPublished` or `StatusApproved` is encountered.

## Cart Identity
- Cart items merge if the exact same `product_variant_id` is selected.
- Different variants of the same product are maintained as separate line items.

## Stock Calculation
- Initial stock can be provided for new variants and is written to `inventory_items` once.
- Subsequent updates do not overwrite stock; stock operations are explicitly handled through the inventory/reservations flow.

## Reservation Locking & Concurrency
- `Promise.all` concurrency check verified that simultaneous requests for the final stock unit result in one success and one failure (`API Error 500: insufficient stock available`).
- Only 1 active reservation is created in such scenarios.
- `total - reserved AS available` correctly drops to 0. Negative stock is prevented.

## Order Snapshot
- At checkout, variant properties (ID, Size, Color, SKU, UnitPrice) are snapshotted into `order_items`.
- This snapshot survives later variant editing or soft-deletions by the seller.
- The snapshot is visible accurately in the Customer Order view, Seller Fulfillment view, and Admin view.

## Cancel / Sale / Return Lifecycle
- **Cancel**: Releases reserved stock (idempotent, no double release).
- **Sale**: Reduces total stock (idempotent, no double reduction).
- **Return Restock**: Restores stock once (Known Gap: Partial returns of variant items are handled as whole lines currently).
