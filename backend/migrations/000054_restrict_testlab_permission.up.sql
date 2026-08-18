-- 000054_restrict_testlab_permission.up.sql
DELETE FROM staff_role_permissions
WHERE permission = 'testing.manage'
AND role_id IN (
    SELECT id FROM staff_roles WHERE code = 'admin'
);
