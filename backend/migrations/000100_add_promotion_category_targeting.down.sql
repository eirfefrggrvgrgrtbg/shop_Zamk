-- Migration 000100 down: Revert promotion category targeting

DROP TABLE IF EXISTS promo_code_category_targets;

ALTER TABLE promo_codes
    DROP CONSTRAINT IF EXISTS chk_pc_product_scope,
    ADD CONSTRAINT chk_pc_product_scope CHECK (product_scope IN ('ENTIRE_STORE', 'SELECTED_PRODUCTS'));
