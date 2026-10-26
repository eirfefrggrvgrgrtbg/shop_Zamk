// Package storefront holds the single definition of "a product is visible in the public Shop catalog". It has no
// dependencies so that products (catalog, seller views) and cart can share it without an import cycle.
package storefront

import "fmt"

// MinFreeSellableUnits is the canonical minimum of free sellable units across a product's active variants required
// for the product to be publicly visible. It is a visibility threshold for the product as a whole, not a per-SKU or
// per-purchase minimum.
const MinFreeSellableUnits = 2

// ProductFreeStockSQL returns the SQL expression for the canonical free sellable units across active variants of the
// product whose id is productIDExpr.
func ProductFreeStockSQL(productIDExpr string) string {
	return fmt.Sprintf(`(
		SELECT COALESCE(SUM(GREATEST(0, ii.total_stock - ii.reserved_stock)), 0)
		FROM product_variants pv
		LEFT JOIN inventory_items ii ON pv.id = ii.product_variant_id
		WHERE pv.product_id = %s AND pv.is_active = true
	)`, productIDExpr)
}

// EligibilitySQL returns the SQL predicate for canonical storefront eligibility: published status, active seller and
// free sellable stock meeting the minimum.
func EligibilitySQL(productAlias, sellerAlias string) string {
	return fmt.Sprintf(
		"%s.status = 'published' AND %s.status = 'active' AND %s >= %d",
		productAlias,
		sellerAlias,
		ProductFreeStockSQL(productAlias+".id"),
		MinFreeSellableUnits,
	)
}
