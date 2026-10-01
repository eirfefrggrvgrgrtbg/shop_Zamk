CREATE TABLE product_price_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    product_variant_id UUID REFERENCES product_variants(id) ON DELETE SET NULL,
    old_price_cents BIGINT NOT NULL CHECK (old_price_cents >= 0),
    new_price_cents BIGINT NOT NULL CHECK (new_price_cents >= 0),
    changed_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    source TEXT NOT NULL DEFAULT 'seller',
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_pph_source CHECK (source IN ('seller', 'admin', 'system', 'import'))
);

CREATE INDEX idx_product_price_history_product ON product_price_history(product_id, created_at DESC);
CREATE INDEX idx_product_price_history_variant ON product_price_history(product_variant_id, created_at DESC);
