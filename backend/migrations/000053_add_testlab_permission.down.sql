-- 000053_add_testlab_permission.down.sql
DELETE FROM staff_role_permissions WHERE permission = 'testing.manage';
