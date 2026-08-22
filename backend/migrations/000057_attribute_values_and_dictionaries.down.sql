DELETE FROM attribute_definitions WHERE code IN ('COLOR', 'SIZE', 'MATERIAL_COMPOSITION');

DELETE FROM size_values WHERE size_system_id IN (SELECT id FROM size_systems WHERE code = 'EU');
DELETE FROM size_systems WHERE code = 'EU';

DROP TABLE IF EXISTS variant_attribute_values CASCADE;
DROP TABLE IF EXISTS product_attribute_values CASCADE;

ALTER TABLE category_attribute_definitions DROP COLUMN dictionary_id;
ALTER TABLE category_attribute_definitions ADD COLUMN dictionary_id TEXT;

DROP TABLE IF EXISTS attribute_dictionary_values CASCADE;
DROP TABLE IF EXISTS attribute_dictionaries CASCADE;
