package marketing

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *AnalyticsService) ExportQuery(ctx context.Context, req ExportRequest) ([]byte, string, string, error) {
	if err := req.Validate(); err != nil {
		return nil, "", "", err
	}

	// Reuse canonical M5 query execution
	resp, err := s.ExecuteQuery(ctx, req.Query)
	if err != nil {
		return nil, "", "", err
	}

	cols := BuildExportColumns(resp.Columns)
	dateStr := time.Now().UTC().Format("2006-01-02")
	fmtLower := ExportFormat(strings.ToLower(string(req.Format)))

	switch fmtLower {
	case ExportFormatCSV:
		data, err := GenerateCSV(cols, resp.Rows)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to generate CSV: %w", err)
		}
		filename := fmt.Sprintf("zamk-marketing-report-%s.csv", dateStr)
		return data, "text/csv; charset=utf-8", filename, nil

	case ExportFormatXLSX:
		data, err := GenerateXLSX(cols, resp.Rows)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to generate XLSX: %w", err)
		}
		filename := fmt.Sprintf("zamk-marketing-report-%s.xlsx", dateStr)
		return data, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", filename, nil

	default:
		return nil, "", "", NewValidationError("invalid_export_format", "unsupported export format")
	}
}
