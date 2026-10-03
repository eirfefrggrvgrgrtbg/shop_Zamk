-- Migration 000102: Evolve promo customer audience targeting
-- Down:
-- 1. Recreate legacy first_paid_order_only column
ALTER TABLE promo_codes
ADD COLUMN first_paid_order_only BOOLEAN NOT NULL DEFAULT false;

-- 2. Reverse backfill: FIRST_PAID_ORDER -> true, ALL_CUSTOMERS / REPEAT_CUSTOMERS -> false (lossy for REPEAT_CUSTOMERS)
UPDATE promo_codes
SET first_paid_order_only = (audience_type = 'FIRST_PAID_ORDER');

-- 3. Drop constraint and audience_type column
ALTER TABLE promo_codes
DROP CONSTRAINT IF EXISTS chk_promo_codes_audience_type;

ALTER TABLE promo_codes
DROP COLUMN audience_type;
