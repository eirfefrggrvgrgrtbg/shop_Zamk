package auth

import "errors"

var (
	ErrInvalidCredentials  = errors.New("Неверный email или пароль")
	ErrUserBlocked         = errors.New("user is blocked")
	ErrUserDeleted         = errors.New("user is deleted")
	ErrDuplicateEmail      = errors.New("Аккаунт с таким email уже существует")
	ErrInvalidToken        = errors.New("invalid token")
	ErrSessionExpired      = errors.New("session expired")
	ErrSessionRevoked      = errors.New("session revoked")
	ErrSessionNotFound     = errors.New("session not found")
)
