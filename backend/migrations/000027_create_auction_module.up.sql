CREATE TABLE auction_events (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    bid_step_cents BIGINT NOT NULL DEFAULT 10000,
    payment_deadline_hours INT NOT NULL DEFAULT 24,
    anti_sniping_enabled BOOLEAN NOT NULL DEFAULT false,
    anti_sniping_trigger_seconds INT NOT NULL DEFAULT 300,
    anti_sniping_extension_seconds INT NOT NULL DEFAULT 300,
    max_bids_per_user_per_lot_per_minute INT NOT NULL DEFAULT 10,
    max_rejected_bids_per_user_per_minute INT NOT NULL DEFAULT 10,
    no_bids_policy TEXT NOT NULL DEFAULT 'manual_review',
    unpaid_winner_policy TEXT NOT NULL DEFAULT 'manual_review',
    is_public BOOLEAN NOT NULL DEFAULT false,
    show_on_homepage BOOLEAN NOT NULL DEFAULT false,
    highlight_in_nav BOOLEAN NOT NULL DEFAULT false,
    bidding_enabled BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status IN ('draft', 'scheduled', 'live', 'ended', 'cancelled', 'paused'))
);

CREATE TABLE auction_lots (
    id UUID PRIMARY KEY,
    auction_id UUID NOT NULL REFERENCES auction_events(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    image_url TEXT,
    start_price_cents BIGINT NOT NULL,
    current_bid_cents BIGINT,
    bid_step_cents BIGINT NOT NULL,
    current_winner_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    status TEXT NOT NULL,
    order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
    payment_deadline_at TIMESTAMPTZ,
    can_relaunch BOOLEAN NOT NULL DEFAULT true,
    can_move_to_direct_sale BOOLEAN NOT NULL DEFAULT true,
    direct_sale_price_cents BIGINT,
    direct_sale_product_id UUID,
    admin_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status IN ('draft', 'active', 'ended_no_bids', 'won_pending_payment', 'paid', 'unpaid_manual_review', 'moved_to_direct_sale', 'cancelled'))
);

CREATE TABLE auction_lot_images (
    id UUID PRIMARY KEY,
    lot_id UUID NOT NULL REFERENCES auction_lots(id) ON DELETE CASCADE,
    image_url TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auction_lot_attributes (
    id UUID PRIMARY KEY,
    lot_id UUID NOT NULL REFERENCES auction_lots(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    value TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0
);

CREATE TABLE auction_bids (
    id UUID PRIMARY KEY,
    auction_id UUID NOT NULL REFERENCES auction_events(id) ON DELETE CASCADE,
    lot_id UUID NOT NULL REFERENCES auction_lots(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL,
    idempotency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auction_logs (
    id UUID PRIMARY KEY,
    auction_id UUID REFERENCES auction_events(id) ON DELETE CASCADE,
    lot_id UUID REFERENCES auction_lots(id) ON DELETE CASCADE,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auction_suspicious_events (
    id UUID PRIMARY KEY,
    auction_id UUID REFERENCES auction_events(id) ON DELETE CASCADE,
    lot_id UUID REFERENCES auction_lots(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_auction_events_status ON auction_events(status);
CREATE INDEX idx_auction_events_starts_at ON auction_events(starts_at);
CREATE INDEX idx_auction_events_ends_at ON auction_events(ends_at);
CREATE INDEX idx_auction_events_is_public ON auction_events(is_public);
CREATE INDEX idx_auction_events_show_on_homepage ON auction_events(show_on_homepage);
CREATE INDEX idx_auction_events_highlight_in_nav ON auction_events(highlight_in_nav);

CREATE INDEX idx_auction_lots_auction_id ON auction_lots(auction_id);
CREATE INDEX idx_auction_lots_status ON auction_lots(status);
CREATE INDEX idx_auction_lots_current_winner_user_id ON auction_lots(current_winner_user_id);

CREATE INDEX idx_auction_lot_images_lot_id_sort_order ON auction_lot_images(lot_id, sort_order);
CREATE INDEX idx_auction_lot_attributes_lot_id_sort_order ON auction_lot_attributes(lot_id, sort_order);

CREATE INDEX idx_auction_bids_lot_id_created_at ON auction_bids(lot_id, created_at DESC);
CREATE INDEX idx_auction_bids_user_id_created_at ON auction_bids(user_id, created_at DESC);

CREATE UNIQUE INDEX idx_auction_bids_lot_user_idempotency ON auction_bids(lot_id, user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_auction_logs_auction_id_created_at ON auction_logs(auction_id, created_at DESC);
CREATE INDEX idx_auction_suspicious_events_user_id_created_at ON auction_suspicious_events(user_id, created_at DESC);
