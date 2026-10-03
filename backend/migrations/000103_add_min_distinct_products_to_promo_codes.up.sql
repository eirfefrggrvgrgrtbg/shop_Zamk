-- Migration 000103: Add minimum distinct products condition to promo_codes
ALTER TABLE promo_codes
    ADD COLUMN min_distinct_products INT,
    ADD CONSTRAINT chk_promo_codes_min_distinct_products CHECK (min_distinct_products IS NULL OR min_distinct_products >= 1);
