CREATE TABLE product_media_staging (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    client_media_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'uploading',
    object_key VARCHAR(1024) NOT NULL,
    image_url VARCHAR(1024) NOT NULL,
    content_sha256 VARCHAR(64) NOT NULL,
    byte_size BIGINT NOT NULL,
    width INT NOT NULL,
    height INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    consumed_at TIMESTAMPTZ,

    CONSTRAINT status_check CHECK (status IN ('uploading', 'ready', 'consumed')),
    CONSTRAINT dimensions_check CHECK (byte_size > 0 AND width > 0 AND height > 0),
    CONSTRAINT consumed_check CHECK (
        (status = 'consumed' AND consumed_at IS NOT NULL) OR
        (status != 'consumed' AND consumed_at IS NULL)
    ),
    CONSTRAINT sha256_format_check CHECK (content_sha256 ~ '^[a-f0-9]{64}$'),
    UNIQUE (seller_id, product_id, client_media_id)
);

CREATE INDEX idx_product_media_staging_ttl ON product_media_staging(status, created_at, consumed_at);

CREATE TABLE product_media_cleanup_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    object_key VARCHAR(1024) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT
);
