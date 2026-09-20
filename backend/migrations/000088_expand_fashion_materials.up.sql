-- Reorder existing materials to follow: natural -> regenerated -> synthetic -> specialty
UPDATE materials SET sort_order = 100 WHERE code = 'COTTON';
UPDATE materials SET sort_order = 120 WHERE code = 'WOOL';
UPDATE materials SET sort_order = 300 WHERE code = 'POLYESTER';
UPDATE materials SET sort_order = 330 WHERE code = 'ELASTANE';
UPDATE materials SET sort_order = 110 WHERE code = 'LINEN';
UPDATE materials SET sort_order = 400 WHERE code = 'LEATHER';

-- Insert new canonical materials
INSERT INTO materials (id, code, name_ru, sort_order, is_active) VALUES
(gen_random_uuid(), 'MERINO_WOOL', 'Мериносовая шерсть', 130, true),
(gen_random_uuid(), 'CASHMERE', 'Кашемир', 140, true),
(gen_random_uuid(), 'SILK', 'Шелк', 150, true),
(gen_random_uuid(), 'ALPACA', 'Альпака', 160, true),
(gen_random_uuid(), 'MOHAIR', 'Мохер', 170, true),
(gen_random_uuid(), 'ANGORA', 'Ангора', 180, true),
(gen_random_uuid(), 'HEMP', 'Конопля', 190, true),
(gen_random_uuid(), 'RAMIE', 'Рами', 195, true),
(gen_random_uuid(), 'VISCOSE', 'Вискоза', 200, true),
(gen_random_uuid(), 'MODAL', 'Модал', 210, true),
(gen_random_uuid(), 'LYOCELL', 'Лиоцелл', 220, true),
(gen_random_uuid(), 'CUPRO', 'Купро', 230, true),
(gen_random_uuid(), 'ACETATE', 'Ацетат', 240, true),
(gen_random_uuid(), 'POLYAMIDE', 'Полиамид', 310, true),
(gen_random_uuid(), 'ACRYLIC', 'Акрил', 320, true),
(gen_random_uuid(), 'POLYURETHANE', 'Полиуретан', 340, true),
(gen_random_uuid(), 'SUEDE', 'Замша', 410, true)
ON CONFLICT (code) DO NOTHING;
