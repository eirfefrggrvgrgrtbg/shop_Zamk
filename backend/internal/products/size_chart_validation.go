package products

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"math"
)

const (
	maxSizeChartRows = 100
	maxMeasurementCm = 300.0
	minMeasurementCm = 0.1
)

// validateSizeChartRows validates garment measurements submitted by the seller.
// Rules: unique sizes, sizes must be active values of a size system allowed for the category,
// measurement codes must be defined for the category, values are finite numbers in (0, 300] cm.
// An empty chart is always valid (the table is optional unless the category requires it, checked elsewhere).
func (s *Service) validateSizeChartRows(ctx context.Context, categoryID uuid.UUID, rows []ProductSizeChartRowRequest) error {
	if len(rows) == 0 {
		return nil
	}
	if len(rows) > maxSizeChartRows {
		return fmt.Errorf("size chart has too many rows (max %d)", maxSizeChartRows)
	}

	allowedCodes := map[string]bool{}
	codeRows, err := s.dbPool.Pool.Query(ctx, `SELECT code FROM category_size_chart_fields WHERE category_id = $1`, categoryID)
	if err != nil {
		return err
	}
	for codeRows.Next() {
		var code string
		if err := codeRows.Scan(&code); err != nil {
			codeRows.Close()
			return err
		}
		allowedCodes[code] = true
	}
	codeRows.Close()
	if err := codeRows.Err(); err != nil {
		return err
	}

	seen := map[string]bool{}
	for _, row := range rows {
		key := row.SizeValueID.String()
		if seen[key] {
			return fmt.Errorf("size chart contains a duplicate size row")
		}
		seen[key] = true

		// When the category configures size systems the size must belong to one of them;
		// categories without a configured system only require an existing active size value.
		var ok bool
		err := s.dbPool.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM size_values sv
				WHERE sv.id = $1 AND sv.is_active = true
				AND (
					NOT EXISTS (SELECT 1 FROM category_size_systems WHERE category_id = $2)
					OR sv.size_system_id IN (SELECT size_system_id FROM category_size_systems WHERE category_id = $2)
				)
			)`, row.SizeValueID, categoryID).Scan(&ok)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("size chart row uses a size that is not valid for this category")
		}

		for code, raw := range row.Measurements {
			if !allowedCodes[code] {
				return fmt.Errorf("size chart measurement %q is not defined for this category", code)
			}
			var v float64
			switch n := raw.(type) {
			case float64:
				v = n
			case int:
				v = float64(n)
			case int64:
				v = float64(n)
			default:
				return fmt.Errorf("size chart measurement %q must be a number", code)
			}
			if math.IsNaN(v) || math.IsInf(v, 0) || v < minMeasurementCm || v > maxMeasurementCm {
				return fmt.Errorf("size chart measurement %q must be between %.1f and %.0f cm", code, minMeasurementCm, maxMeasurementCm)
			}
		}
	}
	return nil
}
