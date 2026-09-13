package staff_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/stretchr/testify/require"
)

func TestPermissions_KnownCapabilities(t *testing.T) {
	// Prove that staff.permissions.manage is known.
	require.True(t, staff.IsKnownPermission(staff.PermissionStaffPermissionsManage))
	require.True(t, staff.IsKnownPermission("staff.read"))
	require.True(t, staff.IsKnownPermission("products.approve"))
	require.True(t, staff.IsKnownPermission("warehouse.picking"))
	require.True(t, staff.IsKnownPermission("testing.manage"))
	require.True(t, staff.IsKnownPermission("payouts.create"))
	require.True(t, staff.IsKnownPermission("auctions.resume"))

	// Prove unknown strings are rejected
	require.False(t, staff.IsKnownPermission("unknown.permission"))
	require.False(t, staff.IsKnownPermission("staff.permissions.manage.wildcard"))
	require.False(t, staff.IsKnownPermission(""))
	require.False(t, staff.IsKnownPermission("admin"))

	all := staff.AllPermissions()
	require.NotEmpty(t, all)
	require.Equal(t, 86, len(all))
	require.Contains(t, all, staff.PermissionStaffPermissionsManage)
}

func TestPermissions_AllMigrationsCompleteness(t *testing.T) {
	// Discover all migration files
	migFiles, err := filepath.Glob("../../migrations/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, migFiles)

	// regex for permission values in migrations: ('some.permission')
	re := regexp.MustCompile(`\('([a-z0-9_]+\.[a-z0-9_]+(?:\.[a-z0-9_]+)?)'\)`)

	seeded := make(map[string]string)
	for _, f := range migFiles {
		content, err := os.ReadFile(f)
		require.NoError(t, err)
		matches := re.FindAllStringSubmatch(string(content), -1)
		for _, m := range matches {
			seeded[m[1]] = filepath.Base(f)
		}
	}

	for perm, file := range seeded {
		require.True(t, staff.IsKnownPermission(perm), "migration %s seeded permission %q is missing from canonical registry", file, perm)
	}
}

func TestPermissions_RouterCompleteness(t *testing.T) {
	routerContent, err := os.ReadFile("../http/router/router.go")
	require.NoError(t, err)
	routerCode := string(routerContent)

	// Extract all string literals passed to perm(...) and permAny(...) and RequirePermission(...)
	rePerm := regexp.MustCompile(`(?:perm|permAny|RequirePermission)\(([^)]+)\)`)
	reStringLiteral := regexp.MustCompile(`"([a-z0-9_.]+)"`)

	var routerPerms []string
	for _, m := range rePerm.FindAllStringSubmatch(routerCode, -1) {
		argList := m[1]
		for _, strMatch := range reStringLiteral.FindAllStringSubmatch(argList, -1) {
			routerPerms = append(routerPerms, strMatch[1])
		}
	}

	require.NotEmpty(t, routerPerms)
	for _, p := range routerPerms {
		require.True(t, staff.IsKnownPermission(p), "router permission %q is missing from canonical registry", p)
	}
}
