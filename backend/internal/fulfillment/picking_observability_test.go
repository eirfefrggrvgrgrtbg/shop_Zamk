package fulfillment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/observability"
)

type logCapture struct {
	buf bytes.Buffer
}

func (c *logCapture) entries() []map[string]interface{} {
	lines := bytes.Split(bytes.TrimSpace(c.buf.Bytes()), []byte("\n"))
	var result []map[string]interface{}
	for _, l := range lines {
		if len(l) == 0 {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(l, &m); err == nil {
			result = append(result, m)
		}
	}
	return result
}

func (c *logCapture) findEvents(eventName string) []map[string]interface{} {
	var matches []map[string]interface{}
	for _, e := range c.entries() {
		if name, ok := e["event_name"].(string); ok && name == eventName {
			matches = append(matches, e)
		}
	}
	return matches
}

func (c *logCapture) clear() {
	c.buf.Reset()
}

func setupObservabilityFixture(t *testing.T, ctx context.Context) (*pickingFixture, *logCapture) {
	f := setupPickingFixture(t, ctx)
	cap := &logCapture{}
	jsonHandler := slog.NewJSONHandler(&cap.buf, &slog.HandlerOptions{})
	logger := slog.New(jsonHandler)
	f.svc.SetLogger(logger)
	return f, cap
}

func TestPickingObservability_ResultMatrix(t *testing.T) {
	ctx := context.Background()
	f, cap := setupObservabilityFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)
	f.createAllocation(t, ctx, itemID, false)
	f.createAllocation(t, ctx, itemID, false)

	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	allocs := po.Items[0].AllocatedUnits
	require.Len(t, allocs, 2)

	// 1. OK (new valid pick)
	cap.clear()
	res, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, allocs[0].UnitCode, nil)
	require.NoError(t, err)
	assert.True(t, res.ScanResult.NewlyPicked)
	scannedEvents := cap.findEvents("fulfillment.picking_unit_scanned")
	require.Len(t, scannedEvents, 1)
	assert.Equal(t, "ok", scannedEvents[0]["result"])
	assert.Equal(t, "INFO", scannedEvents[0]["level"])

	// 2. ALREADY_PICKED (duplicate scan)
	cap.clear()
	res2, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, allocs[0].UnitCode, nil)
	require.NoError(t, err)
	assert.True(t, res2.ScanResult.AlreadyPicked)
	scannedEvents = cap.findEvents("fulfillment.picking_unit_scanned")
	require.Len(t, scannedEvents, 1)
	assert.Equal(t, "already_picked", scannedEvents[0]["result"])
	assert.Equal(t, "WARN", scannedEvents[0]["level"])

	// 3. NOT_FOUND (unknown picking code)
	cap.clear()
	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, "UNKNOWN-NONEXISTENT-CODE", nil)
	assert.ErrorIs(t, err, fulfillment.ErrCodeNotFound)
	scannedEvents = cap.findEvents("fulfillment.picking_unit_scanned")
	require.Len(t, scannedEvents, 1)
	assert.Equal(t, "not_found", scannedEvents[0]["result"])
	assert.Equal(t, "WARN", scannedEvents[0]["level"])

	// 4. WRONG_VARIANT (scan ZMU belonging to a different variant)
	// Create order B with a different item/variant
	orderID_B, fulfillmentID_B := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID_B := f.createOrderItem(t, ctx, orderID_B, fulfillmentID_B, 1, 0)
	_, foreignUnallocatedZMU := f.createUnitWithStatus(t, ctx, itemID_B, "warehouse")
	// Clear allocation so it is a free unit of variant B
	_, err = f.db.Exec(ctx, `DELETE FROM order_item_allocations WHERE order_item_id = $1`, itemID_B)
	require.NoError(t, err)

	cap.clear()
	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, foreignUnallocatedZMU, nil)
	assert.ErrorIs(t, err, fulfillment.ErrUnitVariantMismatch)
	scannedEvents = cap.findEvents("fulfillment.picking_unit_scanned")
	require.Len(t, scannedEvents, 1)
	assert.Equal(t, "wrong_variant", scannedEvents[0]["result"])
	assert.Equal(t, "WARN", scannedEvents[0]["level"])

	// 5. ALLOCATED_TO_OTHER_ORDER (scan unit allocated to order B)
	orderID_C, fulfillmentID_C := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID_C := f.createOrderItem(t, ctx, orderID_C, fulfillmentID_C, 1, 0)
	f.createAllocation(t, ctx, itemID_C, false)
	po_C, err := f.svc.GetPickingOrder(ctx, fulfillmentID_C)
	require.NoError(t, err)
	allocatedToOtherZMU := po_C.Items[0].AllocatedUnits[0].UnitCode

	cap.clear()
	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, allocatedToOtherZMU, nil)
	assert.ErrorIs(t, err, fulfillment.ErrUnitAllocatedToOtherOrder)
	scannedEvents = cap.findEvents("fulfillment.picking_unit_scanned")
	require.Len(t, scannedEvents, 1)
	assert.Equal(t, "allocated_to_other_order", scannedEvents[0]["result"])
	assert.Equal(t, "WARN", scannedEvents[0]["level"])

	// 6. CANNOT_PICK_SERIALIZED_WITH_BARCODE (attempt barcode scan on serialized item)
	var variantBarcode string
	err = f.db.QueryRow(ctx, `SELECT pv.barcode FROM order_items oi JOIN product_variants pv ON pv.id = oi.product_variant_id WHERE oi.id = $1`, itemID).Scan(&variantBarcode)
	require.NoError(t, err)

	cap.clear()
	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, variantBarcode, nil)
	assert.ErrorIs(t, err, fulfillment.ErrCannotPickSerializedWithBarcode)
	scannedEvents = cap.findEvents("fulfillment.picking_unit_scanned")
	require.Len(t, scannedEvents, 1)
	assert.Equal(t, "cannot_pick_serialized_with_barcode", scannedEvents[0]["result"])
	assert.Equal(t, "WARN", scannedEvents[0]["level"])

	// 7. NOT_ALLOCATED (unit exists and matches variant, but is not allocated to this fulfillment)
	orderID_D, fulfillmentID_D := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID_D := f.createOrderItem(t, ctx, orderID_D, fulfillmentID_D, 1, 0)
	var varID_D uuid.UUID
	err = f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemID_D).Scan(&varID_D)
	require.NoError(t, err)
	_, unallocatedUnitCode := f.createUnallocatedUnit(t, ctx, varID_D)

	cap.clear()
	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID_D, unallocatedUnitCode, nil)
	require.ErrorIs(t, err, fulfillment.ErrUnitNotAllocatedToFulfillment)
	scannedEvents = cap.findEvents("fulfillment.picking_unit_scanned")
	require.Len(t, scannedEvents, 1)
	assert.Equal(t, "not_allocated", scannedEvents[0]["result"])
	assert.Equal(t, "WARN", scannedEvents[0]["level"])
	assert.Equal(t, unallocatedUnitCode, scannedEvents[0]["zmu"])
}

