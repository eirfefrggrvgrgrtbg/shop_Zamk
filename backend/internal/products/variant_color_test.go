package products_test

import (
	"context"
	"strings"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestVariantColorAssignment(t *testing.T) {
	db, svc, owner := setupBlockATestDB(t)
	_, _, stranger := setupBlockATestDB(t)
	pool := db.Pool
	ctx := context.Background()
	tag := uuid.NewString()[:6]

	mkColor := func(name string, active bool) uuid.UUID {
		id := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO colors (id, code, name_ru, hex, sort_order, is_active) VALUES ($1,$2,$3,'#111111',0,$4)`, id, "vc-"+name+"-"+tag, "VC "+name, active)
		require.NoError(t, err)
		return id
	}
	black, white, retired := mkColor("black", true), mkColor("white", true), mkColor("retired", false)
	catID, sysID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1,'VC cat',$2)`, catID, "vc-"+tag)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO size_systems (id, code, name) VALUES ($1,$2,'VC')`, sysID, "vc-"+tag)
	require.NoError(t, err)
	sizeS, sizeM := uuid.New(), uuid.New()
	for id, v := range map[uuid.UUID]string{sizeS: "S" + tag, sizeM: "M" + tag} {
		_, err = pool.Exec(ctx, `INSERT INTO size_values (id, size_system_id, value, sort_order) VALUES ($1,$2,$3,0)`, id, sysID, v)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM categories WHERE id=$1`, catID)
		pool.Exec(c, `DELETE FROM size_systems WHERE id=$1`, sysID)
		pool.Exec(c, `DELETE FROM colors WHERE code LIKE '%-'||$1`, tag)
	})

	sku := func(s string) *string { x := s + "-" + tag; return &x }
	vr := func(colorID *uuid.UUID, size uuid.UUID, s string, price int64) products.ProductVariantRequest {
		return products.ProductVariantRequest{SellerSKU: sku(s), ColorID: colorID, SizeValueID: &size, PriceCents: ip(price)}
	}
	bl, wh := black, white
	shade := "Угольный"
	create := vr(&bl, sizeS, "S", 10000)
	create.ShadeName = &shade
	p, err := svc.CreateProductForSeller(ctx, owner, products.CreateProductRequest{
		Title: "VC product", CategoryID: &catID,
		Variants: []products.ProductVariantRequest{create, vr(&bl, sizeM, "M", 12000)},
	})
	require.NoError(t, err)

	type row struct {
		ID                         uuid.UUID
		Color, Shade, Barcode, SKU *string
		ColorID, SizeID            *uuid.UUID
		Price                      int64
	}
	rows := func() map[string]row {
		out := map[string]row{}
		rs, err := pool.Query(ctx, `SELECT id, color, shade_name, barcode, seller_sku, color_id, size_value_id, price_cents FROM product_variants WHERE product_id=$1`, p.ID)
		require.NoError(t, err)
		defer rs.Close()
		for rs.Next() {
			var r row
			require.NoError(t, rs.Scan(&r.ID, &r.Color, &r.Shade, &r.Barcode, &r.SKU, &r.ColorID, &r.SizeID, &r.Price))
			out[strings.SplitN(*r.SKU, "-", 2)[0]] = r
		}
		return out
	}
	var sellerID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT seller_id FROM products WHERE id=$1`, p.ID).Scan(&sellerID))
	for k, r := range rows() {
		_, err = pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock) VALUES (gen_random_uuid(),$1,$2,$3,$4,1)`, p.ID, sellerID, r.ID, map[string]int{"S": 7, "M": 9}[k])
		require.NoError(t, err)
	}
	stock := func() string {
		var out string
		require.NoError(t, pool.QueryRow(ctx, `SELECT string_agg(product_variant_id::text||':'||total_stock||':'||reserved_stock, ',' ORDER BY product_variant_id) FROM inventory_items WHERE product_id=$1`, p.ID).Scan(&out))
		return out
	}

	r0 := rows()
	require.Equal(t, black, *r0["S"].ColorID)
	require.Equal(t, shade, *r0["S"].Shade, "shade stored separately from the primary color")
	stockBefore := stock()

	update := func(user uuid.UUID, vs ...products.ProductVariantRequest) error {
		_, err := svc.UpdateProductForSeller(ctx, user, p.ID, products.UpdateProductRequest{Variants: vs})
		return err
	}
	withID := func(v products.ProductVariantRequest, id uuid.UUID) products.ProductVariantRequest {
		v.ID = &id
		return v
	}

	t.Run("changing the primary color of an existing variant keeps identity, SKU, size, price, barcode, stock", func(t *testing.T) {
		require.NoError(t, update(owner,
			withID(vr(&wh, sizeS, "S", 10000), r0["S"].ID),
			withID(vr(&bl, sizeM, "M", 12000), r0["M"].ID)))
		r := rows()
		require.Equal(t, r0["S"].ID, r["S"].ID)
		require.Equal(t, white, *r["S"].ColorID)
		require.Equal(t, *r0["S"].SKU, *r["S"].SKU)
		require.Equal(t, *r0["S"].SizeID, *r["S"].SizeID)
		require.Equal(t, r0["S"].Price, r["S"].Price)
		require.Equal(t, *r0["S"].Barcode, *r["S"].Barcode)
		require.Equal(t, shade, *r["S"].Shade, "omitted shade is preserved")
		require.Equal(t, stockBefore, stock())
	})
	t.Run("explicit shade changes it; empty string clears it", func(t *testing.T) {
		v := withID(vr(&wh, sizeS, "S", 10000), r0["S"].ID)
		n := "Молочный"
		v.ShadeName = &n
		require.NoError(t, update(owner, v, withID(vr(&bl, sizeM, "M", 12000), r0["M"].ID)))
		require.Equal(t, "Молочный", *rows()["S"].Shade)
		empty := "  "
		v.ShadeName = &empty
		require.NoError(t, update(owner, v, withID(vr(&bl, sizeM, "M", 12000), r0["M"].ID)))
		require.Nil(t, rows()["S"].Shade)
	})
	t.Run("unknown or inactive color is rejected", func(t *testing.T) {
		for name, id := range map[string]uuid.UUID{"unknown": uuid.New(), "inactive": retired} {
			id := id
			err := update(owner, withID(vr(&id, sizeS, "S", 10000), r0["S"].ID), withID(vr(&bl, sizeM, "M", 12000), r0["M"].ID))
			require.Error(t, err, name)
			require.Contains(t, err.Error(), "invalid or inactive")
		}
		require.Equal(t, white, *rows()["S"].ColorID)
	})
	t.Run("duplicate color/size combination and bad shade names are rejected", func(t *testing.T) {
		err := update(owner, vr(&bl, sizeS, "S", 10000), vr(&bl, sizeS, "S2", 10000))
		require.Error(t, err)
		require.Contains(t, err.Error(), "validation failed")
		long := strings.Repeat("я", 61)
		bad := vr(&bl, sizeS, "S", 10000)
		bad.ShadeName = &long
		require.Error(t, update(owner, withID(bad, r0["S"].ID)))
		html := "<b>x</b>"
		bad.ShadeName = &html
		require.Error(t, update(owner, withID(bad, r0["S"].ID)))
	})
	t.Run("another seller cannot change the color", func(t *testing.T) {
		require.Error(t, update(stranger, withID(vr(&bl, sizeS, "S", 10000), r0["S"].ID), withID(vr(&bl, sizeM, "M", 12000), r0["M"].ID)))
		require.Equal(t, white, *rows()["S"].ColorID)
	})
	t.Run("legacy text color without color_id is kept until the seller explicitly picks a dictionary color", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE product_variants SET color_id=NULL, color='Чёрный', shade_name='Стар' WHERE id=$1`, r0["M"].ID)
		require.NoError(t, err)
		// save without touching color: payload has no colorId / color / shadeName
		require.NoError(t, update(owner,
			withID(vr(&wh, sizeS, "S", 10000), r0["S"].ID),
			withID(products.ProductVariantRequest{SellerSKU: sku("M"), SizeValueID: &sizeM, PriceCents: ip(12000)}, r0["M"].ID)))
		r := rows()["M"]
		require.Nil(t, r.ColorID, "no automatic assignment")
		require.Equal(t, "Чёрный", *r.Color, "legacy text preserved")
		require.Equal(t, "Стар", *r.Shade)
		// explicit choice
		require.NoError(t, update(owner,
			withID(vr(&wh, sizeS, "S", 10000), r0["S"].ID),
			withID(vr(&bl, sizeM, "M", 12000), r0["M"].ID)))
		r = rows()["M"]
		require.Equal(t, black, *r.ColorID)
		require.Equal(t, "Чёрный", *r.Color)
		require.Equal(t, "Стар", *r.Shade)
		require.Equal(t, int64(12000), r.Price)
		require.Equal(t, stockBefore, stock())
	})
	t.Run("public color filter uses color_id only; shade does not affect it", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE products SET status='published', published_at=now() WHERE id=$1`, p.ID)
		require.NoError(t, err)
		pub, err := svc.ListPublicProducts(ctx, products.PublicProductFilter{BrandIDs: nil, CategoryID: &catID, ColorIDs: []uuid.UUID{black}, Sizes: []string{"M" + tag}}, 20, 0)
		require.NoError(t, err)
		require.Equal(t, 1, pub.TotalCount)
		pub, err = svc.ListPublicProducts(ctx, products.PublicProductFilter{CategoryID: &catID, ColorIDs: []uuid.UUID{black}, Sizes: []string{"S" + tag}}, 20, 0)
		require.NoError(t, err)
		require.Equal(t, 0, pub.TotalCount, "S is white, M is black")
	})
}
