package router_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// storefrontFixture is an isolated storefront seller of one test: an active seller with its own primary brand and
// published, in-stock (FBO) products. Tests use it instead of picking whatever sellers happen to exist in zamk_test.
type storefrontFixture struct {
	SellerID   uuid.UUID
	SellerSlug string
	BrandName  string
	BrandID    uuid.UUID
	BrandSlug  string
	ProductIDs []uuid.UUID
}

// seedStorefrontSeller creates the fixture and removes exactly its own rows when the test ends.
func seedStorefrontSeller(t *testing.T, pool *pgxpool.Pool, products int) storefrontFixture {
	t.Helper()
	ctx := context.Background()
	tag := uuid.NewString()[:8]
	fx := storefrontFixture{SellerID: uuid.New(), SellerSlug: "sf-seller-" + tag, BrandName: "SF Brand " + tag, BrandID: uuid.New(), BrandSlug: "sf-brand-" + tag}
	catID := uuid.New()
	exec := func(sql string, args ...any) {
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, $2, $3, $4, 'active')`, fx.SellerID, fx.BrandName, fx.SellerSlug, fx.SellerSlug+"@test.local")
	exec(`INSERT INTO brands (id, name, slug, is_active) VALUES ($1, $2, $3, true)`, fx.BrandID, fx.BrandName, fx.BrandSlug)
	exec(`INSERT INTO seller_brands (id, seller_id, brand_id, relationship_type, is_primary, status) VALUES ($1, $2, $3, 'owner', true, 'active')`, uuid.New(), fx.SellerID, fx.BrandID)
	exec(`INSERT INTO categories (id, name, slug) VALUES ($1, $2, $3)`, catID, "SF Cat "+tag, "sf-cat-"+tag)
	for i := 0; i < products; i++ {
		pid, vid := uuid.New(), uuid.New()
		exec(`INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, description, status, currency, price_cents, main_image_url, published_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'Товар витрины для теста.', 'published', 'RUB', 300000, 'https://img.test/sf.jpg', now())`,
			pid, fx.SellerID, fx.BrandID, catID, fmt.Sprintf("SF Product %d %s", i+1, tag), fmt.Sprintf("sf-product-%d-%s", i+1, tag))
		exec(`INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, is_active) VALUES ($1, $2, $3, $3, $4, 300000, true)`,
			vid, pid, fmt.Sprintf("SF-%d-%s", i+1, tag), fmt.Sprintf("SFBC%d%s", i+1, tag))
		exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1, $2, $3, $4, 5, 0)`, uuid.New(), pid, vid, fx.SellerID)
		fx.ProductIDs = append(fx.ProductIDs, pid)
	}
	t.Cleanup(func() {
		c := context.Background()
		for _, q := range []string{
			`DELETE FROM inventory_items WHERE seller_id = $1`,
			`DELETE FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)`,
			`DELETE FROM products WHERE seller_id = $1`,
			`DELETE FROM seller_brands WHERE seller_id = $1`,
		} {
			pool.Exec(c, q, fx.SellerID)
		}
		pool.Exec(c, `DELETE FROM brands WHERE id = $1`, fx.BrandID)
		pool.Exec(c, `DELETE FROM categories WHERE id = $1`, catID)
		pool.Exec(c, `DELETE FROM sellers WHERE id = $1`, fx.SellerID)
	})
	return fx
}
