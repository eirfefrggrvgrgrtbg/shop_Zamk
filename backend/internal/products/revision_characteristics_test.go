package products_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// CATALOG.C5.4: characteristics, composition and the size chart of a published product are edited through the pending
// revision (continueSelling): invisible to the public until approval, kept across autosaves, applied atomically on
// approval, untouched by rejection or by a conflicting revision; approval checks the resulting product.
func TestRevisionCharacteristicsCompositionSizeChart(t *testing.T) {
	db, svc, owner := setupBlockATestDB(t)
	t.Cleanup(db.Close)
	pool := db.Pool
	ctx := context.Background()
	tag := uuid.NewString()[:6]
	yes := true
	exec := func(sql string, args ...any) {
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}

	// ---- own taxonomy: category with a required dictionary characteristic, required composition, required size chart
	adminID, catID, sysID, sizeS, sizeM, black := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	dictID, summer, winter, seasonDef := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	cotton, poly := uuid.New(), uuid.New()
	exec(`INSERT INTO users (id, email, password_hash, role, name) VALUES ($1,$2,'hash','admin','C54 Admin')`, adminID, "c54-admin-"+tag+"@test.com")
	exec(`INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1,'C54 Платья',$2,true)`, catID, "c54-"+tag)
	exec(`INSERT INTO size_systems (id, code, name) VALUES ($1,$2,'C54')`, sysID, "c54-"+tag)
	exec(`INSERT INTO size_values (id, size_system_id, value, sort_order) VALUES ($1,$3,'S',1),($2,$3,'M',2)`, sizeS, sizeM, sysID)
	exec(`INSERT INTO category_size_systems (category_id, size_system_id, is_default) VALUES ($1,$2,true)`, catID, sysID)
	exec(`INSERT INTO category_size_chart_fields (id, category_id, code, name, unit, is_required, sort_order) VALUES (gen_random_uuid(),$1,'chest','Обхват груди','см',true,1)`, catID)
	exec(`INSERT INTO colors (id, code, name_ru, hex) VALUES ($1,$2,'C54 чёрный','#000000')`, black, "c54-black-"+tag)
	exec(`INSERT INTO attribute_dictionaries (id, code, name_ru) VALUES ($1,$2,'C54 Сезон')`, dictID, "c54-season-"+tag)
	exec(`INSERT INTO attribute_dictionary_values (id, dictionary_id, code, name_ru) VALUES ($1,$3,'SUMMER','Лето'),($2,$3,'WINTER','Зима')`, summer, winter, dictID)
	exec(`INSERT INTO attribute_definitions (id, code, name_ru, value_type, value_source, scope, is_active) VALUES ($1,$2,'C54 Сезон','ENUM','DICTIONARY','PRODUCT',true)`, seasonDef, "c54_season_"+tag)
	exec(`INSERT INTO category_attribute_definitions (category_id, attribute_definition_id, required, dictionary_id) VALUES ($1,$2,true,$3)`, catID, seasonDef, dictID)
	var compDef uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM attribute_definitions WHERE value_source='MATERIAL_COMPOSITION' AND scope='PRODUCT' LIMIT 1`).Scan(&compDef))
	exec(`INSERT INTO category_attribute_definitions (category_id, attribute_definition_id, required) VALUES ($1,$2,true)`, catID, compDef)
	exec(`INSERT INTO materials (id, code, name_ru, is_active) VALUES ($1,$3,'C54 Хлопок',true),($2,$4,'C54 Полиэстер',true)`, cotton, poly, "c54-cotton-"+tag, "c54-poly-"+tag)
	t.Cleanup(func() {
		c := context.Background()
		for _, q := range []string{
			`UPDATE products SET live_revision_id=NULL WHERE category_id=$1`,
			`DELETE FROM product_revisions WHERE product_id IN (SELECT id FROM products WHERE category_id=$1)`,
			`DELETE FROM product_moderation_logs WHERE product_id IN (SELECT id FROM products WHERE category_id=$1)`,
			`DELETE FROM product_attribute_values WHERE product_id IN (SELECT id FROM products WHERE category_id=$1)`,
			`DELETE FROM product_material_composition WHERE product_id IN (SELECT id FROM products WHERE category_id=$1)`,
			`DELETE FROM product_size_charts WHERE product_id IN (SELECT id FROM products WHERE category_id=$1)`,
			`DELETE FROM inventory_items WHERE product_id IN (SELECT id FROM products WHERE category_id=$1)`,
			`DELETE FROM product_images WHERE product_id IN (SELECT id FROM products WHERE category_id=$1)`,
			`DELETE FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE category_id=$1)`,
			`DELETE FROM products WHERE category_id=$1`,
			`DELETE FROM category_attribute_definitions WHERE category_id=$1`,
			`DELETE FROM category_size_chart_fields WHERE category_id=$1`,
			`DELETE FROM category_size_systems WHERE category_id=$1`,
			`DELETE FROM categories WHERE id=$1`,
		} {
			pool.Exec(c, q, catID)
		}
		pool.Exec(c, `DELETE FROM attribute_definitions WHERE id=$1`, seasonDef)
		pool.Exec(c, `DELETE FROM attribute_dictionary_values WHERE dictionary_id=$1`, dictID)
		pool.Exec(c, `DELETE FROM attribute_dictionaries WHERE id=$1`, dictID)
		pool.Exec(c, `DELETE FROM materials WHERE id = ANY($1)`, []uuid.UUID{cotton, poly})
		pool.Exec(c, `DELETE FROM size_values WHERE size_system_id=$1`, sysID)
		pool.Exec(c, `DELETE FROM size_systems WHERE id=$1`, sysID)
		pool.Exec(c, `DELETE FROM colors WHERE id=$1`, black)
		pool.Exec(c, `DELETE FROM users WHERE id=$1`, adminID)
	})

	season := func(v uuid.UUID) []products.ProductAttributeValueRequest {
		return []products.ProductAttributeValueRequest{{AttributeDefinitionID: seasonDef, EnumValueID: &v}}
	}
	comp := func(parts map[uuid.UUID]float64) []products.ProductMaterialCompositionRequest {
		var out []products.ProductMaterialCompositionRequest
		for id, pct := range parts {
			out = append(out, products.ProductMaterialCompositionRequest{MaterialID: id, Percentage: pct})
		}
		return out
	}
	chart := func(rows map[uuid.UUID]float64) []products.ProductSizeChartRowRequest {
		var out []products.ProductSizeChartRowRequest
		for id, chest := range rows {
			out = append(out, products.ProductSizeChartRowRequest{SizeValueID: id, Measurements: map[string]interface{}{"chest": chest}})
		}
		return out
	}

	// A published, publishable dress: season=summer, 100% cotton, size chart S=90, one photo, black/S on sale.
	newPublished := func(name string) uuid.UUID {
		sku := "C54-" + name + "-" + tag
		c, s := black, sizeS
		p, err := svc.CreateProductForSeller(ctx, owner, products.CreateProductRequest{
			Title: "C54 " + name, CategoryID: &catID,
			Variants:            []products.ProductVariantRequest{{SellerSKU: &sku, ColorID: &c, SizeValueID: &s, PriceCents: ip(700000)}},
			Attributes:          season(summer),
			MaterialComposition: comp(map[uuid.UUID]float64{cotton: 100}),
			SizeChartRows:       chart(map[uuid.UUID]float64{sizeS: 90}),
		})
		require.NoError(t, err)
		exec(`UPDATE products SET status='published', published_at=now(), description='Платье для проверки характеристик.', main_image_url='https://img.test/c54.jpg' WHERE id=$1`, p.ID)
		exec(`INSERT INTO product_images (id, product_id, image_url, sort_order, is_main) VALUES (gen_random_uuid(),$1,'https://img.test/c54.jpg',0,true)`, p.ID)
		exec(`INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock)
			SELECT gen_random_uuid(), v.product_id, p.seller_id, v.id, 4, 1 FROM product_variants v JOIN products p ON p.id=v.product_id WHERE v.product_id=$1`, p.ID)
		return p.ID
	}

	type state struct {
		Season, Composition, Chart string
		ChartID                    uuid.UUID
		Rows                       int
	}
	describe := func(p products.Product) state {
		st := state{}
		for _, a := range p.Attributes {
			if a.AttributeDefinitionID == seasonDef && a.EnumValueID != nil {
				st.Season = map[uuid.UUID]string{summer: "summer", winter: "winter"}[*a.EnumValueID]
			}
		}
		var parts []string
		for _, c := range p.MaterialComposition {
			parts = append(parts, fmt.Sprintf("%s=%g", map[uuid.UUID]string{cotton: "cotton", poly: "poly"}[c.MaterialID], c.Percentage))
		}
		sort.Strings(parts)
		st.Composition = fmt.Sprint(parts)
		if p.SizeChart != nil {
			st.ChartID = p.SizeChart.ID
			var rows []string
			for _, r := range p.SizeChart.Rows {
				rows = append(rows, fmt.Sprintf("%s=%v", map[uuid.UUID]string{sizeS: "S", sizeM: "M"}[r.SizeValueID], r.Measurements["chest"]))
			}
			sort.Strings(rows)
			st.Chart = fmt.Sprint(rows)
			st.Rows = len(rows)
		}
		return st
	}
	// public: composition and size chart exactly as the public API serves them; characteristics (not part of the public
	// DTO) from the live product the public reads are built from.
	public := func(id uuid.UUID) state {
		pub, err := svc.GetPublicProduct(ctx, id.String())
		require.NoError(t, err)
		live, err := svc.GetAdminProductDetail(ctx, id)
		require.NoError(t, err)
		return describe(products.Product{Attributes: live.Attributes, MaterialComposition: pub.MaterialComposition, SizeChart: pub.SizeChart})
	}
	seller := func(id uuid.UUID) state {
		p, err := svc.GetSellerProduct(ctx, owner, id)
		require.NoError(t, err)
		return describe(p)
	}
	status := func(id uuid.UUID) string {
		var s string
		require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM products WHERE id=$1`, id).Scan(&s))
		return s
	}
	tableRows := func(id uuid.UUID) string {
		var a, c, ch, r int
		require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM product_attribute_values WHERE product_id=$1), (SELECT count(*) FROM product_material_composition WHERE product_id=$1),
			(SELECT count(*) FROM product_size_charts WHERE product_id=$1), (SELECT count(*) FROM product_size_chart_rows r JOIN product_size_charts c ON c.id=r.size_chart_id WHERE c.product_id=$1)`, id).Scan(&a, &c, &ch, &r))
		return fmt.Sprintf("attrs=%d comp=%d charts=%d rows=%d", a, c, ch, r)
	}
	warehouse := func(id uuid.UUID) string {
		var out string
		require.NoError(t, pool.QueryRow(ctx, `SELECT string_agg(v.id||':'||v.seller_sku||':'||COALESCE(v.barcode,'')||':'||i.total_stock||':'||i.reserved_stock, ',') FROM product_variants v JOIN inventory_items i ON i.product_variant_id=v.id WHERE v.product_id=$1`, id).Scan(&out))
		return out
	}
	old := state{Season: "summer", Composition: "[cotton=100]", Chart: "[S=90]", Rows: 1}
	withoutID := func(s state) state { s.ChartID = uuid.Nil; return s }

	t.Run("composition, characteristics and size chart: pending until approval, kept across autosaves, applied atomically", func(t *testing.T) {
		id := newPublished("main")
		live0 := public(id)
		require.Equal(t, old, withoutID(live0))
		wh := warehouse(id)

		edit := func(req products.UpdateProductRequest) {
			req.ContinueSelling = &yes
			_, err := svc.UpdateProductForSeller(ctx, owner, id, req)
			require.NoError(t, err)
			require.Equal(t, products.StatusPublished, status(id))
		}
		edit(products.UpdateProductRequest{MaterialComposition: comp(map[uuid.UUID]float64{cotton: 60, poly: 40})}) // 1
		edit(products.UpdateProductRequest{Attributes: season(winter)})                                             // 2
		edit(products.UpdateProductRequest{SizeChartRows: chart(map[uuid.UUID]float64{sizeS: 92, sizeM: 96})})        // 3
		title := "C54 main v2"
		edit(products.UpdateProductRequest{Title: &title}) // 7: an autosave without them keeps the pending values

		want := state{Season: "winter", Composition: "[cotton=60 poly=40]", Chart: "[M=96 S=92]", Rows: 2}
		require.Equal(t, live0, public(id), "4. the shop still shows the approved characteristics")
		require.Equal(t, "attrs=1 comp=1 charts=1 rows=1", tableRows(id), "live rows untouched before approval")
		require.Equal(t, want, withoutID(seller(id)), "Studio shows (and will autosave) the pending values")

		ap, err := svc.GetAdminProductDetail(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, ap.PendingRevision)
		require.True(t, ap.PendingRevision.AttributesChanged && ap.PendingRevision.MaterialCompositionChanged && ap.PendingRevision.SizeChartChanged)
		require.Equal(t, want, withoutID(describe(products.Product{Attributes: ap.PendingRevision.Attributes, MaterialComposition: ap.PendingRevision.MaterialComposition,
			SizeChart: &products.ProductSizeChart{Rows: ap.PendingRevision.SizeChartRows}})), "Admin sees the proposed values")
		require.NotNil(t, ap.PendingRevision.MaterialComposition[0].MaterialName, "proposed composition carries material names")
		require.Len(t, ap.PendingRevision.AttributeChanges, 1)
		ac := ap.PendingRevision.AttributeChanges[0]
		require.Equal(t, "C54 Сезон", ac.Name)
		require.Equal(t, "Лето", *ac.Current)
		require.Equal(t, "Зима", *ac.Proposed)
		for _, c := range ap.ModerationChecks {
			require.True(t, c.Pass, "check %s on the resulting product", c.Code)
		}

		require.NoError(t, svc.ApproveProduct(ctx, adminID, id, nil))
		after := public(id)
		require.Equal(t, want, withoutID(after), "5. the shop shows the approved new version")
		require.Equal(t, live0.ChartID, after.ChartID, "the size chart keeps its id")
		require.Equal(t, "attrs=1 comp=2 charts=1 rows=2", tableRows(id), "no duplicates")
		require.Equal(t, wh, warehouse(id), "stock, reserves, SKUs and barcodes untouched")
		pp, err := svc.GetPublicProduct(ctx, id.String())
		require.NoError(t, err)
		require.Equal(t, "C54 main v2", pp.Title)

		// 8. repeated approval is refused and changes nothing
		require.ErrorIs(t, svc.ApproveProduct(ctx, adminID, id, nil), products.ErrInvalidStatusTransition)
		require.Equal(t, "attrs=1 comp=2 charts=1 rows=2", tableRows(id))
		require.Equal(t, want, withoutID(public(id)))
	})

	t.Run("rejection keeps the live characteristics", func(t *testing.T) {
		id := newPublished("rej")
		_, err := svc.UpdateProductForSeller(ctx, owner, id, products.UpdateProductRequest{ContinueSelling: &yes,
			Attributes: season(winter), MaterialComposition: comp(map[uuid.UUID]float64{poly: 100}), SizeChartRows: chart(map[uuid.UUID]float64{sizeM: 99})})
		require.NoError(t, err)
		require.NoError(t, svc.RejectProduct(ctx, adminID, id, "состав указан неверно"))
		require.Equal(t, products.StatusPublished, status(id))
		require.Equal(t, old, withoutID(public(id)), "6. live version unchanged")
		require.Equal(t, old, withoutID(seller(id)), "seller is back on the live version")
	})

	t.Run("a revision whose result misses a required characteristic or composition is refused", func(t *testing.T) {
		id := newPublished("req")
		_, err := svc.UpdateProductForSeller(ctx, owner, id, products.UpdateProductRequest{ContinueSelling: &yes, MaterialComposition: comp(map[uuid.UUID]float64{cotton: 50})})
		require.NoError(t, err, "autosave of an incomplete draft is allowed")
		ap, err := svc.GetAdminProductDetail(ctx, id)
		require.NoError(t, err)
		failed := map[string]bool{}
		for _, c := range ap.ModerationChecks {
			if !c.Pass {
				failed[c.Code] = true
			}
		}
		require.Equal(t, map[string]bool{products.CheckRequiredAttributes: true}, failed, "Admin sees the failing check of the RESULT")
		err = svc.ApproveProduct(ctx, adminID, id, nil)
		require.ErrorIs(t, err, products.ErrModerationChecksFailed)
		require.Equal(t, old, withoutID(public(id)))
		var pending int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM product_revisions WHERE product_id=$1 AND status='pending'`, id).Scan(&pending))
		require.Equal(t, 1, pending, "the refused revision stays pending for a decision")
	})

	t.Run("a conflicting revision changes nothing (no partial apply)", func(t *testing.T) {
		id := newPublished("conf")
		vs := func() []products.ProductVariantRequest {
			var vid uuid.UUID
			var sku string
			require.NoError(t, pool.QueryRow(ctx, `SELECT id, seller_sku FROM product_variants WHERE product_id=$1`, id).Scan(&vid, &sku))
			c, s, m := black, sizeS, sizeM
			newSKU := "C54-conf-M-" + tag
			return []products.ProductVariantRequest{{ID: &vid, SellerSKU: &sku, ColorID: &c, SizeValueID: &s, PriceCents: ip(700000)}, {SellerSKU: &newSKU, ColorID: &c, SizeValueID: &m, PriceCents: ip(700000)}}
		}()
		_, err := svc.UpdateProductForSeller(ctx, owner, id, products.UpdateProductRequest{ContinueSelling: &yes, Variants: vs,
			Attributes: season(winter), MaterialComposition: comp(map[uuid.UUID]float64{poly: 100}), SizeChartRows: chart(map[uuid.UUID]float64{sizeS: 91, sizeM: 97})})
		require.NoError(t, err)
		exec(`UPDATE product_variants SET updated_at = now() + interval '1 second' WHERE product_id=$1`, id) // live variants moved after submission
		before := tableRows(id)
		err = svc.ApproveProduct(ctx, adminID, id, nil)
		require.ErrorIs(t, err, products.ErrRevisionConflict)
		require.Equal(t, before, tableRows(id), "11. nothing applied")
		require.Equal(t, old, withoutID(public(id)))
		require.Equal(t, products.StatusPublished, status(id))
	})
}
