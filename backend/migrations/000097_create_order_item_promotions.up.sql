CREATE TABLE order_item_promotions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_item_id UUID NOT NULL UNIQUE REFERENCES order_items(id) ON DELETE RESTRICT,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    campaign_id UUID NOT NULL REFERENCES marketing_campaigns(id) ON DELETE RESTRICT,
    promo_code_id UUID NOT NULL REFERENCES promo_codes(id) ON DELETE RESTRICT,
    base_unit_price_cents BIGINT NOT NULL CHECK (base_unit_price_cents >= 0),
    seller_discount_unit_cents BIGINT NOT NULL DEFAULT 0 CHECK (seller_discount_unit_cents >= 0),
    zamk_subsidy_unit_cents BIGINT NOT NULL DEFAULT 0 CHECK (zamk_subsidy_unit_cents >= 0),
    customer_paid_unit_price_cents BIGINT NOT NULL CHECK (customer_paid_unit_price_cents >= 0),
    commission_base_unit_cents BIGINT NOT NULL CHECK (commission_base_unit_cents >= 0),
    quantity INT NOT NULL CHECK (quantity > 0),
    total_seller_discount_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_seller_discount_cents >= 0),
    total_zamk_subsidy_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_zamk_subsidy_cents >= 0),
    total_customer_paid_cents BIGINT NOT NULL CHECK (total_customer_paid_cents >= 0),
    total_commission_base_cents BIGINT NOT NULL CHECK (total_commission_base_cents >= 0),
    commission_rate_bps INT NOT NULL CHECK (commission_rate_bps >= 0 AND commission_rate_bps <= 10000),
    total_commission_charged_cents BIGINT NOT NULL CHECK (total_commission_charged_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_oip_cust_unit_price CHECK (customer_paid_unit_price_cents = base_unit_price_cents - seller_discount_unit_cents - zamk_subsidy_unit_cents),
    CONSTRAINT chk_oip_comm_base_unit CHECK (commission_base_unit_cents = base_unit_price_cents - seller_discount_unit_cents),
    CONSTRAINT chk_oip_total_seller_discount CHECK (total_seller_discount_cents = seller_discount_unit_cents * quantity),
    CONSTRAINT chk_oip_total_zamk_subsidy CHECK (total_zamk_subsidy_cents = zamk_subsidy_unit_cents * quantity),
    CONSTRAINT chk_oip_total_customer_paid CHECK (total_customer_paid_cents = customer_paid_unit_price_cents * quantity),
    CONSTRAINT chk_oip_total_comm_base CHECK (total_commission_base_cents = commission_base_unit_cents * quantity)
);

CREATE INDEX idx_order_item_promotions_order ON order_item_promotions(order_id);
CREATE INDEX idx_order_item_promotions_campaign ON order_item_promotions(campaign_id);
CREATE INDEX idx_order_item_promotions_seller ON order_item_promotions(seller_id);
