ALTER TABLE products ADD COLUMN IF NOT EXISTS vision_content_version BIGINT NOT NULL DEFAULT 0;

CREATE TABLE product_vision_runs (
    id UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    product_revision_id UUID REFERENCES product_revisions(id) ON DELETE SET NULL,
    vision_content_version BIGINT NOT NULL DEFAULT 0,
    provider VARCHAR(255) NOT NULL,
    model_id VARCHAR(255) NOT NULL,
    prompt_version VARCHAR(255) NOT NULL,
    schema_version VARCHAR(255) NOT NULL,
    vocabulary_version VARCHAR(255) NOT NULL,
    taxonomy_context_hash VARCHAR(255) NOT NULL,
    snapshot_hash VARCHAR(255) NOT NULL,
    snapshot_json JSONB NOT NULL,
    idempotency_key VARCHAR(512) NOT NULL UNIQUE,
    status VARCHAR(50) NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    claimed_at TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    result_observation JSONB,
    result_evaluation JSONB,
    usage_metadata JSONB,
    latency_ms INT,
    cost_cents INT,
    failure_code VARCHAR(100),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_product_vision_runs_product_id ON product_vision_runs(product_id);
CREATE INDEX idx_product_vision_runs_pending_claim ON product_vision_runs(status, next_attempt_at, created_at)
    WHERE status = 'pending';
CREATE INDEX idx_product_vision_runs_processing_stale ON product_vision_runs(status, claimed_at)
    WHERE status = 'processing';

CREATE TABLE product_current_visual_profiles (
    product_id UUID PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES product_vision_runs(id) ON DELETE CASCADE,
    run_created_at TIMESTAMPTZ NOT NULL,
    vision_content_version BIGINT NOT NULL DEFAULT 0,
    model_id VARCHAR(255) NOT NULL,
    snapshot_hash VARCHAR(255) NOT NULL,
    taxonomy_context_hash VARCHAR(255) NOT NULL,
    observation JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
