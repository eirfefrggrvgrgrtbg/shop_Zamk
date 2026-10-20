-- MARKETING.M1.3: advertising campaign lifecycle (draft -> approved -> active <-> paused -> ended; cancel).
-- 'ended' keeps serving as COMPLETED. Only 'paused' is a new status value.
ALTER TABLE marketing_campaigns DROP CONSTRAINT chk_mc_status;
ALTER TABLE marketing_campaigns ADD CONSTRAINT chk_mc_status CHECK (status IN (
    'draft', 'submitted', 'counter_offered', 'approved', 'active', 'paused', 'ended', 'rejected', 'cancelled'
));

-- Before M1.3 an approved advertising campaign already accepted tracking traffic. Rows that exist
-- when this migration runs (and rows written by the previous application version during a rolling
-- deploy, via the default) keep that behaviour until an admin moves them on. The new application
-- writes FALSE explicitly, so new campaigns only accept traffic while active.
-- No status is changed here: nothing is activated, nothing is switched off.
ALTER TABLE marketing_campaigns
    ADD COLUMN IF NOT EXISTS legacy_approved_tracking BOOLEAN NOT NULL DEFAULT TRUE;

COMMENT ON COLUMN marketing_campaigns.legacy_approved_tracking IS
    'TRUE: pre-M1.3 semantics, status approved still accepts tracking traffic. Cleared on any status change by M1.3+.';
