CREATE TABLE auction_order_links (
    id UUID PRIMARY KEY,
    auction_id UUID NOT NULL REFERENCES auction_events(id) ON DELETE CASCADE,
    lot_id UUID NOT NULL REFERENCES auction_lots(id) ON DELETE CASCADE,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    winner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT unique_lot_order UNIQUE(lot_id),
    CONSTRAINT unique_order_id UNIQUE(order_id)
);
