DO $$ 
BEGIN
    -- Fail loudly if there are any order_items with NULL order_fulfillment_id
    IF EXISTS (
        SELECT 1 
        FROM order_items 
        WHERE order_fulfillment_id IS NULL
    ) THEN
        RAISE EXCEPTION 'Cannot enforce NOT NULL on order_items.order_fulfillment_id: NULL values exist.';
    END IF;
END $$;

ALTER TABLE order_items 
ALTER COLUMN order_fulfillment_id SET NOT NULL;
