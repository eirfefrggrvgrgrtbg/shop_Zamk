package marketing

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

func GenerateXLSX(cols []ExportColumnDef, rows []map[string]interface{}) ([]byte, error) {
	f := excelize.NewFile()
	defer func() {
		_ = f.Close()
	}()

	const sheetName = "Отчёт"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to create worksheet: %w", err)
	}
	f.SetActiveSheet(index)
	_ = f.DeleteSheet("Sheet1")

	// Set headers
	for i, col := range cols {
		cellAxis, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		_ = f.SetCellValue(sheetName, cellAxis, col.Header)
	}

	// Set data rows
	for rIdx, row := range rows {
		rowNum := rIdx + 2
		for cIdx, col := range cols {
			cellAxis, err := excelize.CoordinatesToCellName(cIdx+1, rowNum)
			if err != nil {
				return nil, err
			}

			populateXLSXCell(f, sheetName, cellAxis, col.Key, row)
		}
	}

	// Freeze header row
	_ = f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write XLSX to buffer: %w", err)
	}

	return buf.Bytes(), nil
}

func populateXLSXCell(f *excelize.File, sheet, cellAxis, key string, row map[string]interface{}) {
	if key == "source" {
		valStr := formatSourceVal(row)
		valStr = neutralizeFormulaString(valStr)
		_ = f.SetCellValue(sheet, cellAxis, valStr)
		return
	}

	val := row[key]
	if val == nil {
		// Unknown / unavailable -> leave blank cell
		return
	}

	switch key {
	case "revenue":
		_, floatVal, ok := formatMoneyCents(val)
		if !ok {
			return
		}
		_ = f.SetCellValue(sheet, cellAxis, floatVal)

	case "conversion":
		_, floatVal, ok := formatConversionVal(val)
		if !ok {
			return
		}
		_ = f.SetCellValue(sheet, cellAxis, floatVal)

	case "sessions", "orders", "sold_units", "product_views", "favorites", "add_to_cart", "returns", "returned_units":
		_, floatVal, ok := formatCountVal(val)
		if !ok {
			return
		}
		_ = f.SetCellValue(sheet, cellAxis, int64(floatVal))

	default:
		// Text / strings / dates
		strVal := fmt.Sprintf("%v", val)
		strVal = neutralizeFormulaString(strVal)
		_ = f.SetCellValue(sheet, cellAxis, strVal)
	}
}
