-- Migration to add 'in_review' status to products table and add assignment fields

ALTER TABLE products DROP CONSTRAINT IF EXISTS valid_status;

ALTER TABLE products ADD CONSTRAINT valid_status CHECK (
    status IN ('draft', 'pending_moderation', 'in_review', 'approved', 'published', 'rejected', 'hidden', 'blocked', 'out_of_stock', 'archived')
);

ALTER TABLE products ADD COLUMN IF NOT EXISTS assigned_admin_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE products ADD COLUMN IF NOT EXISTS review_started_at TIMESTAMPTZ;
