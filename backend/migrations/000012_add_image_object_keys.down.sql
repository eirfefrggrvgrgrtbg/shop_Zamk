ALTER TABLE product_images DROP COLUMN IF EXISTS object_key;
ALTER TABLE products DROP COLUMN IF EXISTS main_image_object_key;
ALTER TABLE brands DROP COLUMN IF EXISTS logo_object_key;
ALTER TABLE sellers DROP COLUMN IF EXISTS logo_url;
ALTER TABLE sellers DROP COLUMN IF EXISTS logo_object_key;
