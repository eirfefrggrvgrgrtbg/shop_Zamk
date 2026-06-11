package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

var ErrNotFound = errors.New("user not found")

type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

// WithTx returns a new Repository bound to the provided transaction
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{db: tx}
}

func (r *Repository) CreateUser(ctx context.Context, u *User) error {
	query := `
		INSERT INTO users (id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		u.ID, u.Name, u.Email, u.Phone, u.PasswordHash, u.Role, u.Status, u.MustChangePassword, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Name, &u.Email, &u.Phone, &u.PasswordHash, &u.Role, &u.Status, &u.MustChangePassword, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, name, email, phone, password_hash, role, status, must_change_password, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Name, &u.Email, &u.Phone, &u.PasswordHash, &u.Role, &u.Status, &u.MustChangePassword, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return &u, nil
}

func (r *Repository) UpdateUserStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := `UPDATE users SET status = $1, updated_at = now() WHERE id = $2`
	res, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpdatePasswordAndMustChange(ctx context.Context, id uuid.UUID, passwordHash string, mustChange bool) error {
	query := `
		UPDATE users
		SET password_hash = $1, must_change_password = $2, updated_at = now()
		WHERE id = $3
	`
	res, err := r.db.Exec(ctx, query, passwordHash, mustChange, id)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateCustomerProfile(ctx context.Context, id uuid.UUID, name, phone string) error {
	query := `
		UPDATE users
		SET name = $1, phone = $2, updated_at = now()
		WHERE id = $3 AND role = 'customer'
	`
	res, err := r.db.Exec(ctx, query, name, phone, id)
	if err != nil {
		return fmt.Errorf("failed to update customer profile: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

