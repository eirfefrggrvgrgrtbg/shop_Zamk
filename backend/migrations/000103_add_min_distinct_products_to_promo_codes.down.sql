-- Migration 000103 down: Revert minimum distinct products on promo_codes
ALTER TABLE promo_codes
    DROP CONSTRAINT IF EXISTS chk_promo_codes_min_distinct_products,
    DROP COLUMN IF EXISTS min_distinct_products;
