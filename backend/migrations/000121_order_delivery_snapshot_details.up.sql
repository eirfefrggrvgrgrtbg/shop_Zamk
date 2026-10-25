-- SHOP.DELIVERY.DETAILS.1: the order keeps its own copy of the postal code and the courier comment the customer gave
-- at checkout, next to delivery_address / customer_name / customer_phone. Additive and nullable: existing orders keep
-- NULL (no personal data is rewritten) and stay readable as before.
ALTER TABLE orders
    ADD COLUMN delivery_postal_code TEXT,
    ADD COLUMN delivery_courier_comment TEXT;

ALTER TABLE orders
    ADD CONSTRAINT orders_delivery_postal_code_len CHECK (delivery_postal_code IS NULL OR char_length(delivery_postal_code) BETWEEN 1 AND 12),
    ADD CONSTRAINT orders_delivery_courier_comment_len CHECK (delivery_courier_comment IS NULL OR char_length(delivery_courier_comment) BETWEEN 1 AND 500);
