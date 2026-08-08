CREATE TABLE seller_onboarding_applications (
    id UUID PRIMARY KEY,
    seller_id UUID NOT NULL UNIQUE REFERENCES sellers(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    current_step INTEGER NOT NULL DEFAULT 1,
    payload JSONB NOT NULL DEFAULT '{}',
    review_comment TEXT,
    submitted_at TIMESTAMPTZ,
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT seller_onboarding_valid_status CHECK (status IN ('not_started', 'draft', 'pending_review', 'changes_requested', 'approved', 'rejected')),
    CONSTRAINT valid_current_step CHECK (current_step >= 1 AND current_step <= 6)
);

CREATE INDEX seller_onboarding_status_idx ON seller_onboarding_applications(status);
CREATE INDEX seller_onboarding_updated_idx ON seller_onboarding_applications(updated_at);

CREATE TABLE seller_brands (
    id UUID PRIMARY KEY,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    brand_id UUID NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    relationship_type TEXT NOT NULL DEFAULT 'owner',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (seller_id, brand_id),
    CONSTRAINT valid_relationship_type CHECK (relationship_type IN ('owner', 'authorized_reseller', 'distributor')),
    CONSTRAINT valid_seller_brand_status CHECK (status IN ('active', 'inactive', 'pending'))
);

CREATE INDEX seller_brands_seller_id_idx ON seller_brands(seller_id);
CREATE INDEX seller_brands_brand_id_idx ON seller_brands(brand_id);
CREATE UNIQUE INDEX seller_brands_primary_brand_idx ON seller_brands(seller_id) WHERE is_primary = true;

INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('sellers.create_access'),
    ('sellers.read'),
    ('sellers.update_status')
) AS p(permission)
WHERE r.code IN ('owner', 'co_owner', 'admin')
ON CONFLICT DO NOTHING;
