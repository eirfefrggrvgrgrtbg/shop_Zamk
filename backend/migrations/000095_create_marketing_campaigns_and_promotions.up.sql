CREATE TABLE marketing_campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    description TEXT,
    funding_mode TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    discount_type TEXT NOT NULL DEFAULT 'percent',
    seller_discount_bps INT NOT NULL DEFAULT 0,
    seller_discount_fixed_cents BIGINT NOT NULL DEFAULT 0,
    requested_zamk_share_bps INT NOT NULL DEFAULT 0,
    requested_zamk_budget_cap_cents BIGINT NOT NULL DEFAULT 0,
    approved_zamk_share_bps INT NOT NULL DEFAULT 0,
    approved_zamk_budget_cap_cents BIGINT NOT NULL DEFAULT 0,
    zamk_reserved_cents BIGINT NOT NULL DEFAULT 0,
    zamk_spent_cents BIGINT NOT NULL DEFAULT 0,
    rejection_reason TEXT,
    admin_comment TEXT,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,
    decided_at TIMESTAMPTZ,
    decided_by_staff_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_mc_funding_mode CHECK (funding_mode IN ('seller', 'zamk', 'cofunded')),
    CONSTRAINT chk_mc_status CHECK (status IN ('draft', 'submitted', 'counter_offered', 'approved', 'active', 'ended', 'rejected', 'cancelled')),
    CONSTRAINT chk_mc_discount_type CHECK (discount_type IN ('percent', 'fixed')),
    CONSTRAINT chk_mc_seller_discount_bps CHECK (seller_discount_bps >= 0 AND seller_discount_bps <= 10000),
    CONSTRAINT chk_mc_seller_discount_fixed CHECK (seller_discount_fixed_cents >= 0),
    CONSTRAINT chk_mc_req_zamk_share CHECK (requested_zamk_share_bps >= 0 AND requested_zamk_share_bps <= 2500),
    CONSTRAINT chk_mc_req_zamk_budget CHECK (requested_zamk_budget_cap_cents >= 0),
    CONSTRAINT chk_mc_app_zamk_share CHECK (approved_zamk_share_bps >= 0 AND approved_zamk_share_bps <= 2500),
    CONSTRAINT chk_mc_app_zamk_budget CHECK (approved_zamk_budget_cap_cents >= 0),
    CONSTRAINT chk_mc_zamk_reserved CHECK (zamk_reserved_cents >= 0),
    CONSTRAINT chk_mc_zamk_spent CHECK (zamk_spent_cents >= 0),

    -- Invariant: ZAMK and COFUNDED promotions only support percentage discounts in V1
    CONSTRAINT chk_mc_funding_discount CHECK (funding_mode = 'seller' OR discount_type = 'percent'),

    -- Invariant: SELLER-funded campaign cannot have ZAMK subsidy requested or approved, nor any reserved/spent platform cents
    CONSTRAINT chk_mc_seller_zero_zamk CHECK (
        funding_mode != 'seller' OR (
            requested_zamk_share_bps = 0 AND
            requested_zamk_budget_cap_cents = 0 AND
            approved_zamk_share_bps = 0 AND
            approved_zamk_budget_cap_cents = 0 AND
            zamk_reserved_cents = 0 AND
            zamk_spent_cents = 0
        )
    ),

    -- Invariant: SELLER-funded campaign must have a positive seller discount (percent or fixed)
    CONSTRAINT chk_mc_seller_positive_discount CHECK (
        funding_mode != 'seller' OR (
            seller_discount_bps > 0 OR
            seller_discount_fixed_cents > 0
        )
    ),

    -- Invariant: Pure ZAMK-funded campaign has no seller-funded discount contribution
    CONSTRAINT chk_mc_zamk_seller_zero CHECK (
        funding_mode != 'zamk' OR (
            seller_discount_bps = 0 AND
            seller_discount_fixed_cents = 0
        )
    ),

    -- Invariant: COFUNDED campaign requires positive seller discount percentage contribution
    CONSTRAINT chk_mc_cofunded_seller_contribution CHECK (
        funding_mode != 'cofunded' OR seller_discount_bps > 0
    ),

    -- Invariant: Submitted/active platform-funded campaigns must request positive share and budget cap
    CONSTRAINT chk_mc_submitted_platform_req CHECK (
        funding_mode = 'seller' OR
        status = 'draft' OR (
            requested_zamk_share_bps > 0 AND
            requested_zamk_budget_cap_cents > 0
        )
    ),

    -- Invariant: Unreviewed platform campaigns (draft, submitted) must have zero approved terms
    CONSTRAINT chk_mc_unreviewed_zero_approved CHECK (
        status NOT IN ('draft', 'submitted') OR (
            approved_zamk_share_bps = 0 AND
            approved_zamk_budget_cap_cents = 0
        )
    ),

    -- Invariant: Approved platform-funded campaigns must have positive approved share and budget cap
    CONSTRAINT chk_mc_approved_liability CHECK (
        status NOT IN ('approved', 'active', 'ended') OR
        funding_mode = 'seller' OR (
            approved_zamk_share_bps > 0 AND
            approved_zamk_budget_cap_cents > 0
        )
    ),

    -- Invariant: Approved terms cannot exceed requested terms
    CONSTRAINT chk_mc_approved_ceilings CHECK (
        funding_mode = 'seller' OR (
            approved_zamk_share_bps <= requested_zamk_share_bps AND
            approved_zamk_budget_cap_cents <= requested_zamk_budget_cap_cents
        )
    ),

    -- Invariant: Platform spent + reserved must not exceed approved budget cap
    CONSTRAINT chk_mc_budget_spent CHECK (
        funding_mode = 'seller' OR
        approved_zamk_budget_cap_cents = 0 OR (
            zamk_spent_cents <= approved_zamk_budget_cap_cents AND
            zamk_spent_cents + zamk_reserved_cents <= approved_zamk_budget_cap_cents
        )
    )
);

