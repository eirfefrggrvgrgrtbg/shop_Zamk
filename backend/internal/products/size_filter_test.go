package products_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPublicSizeFilter(t *testing.T) {
	db, _, _ := setupBlockATestDB(t)
	pool := db.Pool
	ctx := context.Background()
	svc := products.NewService(products.NewRepository(pool), sellers.NewRepository(pool), db, nil, nil)

	tag := uuid.NewString()[:6]
	label := func(s string) string { return s + tag }
	sysA, sysB := uuid.New(), uuid.New()
	codeA, codeB := "fa-"+tag, "fb-"+tag
	for _, s := range []struct {
		id         uuid.UUID
		code, name string
	}{{sysA, codeA, "A"}, {sysB, codeB, "B"}} {
		_, err := pool.Exec(ctx, `INSERT INTO size_systems (id, code, name) VALUES ($1,$2,$3)`, s.id, s.code, s.name)
		require.NoError(t, err)
	}
	sv := map[string]uuid.UUID{}
	addSize := func(key string, sys uuid.UUID, value string) {
		id := uuid.New()
		sv[key] = id
		_, err := pool.Exec(ctx, `INSERT INTO size_values (id, size_system_id, value, sort_order) VALUES ($1,$2,$3,0)`, id, sys, value)
		require.NoError(t, err)
	}
	addSize("A-S", sysA, label("S"))
	addSize("A-M", sysA, label("M"))
	addSize("A-L", sysA, label("L"))
	addSize("B-S", sysB, label("S")) // same label, different system
	addSize("B-38", sysB, label("38"))

	activeSeller, blockedSeller := uuid.New(), uuid.New()
	for id, st := range map[uuid.UUID]string{activeSeller: "active", blockedSeller: "blocked"} {
		_, err := pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1,'SF Seller',$2,'t@t.com',$3)`, id, "sf-"+id.String()[:8], st)
		require.NoError(t, err)
	}
	brand, otherBrand := uuid.New(), uuid.New()
	for _, b := range []uuid.UUID{brand, otherBrand} {
		_, err := pool.Exec(ctx, `INSERT INTO brands (id, name, slug) VALUES ($1,'SF Brand',$2)`, b, "sf-"+b.String()[:8])
		require.NoError(t, err)
	}

	type v struct {
		size     string
		free     int
		reserved int
		inactive bool
	}
	mk := func(seller, br uuid.UUID, status string, variants ...v) uuid.UUID {
		pid := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO products (id, seller_id, brand_id, title, slug, status, source, currency, price_cents, published_at, created_at, updated_at)
			VALUES ($1,$2,$3,'SF item',$4,$5,'draft','RUB',1000,now(),now(),now())`, pid, seller, br, pid.String(), status)
		require.NoError(t, err)
		for i, x := range variants {
			vid := uuid.New()
			_, err = pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, size_value_id, seller_sku, is_active) VALUES ($1,$2,$3,$4,$5)`,
				vid, pid, sv[x.size], fmt.Sprintf("sf-%s-%d", pid.String()[:8], i), !x.inactive)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock) VALUES (gen_random_uuid(),$1,$2,$3,$4,$5)`,
				pid, seller, vid, x.free+x.reserved, x.reserved)
			require.NoError(t, err)
		}
		return pid
	}

	// p1: S available, M sold out (fully reserved), L available
	p1 := mk(activeSeller, brand, "published", v{"A-S", 5, 0, false}, v{"A-M", 0, 3, false}, v{"A-L", 4, 0, false})
	// p2: M and L available
	p2 := mk(activeSeller, brand, "published", v{"A-M", 5, 0, false}, v{"A-L", 5, 0, false})
	// p3: other system only (S and 38)
	p3 := mk(activeSeller, otherBrand, "published", v{"B-S", 5, 0, false}, v{"B-38", 5, 0, false})
	// p4: unpublished / blocked seller with S
	mk(activeSeller, brand, "draft", v{"A-S", 5, 0, false}, v{"A-L", 5, 0, false})
	mk(blockedSeller, brand, "published", v{"A-S", 5, 0, false}, v{"A-L", 5, 0, false})
	// p6: S exists but variant inactive; M available
	p6 := mk(activeSeller, brand, "published", v{"A-S", 5, 0, true}, v{"A-M", 5, 0, false})

	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM products WHERE brand_id IN ($1,$2)`, brand, otherBrand)
		pool.Exec(c, `DELETE FROM brands WHERE id IN ($1,$2)`, brand, otherBrand)
		pool.Exec(c, `DELETE FROM sellers WHERE id IN ($1,$2)`, activeSeller, blockedSeller)
		pool.Exec(c, `DELETE FROM size_systems WHERE id IN ($1,$2)`, sysA, sysB)
	})

	list := func(f products.PublicProductFilter, limit, offset int) ([]string, int) {
		resp, err := svc.ListPublicProducts(ctx, f, limit, offset)
		require.NoError(t, err)
		ids := []string{}
		for _, it := range resp.Items {
			ids = append(ids, it.ID.String())
		}
		return ids, resp.TotalCount
	}
	ids := func(ps ...uuid.UUID) []string {
		out := []string{}
		for _, p := range ps {
			out = append(out, p.String())
		}
		return out
	}
	bothBrands := []uuid.UUID{brand, otherBrand}

	t.Run("size present and available", func(t *testing.T) {
		got, total := list(products.PublicProductFilter{BrandIDs: bothBrands, Sizes: []string{label("S")}, SizeSystem: &codeA}, 50, 0)
		require.ElementsMatch(t, ids(p1), got) // p6's S variant is inactive; draft/blocked excluded
		require.Equal(t, 1, total)
	})
	t.Run("size exists but sold out", func(t *testing.T) {
		got, total := list(products.PublicProductFilter{BrandIDs: bothBrands, Sizes: []string{label("M")}}, 50, 0)
		require.ElementsMatch(t, ids(p2, p6), got) // p1 has M but sold out
		require.Equal(t, 2, total)
	})
	t.Run("size absent", func(t *testing.T) {
		got, total := list(products.PublicProductFilter{BrandIDs: bothBrands, Sizes: []string{label("XXL")}}, 50, 0)
		require.Empty(t, got)
		require.Equal(t, 0, total)
	})
	t.Run("several sizes: union without duplicates, exact totalCount", func(t *testing.T) {
		got, total := list(products.PublicProductFilter{BrandIDs: bothBrands, Sizes: []string{label("S"), label("M"), label("L")}, SizeSystem: &codeA}, 50, 0)
		require.ElementsMatch(t, ids(p1, p2, p6), got) // p1 matches via S and L but appears once
		require.Equal(t, 3, total)
	})
	t.Run("incompatible size systems are not mixed", func(t *testing.T) {
		got, _ := list(products.PublicProductFilter{BrandIDs: bothBrands, Sizes: []string{label("S")}, SizeSystem: &codeB}, 50, 0)
		require.ElementsMatch(t, ids(p3), got)
		got, _ = list(products.PublicProductFilter{BrandIDs: bothBrands, Sizes: []string{label("38")}, SizeSystem: &codeA}, 50, 0)
		require.Empty(t, got)
	})
	t.Run("pagination applies after filtering with no duplicates", func(t *testing.T) {
		f := products.PublicProductFilter{BrandIDs: bothBrands, Sizes: []string{label("S"), label("M"), label("L")}, SizeSystem: &codeA}
		seen := map[string]bool{}
		for off := 0; off < 4; off += 2 {
			got, total := list(f, 2, off)
			require.Equal(t, 3, total)
			for _, id := range got {
				require.False(t, seen[id])
				seen[id] = true
			}
		}
		require.Len(t, seen, 3)
	})
	t.Run("combines with brand and price filters", func(t *testing.T) {
		b := brand
		got, _ := list(products.PublicProductFilter{BrandID: &b, Sizes: []string{label("S")}}, 50, 0)
		require.ElementsMatch(t, ids(p1), got)
		o := otherBrand
		got, _ = list(products.PublicProductFilter{BrandID: &o, Sizes: []string{label("L")}}, 50, 0)
		require.Empty(t, got)
		minP := int64(5000)
		got, _ = list(products.PublicProductFilter{BrandIDs: bothBrands, Sizes: []string{label("S")}, MinPriceCents: &minP}, 50, 0)
		require.Empty(t, got)
	})
	t.Run("filter never changes stock", func(t *testing.T) {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE(SUM(total_stock),0) FROM inventory_items WHERE product_id=$1`, p1).Scan(&n))
		require.Equal(t, 5+3+4, n)
	})
	t.Run("query parsing accepts repeated and comma separated sizes", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/api/public/products?size=S,M&size=L&size=S&sizeSystem=INT", nil)
		f, _, ferr := products.ParsePublicProductFilter(r)
		require.Nil(t, ferr)
		require.Equal(t, []string{"S", "M", "L"}, f.Sizes)
		require.Equal(t, "INT", *f.SizeSystem)
	})
}
