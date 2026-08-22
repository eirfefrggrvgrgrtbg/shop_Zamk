DROP INDEX IF EXISTS product_variants_canonical_combination_idx;
DROP INDEX IF EXISTS product_variants_barcode_idx;

ALTER TABLE product_variants DROP COLUMN IF EXISTS seller_sku;
ALTER TABLE product_variants DROP COLUMN IF EXISTS color_id;
ALTER TABLE product_variants DROP COLUMN IF EXISTS size_value_id;
ALTER TABLE product_variants DROP COLUMN IF EXISTS shade_name;

ALTER TABLE products DROP COLUMN IF EXISTS live_revision_id;
ALTER TABLE categories DROP COLUMN IF EXISTS size_chart_required;

DROP TABLE IF EXISTS product_revisions;
DROP TABLE IF EXISTS product_size_chart_rows;
DROP TABLE IF EXISTS product_size_charts;
DROP TABLE IF EXISTS product_material_composition;
DROP TABLE IF EXISTS category_size_chart_fields;
DROP TABLE IF EXISTS size_values;
DROP TABLE IF EXISTS size_systems;
DROP TABLE IF EXISTS materials;
DROP TABLE IF EXISTS colors;
