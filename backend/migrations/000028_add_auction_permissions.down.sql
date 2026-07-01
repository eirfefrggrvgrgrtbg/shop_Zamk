DELETE FROM staff_role_permissions
WHERE permission IN (
    'auctions.read', 'auctions.create', 'auctions.update', 
    'auctions.publish', 'auctions.pause', 'auctions.cancel', 
    'auctions.finalize', 'auctions.manage_settings', 'auctions.move_to_direct_sale'
);
