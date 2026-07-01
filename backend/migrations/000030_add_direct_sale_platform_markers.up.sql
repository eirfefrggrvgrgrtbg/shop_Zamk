ALTER TABLE sellers ADD COLUMN is_platform BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE products ADD COLUMN source TEXT NOT NULL DEFAULT 'seller';

INSERT INTO sellers (
    id, brand_name, slug, description, contact_email, status, is_platform, created_at, updated_at
) VALUES (
    '00000000-0000-4000-8000-000000000000',
    'ZAMK',
    'zamk-platform',
    'Официальные товары и прямая продажа платформы ZAMK',
    'platform@zamk.local',
    'active',
    true,
    now(),
    now()
) ON CONFLICT (slug) DO UPDATE SET
    is_platform = true;
