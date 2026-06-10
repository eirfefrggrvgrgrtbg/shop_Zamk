-- Seller status history
CREATE TABLE IF NOT EXISTS seller_status_history (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id     UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    old_status    TEXT,
    new_status    TEXT NOT NULL,
    reason        TEXT,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_seller_status_history_seller ON seller_status_history(seller_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_seller_status_history_actor ON seller_status_history(actor_user_id, created_at DESC);

-- Seller warnings
CREATE TABLE IF NOT EXISTS seller_warnings (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id        UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    type             TEXT NOT NULL,
    title            TEXT NOT NULL,
    message          TEXT NOT NULL,
    severity         TEXT NOT NULL DEFAULT 'medium' CHECK (severity IN ('low', 'medium', 'high')),
    status           TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'resolved', 'cancelled')),
    actor_user_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at      TIMESTAMPTZ,
    resolved_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    resolution_note  TEXT
);

CREATE INDEX IF NOT EXISTS idx_seller_warnings_seller ON seller_warnings(seller_id, created_at DESC);

-- Seller violations
CREATE TABLE IF NOT EXISTS seller_violations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id           UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    type                TEXT NOT NULL,
    title               TEXT NOT NULL,
    description         TEXT NOT NULL,
    severity            TEXT NOT NULL DEFAULT 'medium' CHECK (severity IN ('low', 'medium', 'high')),
    status              TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'resolved', 'cancelled')),
    counts_for_penalty  BOOLEAN NOT NULL DEFAULT TRUE,
    actor_user_id       UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at         TIMESTAMPTZ,
    resolved_by         UUID REFERENCES users(id) ON DELETE SET NULL,
    resolution_note     TEXT
);

CREATE INDEX IF NOT EXISTS idx_seller_violations_seller ON seller_violations(seller_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_seller_violations_penalty ON seller_violations(seller_id, counts_for_penalty, status);