func TestPickingObservability_PickingStartedSemantics(t *testing.T) {
	ctx := context.Background()
	f, cap := setupObservabilityFixture(t, ctx)
	defer f.db.Close()

	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 2, 0)
	f.createAllocation(t, ctx, itemID, false)
	f.createAllocation(t, ctx, itemID, false)

	po, err := f.svc.GetPickingOrder(ctx, fulfillmentID)
	require.NoError(t, err)
	allocs := po.Items[0].AllocatedUnits
	require.Len(t, allocs, 2)

	// Step 1: Invalid scan must NOT emit picking_started
	cap.clear()
	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, "INVALID_CODE", nil)
	assert.Error(t, err)
	assert.Empty(t, cap.findEvents("fulfillment.picking_started"), "invalid scan must not emit picking_started")

	// Step 2: First valid pick transitions paid -> assembling and emits picking_started
	cap.clear()
	res, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, allocs[0].UnitCode, nil)
	require.NoError(t, err)
	assert.True(t, res.ScanResult.NewlyPicked)
	startedEvents := cap.findEvents("fulfillment.picking_started")
	require.Len(t, startedEvents, 1, "first valid pick must emit picking_started exactly once")
	assert.Equal(t, "start_picking", startedEvents[0]["action"])
	assert.Equal(t, "success", startedEvents[0]["result"])

	// Step 3: Duplicate scan must NOT emit a second picking_started
	cap.clear()
	resDup, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, allocs[0].UnitCode, nil)
	require.NoError(t, err)
	assert.True(t, resDup.ScanResult.AlreadyPicked)
	assert.Empty(t, cap.findEvents("fulfillment.picking_started"), "duplicate scan must not emit picking_started")

	// Step 4: Second valid pick (fulfillment already assembling) must NOT emit picking_started again
	cap.clear()
	res2, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, allocs[1].UnitCode, nil)
	require.NoError(t, err)
	assert.True(t, res2.ScanResult.NewlyPicked)
	assert.Empty(t, cap.findEvents("fulfillment.picking_started"), "subsequent valid picks must not emit picking_started again")

	// Step 5: Second valid pick completed the picking order -> emits picking_completed
	completedEvents := cap.findEvents("fulfillment.picking_completed")
	require.Len(t, completedEvents, 1, "final pick must emit picking_completed")
}

