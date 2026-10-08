package marketing

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QueryRepository struct {
	db       *pgxpool.Pool
	compiler *QueryCompiler
}

func NewQueryRepository(db *pgxpool.Pool) *QueryRepository {
	return &QueryRepository{
		db:       db,
		compiler: NewQueryCompiler(),
	}
}

func (r *QueryRepository) ExecuteQuery(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	sqlStr, args, columns, err := r.compiler.Build(req)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()

	// 1. Calculate tracking coverage
	var viewsTrackingStarted, favTrackingStarted, cartTrackingStarted, sessionsTrackingStarted *time.Time
	_ = r.db.QueryRow(ctx, `
		SELECT
		  (SELECT MIN(occurred_at) FROM behavioral_events WHERE event_type = 'product_view'),
		  (SELECT MIN(occurred_at) FROM behavioral_events WHERE event_type = 'favorite_added'),
		  (SELECT MIN(occurred_at) FROM behavioral_events WHERE event_type = 'add_to_cart'),
		  (SELECT MIN(started_at) FROM analytics_sessions)
	`).Scan(&viewsTrackingStarted, &favTrackingStarted, &cartTrackingStarted, &sessionsTrackingStarted)

	viewsCov := evaluateCoverage(req.Period.From, req.Period.To, viewsTrackingStarted)
	favCov := evaluateCoverage(req.Period.From, req.Period.To, favTrackingStarted)
	cartCov := evaluateCoverage(req.Period.From, req.Period.To, cartTrackingStarted)
	sessionsCov := evaluateCoverage(req.Period.From, req.Period.To, sessionsTrackingStarted)

	hasProductOrDesignerOrCategory := false
	for _, dKey := range req.Dimensions {
		if dKey == "product" || dKey == "designer" || dKey == "category" {
			hasProductOrDesignerOrCategory = true
			break
		}
	}

	var conversionCov MetricCoverage
	if hasProductOrDesignerOrCategory {
		conversionCov = viewsCov
	} else {
		conversionCov = sessionsCov
	}

	coverageMap := map[string]MetricCoverage{
		"product_views": viewsCov,
		"favorites":     favCov,
		"add_to_cart":   cartCov,
		"sessions":      sessionsCov,
		"conversion":    conversionCov,
	}

	// 2. Scan and map result rows
	var resultRows []map[string]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, field := range fields {
			colName := string(field.Name)
			val := values[i]

			if strings.HasPrefix(colName, "_raw_source") {
				raw, ok := val.(string)
				if !ok || raw == "" {
					rowMap["sourceKey"] = nil
					rowMap["sourceKind"] = "unattributed"
					rowMap["source"] = ""
				} else if strings.EqualFold(raw, "direct") {
					rowMap["sourceKey"] = "direct"
					rowMap["sourceKind"] = "direct"
					rowMap["source"] = "direct"
				} else {
					rowMap["sourceKey"] = raw
					rowMap["sourceKind"] = "named"
					rowMap["source"] = raw
				}
			} else {
				// Map DB aliases to public column keys
				targetKey := colName
				switch colName {
				case "product_name":
					targetKey = "productName"
				case "designer_name":
					targetKey = "designerName"
				case "category_name":
					targetKey = "categoryName"
				case "campaign_name":
					targetKey = "campaignName"
				}

				if num, ok := val.(pgtype.Numeric); ok {
					if num.Valid {
						f, _ := num.Float64Value()
						rowMap[targetKey] = f.Float64
					} else {
						rowMap[targetKey] = nil
					}
				} else {
					rowMap[targetKey] = val
				}
			}
		}

		// Blocker 10: If conversion denominator coverage is partial or unavailable, conversion must be null
		if conversionCov.Status != CoverageAvailable {
			if _, exists := rowMap["conversion"]; exists {
				rowMap["conversion"] = nil
			}
		}

		resultRows = append(resultRows, rowMap)
	}

	if resultRows == nil {
		resultRows = []map[string]interface{}{}
	}

	return &QueryResponse{
		Version:  req.Version,
		Query:    req,
		Columns:  columns,
		Rows:     resultRows,
		Coverage: coverageMap,
		Warnings: []string{},
	}, nil
}

func evaluateCoverage(from, to time.Time, trackingStarted *time.Time) MetricCoverage {
	if trackingStarted == nil {
		return MetricCoverage{Status: CoverageUnavailable}
	}
	if from.Before(*trackingStarted) {
		if to.Before(*trackingStarted) {
			return MetricCoverage{Status: CoverageUnavailable, TrackedFrom: trackingStarted}
		}
		return MetricCoverage{Status: CoveragePartial, TrackedFrom: trackingStarted}
	}
	return MetricCoverage{Status: CoverageAvailable, TrackedFrom: trackingStarted}
}
