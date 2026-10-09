package marketing

import (
	"fmt"
	"strings"
)

type ExportFormat string

const (
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatXLSX ExportFormat = "xlsx"
)

type ExportRequest struct {
	Format ExportFormat `json:"format"`
	Query  QueryRequest `json:"query"`
}

func (r ExportRequest) Validate() error {
	fmtLower := ExportFormat(strings.ToLower(string(r.Format)))
	switch fmtLower {
	case ExportFormatCSV, ExportFormatXLSX:
	default:
		return NewValidationError("invalid_export_format", fmt.Sprintf("unsupported export format: %s", r.Format))
	}

	if err := ValidateQueryRequest(r.Query); err != nil {
		return err
	}
	return nil
}
