package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

type Service struct {
	authRepo     *Repository
	userRepo     *users.Repository
	tokenService *TokenService
	refreshTTL   time.Duration
}

func NewService(authRepo *Repository, userRepo *users.Repository, tokenService *TokenService, refreshTTLDays int) *Service {
	return &Service{
		authRepo:     authRepo,
		userRepo:     userRepo,
		tokenService: tokenService,
		refreshTTL:   time.Duration(refreshTTLDays) * 24 * time.Hour,
	}
}

func (s *Service) RegisterCustomer(ctx context.Context, input RegisterRequest, userAgent, ip string) (AuthResponse, string, error) {
	_, err := s.userRepo.GetUserByEmail(ctx, strings.ToLower(input.Email))
	if err == nil {
		return AuthResponse{}, "", ErrDuplicateEmail
	} else if !errors.Is(err, users.ErrNotFound) {
		return AuthResponse{}, "", err
	}

	hash, err := HashPassword(input.Password)
	if err != nil {
		return AuthResponse{}, "", err
	}

	user := &users.User{
		ID:           uuid.New(),
		Name:         input.Name,
		Email:        strings.ToLower(input.Email),
		PasswordHash: hash,
		Role:         users.RoleCustomer,
		Status:       users.StatusActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") || strings.Contains(err.Error(), "SQLSTATE 23505") {
			return AuthResponse{}, "", ErrDuplicateEmail
		}
		return AuthResponse{}, "", err
	}

	return s.createSessionForUser(ctx, user, userAgent, ip)
}

func (s *Service) Login(ctx context.Context, input LoginRequest, userAgent, ip string) (AuthResponse, string, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, strings.ToLower(input.Email))
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return AuthResponse{}, "", ErrInvalidCredentials
		}
		return AuthResponse{}, "", err
	}

	if user.Status == users.StatusBlocked {
		return AuthResponse{}, "", ErrUserBlocked
	}
	if user.Status == users.StatusDeleted {
		return AuthResponse{}, "", ErrUserDeleted
	}

	if !CheckPassword(input.Password, user.PasswordHash) {
		return AuthResponse{}, "", ErrInvalidCredentials
	}

	return s.createSessionForUser(ctx, user, userAgent, ip)
}

func (s *Service) Refresh(ctx context.Context, rawRefreshToken, userAgent, ip string) (AuthResponse, string, error) {
	hash := hashToken(rawRefreshToken)
	session, err := s.authRepo.GetSessionByRefreshTokenHash(ctx, hash)
	if err != nil {
		return AuthResponse{}, "", ErrInvalidToken
	}

	if session.RevokedAt != nil {
		return AuthResponse{}, "", ErrSessionRevoked
	}
	if time.Now().After(session.ExpiresAt) {
		return AuthResponse{}, "", ErrSessionExpired
	}

	user, err := s.userRepo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return AuthResponse{}, "", err
	}

	if user.Status != users.StatusActive {
		return AuthResponse{}, "", ErrUserBlocked
	}

	// Rotate token
	_ = s.authRepo.RevokeSession(ctx, session.ID)

	return s.createSessionForUser(ctx, user, userAgent, ip)
}

func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	hash := hashToken(rawRefreshToken)
	session, err := s.authRepo.GetSessionByRefreshTokenHash(ctx, hash)
	if err != nil {
		return nil // treat as successful logout
	}

	return s.authRepo.RevokeSession(ctx, session.ID)
}

func (s *Service) Me(ctx context.Context, userID uuid.UUID) (MeResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return MeResponse{}, err
	}

	return MeResponse{
		User: UserDTO{
			ID:                 user.ID,
			Name:               user.Name,
			Email:              user.Email,
			Role:               user.Role,
			Status:             user.Status,
			MustChangePassword: user.MustChangePassword,
		},
	}, nil
}

func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if !CheckPassword(currentPassword, user.PasswordHash) {
		return errors.New("invalid current password")
	}

	if currentPassword == newPassword {
		return errors.New("new password must be different from current password")
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.userRepo.UpdatePasswordAndMustChange(ctx, userID, hash, false); err != nil {
		return err
	}

	// Revoke all sessions so user has to login again
	return s.authRepo.RevokeAllUserSessions(ctx, userID)
}

func (s *Service) createSessionForUser(ctx context.Context, user *users.User, userAgent, ip string) (AuthResponse, string, error) {
	accessToken, err := s.tokenService.GenerateAccessToken(user.ID, user.Email, user.Role)
	if err != nil {
		return AuthResponse{}, "", err
	}

	rawRefresh, err := GenerateRefreshToken()
	if err != nil {
		return AuthResponse{}, "", err
	}

	session := &Session{
		ID:               uuid.New(),
		UserID:           user.ID,
		RefreshTokenHash: hashToken(rawRefresh),
		UserAgent:        userAgent,
		IPAddress:        ip,
		ExpiresAt:        time.Now().Add(s.refreshTTL),
		CreatedAt:        time.Now(),
	}

	if err := s.authRepo.CreateSession(ctx, session); err != nil {
		return AuthResponse{}, "", err
	}

	resp := AuthResponse{
		AccessToken: accessToken,
		User: UserDTO{
			ID:                 user.ID,
			Name:               user.Name,
			Email:              user.Email,
			Role:               user.Role,
			Status:             user.Status,
			MustChangePassword: user.MustChangePassword,
		},
	}

	return resp, rawRefresh, nil
}

func hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}
