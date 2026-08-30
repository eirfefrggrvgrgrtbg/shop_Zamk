BEGIN;

ALTER TABLE returns DROP CONSTRAINT IF EXISTS valid_return_status;
ALTER TABLE returns ADD CONSTRAINT valid_return_status CHECK (status IN ('requested', 'approved', 'receiving', 'item_received', 'refunded', 'completed', 'rejected', 'cancelled'));

DROP TABLE IF EXISTS return_messages;

COMMIT;
