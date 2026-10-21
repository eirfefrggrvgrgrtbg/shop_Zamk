-- CATALOG.C5.2: a staged photo (not yet a product_images row) can be cropped to 4:5 before its revision is approved.
-- Same column types as product_images (000062, 000063). All nullable: existing rows and the draft flow are unaffected.
ALTER TABLE product_media_staging ADD COLUMN IF NOT EXISTS crop_x NUMERIC(5,4);
ALTER TABLE product_media_staging ADD COLUMN IF NOT EXISTS crop_y NUMERIC(5,4);
ALTER TABLE product_media_staging ADD COLUMN IF NOT EXISTS crop_width NUMERIC(5,4);
ALTER TABLE product_media_staging ADD COLUMN IF NOT EXISTS crop_height NUMERIC(5,4);
ALTER TABLE product_media_staging ADD COLUMN IF NOT EXISTS rendition_url TEXT;
ALTER TABLE product_media_staging ADD COLUMN IF NOT EXISTS rendition_object_key TEXT;
