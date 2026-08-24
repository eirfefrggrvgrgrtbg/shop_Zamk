CREATE TABLE inventory_units (
    id UUID PRIMARY KEY,
    unit_code VARCHAR(255) NOT NULL,
    product_variant_id UUID NOT NULL REFERENCES product_variants(id),
    origin_supply_id UUID NOT NULL REFERENCES seller_supplies(id),
    origin_supply_item_id UUID NOT NULL REFERENCES seller_supply_items(id),
    origin_box_id UUID REFERENCES seller_supply_boxes(id),
    unit_index INTEGER NOT NULL CHECK (unit_index > 0),
    external_marking_code VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'expected' CHECK (status IN ('expected', 'warehouse', 'damaged', 'written_off', 'shipped')),
    receiving_session_id UUID REFERENCES supply_receiving_sessions(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_inventory_units_unit_code ON inventory_units(unit_code);
CREATE UNIQUE INDEX idx_inventory_units_supply_item_index ON inventory_units(origin_supply_item_id, unit_index);
CREATE UNIQUE INDEX idx_inventory_units_external_marking ON inventory_units(external_marking_code) WHERE external_marking_code IS NOT NULL;
CREATE INDEX idx_inventory_units_origin_supply_id ON inventory_units(origin_supply_id);
CREATE INDEX idx_inventory_units_product_variant_id ON inventory_units(product_variant_id);
