CREATE TABLE IF NOT EXISTS seller_commission_rules (
    id UUID PRIMARY KEY,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    rate_bps INT NOT NULL CHECK (rate_bps >= 0 AND rate_bps <= 10000),
    reason TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_seller_commission_rules_seller_id ON seller_commission_rules(seller_id);

-- Drop old payouts and seller_balance_ledger
DROP TABLE IF EXISTS seller_balance_ledger CASCADE;
DROP TABLE IF EXISTS payouts CASCADE;

CREATE TABLE IF NOT EXISTS payout_batches (
    id UUID PRIMARY KEY,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    status TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'processing', 'paid', 'failed', 'held')),
    scheduled_for TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payout_batches_seller_id ON payout_batches(seller_id);
CREATE INDEX IF NOT EXISTS idx_payout_batches_status ON payout_batches(status);

CREATE TABLE IF NOT EXISTS seller_ledger_entries (
    id UUID PRIMARY KEY,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
    order_item_id UUID REFERENCES order_items(id) ON DELETE SET NULL,
    payout_batch_id UUID REFERENCES payout_batches(id) ON DELETE SET NULL,
    type TEXT NOT NULL CHECK (type IN ('sale_gross', 'zamk_commission', 'seller_earning', 'adjustment', 'payout')),
    amount_cents BIGINT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'RUB' CHECK (currency = 'RUB'),
    available_at TIMESTAMPTZ,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_seller_ledger_entries_seller_id ON seller_ledger_entries(seller_id);
CREATE INDEX IF NOT EXISTS idx_seller_ledger_entries_payout_batch_id ON seller_ledger_entries(payout_batch_id);
