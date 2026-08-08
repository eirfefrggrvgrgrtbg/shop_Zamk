-- Sequence for payment numbers (PAY-000001)
CREATE SEQUENCE IF NOT EXISTS payment_number_seq START 1;

-- Add payment_number column
ALTER TABLE payments ADD COLUMN payment_number TEXT;

-- Backfill existing rows
UPDATE payments SET payment_number = 'PAY-' || LPAD(nextval('payment_number_seq')::text, 6, '0') WHERE payment_number IS NULL;

-- Enforce NOT NULL and UNIQUE on payment_number
ALTER TABLE payments ALTER COLUMN payment_number SET NOT NULL;
ALTER TABLE payments ADD CONSTRAINT payments_payment_number_unique UNIQUE (payment_number);

-- Add partial unique index to ensure only one active payment per order
CREATE UNIQUE INDEX idx_payments_active_per_order
ON payments (order_id)
WHERE status IN ('created', 'pending');
