ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS orders_delivery_courier_comment_len,
    DROP CONSTRAINT IF EXISTS orders_delivery_postal_code_len,
    DROP COLUMN IF EXISTS delivery_courier_comment,
    DROP COLUMN IF EXISTS delivery_postal_code;
