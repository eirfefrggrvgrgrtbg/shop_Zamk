CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. payment_events table
ALTER TABLE payment_events
ADD COLUMN IF NOT EXISTS event_key TEXT;

UPDATE payment_events SET event_key = id::text WHERE event_key IS NULL;

ALTER TABLE payment_events
ALTER COLUMN provider SET NOT NULL,
ALTER COLUMN event_key SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_events_provider_key ON payment_events(provider, event_key);

-- 2. payment_number
ALTER TABLE payments
ALTER COLUMN payment_number SET DEFAULT ('PAY-' || LPAD(nextval('payment_number_seq')::text, 6, '0'));

WITH ranked_payments AS (
  SELECT id, row_number() OVER (ORDER BY created_at ASC, id ASC) as rn
  FROM payments
)
UPDATE payments p
SET payment_number = 'PAY-' || LPAD(rp.rn::text, 6, '0')
FROM ranked_payments rp
WHERE p.id = rp.id AND (p.payment_number LIKE 'PAY-%' AND length(p.payment_number) > 10);

WITH payment_number_state AS (
  SELECT
    MAX(SUBSTRING(payment_number FROM 5)::bigint) AS max_value
  FROM payments
  WHERE payment_number ~ '^PAY-[0-9]+$'
)
SELECT setval(
  'payment_number_seq',
  COALESCE(max_value, 1),
  max_value IS NOT NULL
)
FROM payment_number_state;
