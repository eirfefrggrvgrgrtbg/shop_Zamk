CREATE INDEX IF NOT EXISTS payments_created_at_idx ON payments(created_at DESC);
CREATE INDEX IF NOT EXISTS payments_provider_idx ON payments(provider);
CREATE INDEX IF NOT EXISTS payments_payment_method_idx ON payments(payment_method);

CREATE INDEX IF NOT EXISTS refunds_payment_id_idx ON refunds(payment_id);

