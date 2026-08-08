DROP INDEX IF EXISTS idx_payments_active_per_order;

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_payment_number_unique;
ALTER TABLE payments DROP COLUMN IF EXISTS payment_number;

DROP SEQUENCE IF EXISTS payment_number_seq;
