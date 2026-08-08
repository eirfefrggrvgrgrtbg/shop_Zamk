DROP TABLE IF EXISTS supply_receiving_scans CASCADE;
DROP TABLE IF EXISTS supply_receiving_items CASCADE;
DROP INDEX IF EXISTS idx_active_supply_receiving_session;
DROP TABLE IF EXISTS supply_receiving_sessions CASCADE;

DROP TABLE IF EXISTS seller_supply_box_items CASCADE;
DROP TABLE IF EXISTS seller_supply_boxes CASCADE;
DROP TABLE IF EXISTS seller_supply_items CASCADE;
DROP TABLE IF EXISTS seller_supplies CASCADE;

DROP SEQUENCE IF EXISTS supply_number_seq;
