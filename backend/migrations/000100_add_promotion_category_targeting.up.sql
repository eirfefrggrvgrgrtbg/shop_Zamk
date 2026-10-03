-- Migration 000100: Add promotion category targeting and category targets table

-- Update check constraint on promo_codes to allow SELECTED_CATEGORIES
ALTER TABLE promo_codes
    DROP CONSTRAINT chk_pc_product_scope,
    ADD CONSTRAINT chk_pc_product_scope CHECK (product_scope IN ('ENTIRE_STORE', 'SELECTED_PRODUCTS', 'SELECTED_CATEGORIES'));

CREATE TABLE promo_code_category_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promo_code_id UUID NOT NULL REFERENCES promo_codes(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_pcct_target_type CHECK (target_type IN ('INCLUDE', 'EXCLUDE')),
    CONSTRAINT uq_promo_code_category_target UNIQUE (promo_code_id, category_id)
);

CREATE INDEX idx_pcct_promo_code_id ON promo_code_category_targets(promo_code_id);
CREATE INDEX idx_pcct_category_id ON promo_code_category_targets(category_id);
