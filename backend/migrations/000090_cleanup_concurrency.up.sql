ALTER TABLE product_media_cleanup_jobs
ADD COLUMN generation BIGINT NOT NULL DEFAULT 1,
ADD COLUMN lease_token UUID NULL,
ADD COLUMN lease_until TIMESTAMPTZ NULL;
