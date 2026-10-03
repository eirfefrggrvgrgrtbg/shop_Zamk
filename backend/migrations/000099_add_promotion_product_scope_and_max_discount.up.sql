-- Migration 000099: Add promotion product scope, targets, and maximum discount cap
ALTER TABLE promo_codes
    ADD COLUMN product_scope TEXT NOT NULL DEFAULT 'ENTIRE_STORE',
    ADD COLUMN max_discount_cents BIGINT,
    ADD CONSTRAINT chk_pc_product_scope CHECK (product_scope IN ('ENTIRE_STORE', 'SELECTED_PRODUCTS')),
    ADD CONSTRAINT chk_pc_max_discount CHECK (max_discount_cents IS NULL OR max_discount_cents > 0);

CREATE TABLE promo_code_product_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promo_code_id UUID NOT NULL REFERENCES promo_codes(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_pcpt_target_type CHECK (target_type IN ('INCLUDE', 'EXCLUDE')),
    CONSTRAINT uq_promo_code_product_target UNIQUE (promo_code_id, product_id)
);

CREATE INDEX idx_pcpt_promo_code_id ON promo_code_product_targets(promo_code_id);
CREATE INDEX idx_pcpt_product_id ON promo_code_product_targets(product_id);
