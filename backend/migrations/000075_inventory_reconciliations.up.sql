CREATE TABLE inventory_reconciliation_sessions (
    id UUID PRIMARY KEY,
    product_variant_id UUID NOT NULL REFERENCES product_variants(id),
    status VARCHAR NOT NULL CHECK (status IN ('in_progress', 'review', 'completed', 'cancelled')),
    started_by UUID NOT NULL REFERENCES users(id),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_by UUID REFERENCES users(id),
    completed_at TIMESTAMPTZ,
    cancelled_by UUID REFERENCES users(id),
    cancelled_at TIMESTAMPTZ,
    notes TEXT
);

CREATE UNIQUE INDEX idx_reconciliation_sessions_active
ON inventory_reconciliation_sessions (product_variant_id)
WHERE status IN ('in_progress', 'review');

CREATE TABLE inventory_reconciliation_expected_units (
    session_id UUID NOT NULL REFERENCES inventory_reconciliation_sessions(id) ON DELETE CASCADE,
    inventory_unit_id UUID NOT NULL REFERENCES inventory_units(id),
    expected_status VARCHAR NOT NULL,
    snapshot_created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (session_id, inventory_unit_id)
);

CREATE TABLE inventory_reconciliation_scans (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES inventory_reconciliation_sessions(id) ON DELETE CASCADE,
    inventory_unit_id UUID REFERENCES inventory_units(id),
    raw_code VARCHAR NOT NULL,
    classification VARCHAR NOT NULL CHECK (classification IN ('expected_found', 'unexpected_found', 'duplicate', 'wrong_variant', 'unknown_code')),
    scanned_by UUID NOT NULL REFERENCES users(id),
    scanned_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_reconciliation_scans_idempotent
ON inventory_reconciliation_scans (session_id, inventory_unit_id)
WHERE inventory_unit_id IS NOT NULL;
