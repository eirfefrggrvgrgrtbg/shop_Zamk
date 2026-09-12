CREATE TABLE staff_member_permissions (
    user_id UUID NOT NULL REFERENCES staff_members(user_id) ON DELETE CASCADE,
    permission TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, permission)
);

INSERT INTO staff_member_permissions (user_id, permission, created_at)
SELECT
    sm.user_id,
    srp.permission,
    NOW()
FROM staff_members sm
JOIN staff_role_permissions srp ON srp.role_id = sm.staff_role_id
ON CONFLICT DO NOTHING;
