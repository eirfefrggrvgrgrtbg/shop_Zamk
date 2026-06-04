CREATE TABLE categories (
    id UUID PRIMARY KEY,
    parent_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX categories_slug_idx ON categories(slug);
CREATE INDEX categories_parent_id_idx ON categories(parent_id);

CREATE TABLE brands (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT,
    logo_url TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX brands_slug_idx ON brands(slug);

CREATE TABLE products (
    id UUID PRIMARY KEY,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    brand_id UUID REFERENCES brands(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'draft',
    gender TEXT,
    color TEXT,
    material TEXT,
    care_instructions TEXT,
    price_cents BIGINT NOT NULL,
    old_price_cents BIGINT,
    currency TEXT NOT NULL DEFAULT 'RUB',
    main_image_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at TIMESTAMPTZ,
    approved_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    rejected_at TIMESTAMPTZ,
    moderation_comment TEXT,
    CONSTRAINT valid_status CHECK (status IN ('draft', 'pending_moderation', 'approved', 'published', 'rejected', 'hidden', 'blocked', 'out_of_stock')),
    CONSTRAINT valid_price CHECK (price_cents >= 0),
    CONSTRAINT valid_old_price CHECK (old_price_cents IS NULL OR old_price_cents >= 0),
    CONSTRAINT valid_currency CHECK (currency = 'RUB')
);

CREATE INDEX products_seller_id_idx ON products(seller_id);
CREATE INDEX products_category_id_idx ON products(category_id);
CREATE INDEX products_brand_id_idx ON products(brand_id);
CREATE INDEX products_status_idx ON products(status);
CREATE INDEX products_slug_idx ON products(slug);
CREATE INDEX products_created_at_idx ON products(created_at);

CREATE TABLE product_variants (
    id UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku TEXT,
    size TEXT,
    color TEXT,
    barcode TEXT,
    price_cents BIGINT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX product_variants_product_id_idx ON product_variants(product_id);

CREATE TABLE product_images (
    id UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    image_url TEXT NOT NULL,
    alt_text TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX product_images_product_id_idx ON product_images(product_id);

CREATE TABLE product_moderation_logs (
    id UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    admin_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    from_status TEXT,
    to_status TEXT NOT NULL,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX product_moderation_logs_product_id_idx ON product_moderation_logs(product_id);
