package auth

import "errors"

var (
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrUserBlocked         = errors.New("user is blocked")
	ErrUserDeleted         = errors.New("user is deleted")
	ErrDuplicateEmail      = errors.New("email already in use")
	ErrInvalidToken        = errors.New("invalid token")
	ErrSessionExpired      = errors.New("session expired")
	ErrSessionRevoked      = errors.New("session revoked")
	ErrSessionNotFound     = errors.New("session not found")
)
