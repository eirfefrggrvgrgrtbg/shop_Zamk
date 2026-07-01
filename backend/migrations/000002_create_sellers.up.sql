CREATE TABLE sellers (
    id UUID PRIMARY KEY,
    brand_name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT,
    contact_email TEXT NOT NULL,
    contact_phone TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_status CHECK (status IN ('pending', 'active', 'blocked', 'archived'))
);

CREATE TABLE seller_users (
    id UUID PRIMARY KEY,
    seller_id UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'owner',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_role CHECK (role IN ('owner', 'manager')),
    UNIQUE (user_id),
    UNIQUE (seller_id, user_id)
);

CREATE INDEX sellers_slug_idx ON sellers(slug);
CREATE INDEX sellers_status_idx ON sellers(status);
CREATE INDEX seller_users_seller_id_idx ON seller_users(seller_id);
CREATE INDEX seller_users_user_id_idx ON seller_users(user_id);
