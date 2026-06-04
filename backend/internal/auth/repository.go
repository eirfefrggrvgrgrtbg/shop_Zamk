package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type Session struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	UserAgent        string
	IPAddress        string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
}

type Repository struct {
	db *postgres.Client
}

func NewRepository(db *postgres.Client) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateSession(ctx context.Context, s *Session) error {
	query := `
		INSERT INTO user_sessions (id, user_id, refresh_token_hash, user_agent, ip_address, expires_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		s.ID, s.UserID, s.RefreshTokenHash, s.UserAgent, s.IPAddress, s.ExpiresAt, s.RevokedAt, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

func (r *Repository) GetSessionByRefreshTokenHash(ctx context.Context, hash string) (*Session, error) {
	query := `
		SELECT id, user_id, refresh_token_hash, user_agent, ip_address, expires_at, revoked_at, created_at
		FROM user_sessions
		WHERE refresh_token_hash = $1
	`
	var s Session
	err := r.db.Pool.QueryRow(ctx, query, hash).Scan(
		&s.ID, &s.UserID, &s.RefreshTokenHash, &s.UserAgent, &s.IPAddress, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session by hash: %w", err)
	}
	return &s, nil
}

func (r *Repository) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	query := `UPDATE user_sessions SET revoked_at = now() WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}
	return nil
}

func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE user_sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke user sessions: %w", err)
	}
	return nil
}
