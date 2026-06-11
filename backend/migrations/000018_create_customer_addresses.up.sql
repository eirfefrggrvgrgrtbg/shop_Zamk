CREATE TABLE customer_addresses (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label TEXT,
    recipient_name TEXT NOT NULL,
    phone TEXT NOT NULL,
    city TEXT NOT NULL,
    street TEXT NOT NULL,
    house TEXT NOT NULL,
    apartment TEXT,
    postal_code TEXT,
    comment TEXT,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_customer_addresses_user_id ON customer_addresses(user_id);
CREATE UNIQUE INDEX idx_customer_addresses_default ON customer_addresses(user_id) WHERE is_default = true;
