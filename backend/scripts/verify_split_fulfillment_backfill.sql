-- backend/scripts/verify_split_fulfillment_backfill.sql

SELECT 'Total Orders' AS metric, COUNT(*) AS value FROM orders
UNION ALL
SELECT 'Total Order Fulfillments', COUNT(*) FROM order_fulfillments
UNION ALL
SELECT 'Total Order Items', COUNT(*) FROM order_items
UNION ALL
SELECT 'Order Items with NULL fulfillment_id', COUNT(*) FROM order_items WHERE order_fulfillment_id IS NULL
UNION ALL
SELECT 'Total Shipments', COUNT(*) FROM shipments
UNION ALL
SELECT 'Shipments with NULL fulfillment_id', COUNT(*) FROM shipments WHERE fulfillment_id IS NULL
UNION ALL
SELECT 'Orders with multiple sellers', COUNT(*) FROM (
    SELECT order_id FROM order_items GROUP BY order_id HAVING COUNT(DISTINCT seller_id) > 1
) AS multi_seller_orders
UNION ALL
SELECT 'Duplicate fulfillments (order+seller)', COUNT(*) FROM (
    SELECT order_id, seller_id FROM order_fulfillments GROUP BY order_id, seller_id HAVING COUNT(*) > 1
) AS dupes
UNION ALL
SELECT 'Fulfillments with subtotal mismatch', COUNT(*) FROM (
    SELECT of.id, of.subtotal_cents, SUM(oi.subtotal_price_cents) AS item_sum
    FROM order_fulfillments of
    JOIN order_items oi ON oi.order_fulfillment_id = of.id
    GROUP BY of.id, of.subtotal_cents
    HAVING of.subtotal_cents != SUM(oi.subtotal_price_cents)
) AS mismatches;
