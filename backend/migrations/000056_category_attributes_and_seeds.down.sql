DROP TABLE IF EXISTS category_attribute_definitions CASCADE;
DROP TABLE IF EXISTS attribute_definitions CASCADE;
DROP TYPE IF EXISTS attribute_scope;
DROP TYPE IF EXISTS attribute_value_type;

DELETE FROM size_values WHERE size_system_id IN (SELECT id FROM size_systems WHERE code IN ('INT', 'ONE_SIZE'));
DELETE FROM size_systems WHERE code IN ('INT', 'ONE_SIZE');
DELETE FROM materials WHERE code IN ('COTTON', 'WOOL', 'POLYESTER', 'ELASTANE', 'LINEN', 'LEATHER');
DELETE FROM colors WHERE code IN ('BLACK', 'WHITE', 'GREY', 'BEIGE', 'BROWN', 'BLUE', 'GREEN', 'RED', 'PINK', 'PURPLE', 'YELLOW', 'ORANGE');
