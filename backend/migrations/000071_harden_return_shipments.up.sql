ALTER TABLE return_shipments
    ADD COLUMN customer_name VARCHAR(255),
    ADD COLUMN customer_phone VARCHAR(50),
    ADD COLUMN pickup_address JSONB,
    ADD COLUMN cdek_office_address VARCHAR(255),
    ADD COLUMN destination_address JSONB;

ALTER TABLE return_shipments
    ADD CONSTRAINT chk_provider CHECK (provider IN ('cdek')),
    ADD CONSTRAINT chk_method CHECK (method IN ('cdek_courier', 'cdek_office')),
    ADD CONSTRAINT chk_status CHECK (status IN ('draft', 'awaiting_handover', 'handed_over', 'in_transit', 'arrived_at_zamk', 'cancelled'));

-- Partial unique index for active shipment
CREATE UNIQUE INDEX idx_return_shipments_active_per_return
    ON return_shipments(return_id)
    WHERE status != 'cancelled';
