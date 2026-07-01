INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('auctions.read'), ('auctions.create'), ('auctions.update'),
    ('auctions.publish'), ('auctions.pause'), ('auctions.cancel'),
    ('auctions.finalize'), ('auctions.manage_settings'), ('auctions.move_to_direct_sale')
) AS p(permission)
WHERE r.code IN ('owner', 'co_owner', 'admin')
ON CONFLICT DO NOTHING;
