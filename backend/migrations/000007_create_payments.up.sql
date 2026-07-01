CREATE TABLE payments (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL,
    provider_payment_id TEXT,
    status TEXT NOT NULL,
    amount_cents BIGINT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'RUB',
    payment_url TEXT,
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    CONSTRAINT valid_provider CHECK (provider IN ('tbank', 'yookassa_stub')),
    CONSTRAINT valid_payment_status CHECK (status IN ('created', 'pending', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT valid_amount CHECK (amount_cents >= 0),
    CONSTRAINT valid_currency CHECK (currency = 'RUB')
);

CREATE TABLE payment_events (
    id UUID PRIMARY KEY,
    payment_id UUID REFERENCES payments(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    provider_payment_id TEXT,
    event_type TEXT NOT NULL,
    raw_payload JSONB NOT NULL,
    signature_valid BOOLEAN NOT NULL DEFAULT false,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
