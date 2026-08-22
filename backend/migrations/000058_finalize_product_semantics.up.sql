ALTER TABLE attribute_definitions ADD COLUMN value_source TEXT NOT NULL DEFAULT 'TEXT';
ALTER TABLE attribute_definitions ADD CONSTRAINT valid_value_source CHECK (value_source IN ('VARIANT_COLOR', 'VARIANT_SIZE', 'MATERIAL_COMPOSITION', 'DICTIONARY', 'TEXT', 'NUMBER', 'BOOLEAN'));

UPDATE attribute_definitions SET value_source = 'VARIANT_COLOR' WHERE code = 'COLOR';
UPDATE attribute_definitions SET value_source = 'VARIANT_SIZE' WHERE code = 'SIZE';
UPDATE attribute_definitions SET value_source = 'MATERIAL_COMPOSITION' WHERE code = 'MATERIAL_COMPOSITION';

CREATE TABLE category_size_systems (
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    size_system_id UUID NOT NULL REFERENCES size_systems(id) ON DELETE CASCADE,
    is_default BOOLEAN NOT NULL DEFAULT false,
    sort_order INT NOT NULL DEFAULT 0,
    PRIMARY KEY (category_id, size_system_id)
);
