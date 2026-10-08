package marketing

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSavedQueryNotFound = errors.New("saved query not found")
	ErrSavedQueryConflict = errors.New("saved query name conflict")
)

type SavedQueryRepository struct {
	pool *pgxpool.Pool
}

func NewSavedQueryRepository(pool *pgxpool.Pool) *SavedQueryRepository {
	return &SavedQueryRepository{pool: pool}
}

func (r *SavedQueryRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]*SavedQuery, error) {
	query := `
		SELECT id, name, description, query_version, query_spec, created_by, created_at, updated_at
		FROM marketing_saved_queries
		WHERE created_by = $1
		ORDER BY updated_at DESC, id ASC
	`
	rows, err := r.pool.Query(ctx, query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queries []*SavedQuery
	for rows.Next() {
		var sq SavedQuery
		if err := rows.Scan(
			&sq.ID,
			&sq.Name,
			&sq.Description,
			&sq.QueryVersion,
			&sq.QuerySpec,
			&sq.CreatedBy,
			&sq.CreatedAt,
			&sq.UpdatedAt,
		); err != nil {
			return nil, err
		}
		queries = append(queries, &sq)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if queries == nil {
		queries = []*SavedQuery{}
	}
	return queries, nil
}

func (r *SavedQueryRepository) GetByID(ctx context.Context, id, ownerID uuid.UUID) (*SavedQuery, error) {
	query := `
		SELECT id, name, description, query_version, query_spec, created_by, created_at, updated_at
		FROM marketing_saved_queries
		WHERE id = $1 AND created_by = $2
	`
	row := r.pool.QueryRow(ctx, query, id, ownerID)

	var sq SavedQuery
	err := row.Scan(
		&sq.ID,
		&sq.Name,
		&sq.Description,
		&sq.QueryVersion,
		&sq.QuerySpec,
		&sq.CreatedBy,
		&sq.CreatedAt,
		&sq.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSavedQueryNotFound
		}
		return nil, err
	}
	return &sq, nil
}

func (r *SavedQueryRepository) Create(ctx context.Context, sq *SavedQuery) error {
	query := `
		INSERT INTO marketing_saved_queries (id, name, description, query_version, query_spec, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	sq.ID = uuid.New()
	sq.CreatedAt = time.Now().UTC()
	sq.UpdatedAt = sq.CreatedAt

	_, err := r.pool.Exec(ctx, query,
		sq.ID,
		sq.Name,
		sq.Description,
		sq.QueryVersion,
		sq.QuerySpec,
		sq.CreatedBy,
		sq.CreatedAt,
		sq.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return ErrSavedQueryConflict
		}
		return err
	}
	return nil
}

func (r *SavedQueryRepository) Update(ctx context.Context, sq *SavedQuery) error {
	query := `
		UPDATE marketing_saved_queries
		SET name = $1, description = $2, query_version = $3, query_spec = $4, updated_at = $5
		WHERE id = $6 AND created_by = $7
	`
	sq.UpdatedAt = time.Now().UTC()

	res, err := r.pool.Exec(ctx, query,
		sq.Name,
		sq.Description,
		sq.QueryVersion,
		sq.QuerySpec,
		sq.UpdatedAt,
		sq.ID,
		sq.CreatedBy,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrSavedQueryConflict
		}
		return err
	}

	if res.RowsAffected() == 0 {
		return ErrSavedQueryNotFound
	}
	return nil
}

func (r *SavedQueryRepository) Delete(ctx context.Context, id, ownerID uuid.UUID) error {
	query := `
		DELETE FROM marketing_saved_queries
		WHERE id = $1 AND created_by = $2
	`
	res, err := r.pool.Exec(ctx, query, id, ownerID)
	if err != nil {
		return err
	}

	if res.RowsAffected() == 0 {
		return ErrSavedQueryNotFound
	}
	return nil
}
