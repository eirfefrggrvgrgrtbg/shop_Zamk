-- Migration 000101: Add minimum eligible item quantity to promo_codes
ALTER TABLE promo_codes
    ADD COLUMN min_eligible_quantity INT,
    ADD CONSTRAINT chk_pc_min_eligible_quantity CHECK (min_eligible_quantity IS NULL OR min_eligible_quantity > 0);
