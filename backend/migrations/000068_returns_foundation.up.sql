ALTER TABLE order_fulfillments ADD CONSTRAINT uq_order_fulfillments_id_order_id UNIQUE (id, order_id);

ALTER TABLE returns ADD COLUMN fulfillment_id UUID;

DO $$ 
BEGIN
  IF EXISTS (SELECT 1 FROM returns) THEN
    -- Backfill exactly one fulfillment if distinct count is 1 and no null fulfillments
    WITH valid_fulfillments AS (
      SELECT ri.return_id, (ARRAY_AGG(oi.order_fulfillment_id))[1] as fulfillment_id
      FROM return_items ri
      JOIN order_items oi ON ri.order_item_id = oi.id
      GROUP BY ri.return_id
      HAVING COUNT(DISTINCT oi.order_fulfillment_id) = 1
         AND COUNT(oi.order_fulfillment_id) = COUNT(ri.id)
    )
    UPDATE returns r
    SET fulfillment_id = vf.fulfillment_id
    FROM valid_fulfillments vf
    WHERE r.id = vf.return_id;
    
    IF EXISTS (SELECT 1 FROM returns WHERE fulfillment_id IS NULL) THEN
      RAISE EXCEPTION 'Cannot safely backfill returns spanning multiple fulfillments or missing fulfillments';
    END IF;
  END IF;
END $$;

ALTER TABLE returns ALTER COLUMN fulfillment_id SET NOT NULL;

ALTER TABLE returns 
ADD CONSTRAINT fk_returns_fulfillment_order 
FOREIGN KEY (fulfillment_id, order_id) REFERENCES order_fulfillments(id, order_id);

ALTER TABLE returns ADD COLUMN receiving_started_at TIMESTAMPTZ;

ALTER TABLE returns ADD CONSTRAINT valid_return_status CHECK (status IN ('requested', 'approved', 'receiving', 'item_received', 'refunded', 'completed', 'rejected', 'cancelled'));

ALTER TABLE return_items
ADD COLUMN accepted_quantity INTEGER NOT NULL DEFAULT 0,
ADD COLUMN damaged_quantity INTEGER NOT NULL DEFAULT 0,
ADD COLUMN rejected_quantity INTEGER NOT NULL DEFAULT 0;

ALTER TABLE return_items
ADD CONSTRAINT valid_inspection_qtys CHECK (accepted_quantity >= 0 AND damaged_quantity >= 0 AND rejected_quantity >= 0),
ADD CONSTRAINT check_inspection_sum CHECK (accepted_quantity + damaged_quantity + rejected_quantity <= quantity);

CREATE TABLE return_item_units (
    id UUID PRIMARY KEY,
    return_item_id UUID NOT NULL REFERENCES return_items(id) ON DELETE CASCADE,
    order_item_allocation_id UUID NOT NULL REFERENCES order_item_allocations(id),
    scanned_at TIMESTAMPTZ,
    inspected_condition TEXT,
    disposition TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_order_item_allocation_id UNIQUE (order_item_allocation_id)
);
