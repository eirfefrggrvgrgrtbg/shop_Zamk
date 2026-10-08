package marketing

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SavedQuery struct {
	ID           uuid.UUID       `json:"id"`
	Name         string          `json:"name"`
	Description  *string         `json:"description"`
	QueryVersion int             `json:"queryVersion"`
	QuerySpec    json.RawMessage `json:"querySpec"`
	CreatedBy    uuid.UUID       `json:"-"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type SavedQueryCreateRequest struct {
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	QuerySpec   json.RawMessage `json:"querySpec"`
}

type SavedQueryUpdateRequest struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	QuerySpec   json.RawMessage `json:"querySpec,omitempty"`
}
