package products_test

import (
	"context"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// CATALOG.C5.5: the first publication of a product (approval from moderation, publication of an approved product) is
// gated by the same moderation checks as a revision; nothing is written when a check fails; products already on sale
// are not taken off sale; the public API serves only the approved characteristics.
func TestFirstPublicationChecksAndPublicCharacteristics(t *testing.T) {
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

	// own taxonomy: a dress category requiring a season, composition and a size chart; a plain category requiring nothing
	adminID, catID, plainCat, sysID, sizeS := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	dictID, summer, winter, seasonDef, cotton, black := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO users (id, email, password_hash, role, name) VALUES ($1,$2,'hash','admin','C55 Admin')`, adminID, "c55-admin-"+tag+"@test.com")
	exec(`INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1,'C55 Платья',$2,true),($3,'C55 Аксессуары',$4,false)`, catID, "c55-"+tag, plainCat, "c55-plain-"+tag)
	exec(`INSERT INTO size_systems (id, code, name) VALUES ($1,$2,'C55')`, sysID, "c55-"+tag)
	exec(`INSERT INTO size_values (id, size_system_id, value, sort_order) VALUES ($1,$2,'S',1)`, sizeS, sysID)
	exec(`INSERT INTO category_size_systems (category_id, size_system_id, is_default) VALUES ($1,$2,true)`, catID, sysID)
	exec(`INSERT INTO category_size_chart_fields (id, category_id, code, name, unit, is_required, sort_order) VALUES (gen_random_uuid(),$1,'chest','Обхват груди','см',true,1)`, catID)
	exec(`INSERT INTO colors (id, code, name_ru, hex) VALUES ($1,$2,'C55 чёрный','#000000')`, black, "c55-black-"+tag)
	exec(`INSERT INTO attribute_dictionaries (id, code, name_ru) VALUES ($1,$2,'C55 Сезон')`, dictID, "c55-season-"+tag)
	exec(`INSERT INTO attribute_dictionary_values (id, dictionary_id, code, name_ru) VALUES ($1,$3,'SUMMER','Лето'),($2,$3,'WINTER','Зима')`, summer, winter, dictID)
	exec(`INSERT INTO attribute_definitions (id, code, name_ru, value_type, value_source, scope, is_active) VALUES ($1,$2,'Сезон','ENUM','DICTIONARY','PRODUCT',true)`, seasonDef, "c55_season_"+tag)
	exec(`INSERT INTO category_attribute_definitions (category_id, attribute_definition_id, required, dictionary_id) VALUES ($1,$2,true,$3)`, catID, seasonDef, dictID)
	var compDef uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM attribute_definitions WHERE value_source='MATERIAL_COMPOSITION' AND scope='PRODUCT' LIMIT 1`).Scan(&compDef))
	exec(`INSERT INTO category_attribute_definitions (category_id, attribute_definition_id, required) VALUES ($1,$2,true)`, catID, compDef)
	exec(`INSERT INTO materials (id, code, name_ru, is_active) VALUES ($1,$2,'C55 Хлопок',true)`, cotton, "c55-cotton-"+tag)
	t.Cleanup(func() {
		c := context.Background()
		cats := []uuid.UUID{catID, plainCat}
		for _, q := range []string{
			`UPDATE products SET live_revision_id=NULL WHERE category_id = ANY($1)`,
			`DELETE FROM product_revisions WHERE product_id IN (SELECT id FROM products WHERE category_id = ANY($1))`,
			`DELETE FROM product_moderation_logs WHERE product_id IN (SELECT id FROM products WHERE category_id = ANY($1))`,
			`DELETE FROM product_attribute_values WHERE product_id IN (SELECT id FROM products WHERE category_id = ANY($1))`,
			`DELETE FROM product_material_composition WHERE product_id IN (SELECT id FROM products WHERE category_id = ANY($1))`,
			`DELETE FROM product_size_charts WHERE product_id IN (SELECT id FROM products WHERE category_id = ANY($1))`,
			`DELETE FROM inventory_items WHERE product_id IN (SELECT id FROM products WHERE category_id = ANY($1))`,
			`DELETE FROM product_images WHERE product_id IN (SELECT id FROM products WHERE category_id = ANY($1))`,
			`DELETE FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE category_id = ANY($1))`,
			`DELETE FROM products WHERE category_id = ANY($1)`,
			`DELETE FROM category_attribute_definitions WHERE category_id = ANY($1)`,
			`DELETE FROM category_size_chart_fields WHERE category_id = ANY($1)`,
			`DELETE FROM category_size_systems WHERE category_id = ANY($1)`,
			`DELETE FROM categories WHERE id = ANY($1)`,
		} {
			pool.Exec(c, q, cats)
		}
		pool.Exec(c, `DELETE FROM attribute_definitions WHERE id=$1`, seasonDef)
		pool.Exec(c, `DELETE FROM attribute_dictionary_values WHERE dictionary_id=$1`, dictID)
		pool.Exec(c, `DELETE FROM attribute_dictionaries WHERE id=$1`, dictID)
		pool.Exec(c, `DELETE FROM materials WHERE id=$1`, cotton)
		pool.Exec(c, `DELETE FROM size_values WHERE size_system_id=$1`, sysID)
		pool.Exec(c, `DELETE FROM size_systems WHERE id=$1`, sysID)
		pool.Exec(c, `DELETE FROM colors WHERE id=$1`, black)
		pool.Exec(c, `DELETE FROM users WHERE id=$1`, adminID)
	})

	type opts struct {
		category                          uuid.UUID
		photo, composition, chart, season bool
		status                            string
	}
	full := func(st string) opts { return opts{catID, true, true, true, true, st} }
	newProduct := func(name string, o opts) uuid.UUID {
		sku := "C55-" + name + "-" + tag
		c, s := black, sizeS
		req := products.CreateProductRequest{Title: "C55 " + name, CategoryID: &o.category,
			Variants: []products.ProductVariantRequest{{SellerSKU: &sku, ColorID: &c, SizeValueID: &s, PriceCents: ip(500000)}}}
		if o.season {
			req.Attributes = []products.ProductAttributeValueRequest{{AttributeDefinitionID: seasonDef, EnumValueID: &summer}}
		}
		if o.composition {
			req.MaterialComposition = []products.ProductMaterialCompositionRequest{{MaterialID: cotton, Percentage: 100}}
		}
		if o.chart {
			req.SizeChartRows = []products.ProductSizeChartRowRequest{{SizeValueID: sizeS, Measurements: map[string]interface{}{"chest": 90.0}}}
		}
		p, err := svc.CreateProductForSeller(ctx, owner, req)
		require.NoError(t, err)
		exec(`UPDATE products SET status=$2, description='Платье для проверки первой публикации.', submitted_at=now() WHERE id=$1`, p.ID, o.status)
		if o.photo {
			exec(`UPDATE products SET main_image_url='https://img.test/c55.jpg' WHERE id=$1`, p.ID)
			exec(`INSERT INTO product_images (id, product_id, image_url, sort_order, is_main) VALUES (gen_random_uuid(),$1,'https://img.test/c55.jpg',0,true)`, p.ID)
		}
		exec(`INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock)
			SELECT gen_random_uuid(), v.product_id, p.seller_id, v.id, 5, 0 FROM product_variants v JOIN products p ON p.id=v.product_id WHERE v.product_id=$1`, p.ID)
		return p.ID
	}
	type row struct {
		Status        string
		Approved, Pub bool
		Logs          int
	}
	state := func(id uuid.UUID) row {
		var r row
		require.NoError(t, pool.QueryRow(ctx, `SELECT status, approved_at IS NOT NULL, published_at IS NOT NULL, (SELECT count(*) FROM product_moderation_logs WHERE product_id=$1) FROM products WHERE id=$1`, id).Scan(&r.Status, &r.Approved, &r.Pub, &r.Logs))
		return r
	}
	failedOf := func(err error) []string {
		var ce *products.ModerationChecksError
		require.ErrorAs(t, err, &ce)
		require.False(t, ce.Revision, "a first publication, not a revision")
		return ce.Failed
	}

	t.Run("1. a valid product is published on approval", func(t *testing.T) {
		id := newProduct("valid", full(products.StatusPendingModeration))
		require.NoError(t, svc.ApproveProduct(ctx, adminID, id, nil))
		require.Equal(t, products.StatusPublished, state(id).Status)
	})

	t.Run("2/6. a missing photo blocks the approval and nothing is written", func(t *testing.T) {
		o := full(products.StatusPendingModeration)
		o.photo = false
		id := newProduct("nophoto", o)
		before := state(id)
		err := svc.ApproveProduct(ctx, adminID, id, nil)
		require.ErrorIs(t, err, products.ErrModerationChecksFailed)
		require.Equal(t, []string{products.CheckMainImage}, failedOf(err))
		require.Equal(t, before, state(id), "no status, timestamp or moderation log change")
		require.Equal(t, products.StatusPendingModeration, state(id).Status)
	})

	t.Run("3. a missing required composition blocks the approval", func(t *testing.T) {
		o := full(products.StatusPendingModeration)
		o.composition = false
		id := newProduct("nocomp", o)
		err := svc.ApproveProduct(ctx, adminID, id, nil)
		require.Equal(t, []string{products.CheckRequiredAttributes}, failedOf(err))
		require.Equal(t, products.StatusPendingModeration, state(id).Status)
	})

	t.Run("4. the size chart is required only where the category requires it", func(t *testing.T) {
		o := full(products.StatusPendingModeration)
		o.chart = false
		id := newProduct("nochart", o)
		require.Equal(t, []string{products.CheckSizeChart}, failedOf(svc.ApproveProduct(ctx, adminID, id, nil)))

		plain := opts{plainCat, true, false, false, false, products.StatusPendingModeration}
		pid := newProduct("plain", plain)
		require.NoError(t, svc.ApproveProduct(ctx, adminID, pid, nil), "a category without requirements needs no chart, composition or season")
	})

	t.Run("5. an approved product cannot be published around the moderation approval", func(t *testing.T) {
		// PublishProduct only re-publishes hidden products (eligibility requires a moderated status); an approved
		// product, valid or not, is refused and keeps its status, so ApproveProduct (gated above) is the only way in.
		o := full(products.StatusApproved)
		o.composition = false
		for _, id := range []uuid.UUID{newProduct("approved-bad", o), newProduct("approved-good", full(products.StatusApproved))} {
			before := state(id)
			require.Error(t, svc.PublishProduct(ctx, adminID, id, nil))
			require.Equal(t, before, state(id))
		}
	})

	t.Run("7. products already on sale stay on sale (no retroactive unpublishing)", func(t *testing.T) {
		o := full(products.StatusPublished)
		o.photo, o.composition = false, false
		id := newProduct("historic", o)
		ap, err := svc.GetAdminProductDetail(ctx, id)
		require.NoError(t, err)
		require.NotEmpty(t, ap.ModerationChecks)
		require.Equal(t, products.StatusPublished, state(id).Status, "reading the checks never changes the status")
		_, err = svc.GetPublicProduct(ctx, id.String())
		require.NoError(t, err, "still sold")
		require.NoError(t, svc.HideProduct(ctx, adminID, id, nil))
		require.NoError(t, svc.PublishProduct(ctx, adminID, id, nil), "un-hiding a product that was on sale is not a new publication")
		require.Equal(t, products.StatusPublished, state(id).Status)
	})

	t.Run("8/9/10. the public API serves only the approved characteristics", func(t *testing.T) {
		id := newProduct("chars", full(products.StatusPendingModeration))
		require.NoError(t, svc.ApproveProduct(ctx, adminID, id, nil))
		chars := func() []products.PublicCharacteristic {
			p, err := svc.GetPublicProduct(ctx, id.String())
			require.NoError(t, err)
			return p.Characteristics
		}
		require.Equal(t, []products.PublicCharacteristic{{Name: "Сезон", Value: "Лето"}}, chars(), "8. readable, approved; composition is not repeated here")

		_, err := svc.UpdateProductForSeller(ctx, owner, id, products.UpdateProductRequest{ContinueSelling: &yes,
			Attributes: []products.ProductAttributeValueRequest{{AttributeDefinitionID: seasonDef, EnumValueID: &winter}}})
		require.NoError(t, err)
		require.Equal(t, []products.PublicCharacteristic{{Name: "Сезон", Value: "Лето"}}, chars(), "9. the pending value stays hidden")

		require.NoError(t, svc.ApproveProduct(ctx, adminID, id, nil))
		require.Equal(t, []products.PublicCharacteristic{{Name: "Сезон", Value: "Зима"}}, chars(), "10. visible after approval")
	})
}
