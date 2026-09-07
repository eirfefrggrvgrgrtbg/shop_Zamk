-- Add physical return receiving permission
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('warehouse.returns')
) AS p(permission)
WHERE r.code IN ('owner', 'co_owner', 'admin', 'warehouse_operator')
ON CONFLICT DO NOTHING;
