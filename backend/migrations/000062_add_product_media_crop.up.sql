ALTER TABLE product_images ADD COLUMN width INT;
ALTER TABLE product_images ADD COLUMN height INT;

ALTER TABLE product_images ADD COLUMN crop_x NUMERIC(5,4);
ALTER TABLE product_images ADD COLUMN crop_y NUMERIC(5,4);
ALTER TABLE product_images ADD COLUMN crop_width NUMERIC(5,4);
ALTER TABLE product_images ADD COLUMN crop_height NUMERIC(5,4);

ALTER TABLE product_images ADD COLUMN is_main BOOLEAN NOT NULL DEFAULT false;

CREATE UNIQUE INDEX product_images_single_main_idx
    ON product_images (product_id)
    WHERE is_main = true;
