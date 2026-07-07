package notifications

import (
	"context"

	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type Repository struct {
	db *postgres.Client
}

func NewRepository(db *postgres.Client) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateNotificationTx(ctx context.Context, tx pgx.Tx, n *Notification) error {
	query := `
		INSERT INTO notifications (
			id, recipient_user_id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, metadata, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`
	_, err := tx.Exec(ctx, query,
		n.ID, n.RecipientUserID, n.RecipientSellerID, n.RecipientKind, n.Type, n.Title, n.Body, n.EntityType, n.EntityID, n.Metadata, n.CreatedAt,
	)
	return err
}

func (r *Repository) CheckExistsTx(ctx context.Context, tx pgx.Tx, recipientKind, typ, entityType string, entityID uuid.UUID, recipientUserID *uuid.UUID) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM notifications
			WHERE recipient_kind = $1 AND type = $2 AND entity_type = $3 AND entity_id = $4
	`
	args := []interface{}{recipientKind, typ, entityType, entityID}
	
	if recipientUserID != nil {
		query += " AND recipient_user_id = $5"
		args = append(args, *recipientUserID)
	} else {
		query += " AND recipient_user_id IS NULL"
	}
	query += ")"

	err := tx.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}

func (r *Repository) CreateManyNotificationsTx(ctx context.Context, tx pgx.Tx, ns []Notification) error {
	if len(ns) == 0 {
		return nil
	}
	
	// pgx.Batch could be used, or simply loop
	for _, n := range ns {
		if err := r.CreateNotificationTx(ctx, tx, &n); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListNotifications(ctx context.Context, userID, sellerID *uuid.UUID, kind string, limit, offset int) ([]Notification, int, error) {
	whereClause := "WHERE recipient_kind = $1"
	args := []interface{}{kind}
	
	if userID != nil {
		whereClause += " AND recipient_user_id = $2"
		args = append(args, *userID)
	} else if sellerID != nil {
		whereClause += " AND recipient_seller_id = $2"
		args = append(args, *sellerID)
	}

	countQuery := "SELECT count(*) FROM notifications " + whereClause
	var total int
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, recipient_user_id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, metadata, read_at, created_at
		FROM notifications
		` + whereClause + `
		ORDER BY created_at DESC
		LIMIT $` + r.placeholder(len(args)+1) + ` OFFSET $` + r.placeholder(len(args)+2)
	
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(
			&n.ID, &n.RecipientUserID, &n.RecipientSellerID, &n.RecipientKind, &n.Type, &n.Title, &n.Body, &n.EntityType, &n.EntityID, &n.Metadata, &n.ReadAt, &n.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		notifications = append(notifications, n)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return notifications, total, nil
}

func (r *Repository) placeholder(idx int) string {
	return strconv.Itoa(idx)
}

func (r *Repository) MarkRead(ctx context.Context, id uuid.UUID, userID, sellerID *uuid.UUID, kind string) error {
	query := `UPDATE notifications SET read_at = now() WHERE id = $1 AND recipient_kind = $2`
	args := []interface{}{id, kind}
	
	if userID != nil {
		query += " AND recipient_user_id = $3"
		args = append(args, *userID)
	} else if sellerID != nil {
		query += " AND recipient_seller_id = $3"
		args = append(args, *sellerID)
	}

	res, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) MarkAllRead(ctx context.Context, userID, sellerID *uuid.UUID, kind string) error {
	query := `UPDATE notifications SET read_at = now() WHERE read_at IS NULL AND recipient_kind = $1`
	args := []interface{}{kind}
	
	if userID != nil {
		query += " AND recipient_user_id = $2"
		args = append(args, *userID)
	} else if sellerID != nil {
		query += " AND recipient_seller_id = $2"
		args = append(args, *sellerID)
	}

	_, err := r.db.Pool.Exec(ctx, query, args...)
	return err
}

func (r *Repository) CountUnread(ctx context.Context, userID, sellerID *uuid.UUID, kind string) (int, error) {
	query := `SELECT count(*) FROM notifications WHERE read_at IS NULL AND recipient_kind = $1`
	args := []interface{}{kind}
	
	if userID != nil {
		query += " AND recipient_user_id = $2"
		args = append(args, *userID)
	} else if sellerID != nil {
		query += " AND recipient_seller_id = $2"
		args = append(args, *sellerID)
	}

	var count int
	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *Repository) GetSellerIDByUserID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var sellerID uuid.UUID
	query := `SELECT seller_id FROM seller_users WHERE user_id = $1 LIMIT 1`
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&sellerID)
	return sellerID, err
}
