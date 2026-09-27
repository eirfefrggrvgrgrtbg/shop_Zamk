CREATE TABLE behavioral_events (
    id UUID PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    source VARCHAR(20) NOT NULL CHECK (source IN ('client', 'server')),
    visitor_id UUID NULL,
    user_id UUID NULL,
    product_id UUID NULL,
    variant_id UUID NULL,
    category_id UUID NULL,
    order_id UUID NULL,
    return_id UUID NULL,
    order_item_id UUID NULL,
    quantity INTEGER NULL,
    placement VARCHAR(255) NULL,
    route VARCHAR(1024) NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_behav_visitor_time ON behavioral_events(visitor_id, occurred_at DESC) WHERE visitor_id IS NOT NULL;
CREATE INDEX idx_behav_user_time ON behavioral_events(user_id, occurred_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX idx_behav_type_time ON behavioral_events(event_type, occurred_at DESC);
CREATE INDEX idx_behav_prod_type_time ON behavioral_events(product_id, event_type, occurred_at DESC) WHERE product_id IS NOT NULL;
