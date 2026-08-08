-- Sequences for order_number and receiving_code
CREATE SEQUENCE IF NOT EXISTS order_number_seq START WITH 100000 INCREMENT BY 1;
CREATE SEQUENCE IF NOT EXISTS fulfillment_receiving_code_seq START WITH 100000 INCREMENT BY 1;

-- Backfill existing NULL order_numbers deterministically
UPDATE orders 
SET order_number = 'ZMK-' || lpad(nextval('order_number_seq')::text, 6, '0') 
WHERE order_number IS NULL;

-- Receiving Sessions Table
CREATE TABLE IF NOT EXISTS fulfillment_receiving_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fulfillment_id UUID NOT NULL REFERENCES order_fulfillments(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'accepted', 'discrepancy', 'cancelled')),
    version INT NOT NULL DEFAULT 1,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_by_staff_id UUID NULL REFERENCES users(id),
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Ensure only ONE active receiving session exists per fulfillment
CREATE UNIQUE INDEX IF NOT EXISTS idx_active_receiving_session ON fulfillment_receiving_sessions(fulfillment_id) WHERE status = 'active';

-- Receiving Items Table
CREATE TABLE IF NOT EXISTS fulfillment_receiving_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES fulfillment_receiving_sessions(id) ON DELETE CASCADE,
    fulfillment_item_id UUID NULL REFERENCES order_items(id) ON DELETE SET NULL,
    variant_id UUID NULL,
    sku VARCHAR(128) NOT NULL,
    barcode VARCHAR(128) NULL,
    product_title VARCHAR(255) NOT NULL DEFAULT '',
    expected_quantity INT NOT NULL DEFAULT 0,
    scanned_quantity INT NOT NULL DEFAULT 0,
    damaged_quantity INT NOT NULL DEFAULT 0,
    unexpected_quantity INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_session_item UNIQUE(session_id, sku)
);

-- Idempotency scans table
CREATE TABLE IF NOT EXISTS fulfillment_receiving_scans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES fulfillment_receiving_sessions(id) ON DELETE CASCADE,
    idempotency_key VARCHAR(128) NOT NULL,
    barcode VARCHAR(128) NOT NULL,
    scanned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_session_idempotency UNIQUE(session_id, idempotency_key)
);
