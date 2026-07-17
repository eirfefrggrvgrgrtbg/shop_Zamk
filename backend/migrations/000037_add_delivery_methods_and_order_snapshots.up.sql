CREATE TABLE delivery_methods (
    id UUID PRIMARY KEY,
    code VARCHAR NOT NULL UNIQUE,
    name VARCHAR NOT NULL,
    description VARCHAR NULL,
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    estimated_days_min INTEGER NULL CHECK (estimated_days_min >= 0),
    estimated_days_max INTEGER NULL CHECK (estimated_days_max >= estimated_days_min),
    is_active BOOLEAN NOT NULL DEFAULT true,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed data for MVP delivery methods corresponding to existing UI
INSERT INTO delivery_methods (id, code, name, price_cents, estimated_days_min, estimated_days_max, sort_order) VALUES
('b31d2e5a-73d8-4f81-9b7e-9761e38de8f0', 'courier', 'Курьер 1-2 дня', 50000, 1, 2, 10),
('f1a238b1-3829-450f-a36c-2f3b9cd4ab57', 'pickup', 'Самовывоз', 0, 0, 0, 20),
('c531940b-72ea-4d0c-a9a3-5c2007f35ea9', 'express', 'Экспресс 3 часа', 100000, 0, 0, 30);

ALTER TABLE orders 
ADD COLUMN delivery_method_id UUID NULL REFERENCES delivery_methods(id),
ADD COLUMN delivery_method_code VARCHAR NULL,
ADD COLUMN delivery_method_name VARCHAR NULL,
ADD COLUMN delivery_price_cents BIGINT NULL,
ADD COLUMN delivery_estimated_days_min INTEGER NULL,
ADD COLUMN delivery_estimated_days_max INTEGER NULL,
ADD COLUMN checkout_idempotency_key UUID NULL;

CREATE UNIQUE INDEX IF NOT EXISTS orders_customer_id_idempotency_key_idx ON orders (user_id, checkout_idempotency_key) WHERE checkout_idempotency_key IS NOT NULL;
