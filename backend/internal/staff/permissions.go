package staff

// PermissionStaffPermissionsManage is the canonical capability required to manage individual staff capabilities.
const PermissionStaffPermissionsManage = "staff.permissions.manage"

var allPermissions = []string{
	"analytics.read",
	"auctions.cancel",
	"auctions.create",
	"auctions.finalize",
	"auctions.manage_settings",
	"auctions.move_to_direct_sale",
	"auctions.pause",
	"auctions.publish",
	"auctions.read",
	"auctions.resume",
	"auctions.update",
	"audit.read",
	"brands.create",
	"brands.delete",
	"brands.read",
	"brands.update",
	"categories.create",
	"categories.delete",
	"categories.read",
	"categories.update",
	"commission.manage",
	"complaints.read",
	"complaints.resolve",
	"dashboard.read",
	"exports.excel",
	"inventory.adjust",
	"inventory.movements.read",
	"inventory.read",
	"inventory.receipt",
	"inventory.write_off",
	"orders.read",
	"orders.update_status",
	"payments.read",
	"payouts.approve",
	"payouts.create",
	"payouts.mark_paid",
	"payouts.read",
	"payouts.reject",
	"payouts.update",
	"products.approve",
	"products.block",
	"products.hide",
	"products.moderate",
	"products.publish",
	"products.read",
	"products.reject",
	"refunds.create",
	"refunds.read",
	"reports.read",
	"returns.read",
	"returns.update_status",
	"reviews.approve",
	"reviews.block",
	"reviews.hide",
	"reviews.read",
	"reviews.reject",
	"roles.manage",
	"roles.read",
	"security.read",
	"sellers.create_access",
	"sellers.message",
	"sellers.read",
	"sellers.update_status",
	"sellers.verify",
	"sellers.warn",
	"settings.manage",
	"settings.read",
	"shipments.create",
	"shipments.read",
	"shipments.update_status",
	"staff.block",
	"staff.create",
	"staff.permissions.manage",
	"staff.read",
	"staff.update",
	"storefront.manage",
	"support.close",
	"support.read",
	"support.respond",
	"testing.manage",
	"users.read",
	"warehouse.dispatch",
	"warehouse.packing",
	"warehouse.picking",
	"warehouse.receiving",
	"warehouse.returns",
}

var permissionSet map[string]struct{}

func init() {
	permissionSet = make(map[string]struct{}, len(allPermissions))
	for _, p := range allPermissions {
		permissionSet[p] = struct{}{}
	}
}

// AllPermissions returns a copy of all known valid permission capabilities.
func AllPermissions() []string {
	res := make([]string, len(allPermissions))
	copy(res, allPermissions)
	return res
}

// IsKnownPermission checks if the given capability is explicitly valid.
func IsKnownPermission(p string) bool {
	_, ok := permissionSet[p]
	return ok
}
