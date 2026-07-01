package auth

import "github.com/google/uuid"

type RegisterRequest struct {
	FirstName       string `json:"firstName" validate:"required,min=2,max=80"`
	LastName        string `json:"lastName" validate:"required,min=2,max=80"`
	MiddleName      string `json:"middleName,omitempty" validate:"omitempty,max=80"`
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=10,max=128"`
	PasswordConfirm string `json:"passwordConfirm" validate:"required"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type UserDTO struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	FirstName          *string   `json:"firstName,omitempty"`
	LastName           *string   `json:"lastName,omitempty"`
	MiddleName         *string   `json:"middleName,omitempty"`
	Email              string    `json:"email"`
	Role               string    `json:"role"`
	Status             string    `json:"status"`
	MustChangePassword bool      `json:"mustChangePassword"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required,min=8,max=128"`
}

type AuthResponse struct {
	AccessToken string  `json:"accessToken"`
	User        UserDTO `json:"user"`
}

type MeResponse struct {
	User UserDTO `json:"user"`
}

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
