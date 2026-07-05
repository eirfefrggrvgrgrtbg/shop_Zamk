-- Add users.read permission to owner, co_owner, and admin roles
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('users.read')
) AS p(permission)
WHERE r.code IN ('owner', 'co_owner', 'admin', 'support')
ON CONFLICT DO NOTHING;
