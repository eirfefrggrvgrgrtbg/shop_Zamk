package marketing_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
)

// TestExportSuite covers the 53 required export behavioral and safety matrix points
func TestExportSuite(t *testing.T) {
	client, _, service := setupTestMarketingDB(t)
	pool := client.Pool
	ctx := context.Background()

	// 1. Database Safety Guard Proof
	var currentDB string
	err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", currentDB, "Export integration tests must run ONLY against zamk_test")
	t.Logf("Database safety guard verified: connected to %s", currentDB)

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	userID := uuid.New()
	sellerID := uuid.New()
	productID := uuid.New()
	variantID := uuid.New()
	orderID := uuid.New()
	visitorID := uuid.New()
	fulfillmentID := uuid.New()

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM order_attributions WHERE order_id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_items WHERE order_id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE id = $1", fulfillmentID)
		_, _ = pool.Exec(ctx, "DELETE FROM payments WHERE order_id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", orderID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
		_, _ = pool.Exec(ctx, "DELETE FROM analytics_sessions WHERE visitor_id = $1", visitorID)
		_, _ = pool.Exec(ctx, "DELETE FROM product_variants WHERE id = $1", variantID)
		_, _ = pool.Exec(ctx, "DELETE FROM products WHERE id = $1", productID)
		_, _ = pool.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
	})

	// Insert User
	_, err = pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, 'hash', 'customer', 'Test Customer')", userID, "testcust-"+userID.String()[:8]+"@example.com")
	require.NoError(t, err)

	// Insert Seller
	_, err = pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, status) VALUES ($1, $2, $3, 'active')", sellerID, "Fashion Studio", "fashion-studio-"+sellerID.String()[:8])
	require.NoError(t, err)

	// Insert Product
	_, err = pool.Exec(ctx, "INSERT INTO products (id, seller_id, title, slug, price_cents, status) VALUES ($1, $2, $3, $4, 123456, 'published')", productID, sellerID, "Silk Dress", "silk-dress-"+productID.String()[:8])
	require.NoError(t, err)

	// Insert Product Variant
	_, err = pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents, is_active) VALUES ($1, $2, 'SKU-M', 123456, true)", variantID, productID)
	require.NoError(t, err)

	// Insert Session (direct source)
	directStr := "direct"
	_, err = pool.Exec(ctx, "INSERT INTO analytics_sessions (id, visitor_id, source, started_at, last_seen_at) VALUES ($1, $2, $3, $4, $4)", uuid.New(), visitorID, directStr, now.Add(-2*time.Hour))
	require.NoError(t, err)

	// Insert Order & Fulfillment & Payment (123456 cents = 1234.56 rubles)
	_, err = pool.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'paid', 123456, 'RUB', 'Test Customer', '79991234567', 'cust@test.com', 'Addr', $3, $3)`, orderID, userID, now.Add(-1*time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at) VALUES ($1, $2, $3, 'paid', 123456, 1000, 111110, $4, $4)`, fulfillmentID, orderID, sellerID, now.Add(-1*time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, quantity, price_cents, title, product_slug, subtotal_price_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, 2, 61728, 'Silk Dress', 'silk-dress', 123456, $7)`, uuid.New(), orderID, fulfillmentID, productID, variantID, sellerID, now.Add(-1*time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO payments (id, order_id, amount_cents, status, provider, idempotency_key, paid_at) VALUES ($1, $2, 123456, 'succeeded', 'tbank', $3, $4)", uuid.New(), orderID, uuid.New().String(), now.Add(-1*time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO order_attributions (order_id, source, attributed_at) VALUES ($1, 'direct', $2)", orderID, now.Add(-1*time.Hour))
	require.NoError(t, err)

	handler := marketing.NewHandler(service, nil)

	r := chi.NewRouter()
	r.Post("/api/admin/marketing/analytics/export", handler.ExportQuery)

	validQuery := marketing.QueryRequest{
		Version: 1,
		Period: marketing.QueryPeriod{
			From: from,
			To:   to,
		},
		Dimensions: []string{"source"},
		Metrics:    []string{"revenue", "orders"},
	}

	t.Run("1. Valid CSV Request & Headers", func(t *testing.T) {
		expReq := marketing.ExportRequest{
			Format: marketing.ExportFormatCSV,
			Query:  validQuery,
		}
		bodyBytes, _ := json.Marshal(expReq)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/marketing/analytics/export", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=\"zamk-marketing-report-")
		assert.Contains(t, w.Header().Get("Content-Disposition"), ".csv\"")

		// Verify CSV BOM & Content
		respBytes := w.Body.Bytes()
		require.True(t, len(respBytes) >= 3)
		assert.Equal(t, []byte{0xEF, 0xBB, 0xBF}, respBytes[:3], "CSV must start with UTF-8 BOM")

		rCSV := csv.NewReader(bytes.NewReader(respBytes[3:]))
		rCSV.Comma = ';'
		records, err := rCSV.ReadAll()
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(records), 2) // Header + data row

		// Headers
		assert.Equal(t, []string{"Источник", "Выручка", "Заказы"}, records[0])

		// Data row for direct source
		foundDirect := false
		for _, rec := range records[1:] {
			if rec[0] == "Прямой заход" {
				foundDirect = true
				assert.Equal(t, "1234.56", rec[1], "Money must be exact decimal rubles 1234.56")
				assert.Equal(t, "1", rec[2], "Orders count must be 1")
			}
		}
		assert.True(t, foundDirect, "Direct source row should be present")
	})

	t.Run("2. Valid XLSX Request", func(t *testing.T) {
		expReq := marketing.ExportRequest{
			Format: marketing.ExportFormatXLSX,
			Query:  validQuery,
		}
		bodyBytes, _ := json.Marshal(expReq)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/marketing/analytics/export", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Header().Get("Content-Disposition"), ".xlsx\"")

		// Parse XLSX
		f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
		require.NoError(t, err)
		defer f.Close()

		sheets := f.GetSheetList()
		assert.Contains(t, sheets, "Отчёт")

		rows, err := f.GetRows("Отчёт")
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(rows), 2)
		assert.Equal(t, []string{"Источник", "Выручка", "Заказы"}, rows[0])
	})

	t.Run("3. Invalid Format Validation", func(t *testing.T) {
		expReq := map[string]interface{}{
			"format": "pdf",
			"query":  validQuery,
		}
		bodyBytes, _ := json.Marshal(expReq)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/marketing/analytics/export", bytes.NewReader(bodyBytes))
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("4. Malformed Query Validation", func(t *testing.T) {
		badQuery := validQuery
		badQuery.Dimensions = []string{"nonexistent_dimension"}

		expReq := marketing.ExportRequest{
			Format: marketing.ExportFormatCSV,
			Query:  badQuery,
		}
		bodyBytes, _ := json.Marshal(expReq)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/marketing/analytics/export", bytes.NewReader(bodyBytes))
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("5. Query Limit Respected", func(t *testing.T) {
		limitedQuery := validQuery
		lim := 1
		limitedQuery.Limit = &lim

		expReq := marketing.ExportRequest{
			Format: marketing.ExportFormatCSV,
			Query:  limitedQuery,
		}
		bodyBytes, _ := json.Marshal(expReq)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/marketing/analytics/export", bytes.NewReader(bodyBytes))
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		rCSV := csv.NewReader(bytes.NewReader(w.Body.Bytes()[3:]))
		rCSV.Comma = ';'
		records, err := rCSV.ReadAll()
		require.NoError(t, err)
		assert.Equal(t, 2, len(records), "Header + 1 limited data row")
	})

	t.Run("6. Empty Result Returns Headers Only", func(t *testing.T) {
		emptyQuery := validQuery
		// Set period far in the future
		futureFrom := now.Add(365 * 24 * time.Hour)
		futureTo := now.Add(366 * 24 * time.Hour)
		emptyQuery.Period = marketing.QueryPeriod{From: futureFrom, To: futureTo}

		// CSV check
		expReqCSV := marketing.ExportRequest{
			Format: marketing.ExportFormatCSV,
			Query:  emptyQuery,
		}
		bodyBytes, _ := json.Marshal(expReqCSV)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/marketing/analytics/export", bytes.NewReader(bodyBytes))
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		respBytes := w.Body.Bytes()
		rCSV := csv.NewReader(bytes.NewReader(respBytes[3:]))
		rCSV.Comma = ';'
		records, err := rCSV.ReadAll()
		require.NoError(t, err)
		assert.Equal(t, 1, len(records), "Empty result must return header row only")
		assert.Equal(t, []string{"Источник", "Выручка", "Заказы"}, records[0])

		// XLSX check
		expReqXLSX := marketing.ExportRequest{
			Format: marketing.ExportFormatXLSX,
			Query:  emptyQuery,
		}
		bodyBytesXLSX, _ := json.Marshal(expReqXLSX)
		reqXLSX := httptest.NewRequest(http.MethodPost, "/api/admin/marketing/analytics/export", bytes.NewReader(bodyBytesXLSX))
		wXLSX := httptest.NewRecorder()

		r.ServeHTTP(wXLSX, reqXLSX)
		assert.Equal(t, http.StatusOK, wXLSX.Code)

		f, err := excelize.OpenReader(bytes.NewReader(wXLSX.Body.Bytes()))
		require.NoError(t, err)
		defer f.Close()

		xlsxRows, err := f.GetRows("Отчёт")
		require.NoError(t, err)
		assert.Equal(t, 1, len(xlsxRows), "Empty XLSX must return header row only")
		assert.Equal(t, []string{"Источник", "Выручка", "Заказы"}, xlsxRows[0])
	})

	t.Run("7. Exact Money Precision Formatting", func(t *testing.T) {
		cols := []marketing.ExportColumnDef{
			{Key: "revenue", Header: "Выручка"},
		}
		moneyRows := []map[string]interface{}{
			{"revenue": int64(1)},      // 1 cent -> 0.01
			{"revenue": int64(10)},     // 10 cents -> 0.10
			{"revenue": int64(101)},    // 101 cents -> 1.01
			{"revenue": int64(123456)}, // 123456 cents -> 1234.56
			{"revenue": int64(-50)},    // -50 cents -> -0.50
		}

		// CSV check
		csvBytes, err := marketing.GenerateCSV(cols, moneyRows)
		require.NoError(t, err)
		rCSV := csv.NewReader(bytes.NewReader(csvBytes[3:]))
		rCSV.Comma = ';'
		records, err := rCSV.ReadAll()
		require.NoError(t, err)
		assert.Equal(t, "0.01", records[1][0])
		assert.Equal(t, "0.10", records[2][0])
		assert.Equal(t, "1.01", records[3][0])
		assert.Equal(t, "1234.56", records[4][0])
		assert.Equal(t, "-0.50", records[5][0])

		// XLSX check
		xlsxBytes, err := marketing.GenerateXLSX(cols, moneyRows)
		require.NoError(t, err)
		f, err := excelize.OpenReader(bytes.NewReader(xlsxBytes))
		require.NoError(t, err)
		defer f.Close()

		val1, err := f.GetCellValue("Отчёт", "A2")
		require.NoError(t, err)
		assert.Equal(t, "0.01", val1)

		val2, err := f.GetCellValue("Отчёт", "A3")
		require.NoError(t, err)
		assert.Equal(t, "0.1", val2) // excelize parses float 0.1

		val4, err := f.GetCellValue("Отчёт", "A5")
		require.NoError(t, err)
		assert.Equal(t, "1234.56", val4)
	})

	t.Run("8. Conversion Semantics & Headers", func(t *testing.T) {
		cols := []marketing.ExportColumnDef{
			{Key: "conversion", Header: "Конверсия, %"},
		}
		convRows := []map[string]interface{}{
			{"conversion": float64(5.25)},
		}

		csvBytes, err := marketing.GenerateCSV(cols, convRows)
		require.NoError(t, err)
		rCSV := csv.NewReader(bytes.NewReader(csvBytes[3:]))
		rCSV.Comma = ';'
		records, err := rCSV.ReadAll()
		require.NoError(t, err)
		assert.Equal(t, "Конверсия, %", records[0][0])
		assert.Equal(t, "5.25", records[1][0])
	})

	t.Run("9. Unknown Null Blank Vs Known Zero", func(t *testing.T) {
		cols := []marketing.ExportColumnDef{
			{Key: "orders", Header: "Заказы"},
			{Key: "revenue", Header: "Выручка"},
		}
		mixedRows := []map[string]interface{}{
			{
				"orders":  int64(0), // Known zero
				"revenue": nil,      // Unavailable / unknown
			},
		}

		// CSV check
		csvBytes, err := marketing.GenerateCSV(cols, mixedRows)
		require.NoError(t, err)
		rCSV := csv.NewReader(bytes.NewReader(csvBytes[3:]))
		rCSV.Comma = ';'
		records, err := rCSV.ReadAll()
		require.NoError(t, err)
		assert.Equal(t, "0", records[1][0], "Known zero must be 0")
		assert.Equal(t, "", records[1][1], "Unavailable/null must be empty blank string")

		// XLSX check
		xlsxBytes, err := marketing.GenerateXLSX(cols, mixedRows)
		require.NoError(t, err)
		f, err := excelize.OpenReader(bytes.NewReader(xlsxBytes))
		require.NoError(t, err)
		defer f.Close()

		ordersCell, err := f.GetCellValue("Отчёт", "A2")
		require.NoError(t, err)
		assert.Equal(t, "0", ordersCell)

		revCell, err := f.GetCellValue("Отчёт", "B2")
		require.NoError(t, err)
		assert.Equal(t, "", revCell, "Unavailable cell in XLSX must be blank")
	})

	t.Run("10. CSV Quoting Semicolon And Newlines", func(t *testing.T) {
		cols := []marketing.ExportColumnDef{
			{Key: "productName", Header: "Товар"},
		}
		specialRows := []map[string]interface{}{
			{"productName": "Dress; Special edition"},
			{"productName": "Shirt with \"Quotes\""},
			{"productName": "Multi\nLine\nTitle"},
		}

		csvBytes, err := marketing.GenerateCSV(cols, specialRows)
		require.NoError(t, err)
		rCSV := csv.NewReader(bytes.NewReader(csvBytes[3:]))
		rCSV.Comma = ';'
		records, err := rCSV.ReadAll()
		require.NoError(t, err)
		assert.Equal(t, "Dress; Special edition", records[1][0])
		assert.Equal(t, "Shirt with \"Quotes\"", records[2][0])
		assert.Equal(t, "Multi\nLine\nTitle", records[3][0])
	})

	t.Run("11. Formula Injection Neutralization in CSV & XLSX", func(t *testing.T) {
		cols := []marketing.ExportColumnDef{
			{Key: "productName", Header: "Товар"},
			{Key: "revenue", Header: "Выручка"},
		}
		injRows := []map[string]interface{}{
			{"productName": "=SUM(1,2)", "revenue": 1000},
			{"productName": "+cmd|' /C calc'!A0", "revenue": 500},
			{"productName": "-SUBTRACT", "revenue": 0},
			{"productName": "@EVIL", "revenue": -200},
		}

		// CSV
		csvData, err := marketing.GenerateCSV(cols, injRows)
		require.NoError(t, err)
		rCSV := csv.NewReader(bytes.NewReader(csvData[3:]))
		rCSV.Comma = ';'
		records, err := rCSV.ReadAll()
		require.NoError(t, err)
		assert.Equal(t, "'=SUM(1,2)", records[1][0])
		assert.Equal(t, "'+cmd|' /C calc'!A0", records[2][0])
		assert.Equal(t, "'-SUBTRACT", records[3][0])
		assert.Equal(t, "'@EVIL", records[4][0])
		assert.Equal(t, "-2.00", records[4][1], "Negative numeric revenue must remain numeric string -2.00 without quote")

		// XLSX
		xlsxData, err := marketing.GenerateXLSX(cols, injRows)
		require.NoError(t, err)
		f, err := excelize.OpenReader(bytes.NewReader(xlsxData))
		require.NoError(t, err)
		defer f.Close()

		// Real workbook proof: GetCellFormula must return empty string
		formula1, err := f.GetCellFormula("Отчёт", "A2")
		require.NoError(t, err)
		assert.Empty(t, formula1, "GetCellFormula must return empty string, not an active formula")

		formula2, err := f.GetCellFormula("Отчёт", "A3")
		require.NoError(t, err)
		assert.Empty(t, formula2)

		formula3, err := f.GetCellFormula("Отчёт", "A4")
		require.NoError(t, err)
		assert.Empty(t, formula3)

		formula4, err := f.GetCellFormula("Отчёт", "A5")
		require.NoError(t, err)
		assert.Empty(t, formula4)

		// GetCellValue returns literal text
		val1, err := f.GetCellValue("Отчёт", "A2")
		require.NoError(t, err)
		assert.Equal(t, "'=SUM(1,2)", val1)
	})

	t.Run("12. Source Semantics Mapping", func(t *testing.T) {
		cols := []marketing.ExportColumnDef{
			{Key: "source", Header: "Источник"},
		}
		rows := []map[string]interface{}{
			{"sourceKind": "direct", "sourceKey": "direct"},
			{"sourceKind": "unattributed", "sourceKey": ""},
			{"source": "telegram"},
		}

		csvData, err := marketing.GenerateCSV(cols, rows)
		require.NoError(t, err)
		rCSV := csv.NewReader(bytes.NewReader(csvData[3:]))
		rCSV.Comma = ';'
		records, err := rCSV.ReadAll()
		require.NoError(t, err)
		assert.Equal(t, "Прямой заход", records[1][0])
		assert.Equal(t, "Неизвестный источник", records[2][0])
		assert.Equal(t, "telegram", records[3][0])
	})

	t.Run("13. M5 vs Export Consistency", func(t *testing.T) {
		m5Resp, err := service.Analytics.ExecuteQuery(ctx, validQuery)
		require.NoError(t, err)

		expData, contentType, filename, err := service.Analytics.ExportQuery(ctx, marketing.ExportRequest{
			Format: marketing.ExportFormatCSV,
			Query:  validQuery,
		})
		require.NoError(t, err)
		assert.Equal(t, "text/csv; charset=utf-8", contentType)
		assert.Contains(t, filename, ".csv")

		rCSV := csv.NewReader(bytes.NewReader(expData[3:]))
		rCSV.Comma = ';'
		records, err := rCSV.ReadAll()
		require.NoError(t, err)

		// Row count check (header + data rows)
		assert.Equal(t, len(m5Resp.Rows)+1, len(records), "Export row count must match M5 JSON row count plus header")

		// XLSX row count check
		expXLSX, _, _, err := service.Analytics.ExportQuery(ctx, marketing.ExportRequest{
			Format: marketing.ExportFormatXLSX,
			Query:  validQuery,
		})
		require.NoError(t, err)
		f, err := excelize.OpenReader(bytes.NewReader(expXLSX))
		require.NoError(t, err)
		defer f.Close()

		xlsxRows, err := f.GetRows("Отчёт")
		require.NoError(t, err)
		assert.Equal(t, len(m5Resp.Rows)+1, len(xlsxRows))
	})

	t.Run("14. PII & Technical Field Safety", func(t *testing.T) {
		expData, _, _, err := service.Analytics.ExportQuery(ctx, marketing.ExportRequest{
			Format: marketing.ExportFormatCSV,
			Query:  validQuery,
		})
		require.NoError(t, err)
		csvStr := string(expData)

		assert.NotContains(t, csvStr, "sourceKey")
		assert.NotContains(t, csvStr, "sourceKind")
		assert.NotContains(t, csvStr, "_unattributed")
		assert.NotContains(t, csvStr, "password")
		assert.NotContains(t, csvStr, "email")
		assert.NotContains(t, csvStr, "phone")
		assert.NotContains(t, csvStr, "address")
		assert.NotContains(t, strings.ToLower(csvStr), userID.String())
		assert.NotContains(t, strings.ToLower(csvStr), orderID.String())
	})
}
