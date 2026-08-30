DROP TABLE IF EXISTS return_item_units;

ALTER TABLE return_items
DROP CONSTRAINT IF EXISTS check_inspection_sum,
DROP CONSTRAINT IF EXISTS valid_inspection_qtys,
DROP COLUMN IF EXISTS accepted_quantity,
DROP COLUMN IF EXISTS damaged_quantity,
DROP COLUMN IF EXISTS rejected_quantity;

ALTER TABLE returns
DROP CONSTRAINT IF EXISTS valid_return_status,
DROP CONSTRAINT IF EXISTS fk_returns_fulfillment_order,
DROP COLUMN IF EXISTS receiving_started_at,
DROP COLUMN IF EXISTS fulfillment_id;

ALTER TABLE order_fulfillments DROP CONSTRAINT IF EXISTS uq_order_fulfillments_id_order_id;
