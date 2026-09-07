DELETE FROM staff_role_permissions
WHERE role_id IN (
    SELECT id FROM staff_roles WHERE code IN ('owner', 'co_owner', 'admin', 'warehouse_operator')
)
AND permission = 'warehouse.returns';
