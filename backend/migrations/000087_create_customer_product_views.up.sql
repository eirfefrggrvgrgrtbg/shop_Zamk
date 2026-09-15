CREATE TABLE customer_product_views (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    last_viewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    view_count BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (user_id, product_id)
);

CREATE INDEX idx_customer_product_views_user_recent ON customer_product_views (user_id, last_viewed_at DESC);
