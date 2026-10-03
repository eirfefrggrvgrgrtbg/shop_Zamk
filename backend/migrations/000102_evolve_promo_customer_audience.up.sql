-- Migration 000102: Evolve promo customer audience targeting
-- Up:
-- 1. Add audience_type column with default 'ALL_CUSTOMERS'
ALTER TABLE promo_codes
ADD COLUMN audience_type VARCHAR(32) NOT NULL DEFAULT 'ALL_CUSTOMERS';

-- 2. Backfill existing rows from first_paid_order_only
UPDATE promo_codes
SET audience_type = 'FIRST_PAID_ORDER'
WHERE first_paid_order_only = true;

-- 3. Add check constraint for supported audience types
ALTER TABLE promo_codes
ADD CONSTRAINT chk_promo_codes_audience_type
CHECK (audience_type IN ('ALL_CUSTOMERS', 'FIRST_PAID_ORDER', 'REPEAT_CUSTOMERS'));

-- 4. Drop legacy boolean column to ensure single source of truth
ALTER TABLE promo_codes
DROP COLUMN first_paid_order_only;
