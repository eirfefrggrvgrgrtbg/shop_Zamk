package audit

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID          uuid.UUID       `json:"id"`
	ActorUserID *uuid.UUID      `json:"actorUserId,omitempty"`
	ActorEmail  *string         `json:"actorEmail,omitempty"`
	ActorRole   *string         `json:"actorRole,omitempty"`
	Permission  *string         `json:"permission,omitempty"`
	Action      string          `json:"action"`
	EntityType  *string         `json:"entityType,omitempty"`
	EntityID    *uuid.UUID      `json:"entityId,omitempty"`
	RequestID   *string         `json:"requestId,omitempty"`
	IP          *string         `json:"ip,omitempty"`
	UserAgent   *string         `json:"userAgent,omitempty"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"createdAt"`
}

type AuditLogFilters struct {
	Query      string
	ActorID    *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	DateFrom   *time.Time
	DateTo     *time.Time
	Limit      int
	Offset     int
}

type AuditLogListResponse struct {
	Items      []AuditLog `json:"items"`
	TotalCount int        `json:"totalCount"`
}
