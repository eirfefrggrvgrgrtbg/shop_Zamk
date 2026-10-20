package products_test

import (
	"context"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func sp(s string) *string { return &s }
func ip(i int64) *int64   { return &i }

func TestSizeChartMeasurements(t *testing.T) {
	db, svc, owner := setupBlockATestDB(t)
	_, _, stranger := setupBlockATestDB(t)
	ctx := context.Background()
	pool := db.Pool

	catID, sysID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1,'SC Cat',$2,false)`, catID, "sc-"+catID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO size_systems (id, code, name) VALUES ($1,$2,'SC sys')`, sysID, "sc-"+sysID.String()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO category_size_systems (category_id, size_system_id, is_default, sort_order) VALUES ($1,$2,true,0)`, catID, sysID)
	require.NoError(t, err)
	sizeIDs := map[string]uuid.UUID{}
	for i, v := range []string{"S", "M", "L"} {
		id := uuid.New()
		sizeIDs[v] = id
		_, err = pool.Exec(ctx, `INSERT INTO size_values (id, size_system_id, value, sort_order) VALUES ($1,$2,$3,$4)`, id, sysID, v, i)
		require.NoError(t, err)
	}
	for i, f := range []struct{ code, name string }{{"CHEST", "Обхват груди"}, {"LENGTH", "Длина изделия"}, {"SLEEVE", "Длина рукава"}} {
		_, err = pool.Exec(ctx, `INSERT INTO category_size_chart_fields (id, category_id, code, name, unit, is_required, sort_order) VALUES (gen_random_uuid(),$1,$2,$3,'cm',false,$4)`, catID, f.code, f.name, i)
		require.NoError(t, err)
	}
	otherSys, otherSize := uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO size_systems (id, code, name) VALUES ($1,$2,'other')`, otherSys, "so-"+otherSys.String()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO size_values (id, size_system_id, value, sort_order) VALUES ($1,$2,'XXL',0)`, otherSize, otherSys)
	require.NoError(t, err)
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM size_systems WHERE id IN ($1,$2)`, sysID, otherSys)
		pool.Exec(c, `DELETE FROM categories WHERE id=$1`, catID)
	})

	row := func(size string, m map[string]interface{}) products.ProductSizeChartRowRequest {
		return products.ProductSizeChartRowRequest{SizeValueID: sizeIDs[size], Measurements: m}
	}
	variants := []products.ProductVariantRequest{
		{SellerSKU: sp("SC-S-" + catID.String()[:6]), SizeValueID: func() *uuid.UUID { i := sizeIDs["S"]; return &i }(), PriceCents: ip(10000)},
		{SellerSKU: sp("SC-M-" + catID.String()[:6]), SizeValueID: func() *uuid.UUID { i := sizeIDs["M"]; return &i }(), PriceCents: ip(12000)},
	}
	p, err := svc.CreateProductForSeller(ctx, owner, products.CreateProductRequest{
		Title: "SC product", CategoryID: &catID, Variants: variants,
		SizeChartRows: []products.ProductSizeChartRowRequest{
			row("M", map[string]interface{}{"CHEST": 104.5, "LENGTH": 70.0}),
			row("S", map[string]interface{}{"CHEST": 100.0, "LENGTH": 68.0, "SLEEVE": 62.0}),
		},
	})
	require.NoError(t, err)

	got, err := svc.GetSellerProduct(ctx, owner, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got.SizeChart)
	require.Len(t, got.SizeChart.Rows, 2)
	require.Equal(t, "S", *got.SizeChart.Rows[0].SizeValueName, "rows are ordered by size sort order")
	require.Equal(t, "M", *got.SizeChart.Rows[1].SizeValueName)

	// stock/price fixtures to prove chart edits never touch them
	var variantID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM product_variants WHERE product_id=$1 AND price_cents=12000`, p.ID).Scan(&variantID))
	var sellerID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT seller_id FROM products WHERE id=$1`, p.ID).Scan(&sellerID))
	_, err = pool.Exec(ctx, `INSERT INTO inventory_items (id, product_id, seller_id, product_variant_id, total_stock, reserved_stock) VALUES (gen_random_uuid(),$1,$2,$3,7,2)`, p.ID, sellerID, variantID)
	require.NoError(t, err)
	snapshot := func() string {
		var out string
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT COALESCE(string_agg(pv.id::text||':'||COALESCE(pv.price_cents,0)||':'||COALESCE(pv.size_value_id::text,'')||':'||COALESCE(ii.total_stock,0)||':'||COALESCE(ii.reserved_stock,0), ',' ORDER BY pv.id),'')
			FROM product_variants pv LEFT JOIN inventory_items ii ON ii.product_variant_id = pv.id WHERE pv.product_id=$1`, p.ID).Scan(&out))
		return out
	}
	before := snapshot()
	var priceBefore int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT price_cents FROM products WHERE id=$1`, p.ID).Scan(&priceBefore))

	t.Run("update replaces chart; removed parameter and row are gone", func(t *testing.T) {
		_, err := svc.UpdateProductForSeller(ctx, owner, p.ID, products.UpdateProductRequest{
			SizeChartRows: []products.ProductSizeChartRowRequest{row("S", map[string]interface{}{"CHEST": 101.0})},
		})
		require.NoError(t, err)
		got, err := svc.GetSellerProduct(ctx, owner, p.ID)
		require.NoError(t, err)
		require.Len(t, got.SizeChart.Rows, 1)
		require.Equal(t, map[string]interface{}{"CHEST": 101.0}, got.SizeChart.Rows[0].Measurements)
	})

	t.Run("empty chart is allowed and clears the table", func(t *testing.T) {
		_, err := svc.UpdateProductForSeller(ctx, owner, p.ID, products.UpdateProductRequest{SizeChartRows: []products.ProductSizeChartRowRequest{}})
		require.NoError(t, err)
		got, err := svc.GetSellerProduct(ctx, owner, p.ID)
		require.NoError(t, err)
		require.True(t, got.SizeChart == nil || len(got.SizeChart.Rows) == 0)
	})

	invalid := map[string][]products.ProductSizeChartRowRequest{
		"duplicate size":      {row("S", map[string]interface{}{"CHEST": 100.0}), row("S", map[string]interface{}{"CHEST": 101.0})},
		"unknown parameter":   {row("S", map[string]interface{}{"HIPS_X": 100.0})},
		"html in value":       {row("S", map[string]interface{}{"CHEST": "<script>alert(1)</script>"})},
		"numeric string":      {row("S", map[string]interface{}{"CHEST": "100"})},
		"zero":                {row("S", map[string]interface{}{"CHEST": 0.0})},
		"negative":            {row("S", map[string]interface{}{"CHEST": -5.0})},
		"too large":           {row("S", map[string]interface{}{"CHEST": 301.0})},
		"size outside system": {{SizeValueID: otherSize, Measurements: map[string]interface{}{"CHEST": 100.0}}},
		"unknown size":        {{SizeValueID: uuid.New(), Measurements: map[string]interface{}{"CHEST": 100.0}}},
	}
	for name, rows := range invalid {
		t.Run("rejects "+name, func(t *testing.T) {
			_, err := svc.UpdateProductForSeller(ctx, owner, p.ID, products.UpdateProductRequest{SizeChartRows: rows})
			require.Error(t, err)
			require.Contains(t, err.Error(), "size chart")
		})
	}

	t.Run("another seller cannot change the chart", func(t *testing.T) {
		_, err := svc.UpdateProductForSeller(ctx, stranger, p.ID, products.UpdateProductRequest{
			SizeChartRows: []products.ProductSizeChartRowRequest{row("M", map[string]interface{}{"CHEST": 1.0})},
		})
		require.Error(t, err)
	})

	t.Run("public response carries sizes and measurements only after publish", func(t *testing.T) {
		_, err := svc.UpdateProductForSeller(ctx, owner, p.ID, products.UpdateProductRequest{
			SizeChartRows: []products.ProductSizeChartRowRequest{
				row("M", map[string]interface{}{"CHEST": 104.5}), row("S", map[string]interface{}{"CHEST": 100.0}),
			},
		})
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE products SET status='published', published_at=now() WHERE id=$1`, p.ID)
		require.NoError(t, err)
		pub, err := svc.GetPublicProduct(ctx, p.ID.String())
		require.NoError(t, err)
		require.NotNil(t, pub.SizeChart)
		require.Len(t, pub.SizeChart.Rows, 2)
		require.Equal(t, "S", *pub.SizeChart.Rows[0].SizeValueName)
		require.Equal(t, 100.0, pub.SizeChart.Rows[0].Measurements["CHEST"])
	})

	require.Equal(t, before, snapshot(), "variants, prices and stock are untouched by chart edits")
	var priceAfter int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT price_cents FROM products WHERE id=$1`, p.ID).Scan(&priceAfter))
	require.Equal(t, priceBefore, priceAfter)
}
