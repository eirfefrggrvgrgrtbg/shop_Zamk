ALTER TABLE products
ADD COLUMN create_idempotency_key UUID NULL,
ADD COLUMN create_request_hash TEXT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS products_seller_id_create_idempotency_key_idx
ON products (seller_id, create_idempotency_key)
WHERE create_idempotency_key IS NOT NULL;
