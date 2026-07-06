package audit

import (
	"context"
	"encoding/json"
	"log/slog"
)

type Service struct {
	repo   *Repository
	logger *slog.Logger
}

func NewService(repo *Repository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

func (s *Service) ListLogs(ctx context.Context, filters AuditLogFilters) (*AuditLogListResponse, error) {
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	items, total, err := s.repo.ListLogs(ctx, filters)
	if err != nil {
		s.logger.Error("failed to list audit logs", "error", err)
		return nil, err
	}

	// Sanitize metadata
	for i := range items {
		items[i].Metadata = sanitizeMetadata(items[i].Metadata)
	}

	return &AuditLogListResponse{
		Items:      items,
		TotalCount: total,
	}, nil
}

// sanitizeMetadata removes potentially sensitive keys from the metadata JSON.
func sanitizeMetadata(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return raw // return as is if not a valid JSON object
	}

	sensitiveKeys := []string{"password", "passwordHash", "token", "accessToken", "refreshToken", "card", "bank", "raw_payload"}
	
	dirty := false
	for _, key := range sensitiveKeys {
		if _, ok := data[key]; ok {
			delete(data, key)
			dirty = true
		}
	}

	if dirty {
		cleanRaw, err := json.Marshal(data)
		if err == nil {
			return cleanRaw
		}
	}

	return raw
}
