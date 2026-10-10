DROP TABLE IF EXISTS product_current_visual_profiles;
DROP TABLE IF EXISTS product_vision_runs;
ALTER TABLE products DROP COLUMN IF EXISTS vision_content_version;
