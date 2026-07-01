-- =================================================================================
-- Fulfillment Data Integrity Audit Script
-- Phase C9 Hardening
-- This script safely counts potential inconsistencies in the new split-fulfillment
-- architecture. It does NOT output customer PII, only counts and UUIDs if grouped.
-- =================================================================================

-- 1. order_items with NULL order_fulfillment_id
SELECT count(*) AS null_fulfillment_id_in_items
FROM order_items
WHERE order_fulfillment_id IS NULL;

-- 2. order_items where order_fulfillment_id points to missing fulfillment
SELECT count(*) AS orphan_fulfillment_ids_in_items
FROM order_items oi
LEFT JOIN order_fulfillments of ON oi.order_fulfillment_id = of.id
WHERE oi.order_fulfillment_id IS NOT NULL AND of.id IS NULL;

-- 3. order_items where fulfillment.order_id != order_items.order_id
SELECT count(*) AS mismatched_order_id_in_items
FROM order_items oi
JOIN order_fulfillments of ON oi.order_fulfillment_id = of.id
WHERE oi.order_id != of.order_id;

-- 4. order_items where fulfillment.seller_id does not match the item seller
SELECT count(*) AS mismatched_seller_id_in_items
FROM order_items oi
JOIN order_fulfillments of ON oi.order_fulfillment_id = of.id
WHERE oi.seller_id != of.seller_id;

-- 5. order_fulfillments without items
SELECT count(*) AS empty_fulfillments
FROM order_fulfillments of
LEFT JOIN order_items oi ON of.id = oi.order_fulfillment_id
WHERE oi.id IS NULL;

-- 6. duplicate fulfillments for same order_id + seller_id
SELECT sum(duplicates) AS duplicate_fulfillments
FROM (
    SELECT count(*) - 1 AS duplicates
    FROM order_fulfillments
    GROUP BY order_id, seller_id
    HAVING count(*) > 1
) sub;

-- 7. fulfillment subtotal mismatch versus sum(order_items.line_total_cents)
-- Note: order_items.subtotal_price_cents is the item subtotal (price * quantity).
SELECT count(*) AS subtotal_mismatches
FROM order_fulfillments of
JOIN (
    SELECT order_fulfillment_id, sum(subtotal_price_cents) as calculated_total
    FROM order_items
    WHERE order_fulfillment_id IS NOT NULL
    GROUP BY order_fulfillment_id
) calc ON of.id = calc.order_fulfillment_id
WHERE of.subtotal_cents != calc.calculated_total;

-- 8. fulfillments with invalid status
SELECT count(*) AS invalid_status_fulfillments
FROM order_fulfillments
WHERE status NOT IN ('awaiting_payment', 'paid', 'assembling', 'packed', 'shipped', 'delivered', 'cancelled', 'returned', 'refunded');

-- 9. shipments with NULL fulfillment_id
SELECT count(*) AS legacy_null_shipments
FROM shipments
WHERE fulfillment_id IS NULL;

-- 10. shipments with fulfillment_id where shipments.order_id != fulfillment.order_id
SELECT count(*) AS mismatched_order_id_in_shipments
FROM shipments s
JOIN order_fulfillments of ON s.fulfillment_id = of.id
WHERE s.order_id != of.order_id;

-- 11. shipments with fulfillment_id pointing to missing fulfillment
SELECT count(*) AS orphan_fulfillment_ids_in_shipments
FROM shipments s
LEFT JOIN order_fulfillments of ON s.fulfillment_id = of.id
WHERE s.fulfillment_id IS NOT NULL AND of.id IS NULL;

-- 12. multiple shipments per fulfillment
SELECT sum(duplicates) AS duplicate_shipments_per_fulfillment
FROM (
    SELECT count(*) - 1 AS duplicates
    FROM shipments
    WHERE fulfillment_id IS NOT NULL
    GROUP BY fulfillment_id
    HAVING count(*) > 1
) sub;
