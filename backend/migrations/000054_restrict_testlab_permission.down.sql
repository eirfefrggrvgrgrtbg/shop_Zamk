-- 000054_restrict_testlab_permission.down.sql
INSERT INTO staff_role_permissions (role_id, permission)
SELECT id, 'testing.manage'
FROM staff_roles
WHERE code = 'admin'
ON CONFLICT DO NOTHING;
