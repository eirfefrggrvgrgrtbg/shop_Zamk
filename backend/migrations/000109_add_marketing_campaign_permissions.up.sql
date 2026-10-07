-- Add marketing permissions to canonical administrative roles
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('marketing.campaigns.read'),
    ('marketing.campaigns.write')
) AS p(permission)
WHERE r.code IN ('owner', 'co_owner', 'admin')
ON CONFLICT DO NOTHING;

-- Backfill staff_member_permissions for existing users with owner, co_owner, admin roles
INSERT INTO staff_member_permissions (user_id, permission, created_at)
SELECT sm.user_id, p.permission, now()
FROM staff_members sm
JOIN staff_roles sr ON sr.id = sm.staff_role_id
CROSS JOIN (VALUES
    ('marketing.campaigns.read'),
    ('marketing.campaigns.write')
) AS p(permission)
WHERE sr.code IN ('owner', 'co_owner', 'admin')
ON CONFLICT (user_id, permission) DO NOTHING;
