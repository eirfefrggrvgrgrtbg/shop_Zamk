CREATE TABLE variant_stock_forecasts (
    product_variant_id UUID PRIMARY KEY REFERENCES product_variants(id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('insufficient_data', 'no_sales', 'healthy', 'warning', 'critical')),
    days_of_cover NUMERIC,
    calculated_at TIMESTAMPTZ NOT NULL
);
