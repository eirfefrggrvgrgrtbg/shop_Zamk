DROP TABLE IF EXISTS seller_brands CASCADE;
DROP TABLE IF EXISTS seller_onboarding_applications CASCADE;

DELETE FROM staff_role_permissions WHERE permission IN ('sellers.create_access', 'sellers.read', 'sellers.update_status');
