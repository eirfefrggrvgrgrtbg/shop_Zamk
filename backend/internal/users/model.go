package users

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleCustomer = "customer"
	RoleSeller   = "seller"
	RoleAdmin    = "admin"

	StatusActive  = "active"
	StatusBlocked = "blocked"
	StatusDeleted = "deleted"
)

type User struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	FirstName          *string   `json:"firstName,omitempty"`
	LastName           *string   `json:"lastName,omitempty"`
	MiddleName         *string   `json:"middleName,omitempty"`
	Email              string    `json:"email"`
	Phone              string    `json:"phone"`
	PasswordHash       string    `json:"-"`
	Role               string    `json:"role"`
	Status             string    `json:"status"`
	MustChangePassword bool      `json:"mustChangePassword"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}
