DELETE FROM staff_member_permissions
WHERE permission IN ('marketing.campaigns.read', 'marketing.campaigns.write');

DELETE FROM staff_role_permissions
WHERE permission IN ('marketing.campaigns.read', 'marketing.campaigns.write');
