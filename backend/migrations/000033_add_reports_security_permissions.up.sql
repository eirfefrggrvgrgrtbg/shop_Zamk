-- Add reports.read and security.read permissions to existing roles

INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('reports.read'),
    ('security.read')
) AS p(permission)
WHERE r.code IN ('owner', 'co_owner', 'admin')
ON CONFLICT DO NOTHING;

INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('reports.read')
) AS p(permission)
WHERE r.code IN ('finance', 'content_manager', 'support')
ON CONFLICT DO NOTHING;
