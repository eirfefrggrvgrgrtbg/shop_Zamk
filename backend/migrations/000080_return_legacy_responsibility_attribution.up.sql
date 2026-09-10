ALTER TABLE return_responsibility_allocations
    ADD COLUMN legacy_disposition TEXT;

ALTER TABLE return_responsibility_allocation_history
    ADD COLUMN legacy_disposition TEXT;

ALTER TABLE return_responsibility_allocations
    ADD CONSTRAINT chk_rra_legacy_disp CHECK (
        (legacy_disposition IS NULL OR legacy_disposition IN ('accepted', 'damaged', 'rejected', 'unreceived')) AND
        NOT (order_item_allocation_id IS NOT NULL AND legacy_disposition IS NOT NULL)
    );

ALTER TABLE return_responsibility_allocation_history
    ADD CONSTRAINT chk_rrah_legacy_disp CHECK (
        (legacy_disposition IS NULL OR legacy_disposition IN ('accepted', 'damaged', 'rejected', 'unreceived')) AND
        NOT (order_item_allocation_id IS NOT NULL AND legacy_disposition IS NOT NULL)
    );
