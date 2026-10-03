-- Migration 000101 down: Revert minimum eligible item quantity on promo_codes
ALTER TABLE promo_codes
    DROP CONSTRAINT IF EXISTS chk_pc_min_eligible_quantity,
    DROP COLUMN IF EXISTS min_eligible_quantity;