func TestPickingObservability_ScannerPrivacy(t *testing.T) {
	ctx := context.Background()
	f, cap := setupObservabilityFixture(t, ctx)
	defer f.db.Close()

	_, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")

	malformedPayload := "MALFORMED' OR 1=1; DROP TABLE users; <script>"

	cap.clear()
	_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, malformedPayload, nil)
	assert.Error(t, err)

	events := cap.findEvents("fulfillment.picking_unit_scanned")
	require.Len(t, events, 1)

	// Event must record rejection with safe reason
	assert.Equal(t, "not_found", events[0]["result"])
	assert.Equal(t, "malformed_code", events[0]["reason"])

	// The dangerous payload must NEVER appear in zmu, barcode, or code attributes
	assert.Nil(t, events[0]["zmu"], "zmu attribute must not contain malformed input")
	assert.Nil(t, events[0]["barcode"], "barcode attribute must not contain malformed input")
	assert.Nil(t, events[0]["code"], "code attribute must not contain malformed input")

	// Verify buffer text does not contain raw dangerous substrings
	rawBuf := cap.buf.String()
	assert.NotContains(t, rawBuf, "DROP TABLE")
	assert.NotContains(t, rawBuf, "<script>")

	// Non-canonical arbitrary string must also be shielded
	cap.clear()
	arbitraryString := "PRIVATE_INTERNAL_VALUE_123"
	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, arbitraryString, nil)
	assert.Error(t, err)

	events = cap.findEvents("fulfillment.picking_unit_scanned")
	require.Len(t, events, 1)
	assert.Equal(t, "not_found", events[0]["result"])
	assert.Equal(t, "malformed_code", events[0]["reason"])
	assert.Nil(t, events[0]["zmu"], "zmu must not contain arbitrary string")
	assert.Nil(t, events[0]["barcode"], "barcode must not contain arbitrary string")
	assert.Nil(t, events[0]["code"], "code must not contain arbitrary string")
	assert.NotContains(t, cap.buf.String(), arbitraryString)
}

