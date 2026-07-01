-- Revert shipments
ALTER TABLE shipments
DROP COLUMN IF EXISTS fulfillment_id;

-- Revert order_items
ALTER TABLE order_items
DROP COLUMN IF EXISTS order_fulfillment_id;

-- Drop table (will drop its indexes and constraints)
DROP TABLE IF EXISTS order_fulfillments;
