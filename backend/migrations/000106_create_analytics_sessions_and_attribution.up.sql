CREATE TABLE analytics_sessions (
    id UUID PRIMARY KEY,
    visitor_id UUID NOT NULL,
    user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    landing_path VARCHAR(1024) NULL,
    referrer VARCHAR(1024) NULL,
    source VARCHAR(255) NULL,
    medium VARCHAR(255) NULL,
    utm_source VARCHAR(255) NULL,
    utm_medium VARCHAR(255) NULL,
    utm_campaign VARCHAR(255) NULL,
    utm_term VARCHAR(255) NULL,
    utm_content VARCHAR(255) NULL,
    campaign_id UUID NULL REFERENCES marketing_campaigns(id) ON DELETE SET NULL,
    attribution_captured_at TIMESTAMPTZ NULL,
    attribution_expires_at TIMESTAMPTZ NULL
);

CREATE INDEX idx_analytics_sessions_visitor_id ON analytics_sessions(visitor_id);
CREATE INDEX idx_analytics_sessions_user_id ON analytics_sessions(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_analytics_sessions_last_seen ON analytics_sessions(last_seen_at DESC);
CREATE INDEX idx_analytics_sessions_visitor_attr ON analytics_sessions(visitor_id, attribution_captured_at DESC) WHERE attribution_captured_at IS NOT NULL;

CREATE TABLE order_attributions (
    order_id UUID PRIMARY KEY REFERENCES orders(id) ON DELETE CASCADE,
    session_id UUID NULL REFERENCES analytics_sessions(id) ON DELETE SET NULL,
    visitor_id UUID NULL,
    source VARCHAR(255) NULL,
    medium VARCHAR(255) NULL,
    utm_source VARCHAR(255) NULL,
    utm_medium VARCHAR(255) NULL,
    utm_campaign VARCHAR(255) NULL,
    utm_term VARCHAR(255) NULL,
    utm_content VARCHAR(255) NULL,
    campaign_id UUID NULL REFERENCES marketing_campaigns(id) ON DELETE SET NULL,
    promo_code_id UUID NULL REFERENCES promo_codes(id) ON DELETE SET NULL,
    attributed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_attributions_session_id ON order_attributions(session_id) WHERE session_id IS NOT NULL;
CREATE INDEX idx_order_attributions_visitor_id ON order_attributions(visitor_id);

ALTER TABLE behavioral_events ADD COLUMN session_id UUID NULL;
CREATE INDEX idx_behav_session_time ON behavioral_events(session_id, occurred_at DESC) WHERE session_id IS NOT NULL;
