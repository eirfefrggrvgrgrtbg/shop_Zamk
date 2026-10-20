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

func TestPublicColorFilter(t *testing.T) {
	db, _, _ := setupBlockATestDB(t)
	pool := db.Pool
	ctx := context.Background()
	svc := products.NewService(products.NewRepository(pool), sellers.NewRepository(pool), db, nil, nil)
	tag := uuid.NewString()[:6]

	color := map[string]uuid.UUID{}
	for name, hex := range map[string]string{"black": "#000000", "white": "#FFFFFF", "red": "#FF0000", "unused": "#00FF00"} {
		id := uuid.New()
		color[name] = id
		_, err := pool.Exec(ctx, `INSERT INTO colors (id, code, name_ru, hex, sort_order) VALUES ($1,$2,$3,$4,0)`, id, "cf-"+name+"-"+tag, "CF "+name, hex)
		require.NoError(t, err)
	}
	sysID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO size_systems (id, code, name) VALUES ($1,$2,'CF')`, sysID, "cf-"+tag)
	require.NoError(t, err)
	size := map[string]uuid.UUID{}
	sizeLabel := map[string]string{"S": "S" + tag, "M": "M" + tag}
	for k, label := range sizeLabel {
		id := uuid.New()
		size[k] = id
		_, err = pool.Exec(ctx, `INSERT INTO size_values (id, size_system_id, value, sort_order) VALUES ($1,$2,$3,0)`, id, sysID, label)
		require.NoError(t, err)
	}
	activeSeller, blockedSeller := uuid.New(), uuid.New()
	for id, st := range map[uuid.UUID]string{activeSeller: "active", blockedSeller: "blocked"} {
		_, err = pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1,'CF Seller',$2,'t@t.com',$3)`, id, "cf-"+id.String()[:8], st)
		require.NoError(t, err)
	}
	brandA, brandB := uuid.New(), uuid.New()
	for _, b := range []uuid.UUID{brandA, brandB} {
		_, err = pool.Exec(ctx, `INSERT INTO brands (id, name, slug) VALUES ($1,'CF Brand',$2)`, b, "cf-"+b.String()[:8])
		require.NoError(t, err)
	}
	cat := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1,'CF cat',$2)`, cat, "cf-"+cat.String()[:8])
	require.NoError(t, err)

	type v struct {
		color, size string
		free, rsv   int
		legacyText  string
	}
	mk := func(seller, brand uuid.UUID, status string, price int, vs ...v) uuid.UUID {
		pid := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, status, source, currency, price_cents, published_at, created_at, updated_at)
			VALUES ($1,$2,$3,$4,'CF item',$5,$6,'draft','RUB',$7,now(),now(),now())`, pid, seller, brand, cat, pid.String(), status, price*100)
		require.NoError(t, err)
		for i, x := range vs {
			vid := uuid.New()
			var cid, sid *uuid.UUID
			if c, ok := color[x.color]; ok {
				cid = &c
			}
			if sz, ok := size[x.size]; ok {
				sid = &sz
			}
			var legacy *string
			if x.legacyText != "" {
				legacy = &x.legacyText
			}
			_, err = pool.Exec(ctx, `INSERT INTO product_variants (id, product_id, color_id, color, size_value_id, seller_sku, is_active) VALUES ($1,$2,$3,$4,$5,$6,true)`,
				vid, pid, cid, legacy, sid, fmt.Sprintf("cf-%s-%d", pid.String()[:8], i))
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock) VALUES (gen_random_uuid(),$1,$2,$3,$4,$5)`,
				pid, seller, vid, x.free+x.rsv, x.rsv)
			require.NoError(t, err)
		}
		return pid
	}

	// critical product: black/S and white/M both in stock
	p1 := mk(activeSeller, brandA, "published", 3000, v{"black", "S", 5, 0, ""}, v{"white", "M", 5, 0, ""})
	// black/M sold out, red/S available
	p2 := mk(activeSeller, brandA, "published", 5000, v{"black", "M", 0, 4, ""}, v{"red", "S", 5, 0, ""})
	// two matching black variants (S and M): must appear once
	p3 := mk(activeSeller, brandB, "published", 8000, v{"black", "S", 5, 0, ""}, v{"black", "M", 5, 0, ""})
	// white/S only
	p4 := mk(activeSeller, brandB, "published", 9000, v{"white", "S", 6, 0, ""})
	// not purchasable at all by color: black unpublished / blocked seller / legacy text only
	mk(activeSeller, brandA, "draft", 3000, v{"black", "S", 5, 0, ""}, v{"white", "S", 5, 0, ""})
	mk(blockedSeller, brandA, "published", 3000, v{"black", "S", 5, 0, ""}, v{"white", "S", 5, 0, ""})
	p7 := mk(activeSeller, brandA, "published", 3000, v{"", "S", 6, 0, "black"}) // free-text color: never matched by name

	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM products WHERE category_id=$1`, cat)
		pool.Exec(c, `DELETE FROM categories WHERE id=$1`, cat)
		pool.Exec(c, `DELETE FROM brands WHERE id IN ($1,$2)`, brandA, brandB)
		pool.Exec(c, `DELETE FROM sellers WHERE id IN ($1,$2)`, activeSeller, blockedSeller)
		pool.Exec(c, `DELETE FROM size_systems WHERE id=$1`, sysID)
		for _, id := range color {
			pool.Exec(c, `DELETE FROM colors WHERE id=$1`, id)
		}
	})
	_ = p7

	in := []uuid.UUID{brandA, brandB}
	list := func(f products.PublicProductFilter, limit, offset int) ([]string, int) {
		if f.BrandIDs == nil {
			f.BrandIDs = in
		}
		resp, err := svc.ListPublicProducts(ctx, f, limit, offset)
		require.NoError(t, err)
		out := []string{}
		for _, it := range resp.Items {
			out = append(out, it.ID.String())
		}
		return out, resp.TotalCount
	}
	ids := func(ps ...uuid.UUID) []string {
		out := []string{}
		for _, p := range ps {
			out = append(out, p.String())
		}
		return out
	}
	black, white := []uuid.UUID{color["black"]}, []uuid.UUID{color["white"]}

	t.Run("one color (purchasable variant only; unpublished, blocked and free-text excluded)", func(t *testing.T) {
		got, total := list(products.PublicProductFilter{ColorIDs: black}, 50, 0)
		require.ElementsMatch(t, ids(p1, p3), got) // p2's black variant is sold out
		require.Equal(t, 2, total)
	})
	t.Run("several colors are OR", func(t *testing.T) {
		got, total := list(products.PublicProductFilter{ColorIDs: []uuid.UUID{color["black"], color["red"]}}, 50, 0)
		require.ElementsMatch(t, ids(p1, p2, p3), got)
		require.Equal(t, 3, total)
	})
	t.Run("color that no purchasable variant has", func(t *testing.T) {
		got, total := list(products.PublicProductFilter{ColorIDs: []uuid.UUID{color["unused"]}}, 50, 0)
		require.Empty(t, got)
		require.Equal(t, 0, total)
		got, _ = list(products.PublicProductFilter{ColorIDs: []uuid.UUID{uuid.New()}}, 50, 0)
		require.Empty(t, got)
	})
	t.Run("color without stock is not matched", func(t *testing.T) {
		got, _ := list(products.PublicProductFilter{ColorIDs: []uuid.UUID{color["red"]}, Sizes: []string{sizeLabel["M"]}}, 50, 0)
		require.Empty(t, got)
		got, _ = list(products.PublicProductFilter{ColorIDs: black, Sizes: []string{sizeLabel["M"]}}, 50, 0)
		require.ElementsMatch(t, ids(p3), got) // p2 black/M is sold out
	})
	t.Run("CRITICAL: black + S matches, black + M does not for black/S + white/M", func(t *testing.T) {
		got, _ := list(products.PublicProductFilter{ColorIDs: black, Sizes: []string{sizeLabel["S"]}}, 50, 0)
		require.Contains(t, got, p1.String())
		got, _ = list(products.PublicProductFilter{ColorIDs: black, Sizes: []string{sizeLabel["M"]}}, 50, 0)
		require.NotContains(t, got, p1.String(), "black and M exist on different variants")
		got, _ = list(products.PublicProductFilter{ColorIDs: white, Sizes: []string{sizeLabel["M"]}}, 50, 0)
		require.Contains(t, got, p1.String())
		got, _ = list(products.PublicProductFilter{ColorIDs: white, Sizes: []string{sizeLabel["S"]}}, 50, 0)
		require.ElementsMatch(t, ids(p4), got) // p1 has white only in M
	})
	t.Run("color OR + size OR is still one-variant AND", func(t *testing.T) {
		got, _ := list(products.PublicProductFilter{ColorIDs: []uuid.UUID{color["black"], color["white"]}, Sizes: []string{sizeLabel["S"]}}, 50, 0)
		require.ElementsMatch(t, ids(p1, p3, p4), got)
	})
	t.Run("combines with brand, category and price", func(t *testing.T) {
		b := brandB
		got, _ := list(products.PublicProductFilter{ColorIDs: black, BrandID: &b, BrandIDs: []uuid.UUID{brandB}}, 50, 0)
		require.ElementsMatch(t, ids(p3), got)
		c := cat
		max := int64(4000 * 100)
		got, _ = list(products.PublicProductFilter{ColorIDs: black, CategoryID: &c, MaxPriceCents: &max}, 50, 0)
		require.ElementsMatch(t, ids(p1), got)
		other := uuid.New()
		got, _ = list(products.PublicProductFilter{ColorIDs: black, CategoryID: &other}, 50, 0)
		require.Empty(t, got)
	})
	t.Run("pagination: filtered before paging, exact totalCount, no duplicates (p3 has two black variants)", func(t *testing.T) {
		f := products.PublicProductFilter{ColorIDs: []uuid.UUID{color["black"], color["white"]}}
		seen := map[string]bool{}
		for off := 0; off < 3; off++ {
			got, total := list(f, 1, off)
			require.Equal(t, 3, total)
			require.Len(t, got, 1)
			require.False(t, seen[got[0]])
			seen[got[0]] = true
		}
		require.Len(t, seen, 3)
	})
	t.Run("query parsing", func(t *testing.T) {
		a, b := uuid.NewString(), uuid.NewString()
		r := httptest.NewRequest("GET", "/api/public/products?colorId="+a+","+b+"&colorId="+a+"&colorId=not-a-uuid", nil)
		f, _, ferr := products.ParsePublicProductFilter(r)
		require.Nil(t, ferr)
		require.Len(t, f.ColorIDs, 2)
	})
	t.Run("public colors list contains only colors present on purchasable eligible variants", func(t *testing.T) {
		cs, err := svc.ListPublicColors(ctx)
		require.NoError(t, err)
		got := map[uuid.UUID]bool{}
		for _, c := range cs {
			got[c.ID] = true
		}
		for _, name := range []string{"black", "white", "red"} {
			require.True(t, got[color[name]], name)
		}
		require.False(t, got[color["unused"]])
	})
}
