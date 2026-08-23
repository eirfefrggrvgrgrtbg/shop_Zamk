DROP INDEX IF EXISTS product_images_single_main_idx;

ALTER TABLE product_images DROP COLUMN IF EXISTS is_main;
ALTER TABLE product_images DROP COLUMN IF EXISTS crop_height;
ALTER TABLE product_images DROP COLUMN IF EXISTS crop_width;
ALTER TABLE product_images DROP COLUMN IF EXISTS crop_y;
ALTER TABLE product_images DROP COLUMN IF EXISTS crop_x;
ALTER TABLE product_images DROP COLUMN IF EXISTS height;
ALTER TABLE product_images DROP COLUMN IF EXISTS width;
