package notifications

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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
			id, recipient_user_id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, metadata, created_at, kind, severity, status, dedupe_key, action_url, resolved_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		)
	`
	_, err := tx.Exec(ctx, query,
		n.ID, n.RecipientUserID, n.RecipientSellerID, n.RecipientKind, n.Type, n.Title, n.Body, n.EntityType, n.EntityID, n.Metadata, n.CreatedAt, n.Kind, n.Severity, n.Status, n.DedupeKey, n.ActionURL, n.ResolvedAt,
	)
	return err
}

func (r *Repository) CheckExistsTx(ctx context.Context, tx pgx.Tx, recipientKind, typ, entityType string, entityID uuid.UUID, recipientUserID *uuid.UUID, recipientSellerID *uuid.UUID) (bool, error) {
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
	} else if recipientSellerID != nil {
		query += " AND recipient_seller_id = $5"
		args = append(args, *recipientSellerID)
	} else {
		query += " AND recipient_user_id IS NULL AND recipient_seller_id IS NULL"
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
		SELECT id, recipient_user_id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, metadata, read_at, created_at, kind, severity, status, dedupe_key, action_url, resolved_at
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
			&n.ID, &n.RecipientUserID, &n.RecipientSellerID, &n.RecipientKind, &n.Type, &n.Title, &n.Body, &n.EntityType, &n.EntityID, &n.Metadata, &n.ReadAt, &n.CreatedAt, &n.Kind, &n.Severity, &n.Status, &n.DedupeKey, &n.ActionURL, &n.ResolvedAt,
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


func (r *Repository) UpsertActiveSellerAlertTx(ctx context.Context, tx pgx.Tx, n *Notification) (uuid.UUID, error) {
	if n == nil {
		return uuid.Nil, errors.New("notification cannot be nil")
	}
	if n.RecipientKind != RecipientKindSeller {
		return uuid.Nil, fmt.Errorf("%w: recipient_kind must be seller", ErrMalformedAlert)
	}
	if n.RecipientSellerID == nil || *n.RecipientSellerID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: recipient_seller_id is required", ErrMalformedAlert)
	}
	if n.DedupeKey == nil || strings.TrimSpace(*n.DedupeKey) == "" {
		return uuid.Nil, fmt.Errorf("%w: dedupe_key is required", ErrMalformedAlert)
	}
	if n.Kind != KindAlert {
		return uuid.Nil, fmt.Errorf("%w: kind must be alert", ErrMalformedAlert)
	}
	if n.Severity != SeverityInfo && n.Severity != SeverityWarning && n.Severity != SeverityCritical {
		return uuid.Nil, fmt.Errorf("%w: invalid severity %q", ErrMalformedAlert, n.Severity)
	}

	active := StatusActive
	n.Status = &active
	n.ResolvedAt = nil

	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.EntityType == "" {
		n.EntityType = "alert"
	}
	if n.EntityID == uuid.Nil {
		n.EntityID = n.ID
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	if n.Metadata == nil {
		n.Metadata = map[string]interface{}{}
	}

	query := `
		INSERT INTO notifications (
			id, recipient_user_id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, metadata, created_at, kind, severity, status, dedupe_key, action_url, resolved_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		)
		ON CONFLICT (recipient_seller_id, dedupe_key) WHERE kind = 'alert' AND status = 'active'
		DO UPDATE SET
			title = EXCLUDED.title,
			body = EXCLUDED.body,
			severity = EXCLUDED.severity,
			action_url = EXCLUDED.action_url,
			metadata = EXCLUDED.metadata
		RETURNING id
	`
	var returnedID uuid.UUID
	err := tx.QueryRow(ctx, query,
		n.ID, n.RecipientUserID, n.RecipientSellerID, n.RecipientKind, n.Type, n.Title, n.Body, n.EntityType, n.EntityID, n.Metadata, n.CreatedAt, n.Kind, n.Severity, n.Status, n.DedupeKey, n.ActionURL, n.ResolvedAt,
	).Scan(&returnedID)
	if err != nil {
		return uuid.Nil, err
	}
	n.ID = returnedID
	return returnedID, nil
}

func (r *Repository) ResolveSellerAlertTx(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID, dedupeKey string) error {
	if sellerID == uuid.Nil || strings.TrimSpace(dedupeKey) == "" {
		return errors.New("seller_id and non-empty dedupe_key are required to resolve alert")
	}
	query := `
		UPDATE notifications
		SET status = 'resolved', resolved_at = now()
		WHERE recipient_seller_id = $1 AND dedupe_key = $2 AND kind = 'alert' AND status = 'active'
	`
	_, err := tx.Exec(ctx, query, sellerID, dedupeKey)
	return err
}
