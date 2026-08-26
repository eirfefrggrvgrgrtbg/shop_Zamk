ALTER TABLE order_item_allocations
ADD COLUMN IF NOT EXISTS picked_at TIMESTAMPTZ NULL;

ALTER TABLE order_items
ADD COLUMN IF NOT EXISTS picked_quantity INTEGER NOT NULL DEFAULT 0;

ALTER TABLE order_items
ADD CONSTRAINT valid_picked_quantity CHECK (picked_quantity >= 0 AND picked_quantity <= quantity);
