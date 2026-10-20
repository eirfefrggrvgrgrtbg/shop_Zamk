ALTER TABLE sellers
    DROP COLUMN IF EXISTS cover_url,
    DROP COLUMN IF EXISTS cover_object_key,
    DROP COLUMN IF EXISTS history;

ALTER TABLE brands
    DROP COLUMN IF EXISTS cover_url,
    DROP COLUMN IF EXISTS cover_object_key,
    DROP COLUMN IF EXISTS history;
