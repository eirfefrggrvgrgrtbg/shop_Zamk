DROP INDEX IF EXISTS idx_notifications_active_seller_alert;

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS chk_notifications_lifecycle,
    DROP CONSTRAINT IF EXISTS chk_seller_alert_dedupe,
    DROP CONSTRAINT IF EXISTS chk_alert_status;

ALTER TABLE notifications
    DROP COLUMN IF EXISTS kind,
    DROP COLUMN IF EXISTS severity,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS dedupe_key,
    DROP COLUMN IF EXISTS action_url,
    DROP COLUMN IF EXISTS resolved_at;
