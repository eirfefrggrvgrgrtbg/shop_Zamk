-- MARKETING.M0.2: distinguish fulfillments whose commission rate was frozen at checkout (M0.1+)
-- from legacy rows whose commission_bps is only the column default / marketplace config rate.
-- No backfill: existing rows stay FALSE (legacy algorithm) until a Product Owner approved plan says otherwise.
ALTER TABLE order_fulfillments
    ADD COLUMN IF NOT EXISTS commission_frozen BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN order_fulfillments.commission_frozen IS
    'TRUE: commission_bps was resolved and frozen at checkout. FALSE: legacy row, rate chosen at delivery.';
