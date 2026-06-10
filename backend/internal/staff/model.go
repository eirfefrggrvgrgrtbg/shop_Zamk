package staff

import (
	"time"

	"github.com/google/uuid"
)

type StaffRole struct {
	ID          uuid.UUID `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsSystem    bool      `json:"isSystem"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type StaffMember struct {
	UserID      uuid.UUID  `json:"userId"`
	StaffRoleID uuid.UUID  `json:"staffRoleId"`
	Status      string     `json:"status"`
	CreatedBy   *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type StaffMemberStatus string

const (
	StatusActive   StaffMemberStatus = "active"
	StatusBlocked  StaffMemberStatus = "blocked"
	StatusArchived StaffMemberStatus = "archived"
)

// StaffAccess is the full access view for a user.
type StaffAccess struct {
	Role        *StaffRole   `json:"role"`
	Member      *StaffMember `json:"member"`
	Permissions []string     `json:"permissions"`
}
