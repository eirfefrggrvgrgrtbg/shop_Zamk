package staff

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

// AuditEvent holds the data for a single audit log entry.
type AuditEvent struct {
	ActorUserID uuid.UUID
	ActorEmail  string
	ActorRole   string
	Permission  string
	Action      string
	EntityType  string
	EntityID    *uuid.UUID
	RequestID   string
	IP          string
	UserAgent   string
	Metadata    map[string]any
}

// AuditLog is the persisted audit log record.
type AuditLog struct {
	ID          uuid.UUID      `json:"id"`
	ActorUserID *uuid.UUID     `json:"actorUserId,omitempty"`
	ActorEmail  *string        `json:"actorEmail,omitempty"`
	ActorRole   *string        `json:"actorRole,omitempty"`
	Permission  *string        `json:"permission,omitempty"`
	Action      string         `json:"action"`
	EntityType  *string        `json:"entityType,omitempty"`
	EntityID    *uuid.UUID     `json:"entityId,omitempty"`
	RequestID   *string        `json:"requestId,omitempty"`
	IP          *string        `json:"ip,omitempty"`
	UserAgent   *string        `json:"userAgent,omitempty"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type AuditRepository struct {
	db postgres.DBTX
}

func NewAuditRepository(db postgres.DBTX) *AuditRepository {
	return &AuditRepository{db: db}
}

// RecordAudit inserts an audit log entry. Errors are non-fatal — callers should log but not fail.
func (r *AuditRepository) RecordAudit(ctx context.Context, event AuditEvent) error {
	meta := SanitizeMetadata(event.Metadata)
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		metaBytes = []byte("{}")
	}

	var actorUserID *uuid.UUID
	if event.ActorUserID != uuid.Nil {
		id := event.ActorUserID
		actorUserID = &id
	}
	var actorEmail *string
	if event.ActorEmail != "" {
		actorEmail = &event.ActorEmail
	}
	var actorRole *string
	if event.ActorRole != "" {
		actorRole = &event.ActorRole
	}
	var permission *string
	if event.Permission != "" {
		permission = &event.Permission
	}
	var entityType *string
	if event.EntityType != "" {
		entityType = &event.EntityType
	}
	var requestID *string
	if event.RequestID != "" {
		requestID = &event.RequestID
	}
	var ip *string
	if event.IP != "" {
		ip = &event.IP
	}
	var userAgent *string
	if event.UserAgent != "" {
		userAgent = &event.UserAgent
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO audit_logs
			(actor_user_id, actor_email, actor_role, permission, action,
			 entity_type, entity_id, request_id, ip, user_agent, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
	`,
		actorUserID, actorEmail, actorRole, permission, event.Action,
		entityType, event.EntityID, requestID, ip, userAgent, metaBytes,
	)
	if err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}

// ListAuditLogs returns paginated audit logs ordered by most recent first.
func (r *AuditRepository) ListAuditLogs(ctx context.Context, limit, offset int) ([]AuditLog, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, actor_user_id, actor_email, actor_role, permission, action,
		       entity_type, entity_id, request_id, ip, user_agent, metadata, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		var metaBytes []byte
		if err := rows.Scan(
			&l.ID, &l.ActorUserID, &l.ActorEmail, &l.ActorRole, &l.Permission, &l.Action,
			&l.EntityType, &l.EntityID, &l.RequestID, &l.IP, &l.UserAgent, &metaBytes, &l.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &l.Metadata)
		}
		if l.Metadata == nil {
			l.Metadata = map[string]any{}
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

// sensitiveKeys lists metadata keys that must be redacted.
var sensitiveKeys = []string{
	"password", "token", "secret", "authorization",
	"temporaryPassword", "access_token", "refresh_token",
}

// SanitizeMetadata returns a copy of m with sensitive keys removed.
func SanitizeMetadata(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		lower := strings.ToLower(k)
		sensitive := false
		for _, sk := range sensitiveKeys {
			if lower == strings.ToLower(sk) {
				sensitive = true
				break
			}
		}
		if !sensitive {
			out[k] = v
		}
	}
	return out
}