func TestPickingObservability_NotAllocated_RealFixture(t *testing.T) {
	ctx := context.Background()
	f, cap := setupObservabilityFixture(t, ctx)
	defer f.db.Close()

	// 1. Setup in-memory metric reader to verify metric increments
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := meterProvider.Meter("test-fulfillment")
	wm, err := observability.NewWarehouseMetrics(meter)
	require.NoError(t, err)
	observability.SetGlobalWarehouseMetrics(wm)

	// 2. Real DB fixture:
	// - Order and fulfillment created
	// - Order line item exists for variantID
	// - Unit exists for variantID in status 'warehouse'
	// - But unit is NOT allocated to this fulfillment
	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	var variantID uuid.UUID
	err = f.db.QueryRow(ctx, `SELECT product_variant_id FROM order_items WHERE id = $1`, itemID).Scan(&variantID)
	require.NoError(t, err)

	_, unitCode := f.createUnallocatedUnit(t, ctx, variantID)

	cap.clear()
	_, err = f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, unitCode, nil)
	require.ErrorIs(t, err, fulfillment.ErrUnitNotAllocatedToFulfillment)

	// 3. Assert event fields: event_name, result, level, attributes
	scannedEvents := cap.findEvents("fulfillment.picking_unit_scanned")
	require.Len(t, scannedEvents, 1)
	assert.Equal(t, "fulfillment.picking_unit_scanned", scannedEvents[0]["event_name"])
	assert.Equal(t, "not_allocated", scannedEvents[0]["result"])
	assert.Equal(t, "WARN", scannedEvents[0]["level"])
	assert.Equal(t, unitCode, scannedEvents[0]["zmu"])

	// 4. Assert Prometheus/OTel metric increment
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	foundMetric := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "warehouse_picking_scans_total" {
				sum, ok := m.Data.(metricdata.Sum[int64])
				require.True(t, ok)
				for _, dp := range sum.DataPoints {
					if resVal, ok := dp.Attributes.Value("result"); ok && resVal.AsString() == "not_allocated" {
						assert.Equal(t, int64(1), dp.Value)
						foundMetric = true
						break
					}
				}
			}
		}
	}
	assert.True(t, foundMetric, "warehouse_picking_scans_total{result=\"not_allocated\"} must be recorded with count 1")
}

