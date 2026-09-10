ALTER TABLE return_responsibility_allocations
    DROP CONSTRAINT IF EXISTS chk_rra_legacy_disp,
    DROP COLUMN IF EXISTS legacy_disposition;

ALTER TABLE return_responsibility_allocation_history
    DROP CONSTRAINT IF EXISTS chk_rrah_legacy_disp,
    DROP COLUMN IF EXISTS legacy_disposition;