CREATE INDEX idx_marketing_campaigns_seller_status ON marketing_campaigns(seller_id, status);
CREATE INDEX idx_marketing_campaigns_status ON marketing_campaigns(status);

-- Invariant: A seller may have at most ONE open platform-funded application or campaign at a time
CREATE UNIQUE INDEX uq_open_platform_funding_per_seller ON marketing_campaigns (seller_id)
WHERE funding_mode IN ('zamk', 'cofunded') AND status IN ('submitted', 'counter_offered', 'approved', 'active');


CREATE TABLE promo_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES marketing_campaigns(id) ON DELETE RESTRICT,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    code VARCHAR NOT NULL,
    discount_type TEXT NOT NULL,
    discount_value_bps INT NOT NULL DEFAULT 0,
    discount_value_fixed_cents BIGINT NOT NULL DEFAULT 0,
    min_order_subtotal_cents BIGINT NOT NULL DEFAULT 0,
    global_usage_limit INT,
    per_customer_usage_limit INT NOT NULL DEFAULT 1,
    first_paid_order_only BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_pc_discount_type CHECK (discount_type IN ('percent', 'fixed')),
    CONSTRAINT chk_pc_discount_bps CHECK (discount_value_bps >= 0 AND discount_value_bps <= 10000),
    CONSTRAINT chk_pc_discount_fixed CHECK (discount_value_fixed_cents >= 0),
    CONSTRAINT chk_pc_min_subtotal CHECK (min_order_subtotal_cents >= 0),
    CONSTRAINT chk_pc_global_limit CHECK (global_usage_limit IS NULL OR global_usage_limit > 0),
    CONSTRAINT chk_pc_per_customer_limit CHECK (per_customer_usage_limit > 0)
);

-- Case-insensitive uniqueness for promo codes
CREATE UNIQUE INDEX uq_promo_codes_normalized_code ON promo_codes (LOWER(code));
CREATE INDEX idx_promo_codes_campaign_id ON promo_codes(campaign_id);
CREATE INDEX idx_promo_codes_seller_id ON promo_codes(seller_id);


CREATE TABLE promo_code_usages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promo_code_id UUID NOT NULL REFERENCES promo_codes(id) ON DELETE RESTRICT,
    campaign_id UUID NOT NULL REFERENCES marketing_campaigns(id) ON DELETE RESTRICT,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'reserved',
    subsidy_cents BIGINT NOT NULL DEFAULT 0,
    seller_discount_cents BIGINT NOT NULL DEFAULT 0,
    reserved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    consumed_at TIMESTAMPTZ,
    released_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT chk_pcu_status CHECK (status IN ('reserved', 'consumed', 'released', 'expired')),
    CONSTRAINT chk_pcu_subsidy CHECK (subsidy_cents >= 0),
    CONSTRAINT chk_pcu_seller_discount CHECK (seller_discount_cents >= 0),
    -- Invariant: One order may use AT MOST ONE promo code
    CONSTRAINT uq_promo_usage_order UNIQUE (order_id)
);

CREATE INDEX idx_promo_code_usages_user ON promo_code_usages(user_id, promo_code_id, status);
CREATE INDEX idx_promo_code_usages_campaign ON promo_code_usages(campaign_id, status);
