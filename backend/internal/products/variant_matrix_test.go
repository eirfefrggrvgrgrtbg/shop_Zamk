package products_test

import (
	"context"
	"errors"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// CATALOG.C5: incomplete color x size matrix, safe deactivation of existing variants.
func TestIncompleteVariantMatrix(t *testing.T) {
	// Registered first so it runs last (after the fixture cleanups): release both pools, the suite is close to max_connections.
	var clients []*postgres.Client
	t.Cleanup(func() {
		for _, c := range clients {
			c.Close()
		}
	})
	db, svc, owner := setupBlockATestDB(t)
	strangerDB, _, stranger := setupBlockATestDB(t)
	clients = append(clients, db, strangerDB)
	pool := db.Pool
	ctx := context.Background()
	tag := uuid.NewString()[:6]

	mkColor := func(name string) uuid.UUID {
		id := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO colors (id, code, name_ru, hex, sort_order, is_active) VALUES ($1,$2,$3,'#111111',0,true)`, id, "vm-"+name+"-"+tag, "VM "+name)
		require.NoError(t, err)
		return id
	}
	black, white := mkColor("black"), mkColor("white")
	catID, sysID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1,'VM cat',$2)`, catID, "vm-"+tag)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO size_systems (id, code, name) VALUES ($1,$2,'VM')`, sysID, "vm-"+tag)
	require.NoError(t, err)
	sizeS, sizeM := uuid.New(), uuid.New()
	for id, v := range map[uuid.UUID]string{sizeS: "S" + tag, sizeM: "M" + tag} {
		_, err = pool.Exec(ctx, `INSERT INTO size_values (id, size_system_id, value, sort_order) VALUES ($1,$2,$3,0)`, id, sysID, v)
		require.NoError(t, err)
	}
	var orderIDs []uuid.UUID
	t.Cleanup(func() {
		c := context.Background()
		for _, id := range orderIDs {
			pool.Exec(c, `DELETE FROM order_items WHERE order_id=$1`, id)
			pool.Exec(c, `DELETE FROM order_fulfillments WHERE order_id=$1`, id)
			pool.Exec(c, `DELETE FROM orders WHERE id=$1`, id)
		}
		// variants referencing the test colors/sizes go first (the seller fixtures are removed after this cleanup)
		pool.Exec(c, `DELETE FROM inventory_items WHERE product_variant_id IN (SELECT id FROM product_variants WHERE color_id IN (SELECT id FROM colors WHERE code LIKE 'vm-%-'||$1))`, tag)
		pool.Exec(c, `DELETE FROM product_variants WHERE color_id IN (SELECT id FROM colors WHERE code LIKE 'vm-%-'||$1)`, tag)
		pool.Exec(c, `DELETE FROM products WHERE category_id=$1`, catID)
		pool.Exec(c, `DELETE FROM categories WHERE id=$1`, catID)
		pool.Exec(c, `DELETE FROM size_values WHERE size_system_id=$1`, sysID)
		pool.Exec(c, `DELETE FROM size_systems WHERE id=$1`, sysID)
		pool.Exec(c, `DELETE FROM colors WHERE code LIKE 'vm-%-'||$1`, tag)
	})

	vr := func(colorID uuid.UUID, size uuid.UUID, s string, price int64) products.ProductVariantRequest {
		sku := s + "-" + tag
		c, sz := colorID, size
		return products.ProductVariantRequest{SellerSKU: &sku, ColorID: &c, SizeValueID: &sz, PriceCents: ip(price)}
	}
	withID := func(v products.ProductVariantRequest, id uuid.UUID) products.ProductVariantRequest {
		v.ID = &id
		return v
	}

	// Black: S only. White: M only.
	p, err := svc.CreateProductForSeller(ctx, owner, products.CreateProductRequest{
		Title: "VM product", CategoryID: &catID,
		Variants: []products.ProductVariantRequest{vr(black, sizeS, "BS", 10000), vr(white, sizeM, "WM", 11000)},
	})
	require.NoError(t, err)
	var sellerID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT seller_id FROM products WHERE id=$1`, p.ID).Scan(&sellerID))

	type row struct {
		ID     uuid.UUID
		SKU    string
		Price  int64
		Active bool
	}
	rows := func() map[string]row {
		out := map[string]row{}
		rs, err := pool.Query(ctx, `SELECT id, seller_sku, price_cents, is_active FROM product_variants WHERE product_id=$1`, p.ID)
		require.NoError(t, err)
		defer rs.Close()
		for rs.Next() {
			var r row
			require.NoError(t, rs.Scan(&r.ID, &r.SKU, &r.Price, &r.Active))
			_, dup := out[r.SKU]
			require.False(t, dup, "duplicate SKU row %s", r.SKU)
			out[r.SKU] = r
		}
		return out
	}
	key := func(s string) string { return s + "-" + tag }
	update := func(user uuid.UUID, vs ...products.ProductVariantRequest) error {
		_, err := svc.UpdateProductForSeller(ctx, user, p.ID, products.UpdateProductRequest{Variants: vs})
		return err
	}

	r0 := rows()
	require.Len(t, r0, 2, "only the selected combinations are created")
	bs, wm := r0[key("BS")], r0[key("WM")]

	_, err = pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock) VALUES (gen_random_uuid(),$1,$2,$3,5,0)`, p.ID, sellerID, bs.ID)
	require.NoError(t, err)
	stock := func() string {
		var out string
		require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE(string_agg(product_variant_id::text||':'||total_stock||':'||reserved_stock, ',' ORDER BY product_variant_id),'') FROM inventory_items WHERE product_id=$1`, p.ID).Scan(&out))
		return out
	}
	stockBefore := stock()

	t.Run("category schema carries its id so Seller Studio can match it to the draft", func(t *testing.T) {
		raw, err := svc.GetCategoryAttributeSchema(ctx, catID)
		require.NoError(t, err)
		schema := raw.(map[string]interface{})
		require.Equal(t, catID, schema["id"])
		require.Equal(t, "VM cat", schema["name"])
	})

	t.Run("re-saving the same matrix creates no duplicates and keeps ids", func(t *testing.T) {
		require.NoError(t, update(owner, withID(vr(black, sizeS, "BS", 10000), bs.ID), withID(vr(white, sizeM, "WM", 11000), wm.ID)))
		// and once more without ids: matched by combination
		require.NoError(t, update(owner, vr(black, sizeS, "BS", 10000), vr(white, sizeM, "WM", 11000)))
		r := rows()
		require.Len(t, r, 2)
		require.Equal(t, bs.ID, r[key("BS")].ID)
		require.Equal(t, wm.ID, r[key("WM")].ID)
	})

	t.Run("adding a new combination later keeps existing ids, prices and stock", func(t *testing.T) {
		require.NoError(t, update(owner,
			withID(vr(black, sizeS, "BS", 10000), bs.ID),
			withID(vr(white, sizeM, "WM", 11000), wm.ID),
			vr(black, sizeM, "BM", 12000)))
		r := rows()
		require.Len(t, r, 3)
		require.Equal(t, bs.ID, r[key("BS")].ID)
		require.Equal(t, int64(10000), r[key("BS")].Price)
		require.Equal(t, int64(11000), r[key("WM")].Price)
		require.True(t, r[key("BM")].Active)
		require.Equal(t, stockBefore, stock())
	})

	t.Run("removing a combination without operations soft-deactivates it; re-adding restores the same variant", func(t *testing.T) {
		bm := rows()[key("BM")]
		require.NoError(t, update(owner, withID(vr(black, sizeS, "BS", 10000), bs.ID), withID(vr(white, sizeM, "WM", 11000), wm.ID)))
		r := rows()
		require.Len(t, r, 3, "row is kept, not deleted")
		require.False(t, r[key("BM")].Active)

		require.NoError(t, update(owner, withID(vr(black, sizeS, "BS", 10000), bs.ID), withID(vr(white, sizeM, "WM", 11000), wm.ID), vr(black, sizeM, "BM", 12000)))
		r = rows()
		require.Len(t, r, 3, "no new variant for a re-selected combination")
		require.Equal(t, bm.ID, r[key("BM")].ID)
		require.True(t, r[key("BM")].Active)

		// re-selected again by a client that only knows the combination: SKU and price come back
		require.NoError(t, update(owner, withID(vr(black, sizeS, "BS", 10000), bs.ID), withID(vr(white, sizeM, "WM", 11000), wm.ID)))
		c, sz := black, sizeM
		require.NoError(t, update(owner, withID(vr(black, sizeS, "BS", 10000), bs.ID), withID(vr(white, sizeM, "WM", 11000), wm.ID),
			products.ProductVariantRequest{ColorID: &c, SizeValueID: &sz}))
		r = rows()
		require.Len(t, r, 3)
		require.Equal(t, bm.ID, r[key("BM")].ID)
		require.Equal(t, int64(12000), r[key("BM")].Price)
		require.True(t, r[key("BM")].Active)
	})

	t.Run("variant with stock cannot be deactivated", func(t *testing.T) {
		err := update(owner, withID(vr(white, sizeM, "WM", 11000), wm.ID), vr(black, sizeM, "BM", 12000))
		var blocked *products.VariantDeactivationBlockedError
		require.True(t, errors.As(err, &blocked), "got %v", err)
		require.Equal(t, bs.ID, blocked.VariantID)
		require.Equal(t, products.VariantBlockStock, blocked.Reason)
		require.Contains(t, blocked.SellerMessage(), "остатки")
		require.True(t, rows()[key("BS")].Active)
		require.Equal(t, stockBefore, stock(), "stock untouched")
	})

	t.Run("variant with reserve cannot be deactivated and the reserve is untouched", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE inventory_items SET reserved_stock=2 WHERE product_variant_id=$1`, bs.ID)
		require.NoError(t, err)
		reserved := stock()
		err = update(owner, withID(vr(white, sizeM, "WM", 11000), wm.ID), vr(black, sizeM, "BM", 12000))
		var blocked *products.VariantDeactivationBlockedError
		require.True(t, errors.As(err, &blocked))
		require.Equal(t, products.VariantBlockReserved, blocked.Reason)
		require.Equal(t, reserved, stock())
		_, err = pool.Exec(ctx, `UPDATE inventory_items SET reserved_stock=0 WHERE product_variant_id=$1`, bs.ID)
		require.NoError(t, err)
	})

	mkOrder := func(variantID uuid.UUID, status string) {
		orderID, fulfillmentID := uuid.New(), uuid.New()
		orderIDs = append(orderIDs, orderID)
		_, err := pool.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
			VALUES ($1,$2,$3,1500,'RUB','VM Buyer','+79990000000','vm@test.com','Moscow',now(),now())`, orderID, owner, status)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
			VALUES ($1,$2,$3,'paid',1500,1000,1350,now(),now())`, fulfillmentID, orderID, sellerID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, created_at)
			VALUES (gen_random_uuid(),$1,$2,$3,$4,'VM','vm',1500,1,1500,$5,now())`, orderID, p.ID, variantID, sellerID, fulfillmentID)
		require.NoError(t, err)
	}

	t.Run("variant in an open order cannot be deactivated", func(t *testing.T) {
		mkOrder(wm.ID, "paid")
		err := update(owner, withID(vr(black, sizeS, "BS", 10000), bs.ID), vr(black, sizeM, "BM", 12000))
		var blocked *products.VariantDeactivationBlockedError
		require.True(t, errors.As(err, &blocked))
		require.Equal(t, wm.ID, blocked.VariantID)
		require.Equal(t, products.VariantBlockOpenOrder, blocked.Reason)
		require.True(t, rows()[key("WM")].Active)
		_, err = pool.Exec(ctx, `UPDATE orders SET status='delivered' WHERE id=$1`, orderIDs[len(orderIDs)-1])
		require.NoError(t, err)
	})

	t.Run("variant with only finished order history is soft-deactivated, history stays", func(t *testing.T) {
		require.NoError(t, update(owner, withID(vr(black, sizeS, "BS", 10000), bs.ID), vr(black, sizeM, "BM", 12000)))
		r := rows()
		require.False(t, r[key("WM")].Active)
		require.Equal(t, wm.ID, r[key("WM")].ID)
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM order_items WHERE product_variant_id=$1`, wm.ID).Scan(&n))
		require.Equal(t, 1, n)
		// restore for the next checks
		require.NoError(t, update(owner, withID(vr(black, sizeS, "BS", 10000), bs.ID), vr(black, sizeM, "BM", 12000), vr(white, sizeM, "WM", 11000)))
		require.Equal(t, wm.ID, rows()[key("WM")].ID)
	})

	t.Run("another seller cannot change the matrix; a foreign variant id is not taken over", func(t *testing.T) {
		require.Error(t, update(stranger, vr(black, sizeS, "BS", 10000)))
		require.Len(t, rows(), 3)
		require.True(t, rows()[key("BM")].Active)

		foreign, err := svc.CreateProductForSeller(ctx, stranger, products.CreateProductRequest{
			Title: "VM foreign", Variants: []products.ProductVariantRequest{{PriceCents: ip(5000)}},
		})
		require.NoError(t, err)
		var foreignVariant uuid.UUID
		require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM product_variants WHERE product_id=$1`, foreign.ID).Scan(&foreignVariant))
		require.NoError(t, update(owner,
			withID(vr(black, sizeS, "BS", 10000), bs.ID),
			withID(vr(black, sizeM, "BM", 12000), foreignVariant),
			withID(vr(white, sizeM, "WM", 11000), wm.ID)))
		var owner uuid.UUID
		require.NoError(t, pool.QueryRow(ctx, `SELECT product_id FROM product_variants WHERE id=$1`, foreignVariant).Scan(&owner))
		require.Equal(t, foreign.ID, owner)
		require.Len(t, rows(), 3)
	})

	t.Run("public catalog exposes only existing combinations; C2/C3 filters follow them", func(t *testing.T) {
		// deactivate black/M again -> black: S, white: M
		require.NoError(t, update(owner, withID(vr(black, sizeS, "BS", 10000), bs.ID), withID(vr(white, sizeM, "WM", 11000), wm.ID)))
		_, err := pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock) VALUES (gen_random_uuid(),$1,$2,$3,4,0)`, p.ID, sellerID, wm.ID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE products SET status='published', published_at=now() WHERE id=$1`, p.ID)
		require.NoError(t, err)

		count := func(color uuid.UUID, size string) int {
			res, err := svc.ListPublicProducts(ctx, products.PublicProductFilter{CategoryID: &catID, ColorIDs: []uuid.UUID{color}, Sizes: []string{size + tag}}, 20, 0)
			require.NoError(t, err)
			return res.TotalCount
		}
		require.Equal(t, 1, count(black, "S"))
		require.Equal(t, 0, count(black, "M"), "black/M is not produced")
		require.Equal(t, 1, count(white, "M"))
		require.Equal(t, 0, count(white, "S"), "white/S does not exist")

		pub, err := svc.GetPublicProduct(ctx, p.ID.String())
		require.NoError(t, err)
		active := 0
		for _, v := range pub.Variants {
			if v.IsActive {
				active++
			}
		}
		require.Equal(t, 2, active, "deactivated combination is not offered (public payload flags it isActive=false)")
	})
}
