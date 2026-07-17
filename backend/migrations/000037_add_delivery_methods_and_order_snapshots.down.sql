DROP INDEX IF EXISTS orders_customer_id_idempotency_key_idx;

ALTER TABLE orders 
DROP COLUMN IF EXISTS checkout_idempotency_key,
DROP COLUMN IF EXISTS delivery_estimated_days_max,
DROP COLUMN IF EXISTS delivery_estimated_days_min,
DROP COLUMN IF EXISTS delivery_price_cents,
DROP COLUMN IF EXISTS delivery_method_name,
DROP COLUMN IF EXISTS delivery_method_code,
DROP COLUMN IF EXISTS delivery_method_id;

DROP TABLE IF EXISTS delivery_methods;
