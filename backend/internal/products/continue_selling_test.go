package products_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// CATALOG.C5.3: a published product edited from Studio (continueSelling=true on every autosave) keeps selling its last
// approved version until Admin approves the revision; pending data never reaches the public API, and continueSelling
// never puts a blocked, archived, hidden or never-published product on sale.
func TestContinueSellingDuringRevisionModeration(t *testing.T) {
	db, svc, owner := setupBlockATestDB(t)
	t.Cleanup(db.Close)
	pool := db.Pool
	ctx := context.Background()
	tag := uuid.NewString()[:6]
	yes := true

	adminID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, name) VALUES ($1,$2,'hash','admin','CS Admin')`, adminID, fmt.Sprintf("cs-admin-%s@test.com", tag))
	require.NoError(t, err)
	color := func(name string) uuid.UUID {
		id := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO colors (id, code, name_ru, hex) VALUES ($1,$2,$3,'#111111')`, id, "cs-"+name+"-"+tag, "CS "+name)
		require.NoError(t, err)
		return id
	}
	black, white := color("black"), color("white")
	catID, sysID, sizeS, sizeM := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1,'CS cat',$2)`, catID, "cs-"+tag)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO size_systems (id, code, name) VALUES ($1,$2,'CS')`, sysID, "cs-"+tag)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO size_values (id, size_system_id, value, sort_order) VALUES ($1,$3,$4,1),($2,$3,$5,2)`, sizeS, sizeM, sysID, "S"+tag, "M"+tag)
	require.NoError(t, err)
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM inventory_items WHERE product_id IN (SELECT id FROM products WHERE category_id=$1)`, catID)
		pool.Exec(c, `DELETE FROM product_images WHERE product_id IN (SELECT id FROM products WHERE category_id=$1)`, catID)
		pool.Exec(c, `DELETE FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE category_id=$1)`, catID)
		pool.Exec(c, `DELETE FROM products WHERE category_id=$1`, catID)
		pool.Exec(c, `DELETE FROM categories WHERE id=$1`, catID)
		pool.Exec(c, `DELETE FROM size_values WHERE size_system_id=$1`, sysID)
		pool.Exec(c, `DELETE FROM size_systems WHERE id=$1`, sysID)
		pool.Exec(c, `DELETE FROM colors WHERE code LIKE 'cs-%-'||$1`, tag)
		pool.Exec(c, `DELETE FROM users WHERE id=$1`, adminID)
	})

	vr := func(c, sz uuid.UUID, s string) products.ProductVariantRequest {
		sku := s + "-" + tag
		return products.ProductVariantRequest{SellerSKU: &sku, ColorID: &c, SizeValueID: &sz, PriceCents: ip(10000)}
	}
	sku := func(s string) string { return s + "-" + tag }

	// A published dress: black/S and white/M on sale, two photos.
	newPublished := func(title string) uuid.UUID {
		p, err := svc.CreateProductForSeller(ctx, owner, products.CreateProductRequest{
			Title: title, CategoryID: &catID, Variants: []products.ProductVariantRequest{vr(black, sizeS, title+"BS"), vr(white, sizeM, title+"WM")},
		})
		require.NoError(t, err)
		var sellerID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx, `SELECT seller_id FROM products WHERE id=$1`, p.ID).Scan(&sellerID))
		_, err = pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock)
			SELECT gen_random_uuid(), product_id, $2, id, 5, 1 FROM product_variants WHERE product_id=$1`, p.ID, sellerID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO product_images (id, product_id, image_url, sort_order, is_main, rendition_url) VALUES
			(gen_random_uuid(),$1,'https://img.test/a.jpg',0,true,'https://img.test/a_r.jpg'),(gen_random_uuid(),$1,'https://img.test/b.jpg',1,false,'https://img.test/b_r.jpg')`, p.ID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE products SET status='published', published_at=now(), description='Старое описание' WHERE id=$1`, p.ID)
		require.NoError(t, err)
		return p.ID
	}
	status := func(id uuid.UUID) string {
		var s string
		require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM products WHERE id=$1`, id).Scan(&s))
		return s
	}
	warehouse := func(id uuid.UUID) string { // stock, reserves, SKU ids: must never change through moderation
		var out string
		require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE(string_agg(v.id::text||':'||v.seller_sku||':'||COALESCE(v.barcode,'')||':'||i.total_stock||':'||i.reserved_stock, ',' ORDER BY v.seller_sku),'')
			FROM product_variants v JOIN inventory_items i ON i.product_variant_id=v.id WHERE v.product_id=$1`, id).Scan(&out))
		return out
	}
	publicSKUs := func(id uuid.UUID) map[string]bool {
		pub, err := svc.GetPublicProduct(ctx, id.String())
		require.NoError(t, err)
		out := map[string]bool{}
		for _, v := range pub.Variants {
			if v.IsActive && v.SellerSKU != nil {
				out[*v.SellerSKU] = true
			}
		}
		return out
	}
	inPublicList := func(id uuid.UUID) bool {
		var n int
		require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM products p JOIN sellers s ON s.id=p.seller_id WHERE p.id=$1 AND %s`, products.CanonicalStorefrontEligibilitySQL("p", "s")), id).Scan(&n))
		return n == 1
	}
	currentReqs := func(id uuid.UUID) []products.ProductVariantRequest {
		rs, err := pool.Query(ctx, `SELECT id, seller_sku, color_id, size_value_id FROM product_variants WHERE product_id=$1 AND is_active`, id)
		require.NoError(t, err)
		defer rs.Close()
		var out []products.ProductVariantRequest
		for rs.Next() {
			var vid, c, sz uuid.UUID
			var s string
			require.NoError(t, rs.Scan(&vid, &s, &c, &sz))
			v := products.ProductVariantRequest{ID: &vid, SellerSKU: &s, ColorID: &c, SizeValueID: &sz, PriceCents: ip(10000)}
			out = append(out, v)
		}
		return out
	}
	firstImage := func(id uuid.UUID) products.ProductImageRequest {
		var iid uuid.UUID
		var url string
		require.NoError(t, pool.QueryRow(ctx, `SELECT id, image_url FROM product_images WHERE product_id=$1 ORDER BY sort_order LIMIT 1`, id).Scan(&iid, &url))
		zero, main := 0, true
		return products.ProductImageRequest{ID: &iid, ImageURL: url, SortOrder: &zero, IsMain: &main}
	}

	t.Run("published edit stays on sale with the approved version until approval, then applies atomically", func(t *testing.T) {
		id := newPublished("CSdress")
		wh := warehouse(id)
		newDesc, newTitle := "Новое описание", "CSdress v2"
		// Studio autosaves: description, then a new black/M variant and a smaller gallery, then the title.
		edits := []products.UpdateProductRequest{
			{ContinueSelling: &yes, Description: &newDesc},
			{ContinueSelling: &yes, Variants: append(currentReqs(id), vr(black, sizeM, "CSdressBM")), Images: []products.ProductImageRequest{firstImage(id)}},
			{ContinueSelling: &yes, Title: &newTitle},
		}
		for i, e := range edits {
			_, err := svc.UpdateProductForSeller(ctx, owner, id, e)
			require.NoError(t, err, "autosave %d", i+1)
			// 1. the public status stays published after every autosave; 6. no autosave reaches the live rows
			require.Equal(t, products.StatusPublished, status(id), "autosave %d", i+1)
			require.True(t, inPublicList(id), "still listed in the shop after autosave %d", i+1)
		}
		var pending int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM product_revisions WHERE product_id=$1 AND status='pending'`, id).Scan(&pending))
		require.Equal(t, 1, pending, "autosaves update one pending revision")

		pub, err := svc.GetPublicProduct(ctx, id.String())
		require.NoError(t, err)
		require.Equal(t, "CSdress", pub.Title, "buyer sees the approved title")
		require.Equal(t, "Старое описание", *pub.Description)
		require.Len(t, pub.Images, 2, "2. buyer sees the approved gallery")
		require.Equal(t, map[string]bool{sku("CSdressBS"): true, sku("CSdressWM"): true}, publicSKUs(id), "3. the new variant is not sold before approval")
		require.Equal(t, wh, warehouse(id), "8. stock, reserves and SKUs untouched")

		// the seller (Studio reload) sees the proposed state, Admin sees what changes
		sp, err := svc.GetSellerProduct(ctx, owner, id)
		require.NoError(t, err)
		require.Equal(t, newTitle, sp.Title)
		require.Equal(t, newDesc, *sp.Description)
		require.NotNil(t, sp.PendingRevision)
		require.Len(t, sp.Images, 1)
		ap, err := svc.GetAdminProductDetail(ctx, id)
		require.NoError(t, err)
		require.Equal(t, "CSdress", ap.Title, "admin product row is the live one")
		fields := map[string]string{}
		for _, f := range ap.PendingRevision.ChangedFields {
			fields[f.Field] = *f.Proposed
		}
		require.Equal(t, map[string]string{"title": newTitle, "description": newDesc}, fields)
		require.True(t, ap.PendingRevision.VariantsChanged)
		require.True(t, ap.PendingRevision.ImagesChanged)

		// 4. approval applies everything at once
		require.NoError(t, svc.ApproveProduct(ctx, adminID, id, nil))
		require.Equal(t, products.StatusPublished, status(id))
		pub, err = svc.GetPublicProduct(ctx, id.String())
		require.NoError(t, err)
		require.Equal(t, newTitle, pub.Title)
		require.Equal(t, newDesc, *pub.Description)
		require.Len(t, pub.Images, 1)
		require.True(t, publicSKUs(id)[sku("CSdressBM")])
		require.Contains(t, warehouse(id), wh, "8. existing stock rows and SKUs kept")

		// repeat approval is refused and duplicates nothing
		var variantsBefore int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM product_variants WHERE product_id=$1`, id).Scan(&variantsBefore))
		require.ErrorIs(t, svc.ApproveProduct(ctx, adminID, id, nil), products.ErrInvalidStatusTransition)
		var variantsAfter int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM product_variants WHERE product_id=$1`, id).Scan(&variantsAfter))
		require.Equal(t, variantsBefore, variantsAfter)
	})

	t.Run("rejection keeps the approved version on sale", func(t *testing.T) {
		id := newPublished("CSrej")
		wh := warehouse(id)
		desc := "Отклоняемое описание"
		_, err := svc.UpdateProductForSeller(ctx, owner, id, products.UpdateProductRequest{ContinueSelling: &yes, Description: &desc, Variants: append(currentReqs(id), vr(black, sizeM, "CSrejBM"))})
		require.NoError(t, err)
		require.NoError(t, svc.RejectProduct(ctx, adminID, id, "нужны студийные фото"))
		require.Equal(t, products.StatusPublished, status(id), "5. still published")
		require.True(t, inPublicList(id))
		pub, err := svc.GetPublicProduct(ctx, id.String())
		require.NoError(t, err)
		require.Equal(t, "Старое описание", *pub.Description)
		require.False(t, publicSKUs(id)[sku("CSrejBM")])
		require.Equal(t, wh, warehouse(id))
		sp, err := svc.GetSellerProduct(ctx, owner, id)
		require.NoError(t, err)
		require.Nil(t, sp.PendingRevision, "the rejected edit is no longer pending")
		require.Equal(t, "Старое описание", *sp.Description, "seller is back on the live version")
	})

	t.Run("continueSelling never puts a non-published product on sale", func(t *testing.T) {
		for _, st := range []string{products.StatusArchived, products.StatusHidden, products.StatusDraft, products.StatusRejected, products.StatusPendingModeration} {
			id := newPublished("CS" + st)
			_, err := pool.Exec(ctx, `UPDATE products SET status=$2 WHERE id=$1`, id, st)
			require.NoError(t, err)
			desc := "попытка"
			_, _ = svc.UpdateProductForSeller(ctx, owner, id, products.UpdateProductRequest{ContinueSelling: &yes, Description: &desc})
			require.NotEqual(t, products.StatusPublished, status(id), "7. %s must not become published", st)
			require.False(t, inPublicList(id), "7. %s must not be listed", st)
		}
		id := newPublished("CSblocked")
		_, err := pool.Exec(ctx, `UPDATE products SET status='blocked' WHERE id=$1`, id)
		require.NoError(t, err)
		desc := "попытка"
		_, err = svc.UpdateProductForSeller(ctx, owner, id, products.UpdateProductRequest{ContinueSelling: &yes, Description: &desc})
		require.ErrorIs(t, err, products.ErrProductNotEditable)
		require.Equal(t, products.StatusBlocked, status(id))
	})

	t.Run("without continueSelling the old behaviour is unchanged (explicit hide)", func(t *testing.T) {
		id := newPublished("CShide")
		no := false
		desc := "скрыть"
		_, err := svc.UpdateProductForSeller(ctx, owner, id, products.UpdateProductRequest{ContinueSelling: &no, Description: &desc})
		require.NoError(t, err)
		require.Equal(t, products.StatusPendingModeration, status(id))
	})
}
