package supplies_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

func TestProcessFoundUnit_ScannerPrivacy_MalformedInputNotLeaked(t *testing.T) {
	tc := setupTestContext(t)

	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	logger := slog.New(jsonHandler)
	tc.Service.SetLogger(logger)

	malformedInput := "MALFORMED' OR '1'='1; DROP TABLE inventory_units; <script>alert(1)</script>"

	resp, err := tc.Service.ProcessFoundUnit(tc.Ctx, tc.AdminID, supplies.ProcessFoundUnitRequest{
		UnitCode:  malformedInput,
		Condition: "ok",
	})
	require.Error(t, err)
	assert.Nil(t, resp)

	rawLog := buf.String()
	require.NotEmpty(t, rawLog)

	var entry map[string]interface{}
	unmarshalErr := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, unmarshalErr)

	// Event must be emitted as rejected with safe reason
	assert.Equal(t, "warehouse.zmu_received", entry["event_name"])
	assert.Equal(t, "rejected", entry["result"])
	assert.Equal(t, "malformed_code", entry["reason"])

	// The raw malformed input MUST NOT be present in zmu, barcode, or code attributes
	assert.Nil(t, entry["zmu"], "zmu attribute must not contain malformed input")
	assert.Nil(t, entry["barcode"], "barcode attribute must not contain malformed input")
	assert.Nil(t, entry["code"], "code attribute must not contain malformed input")

	// Raw log must never contain the dangerous payload
	assert.NotContains(t, rawLog, "DROP TABLE")
	assert.NotContains(t, rawLog, "<script>")
}

func TestProcessFoundUnit_ScannerPrivacy_NonCanonicalArbitraryStringShield(t *testing.T) {
	tc := setupTestContext(t)

	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	logger := slog.New(jsonHandler)
	tc.Service.SetLogger(logger)

	arbitraryString := "PRIVATE_INTERNAL_VALUE_123"

	resp, err := tc.Service.ProcessFoundUnit(tc.Ctx, tc.AdminID, supplies.ProcessFoundUnitRequest{
		UnitCode:  arbitraryString,
		Condition: "ok",
	})
	require.Error(t, err)
	assert.Nil(t, resp)

	rawLog := buf.String()
	require.NotEmpty(t, rawLog)

	var entry map[string]interface{}
	unmarshalErr := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, unmarshalErr)

	// Event must be emitted as rejected with safe reason "malformed_code"
	assert.Equal(t, "warehouse.zmu_received", entry["event_name"])
	assert.Equal(t, "rejected", entry["result"])
	assert.Equal(t, "malformed_code", entry["reason"])

	// The raw string MUST NOT be present in zmu, barcode, or code attributes
	assert.Nil(t, entry["zmu"], "zmu attribute must not contain arbitrary string")
	assert.Nil(t, entry["barcode"], "barcode attribute must not contain arbitrary string")
	assert.Nil(t, entry["code"], "code attribute must not contain arbitrary string")

	// Raw log must never contain PRIVATE_INTERNAL_VALUE_123
	assert.NotContains(t, rawLog, arbitraryString)
}
