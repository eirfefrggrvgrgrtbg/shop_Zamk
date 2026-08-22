CREATE TABLE colors (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name_ru TEXT NOT NULL,
    hex TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE materials (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name_ru TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE size_systems (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE size_values (
    id UUID PRIMARY KEY,
    size_system_id UUID NOT NULL REFERENCES size_systems(id) ON DELETE CASCADE,
    value TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    UNIQUE (size_system_id, value)
);

CREATE TABLE category_size_chart_fields (
    id UUID PRIMARY KEY,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    unit TEXT,
    is_required BOOLEAN NOT NULL DEFAULT false,
    sort_order INT NOT NULL DEFAULT 0,
    UNIQUE(category_id, code)
);

CREATE TABLE product_material_composition (
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    material_id UUID NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    percentage NUMERIC(5,2) NOT NULL,
    PRIMARY KEY (product_id, material_id),
    CONSTRAINT valid_percentage CHECK (percentage > 0 AND percentage <= 100)
);

CREATE TABLE product_size_charts (
    id UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    UNIQUE (product_id)
);

CREATE TABLE product_size_chart_rows (
    size_chart_id UUID NOT NULL REFERENCES product_size_charts(id) ON DELETE CASCADE,
    size_value_id UUID NOT NULL REFERENCES size_values(id) ON DELETE RESTRICT,
    measurements JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (size_chart_id, size_value_id)
);

CREATE TABLE product_revisions (
    id UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    content_snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_revision_status CHECK (status IN ('pending', 'approved', 'rejected'))
);

ALTER TABLE categories ADD COLUMN IF NOT EXISTS size_chart_required BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE products ADD COLUMN IF NOT EXISTS live_revision_id UUID REFERENCES product_revisions(id) ON DELETE SET NULL;

ALTER TABLE product_variants ADD COLUMN IF NOT EXISTS seller_sku TEXT;
ALTER TABLE product_variants ADD COLUMN IF NOT EXISTS color_id UUID REFERENCES colors(id) ON DELETE RESTRICT;
ALTER TABLE product_variants ADD COLUMN IF NOT EXISTS size_value_id UUID REFERENCES size_values(id) ON DELETE RESTRICT;
ALTER TABLE product_variants ADD COLUMN IF NOT EXISTS shade_name TEXT;

-- Create partial unique index to avoid exact same active variant combination
CREATE UNIQUE INDEX product_variants_canonical_combination_idx 
    ON product_variants (product_id, color_id, size_value_id) 
    WHERE is_active = true AND color_id IS NOT NULL AND size_value_id IS NOT NULL;

-- Make barcode unique globally if present
CREATE UNIQUE INDEX product_variants_barcode_idx ON product_variants (barcode) WHERE barcode IS NOT NULL AND barcode != '';

-- Migrate existing SKU to seller_sku implicitly
UPDATE product_variants SET seller_sku = sku WHERE sku IS NOT NULL AND seller_sku IS NULL;
