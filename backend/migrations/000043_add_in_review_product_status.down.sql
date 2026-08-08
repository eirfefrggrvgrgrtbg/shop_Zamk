ALTER TABLE products DROP COLUMN IF EXISTS review_started_at;
ALTER TABLE products DROP COLUMN IF EXISTS assigned_admin_user_id;

ALTER TABLE products DROP CONSTRAINT IF EXISTS valid_status;
ALTER TABLE products ADD CONSTRAINT valid_status CHECK (
    status IN ('draft', 'pending_moderation', 'approved', 'published', 'rejected', 'hidden', 'blocked', 'out_of_stock')
);
