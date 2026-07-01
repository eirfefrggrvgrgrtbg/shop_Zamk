CREATE TABLE IF NOT EXISTS payouts (
    id UUID PRIMARY KEY,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'requested' CHECK (status IN ('requested', 'approved', 'rejected', 'paid', 'cancelled')),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    currency TEXT NOT NULL DEFAULT 'RUB' CHECK (currency = 'RUB'),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    approved_at TIMESTAMPTZ,
    rejected_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    admin_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS seller_balance_ledger (
    id UUID PRIMARY KEY,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
    order_item_id UUID REFERENCES order_items(id) ON DELETE SET NULL,
    return_id UUID REFERENCES returns(id) ON DELETE SET NULL,
    refund_id UUID REFERENCES refunds(id) ON DELETE SET NULL,
    payout_id UUID REFERENCES payouts(id) ON DELETE SET NULL,
    type TEXT NOT NULL CHECK (type IN ('sale_pending', 'sale_available', 'refund_hold', 'refund_deduction', 'payout_requested', 'payout_rejected', 'payout_paid', 'manual_adjustment')),
    amount_cents BIGINT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'RUB' CHECK (currency = 'RUB'),
    available_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    comment TEXT
);

CREATE INDEX IF NOT EXISTS idx_seller_balance_ledger_seller_id ON seller_balance_ledger(seller_id);
CREATE INDEX IF NOT EXISTS idx_seller_balance_ledger_order_item_id ON seller_balance_ledger(order_item_id);
CREATE INDEX IF NOT EXISTS idx_seller_balance_ledger_payout_id ON seller_balance_ledger(payout_id);
CREATE INDEX IF NOT EXISTS idx_payouts_seller_id ON payouts(seller_id);
