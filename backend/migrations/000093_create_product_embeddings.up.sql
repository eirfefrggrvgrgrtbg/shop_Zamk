CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE product_embeddings (
    product_id UUID PRIMARY KEY
        REFERENCES products(id)
        ON DELETE CASCADE,

    provider VARCHAR(50) NOT NULL,
    model VARCHAR(100) NOT NULL,
    dimensions INTEGER NOT NULL CHECK (dimensions = 1536),
    input_schema_version INTEGER NOT NULL CHECK (input_schema_version > 0),
    content_hash VARCHAR(64) NOT NULL CHECK (length(content_hash) = 64),

    embedding VECTOR(1536) NOT NULL,

    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
