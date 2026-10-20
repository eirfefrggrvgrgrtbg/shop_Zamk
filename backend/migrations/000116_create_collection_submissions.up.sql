-- MARKETING.M1.7: sellers propose their own products for editorial collections; ZAMK staff accept or reject.
-- Additive only: a new table, no change to collections, collection_items or any existing data, no backfill.
-- Accepting a submission adds the product through the existing collection_items mechanism; the submission
-- itself never publishes anything.
CREATE TABLE collection_submissions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_id    UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    product_id       UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    seller_id        UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    status           TEXT NOT NULL DEFAULT 'pending',
    seller_comment   TEXT,
    rejection_reason TEXT,
    reviewed_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_cs_status CHECK (status IN ('pending', 'accepted', 'rejected')),
    CONSTRAINT chk_cs_comment_len CHECK (seller_comment IS NULL OR char_length(seller_comment) <= 300),
    CONSTRAINT chk_cs_reason_len CHECK (rejection_reason IS NULL OR char_length(rejection_reason) <= 300),
    -- a pending submission has no reviewer; a reviewed one always records when (who may be cleared if the user is deleted)
    CONSTRAINT chk_cs_review CHECK (
        (status = 'pending' AND reviewed_at IS NULL AND reviewed_by IS NULL AND rejection_reason IS NULL)
        OR (status <> 'pending' AND reviewed_at IS NOT NULL)
    )
);

-- At most one active (pending) submission per product/collection pair.
CREATE UNIQUE INDEX uq_collection_submissions_pending
    ON collection_submissions (collection_id, product_id) WHERE status = 'pending';
CREATE INDEX idx_collection_submissions_seller ON collection_submissions (seller_id, created_at DESC);
CREATE INDEX idx_collection_submissions_status ON collection_submissions (status, created_at DESC);
