ALTER TABLE marketing_campaigns ALTER COLUMN seller_id DROP NOT NULL;
ALTER TABLE promo_codes ALTER COLUMN seller_id DROP NOT NULL;

ALTER TABLE marketing_campaigns ADD COLUMN campaign_channel TEXT;
ALTER TABLE marketing_campaigns ADD COLUMN campaign_type TEXT;

CREATE TABLE campaign_tracking_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES marketing_campaigns(id) ON DELETE RESTRICT,
    token VARCHAR NOT NULL,
    target_type TEXT NOT NULL,
    target_product_id UUID REFERENCES products(id) ON DELETE RESTRICT,
    target_seller_id UUID REFERENCES sellers(id) ON DELETE RESTRICT,
    target_url TEXT,
    promo_code_id UUID REFERENCES promo_codes(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_ctl_target_type CHECK (target_type IN ('product', 'seller', 'landing')),
    CONSTRAINT chk_ctl_target_product CHECK (target_type != 'product' OR target_product_id IS NOT NULL),
    CONSTRAINT chk_ctl_target_seller CHECK (target_type != 'seller' OR target_seller_id IS NOT NULL),
    CONSTRAINT chk_ctl_target_landing CHECK (target_type != 'landing' OR target_url IS NOT NULL),
    CONSTRAINT uq_ctl_token UNIQUE (token)
);

CREATE INDEX idx_campaign_tracking_links_campaign_id ON campaign_tracking_links(campaign_id);
CREATE INDEX idx_campaign_tracking_links_token ON campaign_tracking_links(token);
