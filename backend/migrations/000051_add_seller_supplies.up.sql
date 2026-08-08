CREATE SEQUENCE IF NOT EXISTS supply_number_seq START 1000;

CREATE TABLE IF NOT EXISTS seller_supplies (
    id UUID PRIMARY KEY,
    supply_number VARCHAR NOT NULL UNIQUE,
    seller_id UUID NOT NULL REFERENCES sellers(id),
    status VARCHAR NOT NULL,
    handoff_method VARCHAR NOT NULL,
    carrier_name VARCHAR NULL,
    tracking_number VARCHAR NULL,
    expected_arrival_date DATE NULL,
    qr_token VARCHAR UNIQUE NULL,
    created_at TIMESTAMPTZ NOT NULL,
    shipped_at TIMESTAMPTZ NULL,
    arrived_at TIMESTAMPTZ NULL,
    receiving_started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT valid_supply_status CHECK (
        status IN (
            'draft', 'ready_to_ship', 'shipped_by_seller', 
            'arrived_at_zamk', 'receiving', 'completed', 
            'completed_with_discrepancies', 'cancelled'
        )
    )
);

CREATE TABLE IF NOT EXISTS seller_supply_items (
    id UUID PRIMARY KEY,
    supply_id UUID NOT NULL REFERENCES seller_supplies(id) ON DELETE CASCADE,
    variant_id UUID NOT NULL REFERENCES product_variants(id),
    expected_quantity INT NOT NULL CHECK (expected_quantity >= 0),
    accepted_quantity INT NOT NULL DEFAULT 0 CHECK (accepted_quantity >= 0),
    damaged_quantity INT NOT NULL DEFAULT 0 CHECK (damaged_quantity >= 0),
    missing_quantity INT NOT NULL DEFAULT 0 CHECK (missing_quantity >= 0),
    extra_quantity INT NOT NULL DEFAULT 0 CHECK (extra_quantity >= 0),
    receiving_comment TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(supply_id, variant_id)
);

CREATE TABLE IF NOT EXISTS seller_supply_boxes (
    id UUID PRIMARY KEY,
    supply_id UUID NOT NULL REFERENCES seller_supplies(id) ON DELETE CASCADE,
    box_number VARCHAR NOT NULL,
    qr_token VARCHAR UNIQUE NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS seller_supply_box_items (
    box_id UUID NOT NULL REFERENCES seller_supply_boxes(id) ON DELETE CASCADE,
    supply_item_id UUID NOT NULL REFERENCES seller_supply_items(id) ON DELETE CASCADE,
    quantity INT NOT NULL CHECK (quantity >= 0),
    PRIMARY KEY (box_id, supply_item_id)
);

CREATE TABLE IF NOT EXISTS supply_receiving_sessions (
    id UUID PRIMARY KEY,
    supply_id UUID NOT NULL REFERENCES seller_supplies(id) ON DELETE CASCADE,
    status VARCHAR NOT NULL CHECK (status IN ('active', 'completed', 'cancelled')),
    version INT NOT NULL DEFAULT 1,
    started_at TIMESTAMPTZ NOT NULL,
    started_by_staff_id UUID NULL REFERENCES users(id),
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_active_supply_receiving_session ON supply_receiving_sessions(supply_id) WHERE status = 'active';

CREATE TABLE IF NOT EXISTS supply_receiving_items (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES supply_receiving_sessions(id) ON DELETE CASCADE,
    supply_item_id UUID NULL REFERENCES seller_supply_items(id) ON DELETE CASCADE,
    variant_id UUID NULL REFERENCES product_variants(id),
    sku VARCHAR NOT NULL,
    barcode VARCHAR NULL,
    product_title VARCHAR NOT NULL,
    expected_quantity INT NOT NULL DEFAULT 0,
    scanned_quantity INT NOT NULL DEFAULT 0,
    damaged_quantity INT NOT NULL DEFAULT 0,
    unexpected_quantity INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS supply_receiving_scans (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES supply_receiving_sessions(id) ON DELETE CASCADE,
    supply_receiving_item_id UUID NOT NULL REFERENCES supply_receiving_items(id) ON DELETE CASCADE,
    staff_id UUID NULL REFERENCES users(id),
    quantity INT NOT NULL DEFAULT 1,
    is_damage BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL
);
