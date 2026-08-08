ALTER TABLE payments
    ADD COLUMN payment_method TEXT NOT NULL DEFAULT 'unknown',
    ADD COLUMN integration_mode TEXT NOT NULL DEFAULT 'unknown';

ALTER TABLE payments
    ADD CONSTRAINT chk_payment_method CHECK (payment_method IN ('card', 'tpay', 'sbp', 'unknown')),
    ADD CONSTRAINT chk_integration_mode CHECK (integration_mode IN ('mock', 'quick_widget', 'hosted_form', 'unknown'));
