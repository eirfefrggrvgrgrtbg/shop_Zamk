CREATE TABLE password_reset_requests (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    user_id UUID REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users(id)
);
