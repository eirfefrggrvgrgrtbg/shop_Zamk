CREATE TABLE marketing_saved_queries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0 AND length(name) <= 120),
    description TEXT CHECK (description IS NULL OR length(description) <= 500),
    query_version INTEGER NOT NULL CHECK (query_version > 0),
    query_spec JSONB NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_marketing_saved_queries_owner_name
    ON marketing_saved_queries (created_by, lower(name));
