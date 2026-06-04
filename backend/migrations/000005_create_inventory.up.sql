CREATE TABLE inventory_items (
    id UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    product_variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    total_stock INT NOT NULL DEFAULT 0,
    reserved_stock INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_total_stock CHECK (total_stock >= 0),
    CONSTRAINT valid_reserved_stock CHECK (reserved_stock >= 0),
    CONSTRAINT valid_stock_bounds CHECK (reserved_stock <= total_stock),
    CONSTRAINT unique_product_variant UNIQUE (product_variant_id)
);

CREATE INDEX inventory_items_product_id_idx ON inventory_items(product_id);
CREATE INDEX inventory_items_seller_id_idx ON inventory_items(seller_id);

CREATE TABLE stock_movements (
    id UUID PRIMARY KEY,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    product_variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    quantity INT NOT NULL,
    reason TEXT,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reference_type TEXT,
    reference_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_movement_type CHECK (type IN ('receipt', 'reservation_created', 'reservation_released', 'sale', 'return', 'adjustment', 'write_off')),
    CONSTRAINT valid_quantity CHECK (quantity > 0)
);

CREATE INDEX stock_movements_inventory_item_id_idx ON stock_movements(inventory_item_id);

CREATE TABLE reservations (
    id UUID PRIMARY KEY,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    product_variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    quantity INT NOT NULL,
    status TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    order_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_at TIMESTAMPTZ,
    CONSTRAINT valid_reservation_status CHECK (status IN ('active', 'released', 'converted', 'expired')),
    CONSTRAINT valid_reservation_quantity CHECK (quantity > 0)
);

CREATE INDEX reservations_inventory_item_id_idx ON reservations(inventory_item_id);
CREATE INDEX reservations_user_id_idx ON reservations(user_id);
CREATE INDEX reservations_status_idx ON reservations(status);
