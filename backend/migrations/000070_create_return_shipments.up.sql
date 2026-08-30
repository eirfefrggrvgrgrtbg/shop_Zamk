CREATE TABLE return_shipments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id UUID NOT NULL REFERENCES returns(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    method VARCHAR(50) NOT NULL,
    tracking_number VARCHAR(100),
    provider_shipment_id VARCHAR(100),
    status VARCHAR(50) NOT NULL,
    selected_cdek_office_code VARCHAR(100),
    snapshots JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_return_shipments_return_id ON return_shipments(return_id);
CREATE INDEX idx_return_shipments_tracking_number ON return_shipments(tracking_number);
