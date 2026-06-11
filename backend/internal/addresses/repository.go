package addresses

import (
	"context"
	"errors"
	"fmt"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrAddressNotFound = errors.New("address not found")
)

type Repository struct {
	db *postgres.Client
}

func NewRepository(db *postgres.Client) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListAddresses(ctx context.Context, userID uuid.UUID) ([]Address, error) {
	query := `
		SELECT id, user_id, label, recipient_name, phone, city, street, house, apartment, postal_code, comment, is_default, created_at, updated_at
		FROM customer_addresses
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list addresses: %w", err)
	}
	defer rows.Close()

	var addresses []Address
	for rows.Next() {
		var a Address
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.Label, &a.RecipientName, &a.Phone, &a.City, &a.Street,
			&a.House, &a.Apartment, &a.PostalCode, &a.Comment, &a.IsDefault, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		addresses = append(addresses, a)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return addresses, nil
}

func (r *Repository) CreateAddress(ctx context.Context, a *Address) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if a.IsDefault {
		if err := r.clearDefaultTx(ctx, tx, a.UserID); err != nil {
			return err
		}
	} else {
		// If this is the first address, make it default automatically
		var count int
		err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM customer_addresses WHERE user_id = $1", a.UserID).Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			a.IsDefault = true
		}
	}

	query := `
		INSERT INTO customer_addresses (id, user_id, label, recipient_name, phone, city, street, house, apartment, postal_code, comment, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err = tx.Exec(ctx, query,
		a.ID, a.UserID, a.Label, a.RecipientName, a.Phone, a.City, a.Street,
		a.House, a.Apartment, a.PostalCode, a.Comment, a.IsDefault, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create address: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetAddressByID(ctx context.Context, id, userID uuid.UUID) (*Address, error) {
	query := `
		SELECT id, user_id, label, recipient_name, phone, city, street, house, apartment, postal_code, comment, is_default, created_at, updated_at
		FROM customer_addresses
		WHERE id = $1 AND user_id = $2
	`
	var a Address
	err := r.db.Pool.QueryRow(ctx, query, id, userID).Scan(
		&a.ID, &a.UserID, &a.Label, &a.RecipientName, &a.Phone, &a.City, &a.Street,
		&a.House, &a.Apartment, &a.PostalCode, &a.Comment, &a.IsDefault, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAddressNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *Repository) UpdateAddress(ctx context.Context, a *Address) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if a.IsDefault {
		if err := r.clearDefaultTx(ctx, tx, a.UserID); err != nil {
			return err
		}
	}

	query := `
		UPDATE customer_addresses
		SET label = $1, recipient_name = $2, phone = $3, city = $4, street = $5, house = $6, apartment = $7, postal_code = $8, comment = $9, is_default = $10, updated_at = now()
		WHERE id = $11 AND user_id = $12
	`
	res, err := tx.Exec(ctx, query,
		a.Label, a.RecipientName, a.Phone, a.City, a.Street, a.House, a.Apartment, a.PostalCode, a.Comment, a.IsDefault,
		a.ID, a.UserID,
	)
	if err != nil {
		return fmt.Errorf("failed to update address: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrAddressNotFound
	}

	return tx.Commit(ctx)
}

func (r *Repository) DeleteAddress(ctx context.Context, id, userID uuid.UUID) error {
	query := `DELETE FROM customer_addresses WHERE id = $1 AND user_id = $2`
	res, err := r.db.Pool.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete address: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrAddressNotFound
	}
	return nil
}

func (r *Repository) SetDefault(ctx context.Context, id, userID uuid.UUID) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := r.clearDefaultTx(ctx, tx, userID); err != nil {
		return err
	}

	query := `UPDATE customer_addresses SET is_default = true, updated_at = now() WHERE id = $1 AND user_id = $2`
	res, err := tx.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to set default address: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrAddressNotFound
	}

	return tx.Commit(ctx)
}

func (r *Repository) clearDefaultTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	query := `UPDATE customer_addresses SET is_default = false, updated_at = now() WHERE user_id = $1 AND is_default = true`
	_, err := tx.Exec(ctx, query, userID)
	return err
}
