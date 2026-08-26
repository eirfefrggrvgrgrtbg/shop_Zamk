CREATE TABLE order_item_allocations (
    id UUID PRIMARY KEY,
    order_item_id UUID NOT NULL REFERENCES order_items(id) ON DELETE CASCADE,
    inventory_unit_id UUID NOT NULL REFERENCES inventory_units(id) ON DELETE RESTRICT,
    reservation_id UUID REFERENCES reservations(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_at TIMESTAMPTZ,
    release_reason TEXT
);

CREATE UNIQUE INDEX idx_order_item_alloc_active_unit ON order_item_allocations(inventory_unit_id) WHERE released_at IS NULL;
CREATE INDEX idx_order_item_alloc_order_item ON order_item_allocations(order_item_id);
CREATE INDEX idx_order_item_alloc_reservation ON order_item_allocations(reservation_id);
