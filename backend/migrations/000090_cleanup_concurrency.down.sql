ALTER TABLE product_media_cleanup_jobs
DROP COLUMN IF EXISTS generation,
DROP COLUMN IF EXISTS lease_token,
DROP COLUMN IF EXISTS lease_until;
