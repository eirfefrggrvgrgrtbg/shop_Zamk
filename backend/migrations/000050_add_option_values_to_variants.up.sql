ALTER TABLE product_variants ADD COLUMN option_values JSONB;

-- Backfill legacy options to prevent data loss
UPDATE product_variants
SET option_values = jsonb_strip_nulls(jsonb_build_object(
    'Size', size,
    'Color', color
))
WHERE size IS NOT NULL OR color IS NOT NULL;
