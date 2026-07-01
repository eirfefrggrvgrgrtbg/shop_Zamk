ALTER TABLE product_images ADD COLUMN object_key TEXT;
ALTER TABLE products ADD COLUMN main_image_object_key TEXT;
ALTER TABLE brands ADD COLUMN logo_object_key TEXT;
ALTER TABLE sellers ADD COLUMN logo_url TEXT;
ALTER TABLE sellers ADD COLUMN logo_object_key TEXT;
