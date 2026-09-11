ALTER TABLE notifications
    ADD COLUMN kind VARCHAR(16) NOT NULL DEFAULT 'event' CHECK (kind IN ('event', 'alert')),
    ADD COLUMN severity VARCHAR(16) NOT NULL DEFAULT 'info' CHECK (severity IN ('info', 'warning', 'critical')),
    ADD COLUMN status VARCHAR(16) NULL,
    ADD COLUMN dedupe_key VARCHAR(128) NULL,
    ADD COLUMN action_url VARCHAR(255) NULL,
    ADD COLUMN resolved_at TIMESTAMPTZ NULL;

ALTER TABLE notifications
    ADD CONSTRAINT chk_notifications_lifecycle CHECK (
        (kind = 'event' AND status IS NULL AND resolved_at IS NULL) OR
        (kind = 'alert' AND status IS NOT NULL AND (
            (status = 'active' AND resolved_at IS NULL) OR
            (status = 'resolved' AND resolved_at IS NOT NULL)
        ))
    );

ALTER TABLE notifications
    ADD CONSTRAINT chk_seller_alert_dedupe CHECK (
        kind != 'alert' OR recipient_kind != 'seller' OR (dedupe_key IS NOT NULL AND length(trim(dedupe_key)) > 0 AND recipient_seller_id IS NOT NULL)
    );

CREATE UNIQUE INDEX idx_notifications_active_seller_alert
    ON notifications(recipient_seller_id, dedupe_key)
    WHERE kind = 'alert' AND status = 'active';
