package marketing

import (
	"bytes"
	"encoding/csv"
	"fmt"
)

func GenerateCSV(cols []ExportColumnDef, rows []map[string]interface{}) ([]byte, error) {
	var buf bytes.Buffer

	// Write UTF-8 BOM
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(&buf)
	writer.Comma = ';'
	writer.UseCRLF = true

	// Header row
	headerRecord := make([]string, len(cols))
	for i, col := range cols {
		headerRecord[i] = col.Header
	}
	if err := writer.Write(headerRecord); err != nil {
		return nil, err
	}

	// Data rows
	for _, row := range rows {
		record := make([]string, len(cols))
		for i, col := range cols {
			record[i] = formatCSVCell(col.Key, row)
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func formatCSVCell(key string, row map[string]interface{}) string {
	if key == "source" {
		return neutralizeFormulaString(formatSourceVal(row))
	}

	val := row[key]
	if val == nil {
		return ""
	}

	switch key {
	case "revenue":
		strVal, _, ok := formatMoneyCents(val)
		if !ok {
			return ""
		}
		return strVal

	case "conversion":
		strVal, _, ok := formatConversionVal(val)
		if !ok {
			return ""
		}
		return strVal

	case "sessions", "orders", "sold_units", "product_views", "favorites", "add_to_cart", "returns", "returned_units":
		strVal, _, ok := formatCountVal(val)
		if !ok {
			return ""
		}
		return strVal

	default:
		// Text / string dimensions (day, campaignName, productName, designerName, categoryName, etc.)
		strVal := fmt.Sprintf("%v", val)
		return neutralizeFormulaString(strVal)
	}
}
