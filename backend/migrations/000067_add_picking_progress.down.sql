ALTER TABLE order_items
DROP CONSTRAINT IF EXISTS valid_picked_quantity;

ALTER TABLE order_items
DROP COLUMN IF EXISTS picked_quantity;

ALTER TABLE order_item_allocations
DROP COLUMN IF EXISTS picked_at;
