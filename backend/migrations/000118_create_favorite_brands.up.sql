-- MARKETING.M1.11: buyers save favorite brands. Additive only: a new table, no backfill.
-- The reference is the stable brands.id (never a name or slug). Seller storefronts without a brands row are not favoritable.
CREATE TABLE favorite_brands (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    brand_id   UUID NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_favorite_brands_user_brand UNIQUE (user_id, brand_id)
);

CREATE INDEX idx_favorite_brands_user_created ON favorite_brands (user_id, created_at DESC);
