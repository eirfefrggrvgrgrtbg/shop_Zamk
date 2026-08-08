ALTER TABLE order_fulfillments DROP CONSTRAINT IF EXISTS valid_fulfillment_status;

ALTER TABLE order_fulfillments ADD CONSTRAINT valid_fulfillment_status CHECK (
    status IN (
        'awaiting_payment', 'paid', 'assembling', 'packed', 
        'accepted', 'discrepancy', 'shipped', 'delivered', 
        'cancelled', 'returned', 'refunded'
    )
);

ALTER TABLE order_fulfillments 
ADD COLUMN IF NOT EXISTS receiving_code VARCHAR UNIQUE NULL,
ADD COLUMN IF NOT EXISTS receiving_qr_token VARCHAR UNIQUE NULL,
ADD COLUMN IF NOT EXISTS packed_at TIMESTAMPTZ NULL,
ADD COLUMN IF NOT EXISTS accepted_at TIMESTAMPTZ NULL,
ADD COLUMN IF NOT EXISTS accepted_by_staff_id UUID NULL REFERENCES users(id),
ADD COLUMN IF NOT EXISTS receiving_result JSONB NULL,
ADD COLUMN IF NOT EXISTS discrepancy_reason VARCHAR NULL,
ADD COLUMN IF NOT EXISTS discrepancy_comment TEXT NULL,
ADD COLUMN IF NOT EXISTS discrepancy_at TIMESTAMPTZ NULL;

ALTER TABLE orders 
ADD COLUMN IF NOT EXISTS order_number VARCHAR UNIQUE NULL;
