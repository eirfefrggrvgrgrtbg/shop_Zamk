-- owner and co_owner get staff.permissions.manage preset
INSERT INTO staff_role_permissions (role_id, permission)
SELECT id, 'staff.permissions.manage'
FROM staff_roles
WHERE code IN ('owner', 'co_owner')
ON CONFLICT DO NOTHING;

-- Backfill staff_member_permissions for existing users with owner or co_owner roles
INSERT INTO staff_member_permissions (user_id, permission, created_at)
SELECT sm.user_id, 'staff.permissions.manage', now()
FROM staff_members sm
JOIN staff_roles sr ON sr.id = sm.staff_role_id
WHERE sr.code IN ('owner', 'co_owner')
ON CONFLICT (user_id, permission) DO NOTHING;
