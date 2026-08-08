ALTER TABLE payments
ALTER COLUMN payment_number DROP DEFAULT;

DROP INDEX IF EXISTS idx_payment_events_provider_key;

ALTER TABLE payment_events
DROP COLUMN IF EXISTS event_key;
