package products_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// CATALOG.C5.1: a seller's variant edit of a published product is applied only on approval, atomically,
// keeping ids, SKUs, barcodes, prices and stock, and re-running the C5 deactivation guard at approval time.
func TestPublishedProductRevisionVariants(t *testing.T) {
	var clients []*postgres.Client
	t.Cleanup(func() {
		for _, c := range clients {
			c.Close()
		}
	})
	db, svc, owner := setupBlockATestDB(t)
	clients = append(clients, db)
	pool := db.Pool
	ctx := context.Background()
	tag := uuid.NewString()[:6]

	adminID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, name) VALUES ($1,$2,'hash','admin','Rev Admin')`, adminID, fmt.Sprintf("rev-admin-%s@test.com", tag))
	require.NoError(t, err)

	mkColor := func(name string) uuid.UUID {
		id := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO colors (id, code, name_ru, hex, sort_order, is_active) VALUES ($1,$2,$3,'#111111',0,true)`, id, "rv-"+name+"-"+tag, "RV "+name)
		require.NoError(t, err)
		return id
	}
	black, white := mkColor("black"), mkColor("white")
	catID, sysID := uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1,'RV cat',$2)`, catID, "rv-"+tag)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO size_systems (id, code, name) VALUES ($1,$2,'RV')`, sysID, "rv-"+tag)
	require.NoError(t, err)
	sizeS, sizeM := uuid.New(), uuid.New()
	for id, v := range map[uuid.UUID]string{sizeS: "S" + tag, sizeM: "M" + tag} {
		_, err = pool.Exec(ctx, `INSERT INTO size_values (id, size_system_id, value, sort_order) VALUES ($1,$2,$3,0)`, id, sysID, v)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM inventory_items WHERE product_variant_id IN (SELECT id FROM product_variants WHERE color_id IN (SELECT id FROM colors WHERE code LIKE 'rv-%-'||$1))`, tag)
		pool.Exec(c, `DELETE FROM product_variants WHERE color_id IN (SELECT id FROM colors WHERE code LIKE 'rv-%-'||$1)`, tag)
		pool.Exec(c, `DELETE FROM products WHERE category_id=$1`, catID)
		pool.Exec(c, `DELETE FROM categories WHERE id=$1`, catID)
		pool.Exec(c, `DELETE FROM size_values WHERE size_system_id=$1`, sysID)
		pool.Exec(c, `DELETE FROM size_systems WHERE id=$1`, sysID)
		pool.Exec(c, `DELETE FROM colors WHERE code LIKE 'rv-%-'||$1`, tag)
		pool.Exec(c, `DELETE FROM users WHERE id=$1`, adminID)
	})

	vr := func(colorID, size uuid.UUID, s string, price int64) products.ProductVariantRequest {
		sku := s + "-" + tag
		c, sz := colorID, size
		return products.ProductVariantRequest{SellerSKU: &sku, ColorID: &c, SizeValueID: &sz, PriceCents: ip(price)}
	}
	withID := func(v products.ProductVariantRequest, id uuid.UUID) products.ProductVariantRequest {
		v.ID = &id
		return v
	}
	key := func(s string) string { return s + "-" + tag }
	yes := true

	p, err := svc.CreateProductForSeller(ctx, owner, products.CreateProductRequest{
		Title: "RV product", CategoryID: &catID,
		Variants: []products.ProductVariantRequest{vr(black, sizeS, "BS", 10000), vr(white, sizeM, "WM", 11000)},
	})
	require.NoError(t, err)
	var sellerID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT seller_id FROM products WHERE id=$1`, p.ID).Scan(&sellerID))

	type row struct {
		ID      uuid.UUID
		SKU     string
		Barcode string
		Price   int64
		Active  bool
	}
	rows := func() map[string]row {
		out := map[string]row{}
		rs, err := pool.Query(ctx, `SELECT id, seller_sku, COALESCE(barcode,''), price_cents, is_active FROM product_variants WHERE product_id=$1`, p.ID)
		require.NoError(t, err)
		defer rs.Close()
		for rs.Next() {
			var r row
			require.NoError(t, rs.Scan(&r.ID, &r.SKU, &r.Barcode, &r.Price, &r.Active))
			_, dup := out[r.SKU]
			require.False(t, dup, "duplicate SKU row %s", r.SKU)
			out[r.SKU] = r
		}
		return out
	}
	stock := func() string {
		var out string
		require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE(string_agg(product_variant_id::text||':'||total_stock||':'||reserved_stock, ',' ORDER BY product_variant_id),'') FROM inventory_items WHERE product_id=$1`, p.ID).Scan(&out))
		return out
	}
	productState := func() (string, string, int64) {
		var title, status string
		var ver int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT title, status, vision_content_version FROM products WHERE id=$1`, p.ID).Scan(&title, &status, &ver))
		return title, status, ver
	}
	publicActive := func() map[string]bool {
		pub, err := svc.GetPublicProduct(ctx, p.ID.String())
		require.NoError(t, err)
		out := map[string]bool{}
		for _, v := range pub.Variants {
			if v.IsActive && v.SellerSKU != nil {
				out[*v.SellerSKU] = true
			}
		}
		return out
	}
	revisionStatuses := func() map[string]int {
		out := map[string]int{}
		rs, err := pool.Query(ctx, `SELECT status FROM product_revisions WHERE product_id=$1`, p.ID)
		require.NoError(t, err)
		defer rs.Close()
		for rs.Next() {
			var s string
			require.NoError(t, rs.Scan(&s))
			out[s]++
		}
		return out
	}
	submit := func(req products.UpdateProductRequest) error {
		_, err := svc.UpdateProductForSeller(ctx, owner, p.ID, req)
		return err
	}

	r0 := rows()
	bs, wm := r0[key("BS")], r0[key("WM")]
	_, err = pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock) VALUES (gen_random_uuid(),$1,$2,$3,5,1),(gen_random_uuid(),$1,$2,$4,4,0)`, p.ID, sellerID, bs.ID, wm.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE products SET status='published', published_at=now() WHERE id=$1`, p.ID)
	require.NoError(t, err)
	stockBefore := stock()

	t.Run("added combination is applied only on approval, existing variants and stock are kept", func(t *testing.T) {
		require.NoError(t, submit(products.UpdateProductRequest{ContinueSelling: &yes, Variants: []products.ProductVariantRequest{
			withID(vr(black, sizeS, "BS", 10000), bs.ID), withID(vr(white, sizeM, "WM", 11000), wm.ID), vr(black, sizeM, "BM", 12000),
		}}))

		require.Len(t, rows(), 2, "nothing reaches the live variants before approval")
		require.False(t, publicActive()[key("BM")], "black/M is not in the public Shop before approval")
		_, status, verBefore := productState()
		require.Equal(t, products.StatusPublished, status)

		seller, err := svc.GetSellerProduct(ctx, owner, p.ID)
		require.NoError(t, err)
		activeInStudio := 0
		for _, v := range seller.Variants {
			if v.IsActive {
				activeInStudio++
			}
		}
		require.Equal(t, 3, activeInStudio, "the seller keeps editing the pending matrix")

		admin, err := svc.GetAdminProductDetail(ctx, p.ID)
		require.NoError(t, err)
		require.NotNil(t, admin.PendingRevision)
		require.True(t, admin.PendingRevision.VariantsChanged)
		require.Len(t, admin.PendingRevision.Variants, 3, "admin sees the proposed matrix")

		require.NoError(t, svc.ApproveProduct(ctx, adminID, p.ID, nil))

		r := rows()
		require.Len(t, r, 3)
		require.Equal(t, bs, r[key("BS")], "id, SKU, barcode and price of black/S unchanged")
		require.Equal(t, wm, r[key("WM")], "id, SKU, barcode and price of white/M unchanged")
		require.True(t, r[key("BM")].Active)
		require.Equal(t, int64(12000), r[key("BM")].Price)
		require.Equal(t, stockBefore, stock(), "stock and reserves untouched")
		require.True(t, publicActive()[key("BM")], "black/M appears in the public Shop")
		_, status, verAfter := productState()
		require.Equal(t, products.StatusPublished, status)
		require.Equal(t, verBefore+1, verAfter)
		require.Equal(t, map[string]int{"approved": 1}, revisionStatuses())
	})

	t.Run("repeated approval is refused and creates no duplicates", func(t *testing.T) {
		err := svc.ApproveProduct(ctx, adminID, p.ID, nil)
		require.ErrorIs(t, err, products.ErrInvalidStatusTransition)
		require.Len(t, rows(), 3)
	})

	bm := rows()[key("BM")]
	current := func() []products.ProductVariantRequest {
		return []products.ProductVariantRequest{withID(vr(black, sizeS, "BS", 10000), bs.ID), withID(vr(white, sizeM, "WM", 11000), wm.ID), withID(vr(black, sizeM, "BM", 12000), bm.ID)}
	}

	t.Run("rejected revision changes nothing", func(t *testing.T) {
		before, title := rows(), "RV rejected title"
		vs := current()
		vs[1].PriceCents = ip(15000)
		require.NoError(t, submit(products.UpdateProductRequest{ContinueSelling: &yes, Title: &title, Variants: vs[:2]}))
		require.NoError(t, svc.RejectProduct(ctx, adminID, p.ID, "not acceptable"))
		require.Equal(t, before, rows())
		gotTitle, status, _ := productState()
		require.Equal(t, "RV product", gotTitle)
		require.Equal(t, products.StatusPublished, status)
		require.Equal(t, 0, revisionStatuses()["pending"])
	})

	t.Run("removing a variant with stock is refused at submission", func(t *testing.T) {
		err := submit(products.UpdateProductRequest{ContinueSelling: &yes, Variants: current()[1:]})
		var blocked *products.VariantDeactivationBlockedError
		require.True(t, errors.As(err, &blocked), "got %v", err)
		require.Equal(t, bs.ID, blocked.VariantID)
		require.Equal(t, 0, revisionStatuses()["pending"], "no revision is created")
	})

	t.Run("warehouse change after submission blocks approval atomically", func(t *testing.T) {
		title := "RV must not leak"
		require.NoError(t, submit(products.UpdateProductRequest{ContinueSelling: &yes, Title: &title, Variants: current()[:2]}))
		// a reservation for black/M appears between submission and approval
		_, err := pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock) VALUES (gen_random_uuid(),$1,$2,$3,1,1)`, p.ID, sellerID, bm.ID)
		require.NoError(t, err)
		before, stockNow := rows(), stock()
		_, _, verBefore := productState()

		err = svc.ApproveProduct(ctx, adminID, p.ID, nil)
		var blocked *products.VariantDeactivationBlockedError
		require.True(t, errors.As(err, &blocked), "got %v", err)
		require.ErrorIs(t, err, products.ErrRevisionNotApplicable)
		require.Equal(t, bm.ID, blocked.VariantID)
		require.Equal(t, products.VariantBlockReserved, blocked.Reason)

		require.Equal(t, before, rows(), "no variant changed")
		require.Equal(t, stockNow, stock(), "reserve untouched")
		gotTitle, status, ver := productState()
		require.Equal(t, "RV product", gotTitle, "no partial apply of product fields")
		require.Equal(t, products.StatusPublished, status)
		require.Equal(t, verBefore, ver)
		require.Equal(t, 1, revisionStatuses()["pending"], "revision stays pending for a decision")

		_, err = pool.Exec(ctx, `DELETE FROM inventory_items WHERE product_variant_id=$1`, bm.ID)
		require.NoError(t, err)
		require.NoError(t, svc.RejectProduct(ctx, adminID, p.ID, "warehouse state changed"))
	})

	t.Run("variants changed after submission make the revision a conflict", func(t *testing.T) {
		vs := current()
		vs[2].PriceCents = ip(13000)
		require.NoError(t, submit(products.UpdateProductRequest{ContinueSelling: &yes, Variants: vs}))
		_, err := svc.UpdateVariantPricesForSeller(ctx, owner, p.ID, products.UpdateVariantPricesRequest{Prices: map[uuid.UUID]int64{wm.ID: 11500}})
		require.NoError(t, err)

		err = svc.ApproveProduct(ctx, adminID, p.ID, nil)
		require.ErrorIs(t, err, products.ErrRevisionConflict)
		r := rows()
		require.Equal(t, int64(11500), r[key("WM")].Price, "the newer live price is not overwritten")
		require.Equal(t, int64(12000), r[key("BM")].Price)
		require.NoError(t, svc.RejectProduct(ctx, adminID, p.ID, "stale"))
	})

	t.Run("revision with an unpriced new variant is refused at approval", func(t *testing.T) {
		c, sz := white, sizeS
		vs := append(current(), products.ProductVariantRequest{ColorID: &c, SizeValueID: &sz})
		require.NoError(t, submit(products.UpdateProductRequest{ContinueSelling: &yes, Variants: vs}))
		before := rows()
		err := svc.ApproveProduct(ctx, adminID, p.ID, nil)
		require.ErrorIs(t, err, products.ErrRevisionNotApplicable)
		require.ErrorIs(t, err, products.ErrProductPriceInvalid)
		require.Equal(t, before, rows())
		require.NoError(t, svc.RejectProduct(ctx, adminID, p.ID, "price missing"))
	})

	t.Run("hidden edit stays a revision across autosaves and carries pending variants forward", func(t *testing.T) {
		vs := current()
		c, sz := white, sizeS
		ws := key("WS")
		vs = append(vs, products.ProductVariantRequest{SellerSKU: &ws, ColorID: &c, SizeValueID: &sz, PriceCents: ip(9000)})
		require.NoError(t, submit(products.UpdateProductRequest{Variants: vs})) // Studio autosave: no continueSelling
		_, status, _ := productState()
		require.Equal(t, products.StatusPendingModeration, status)

		title := "RV product v2"
		require.NoError(t, submit(products.UpdateProductRequest{Title: &title})) // next autosave without variants
		_, status, _ = productState()
		require.Equal(t, products.StatusPendingModeration, status, "not reverted to draft")
		require.Len(t, rows(), 3, "live variants untouched before approval")
		require.Equal(t, 1, revisionStatuses()["pending"], "older pending revision superseded")

		require.NoError(t, svc.ApproveProduct(ctx, adminID, p.ID, nil))
		r := rows()
		require.Len(t, r, 4)
		require.True(t, r[key("WS")].Active)
		require.Equal(t, bs, r[key("BS")])
		gotTitle, status, _ := productState()
		require.Equal(t, title, gotTitle)
		require.Equal(t, products.StatusPublished, status)
		require.Equal(t, stockBefore, stock())
	})
}
