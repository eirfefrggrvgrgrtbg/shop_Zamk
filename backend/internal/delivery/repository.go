package delivery

import (
	"context"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
)

type Repository struct {
	db *postgres.Client
}

func NewRepository(db *postgres.Client) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetActiveMethods(ctx context.Context) ([]DeliveryMethod, error) {
	query := `
		SELECT id, code, name, description, price_cents, estimated_days_min, estimated_days_max, is_active, sort_order, created_at, updated_at
		FROM delivery_methods
		WHERE is_active = true
		ORDER BY sort_order ASC, name ASC
	`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var methods []DeliveryMethod
	for rows.Next() {
		var m DeliveryMethod
		if err := rows.Scan(&m.ID, &m.Code, &m.Name, &m.Description, &m.PriceCents, &m.EstimatedDaysMin, &m.EstimatedDaysMax, &m.IsActive, &m.SortOrder, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		methods = append(methods, m)
	}
	return methods, nil
}

func (r *Repository) GetMethodByID(ctx context.Context, id uuid.UUID) (*DeliveryMethod, error) {
	query := `
		SELECT id, code, name, description, price_cents, estimated_days_min, estimated_days_max, is_active, sort_order, created_at, updated_at
		FROM delivery_methods
		WHERE id = $1
	`
	var m DeliveryMethod
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(&m.ID, &m.Code, &m.Name, &m.Description, &m.PriceCents, &m.EstimatedDaysMin, &m.EstimatedDaysMax, &m.IsActive, &m.SortOrder, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
