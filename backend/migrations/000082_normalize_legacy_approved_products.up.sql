-- Normalize legacy approved products to published with a valid published_at timestamp.
-- Under canonical ZAMK contract, successful moderation approval transitions product directly to 'published'.
-- Storefront visibility is separately controlled by stock gating (free sellable stock >= 2).
UPDATE products
SET
    status = 'published',
    published_at = COALESCE(published_at, approved_at, updated_at, created_at, now()),
    updated_at = now()
WHERE status = 'approved';