func TestPickingObservability_MalformedScannerSemantics(t *testing.T) {
	ctx := context.Background()
	f, cap := setupObservabilityFixture(t, ctx)
	defer f.db.Close()

	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := meterProvider.Meter("test-fulfillment-malformed")
	wm, err := observability.NewWarehouseMetrics(meter)
	require.NoError(t, err)
	observability.SetGlobalWarehouseMetrics(wm)

	// Create order and fulfillment with a serialized item
	orderID, fulfillmentID := f.createOrderAndFulfillment(t, ctx, "paid", "paid")
	itemID := f.createOrderItem(t, ctx, orderID, fulfillmentID, 1, 0)
	f.createAllocation(t, ctx, itemID, false)

	var variantBarcode string
	err = f.db.QueryRow(ctx, `SELECT pv.barcode FROM order_items oi JOIN product_variants pv ON pv.id = oi.product_variant_id WHERE oi.id = $1`, itemID).Scan(&variantBarcode)
	require.NoError(t, err)
	require.NotEmpty(t, variantBarcode)

	// --- Subtest A: Malformed input (PRIVATE_INTERNAL_VALUE_123) in Guided Picking (with targetOrderItemID) ---
	t.Run("A_MalformedScannerInput_GuidedPicking_TreatedAsNotFound", func(t *testing.T) {
		cap.clear()
		malformed := "PRIVATE_INTERNAL_VALUE_123"
		_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, malformed, &itemID)
		require.Error(t, err)
		assert.ErrorIs(t, err, fulfillment.ErrCodeNotFound)
		assert.ErrorIs(t, err, fulfillment.ErrMalformedScannerCode)

		// Assert event semantics: result=not_found, reason=malformed_code, level=WARN
		scannedEvents := cap.findEvents("fulfillment.picking_unit_scanned")
		require.Len(t, scannedEvents, 1)
		assert.Equal(t, "not_found", scannedEvents[0]["result"])
		assert.Equal(t, "malformed_code", scannedEvents[0]["reason"])
		assert.Equal(t, "WARN", scannedEvents[0]["level"])
		assert.Nil(t, scannedEvents[0]["zmu"])
		assert.Nil(t, scannedEvents[0]["barcode"])
		assert.Nil(t, scannedEvents[0]["code"])
		assert.NotContains(t, cap.buf.String(), malformed)

		// Assert Prometheus/OTel metric: warehouse_picking_scans_total{result="not_found"}
		var rm metricdata.ResourceMetrics
		err = reader.Collect(ctx, &rm)
		require.NoError(t, err)
		foundNotFound := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "warehouse_picking_scans_total" {
					sum, ok := m.Data.(metricdata.Sum[int64])
					require.True(t, ok)
					for _, dp := range sum.DataPoints {
						if resVal, ok := dp.Attributes.Value("result"); ok && resVal.AsString() == "not_found" {
							assert.GreaterOrEqual(t, dp.Value, int64(1))
							foundNotFound = true
						}
					}
				}
			}
		}
		assert.True(t, foundNotFound, "warehouse_picking_scans_total{result=\"not_found\"} must increment")
	})

	// --- Subtest B: Canonical barcode on serialized item -> cannot_pick_serialized_with_barcode ---
	t.Run("B_CanonicalBarcode_OnSerializedItem_CannotPickSerialized", func(t *testing.T) {
		cap.clear()
		_, err := f.svc.ScanPickingCode(ctx, f.adminID, fulfillmentID, variantBarcode, &itemID)
		require.ErrorIs(t, err, fulfillment.ErrCannotPickSerializedWithBarcode)

		scannedEvents := cap.findEvents("fulfillment.picking_unit_scanned")
		require.Len(t, scannedEvents, 1)
		assert.Equal(t, "cannot_pick_serialized_with_barcode", scannedEvents[0]["result"])
		assert.Equal(t, "WARN", scannedEvents[0]["level"])
		assert.Equal(t, variantBarcode, scannedEvents[0]["barcode"])

		// Assert Prometheus metric: warehouse_picking_scans_total{result="cannot_pick_serialized_with_barcode"}
		var rm metricdata.ResourceMetrics
		err = reader.Collect(ctx, &rm)
		require.NoError(t, err)
		foundCannotPick := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "warehouse_picking_scans_total" {
					sum, ok := m.Data.(metricdata.Sum[int64])
					require.True(t, ok)
					for _, dp := range sum.DataPoints {
						if resVal, ok := dp.Attributes.Value("result"); ok && resVal.AsString() == "cannot_pick_serialized_with_barcode" {
							assert.GreaterOrEqual(t, dp.Value, int64(1))
							foundCannotPick = true
						}
					}
				}
			}
		}
		assert.True(t, foundCannotPick, "warehouse_picking_scans_total{result=\"cannot_pick_serialized_with_barcode\"} must increment")
	})

	// --- Subtest C: HTTP handler does not expose raw scanner payload in error response ---
	t.Run("C_HTTPHandler_MalformedResponse_DoesNotExposeRawPayload", func(t *testing.T) {
		handler := fulfillment.NewHandler(f.svc)
		malformedPayload := "PRIVATE_INTERNAL_VALUE_123"
		body := `{"code":"` + malformedPayload + `","orderItemId":"` + itemID.String() + `"}`
		req := httptest.NewRequest("POST", "/api/admin/fulfillments/"+fulfillmentID.String()+"/picking/scan", strings.NewReader(body))
		rec := httptest.NewRecorder()

		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("id", fulfillmentID.String())
		req = req.WithContext(context.WithValue(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx), "userID", f.adminID))

		handler.ScanPickingCode(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		respBody := rec.Body.String()
		assert.Contains(t, respBody, "malformed_scanner_code")
		assert.Contains(t, respBody, "Некорректный код сканирования")
		assert.NotContains(t, respBody, malformedPayload, "raw malformed scanner payload must not leak into HTTP response body")
	})
}
