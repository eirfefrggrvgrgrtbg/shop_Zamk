DROP INDEX idx_return_shipments_active_per_return;

ALTER TABLE return_shipments
    DROP CONSTRAINT chk_provider,
    DROP CONSTRAINT chk_method,
    DROP CONSTRAINT chk_status;

ALTER TABLE return_shipments
    DROP COLUMN customer_name,
    DROP COLUMN customer_phone,
    DROP COLUMN pickup_address,
    DROP COLUMN cdek_office_address,
    DROP COLUMN destination_address;
