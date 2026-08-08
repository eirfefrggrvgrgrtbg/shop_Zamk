ALTER TABLE payments
    DROP CONSTRAINT chk_payment_method,
    DROP CONSTRAINT chk_integration_mode,
    DROP COLUMN payment_method,
    DROP COLUMN integration_mode;
