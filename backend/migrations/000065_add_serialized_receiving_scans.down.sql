DROP INDEX IF EXISTS idx_receiving_scans_unit_active;

ALTER TABLE supply_receiving_scans
DROP COLUMN inventory_unit_id,
DROP COLUMN condition,
DROP COLUMN voided_at,
DROP COLUMN voided_by;
