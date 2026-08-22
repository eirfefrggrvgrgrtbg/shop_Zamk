CREATE TABLE attribute_dictionaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name_ru TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE attribute_dictionary_values (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_id UUID NOT NULL REFERENCES attribute_dictionaries(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    name_ru TEXT NOT NULL,
    display_metadata JSONB,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dictionary_id, code)
);

ALTER TABLE category_attribute_definitions DROP COLUMN dictionary_id;
ALTER TABLE category_attribute_definitions ADD COLUMN dictionary_id UUID REFERENCES attribute_dictionaries(id) ON DELETE SET NULL;

CREATE TABLE product_attribute_values (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    attribute_definition_id UUID NOT NULL REFERENCES attribute_definitions(id) ON DELETE CASCADE,
    enum_value_id UUID REFERENCES attribute_dictionary_values(id) ON DELETE SET NULL,
    text_value TEXT,
    number_value NUMERIC,
    bool_value BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE variant_attribute_values (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    attribute_definition_id UUID NOT NULL REFERENCES attribute_definitions(id) ON DELETE CASCADE,
    enum_value_id UUID REFERENCES attribute_dictionary_values(id) ON DELETE SET NULL,
    text_value TEXT,
    number_value NUMERIC,
    bool_value BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seeds
INSERT INTO size_systems (id, code, name, is_active) VALUES (gen_random_uuid(), 'EU', 'European', true) ON CONFLICT (code) DO NOTHING;

DO $$
DECLARE
    sys_eu UUID;
BEGIN
    SELECT id INTO sys_eu FROM size_systems WHERE code = 'EU';
    IF sys_eu IS NOT NULL THEN
        INSERT INTO size_values (id, size_system_id, value, sort_order, is_active) VALUES
        (gen_random_uuid(), sys_eu, '35', 10, true),
        (gen_random_uuid(), sys_eu, '36', 20, true),
        (gen_random_uuid(), sys_eu, '37', 30, true),
        (gen_random_uuid(), sys_eu, '38', 40, true),
        (gen_random_uuid(), sys_eu, '39', 50, true),
        (gen_random_uuid(), sys_eu, '40', 60, true),
        (gen_random_uuid(), sys_eu, '41', 70, true),
        (gen_random_uuid(), sys_eu, '42', 80, true),
        (gen_random_uuid(), sys_eu, '43', 90, true),
        (gen_random_uuid(), sys_eu, '44', 100, true),
        (gen_random_uuid(), sys_eu, '45', 110, true),
        (gen_random_uuid(), sys_eu, '46', 120, true)
        ON CONFLICT (size_system_id, value) DO NOTHING;
    END IF;
END $$;

-- 3. SEED REAL ATTRIBUTE DEFINITIONS
INSERT INTO attribute_definitions (id, code, name_ru, value_type, scope, is_active) VALUES
(gen_random_uuid(), 'COLOR', 'Цвет', 'ENUM', 'VARIANT', true),
(gen_random_uuid(), 'SIZE', 'Размер', 'ENUM', 'VARIANT', true),
(gen_random_uuid(), 'MATERIAL_COMPOSITION', 'Состав', 'COMPOSITION', 'PRODUCT', true)
ON CONFLICT (code) DO NOTHING;
