-- 000053_add_testlab_permission.up.sql
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('testing.manage')
) AS p(permission)
WHERE r.code IN ('owner', 'co_owner', 'admin')
ON CONFLICT DO NOTHING;
