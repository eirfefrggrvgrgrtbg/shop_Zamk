ALTER TABLE supply_receiving_scans
ADD COLUMN inventory_unit_id UUID NULL REFERENCES inventory_units(id) ON DELETE RESTRICT,
ADD COLUMN condition VARCHAR NULL,
ADD COLUMN voided_at TIMESTAMPTZ NULL,
ADD COLUMN voided_by UUID NULL REFERENCES users(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX idx_receiving_scans_unit_active ON supply_receiving_scans(inventory_unit_id) 
WHERE inventory_unit_id IS NOT NULL AND voided_at IS NULL;
