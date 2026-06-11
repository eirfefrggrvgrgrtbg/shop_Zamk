CREATE TABLE order_fulfillments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    seller_id UUID NOT NULL REFERENCES sellers(id),
    status TEXT NOT NULL,
    subtotal_cents BIGINT NOT NULL DEFAULT 0,
    commission_bps INT NOT NULL DEFAULT 900,
    seller_amount_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT unique_order_seller_fulfillment UNIQUE(order_id, seller_id),
    CONSTRAINT valid_fulfillment_status CHECK (status IN ('awaiting_payment', 'paid', 'assembling', 'packed', 'shipped', 'delivered', 'cancelled', 'returned', 'refunded'))
);

CREATE INDEX idx_order_fulfillments_order_id ON order_fulfillments(order_id);
CREATE INDEX idx_order_fulfillments_seller_id ON order_fulfillments(seller_id);
CREATE INDEX idx_order_fulfillments_status ON order_fulfillments(status);
CREATE INDEX idx_order_fulfillments_seller_status ON order_fulfillments(seller_id, status);

ALTER TABLE order_items
ADD COLUMN order_fulfillment_id UUID REFERENCES order_fulfillments(id);

CREATE INDEX idx_order_items_fulfillment_id ON order_items(order_fulfillment_id);

ALTER TABLE shipments
ADD COLUMN fulfillment_id UUID REFERENCES order_fulfillments(id);

CREATE INDEX idx_shipments_fulfillment_id ON shipments(fulfillment_id);

-- Backfill order_fulfillments
INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents)
SELECT 
    gen_random_uuid(),
    oi.order_id,
    oi.seller_id,
    o.status,
    SUM(oi.subtotal_price_cents),
    900,
    SUM(oi.subtotal_price_cents) - (SUM(oi.subtotal_price_cents) * 900 / 10000)
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
GROUP BY oi.order_id, oi.seller_id, o.status
ON CONFLICT (order_id, seller_id) DO NOTHING;

-- Backfill order_items.order_fulfillment_id
UPDATE order_items oi
SET order_fulfillment_id = of.id
FROM order_fulfillments of
WHERE oi.order_id = of.order_id AND oi.seller_id = of.seller_id;

-- Backfill shipments.fulfillment_id safely
-- Only update shipments if the order has exactly ONE fulfillment group
UPDATE shipments s
SET fulfillment_id = of.id
FROM order_fulfillments of
WHERE s.order_id = of.order_id
AND (
    SELECT COUNT(*) 
    FROM order_fulfillments of2 
    WHERE of2.order_id = s.order_id
) = 1;
