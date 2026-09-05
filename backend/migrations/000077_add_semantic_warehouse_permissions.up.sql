-- Add semantic warehouse permissions
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('warehouse.receiving'),
    ('warehouse.picking'),
    ('warehouse.packing'),
    ('warehouse.dispatch')
) AS p(permission)
WHERE r.code IN ('owner', 'co_owner', 'admin', 'warehouse_operator')
ON CONFLICT DO NOTHING;
