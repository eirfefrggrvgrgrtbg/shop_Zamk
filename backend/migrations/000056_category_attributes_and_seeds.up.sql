CREATE TYPE attribute_value_type AS ENUM (
    'ENUM', 'MULTI_ENUM', 'TEXT', 'NUMBER', 'BOOLEAN', 'COMPOSITION'
);

CREATE TYPE attribute_scope AS ENUM (
    'PRODUCT', 'VARIANT'
);

CREATE TABLE attribute_definitions (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name_ru TEXT NOT NULL,
    value_type attribute_value_type NOT NULL,
    scope attribute_scope NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE category_attribute_definitions (
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    attribute_definition_id UUID NOT NULL REFERENCES attribute_definitions(id) ON DELETE CASCADE,
    required BOOLEAN NOT NULL DEFAULT false,
    filterable BOOLEAN NOT NULL DEFAULT false,
    variant_axis BOOLEAN NOT NULL DEFAULT false,
    sort_order INT NOT NULL DEFAULT 0,
    dictionary_id TEXT,
    min_values INT,
    max_values INT,
    PRIMARY KEY (category_id, attribute_definition_id)
);

-- Starter Data Seed

-- Colors
INSERT INTO colors (id, code, name_ru, hex, sort_order, is_active) VALUES
(gen_random_uuid(), 'BLACK', 'Черный', '#000000', 10, true),
(gen_random_uuid(), 'WHITE', 'Белый', '#FFFFFF', 20, true),
(gen_random_uuid(), 'GREY', 'Серый', '#808080', 30, true),
(gen_random_uuid(), 'BEIGE', 'Бежевый', '#F5F5DC', 40, true),
(gen_random_uuid(), 'BROWN', 'Коричневый', '#A52A2A', 50, true),
(gen_random_uuid(), 'BLUE', 'Синий', '#0000FF', 60, true),
(gen_random_uuid(), 'GREEN', 'Зеленый', '#008000', 70, true),
(gen_random_uuid(), 'RED', 'Красный', '#FF0000', 80, true),
(gen_random_uuid(), 'PINK', 'Розовый', '#FFC0CB', 90, true),
(gen_random_uuid(), 'PURPLE', 'Фиолетовый', '#800080', 100, true),
(gen_random_uuid(), 'YELLOW', 'Желтый', '#FFFF00', 110, true),
(gen_random_uuid(), 'ORANGE', 'Оранжевый', '#FFA500', 120, true)
ON CONFLICT (code) DO NOTHING;

-- Materials
INSERT INTO materials (id, code, name_ru, sort_order, is_active) VALUES
(gen_random_uuid(), 'COTTON', 'Хлопок', 10, true),
(gen_random_uuid(), 'WOOL', 'Шерсть', 20, true),
(gen_random_uuid(), 'POLYESTER', 'Полиэстер', 30, true),
(gen_random_uuid(), 'ELASTANE', 'Эластан', 40, true),
(gen_random_uuid(), 'LINEN', 'Лен', 50, true),
(gen_random_uuid(), 'LEATHER', 'Кожа', 60, true)
ON CONFLICT (code) DO NOTHING;

-- Size Systems
INSERT INTO size_systems (id, code, name, is_active) VALUES
(gen_random_uuid(), 'INT', 'International', true),
(gen_random_uuid(), 'ONE_SIZE', 'One Size', true)
ON CONFLICT (code) DO NOTHING;

-- Size Values
DO $$
DECLARE
    sys_int UUID;
    sys_one_size UUID;
BEGIN
    SELECT id INTO sys_int FROM size_systems WHERE code = 'INT';
    SELECT id INTO sys_one_size FROM size_systems WHERE code = 'ONE_SIZE';

    IF sys_int IS NOT NULL THEN
        INSERT INTO size_values (id, size_system_id, value, sort_order, is_active) VALUES
        (gen_random_uuid(), sys_int, 'XXS', 10, true),
        (gen_random_uuid(), sys_int, 'XS', 20, true),
        (gen_random_uuid(), sys_int, 'S', 30, true),
        (gen_random_uuid(), sys_int, 'M', 40, true),
        (gen_random_uuid(), sys_int, 'L', 50, true),
        (gen_random_uuid(), sys_int, 'XL', 60, true),
        (gen_random_uuid(), sys_int, 'XXL', 70, true)
        ON CONFLICT (size_system_id, value) DO NOTHING;
    END IF;

    IF sys_one_size IS NOT NULL THEN
        INSERT INTO size_values (id, size_system_id, value, sort_order, is_active) VALUES
        (gen_random_uuid(), sys_one_size, 'ONE_SIZE', 10, true)
        ON CONFLICT (size_system_id, value) DO NOTHING;
    END IF;
END $$;
