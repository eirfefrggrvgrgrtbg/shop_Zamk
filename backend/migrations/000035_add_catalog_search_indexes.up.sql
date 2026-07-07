-- SEARCH-1: Additional indexes for catalog search/filter performance
-- price_cents index for range filters and price sorting
CREATE INDEX IF NOT EXISTS products_price_cents_idx ON products(price_cents);

-- Composite index for published products by price (most common public query pattern)
CREATE INDEX IF NOT EXISTS products_status_price_cents_idx ON products(status, price_cents) WHERE status = 'published';

-- Size filter on product_variants
CREATE INDEX IF NOT EXISTS product_variants_size_idx ON product_variants(size) WHERE size IS NOT NULL AND is_active = true;
