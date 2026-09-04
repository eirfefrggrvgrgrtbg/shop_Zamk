CREATE TABLE IF NOT EXISTS inventory_reconciliation_resolutions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES inventory_reconciliation_sessions(id) ON DELETE CASCADE,
    inventory_unit_id UUID NOT NULL REFERENCES inventory_units(id) ON DELETE RESTRICT,
    case_type VARCHAR(50) NOT NULL,
    action_id VARCHAR(50) NOT NULL,
    performed_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    performed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    related_allocation_id UUID REFERENCES order_item_allocations(id) ON DELETE RESTRICT,
    replacement_inventory_unit_id UUID REFERENCES inventory_units(id) ON DELETE RESTRICT,
    note TEXT,
    before_context JSONB NOT NULL DEFAULT '{}'::jsonb,
    after_context JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE UNIQUE INDEX idx_inv_recon_res_unique ON inventory_reconciliation_resolutions (session_id, inventory_unit_id) WHERE action_id = 'confirm_missing';
CREATE INDEX idx_inventory_reconciliation_resolutions_session_id ON inventory_reconciliation_resolutions(session_id);
