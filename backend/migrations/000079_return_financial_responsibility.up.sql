CREATE TABLE return_responsibility_allocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    return_item_id UUID NOT NULL REFERENCES return_items(id) ON DELETE CASCADE,
    order_item_allocation_id UUID UNIQUE REFERENCES order_item_allocations(id) ON DELETE RESTRICT,
    quantity INT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    responsible_party TEXT,
    reason_code TEXT,
    decision_source TEXT,
    internal_note TEXT,
    decided_at TIMESTAMPTZ,
    actor_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_rra_qty CHECK (quantity > 0),
    CONSTRAINT chk_rra_serialized_qty CHECK (order_item_allocation_id IS NULL OR quantity = 1),
    CONSTRAINT chk_rra_status CHECK (status IN ('pending', 'resolved', 'not_required')),
    CONSTRAINT chk_rra_party CHECK (responsible_party IN ('seller', 'zamk', 'carrier', 'customer') OR responsible_party IS NULL),
    CONSTRAINT chk_rra_source CHECK (decision_source IN ('system', 'employee') OR decision_source IS NULL),
    CONSTRAINT chk_rra_reason CHECK (reason_code IN (
        'customer_change_of_mind',
        'seller_product_defect',
        'zamk_warehouse_damage',
        'zamk_fulfillment_error',
        'carrier_damage',
        'customer_damage',
        'fraud_or_substitution',
        'unknown'
    ) OR reason_code IS NULL),
    CONSTRAINT chk_rra_invariants CHECK (
        (status = 'pending' AND responsible_party IS NULL AND reason_code IS NULL AND decision_source IS NULL AND actor_id IS NULL AND decided_at IS NULL) OR
        (status = 'not_required' AND responsible_party IS NULL AND reason_code IS NOT NULL AND decision_source IS NOT NULL AND decided_at IS NOT NULL) OR
        (status = 'resolved' AND responsible_party IS NOT NULL AND reason_code IS NOT NULL AND decision_source IS NOT NULL AND decided_at IS NOT NULL)
    ),
    CONSTRAINT chk_rra_provenance CHECK (
        (decision_source IS NULL AND actor_id IS NULL) OR
        (decision_source = 'system' AND actor_id IS NULL) OR
        (decision_source = 'employee' AND actor_id IS NOT NULL)
    ),
    CONSTRAINT chk_rra_reason_party CHECK (
        CASE reason_code
            WHEN 'seller_product_defect' THEN status = 'resolved' AND responsible_party = 'seller'
            WHEN 'zamk_warehouse_damage' THEN status = 'resolved' AND responsible_party = 'zamk'
            WHEN 'zamk_fulfillment_error' THEN status = 'resolved' AND responsible_party = 'zamk'
            WHEN 'carrier_damage' THEN status = 'resolved' AND responsible_party = 'carrier'
            WHEN 'customer_damage' THEN status = 'resolved' AND responsible_party = 'customer'
            WHEN 'customer_change_of_mind' THEN status = 'not_required' AND responsible_party IS NULL
            ELSE TRUE
        END
    )
);

