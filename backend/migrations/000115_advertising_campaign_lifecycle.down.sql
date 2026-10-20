-- Rolling back is only possible after paused campaigns were resumed or completed by an admin:
-- the previous status set has no 'paused' and there is no lossless automatic mapping.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM marketing_campaigns WHERE status = 'paused') THEN
        RAISE EXCEPTION 'Cannot roll back 000115: paused campaigns exist; resume or complete them first';
    END IF;
END $$;

ALTER TABLE marketing_campaigns DROP COLUMN IF EXISTS legacy_approved_tracking;
ALTER TABLE marketing_campaigns DROP CONSTRAINT chk_mc_status;
ALTER TABLE marketing_campaigns ADD CONSTRAINT chk_mc_status CHECK (status IN (
    'draft', 'submitted', 'counter_offered', 'approved', 'active', 'ended', 'rejected', 'cancelled'
));
