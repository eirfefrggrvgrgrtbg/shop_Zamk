-- =================================================================================
-- Safe Backfill: Shipments Fulfillment ID
-- Phase C10 Hardening
-- This script links legacy shipments (fulfillment_id IS NULL) to their corresponding
-- order_fulfillment, IF AND ONLY IF the order has exactly ONE fulfillment.
-- Ambiguous (multi-seller) orders are left untouched to prevent incorrect data.
-- =================================================================================

BEGIN;

-- 1. Identify safe shipments to backfill (exactly one fulfillment for the order)
WITH single_fulfillment_orders AS (
    SELECT order_id
    FROM order_fulfillments
    GROUP BY order_id
    HAVING count(*) = 1
),
safe_backfill_targets AS (
    SELECT s.id AS shipment_id, of.id AS fulfillment_id
    FROM shipments s
    JOIN single_fulfillment_orders sfo ON s.order_id = sfo.order_id
    JOIN order_fulfillments of ON sfo.order_id = of.order_id
    WHERE s.fulfillment_id IS NULL
)
UPDATE shipments s
SET fulfillment_id = t.fulfillment_id
FROM safe_backfill_targets t
WHERE s.id = t.shipment_id;

-- 2. Report on remaining ambiguous shipments
DO $$
DECLARE
    remaining_nulls INT;
BEGIN
    SELECT count(*) INTO remaining_nulls FROM shipments WHERE fulfillment_id IS NULL;
    RAISE NOTICE 'Backfill complete. Remaining ambiguous shipments with NULL fulfillment_id: %', remaining_nulls;
END $$;

COMMIT;