CREATE TABLE return_responsibility_allocation_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    allocation_id UUID NOT NULL REFERENCES return_responsibility_allocations(id) ON DELETE RESTRICT,
    return_item_id UUID NOT NULL REFERENCES return_items(id) ON DELETE RESTRICT,
    quantity INT NOT NULL,
    order_item_allocation_id UUID REFERENCES order_item_allocations(id) ON DELETE RESTRICT,
    status TEXT NOT NULL,
    responsible_party TEXT,
    reason_code TEXT,
    decision_source TEXT,
    internal_note TEXT,
    actor_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_rrah_qty CHECK (quantity > 0),
    CONSTRAINT chk_rrah_serialized_qty CHECK (order_item_allocation_id IS NULL OR quantity = 1),
    CONSTRAINT chk_rrah_status CHECK (status IN ('pending', 'resolved', 'not_required')),
    CONSTRAINT chk_rrah_party CHECK (responsible_party IN ('seller', 'zamk', 'carrier', 'customer') OR responsible_party IS NULL),
    CONSTRAINT chk_rrah_source CHECK (decision_source IN ('system', 'employee') OR decision_source IS NULL),
    CONSTRAINT chk_rrah_reason CHECK (reason_code IN (
        'customer_change_of_mind',
        'seller_product_defect',
        'zamk_warehouse_damage',
        'zamk_fulfillment_error',
        'carrier_damage',
        'customer_damage',
        'fraud_or_substitution',
        'unknown'
    ) OR reason_code IS NULL),
    CONSTRAINT chk_rrah_invariants CHECK (
        (status = 'pending' AND responsible_party IS NULL AND reason_code IS NULL AND decision_source IS NULL AND actor_id IS NULL AND decided_at IS NULL) OR
        (status = 'not_required' AND responsible_party IS NULL AND reason_code IS NOT NULL AND decision_source IS NOT NULL AND decided_at IS NOT NULL) OR
        (status = 'resolved' AND responsible_party IS NOT NULL AND reason_code IS NOT NULL AND decision_source IS NOT NULL AND decided_at IS NOT NULL)
    ),
    CONSTRAINT chk_rrah_provenance CHECK (
        (decision_source IS NULL AND actor_id IS NULL) OR
        (decision_source = 'system' AND actor_id IS NULL) OR
        (decision_source = 'employee' AND actor_id IS NOT NULL)
    ),
    CONSTRAINT chk_rrah_reason_party CHECK (
        CASE reason_code
            WHEN 'seller_product_defect' THEN status = 'resolved' AND responsible_party = 'seller'
            WHEN 'zamk_warehouse_damage' THEN status = 'resolved' AND responsible_party = 'zamk'
            WHEN 'zamk_fulfillment_error' THEN status = 'resolved' AND responsible_party = 'zamk'
            WHEN 'carrier_damage' THEN status = 'resolved' AND responsible_party = 'carrier'
            WHEN 'customer_damage' THEN status = 'resolved' AND responsible_party = 'customer'
            WHEN 'customer_change_of_mind' THEN status = 'not_required' AND responsible_party IS NULL
            ELSE TRUE
        END
    )
);

CREATE INDEX idx_return_resp_alloc_return_item_id ON return_responsibility_allocations(return_item_id);
CREATE INDEX idx_return_resp_alloc_hist_alloc_id ON return_responsibility_allocation_history(allocation_id, created_at DESC);
CREATE INDEX idx_return_resp_alloc_hist_return_item_id ON return_responsibility_allocation_history(return_item_id);

-- Step 1: Insert bound allocations for known serialized units in return_item_units (up to return_items.quantity)
WITH ranked_units AS (
    SELECT
        u.return_item_id,
        u.order_item_allocation_id,
        ROW_NUMBER() OVER (PARTITION BY u.return_item_id ORDER BY u.scanned_at ASC, u.id ASC) AS rn
    FROM (
        SELECT DISTINCT ON (order_item_allocation_id)
            return_item_id,
            order_item_allocation_id,
            COALESCE(scanned_at, created_at) AS scanned_at,
            id
        FROM return_item_units
        ORDER BY order_item_allocation_id, COALESCE(scanned_at, created_at) ASC, id ASC
    ) u
    JOIN return_items ri ON ri.id = u.return_item_id
),
valid_units AS (
    SELECT ru.return_item_id, ru.order_item_allocation_id
    FROM ranked_units ru
    JOIN return_items ri ON ri.id = ru.return_item_id
    WHERE ru.rn <= ri.quantity
)
INSERT INTO return_responsibility_allocations (
    id, return_item_id, order_item_allocation_id, quantity, status, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    vu.return_item_id,
    vu.order_item_allocation_id,
    1,
    'pending',
    NOW(),
    NOW()
FROM valid_units vu;

-- Step 2: Insert unbound allocation for remaining quantity (if any)
INSERT INTO return_responsibility_allocations (
    id, return_item_id, order_item_allocation_id, quantity, status, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    ri.id,
    NULL,
    ri.quantity - COALESCE(b.bound_count, 0),
    'pending',
    NOW(),
    NOW()
FROM return_items ri
LEFT JOIN (
    SELECT return_item_id, COUNT(*) AS bound_count
    FROM return_responsibility_allocations
    WHERE order_item_allocation_id IS NOT NULL
    GROUP BY return_item_id
) b ON b.return_item_id = ri.id
WHERE ri.quantity - COALESCE(b.bound_count, 0) > 0;

-- Step 3: Initialize history snapshots for all created allocations with return_item_id
INSERT INTO return_responsibility_allocation_history (
    id, allocation_id, return_item_id, quantity, order_item_allocation_id, status, created_at
)
SELECT
    gen_random_uuid(),
    a.id,
    a.return_item_id,
    a.quantity,
    a.order_item_allocation_id,
    'pending',
    NOW()
FROM return_responsibility_allocations a;
