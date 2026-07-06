package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListLogs(ctx context.Context, filters AuditLogFilters) ([]AuditLog, int, error) {
	var conditions []string
	var args []interface{}
	argId := 1

	if filters.Query != "" {
		conditions = append(conditions, fmt.Sprintf("(actor_email ILIKE $%d OR action ILIKE $%d OR entity_type ILIKE $%d)", argId, argId, argId))
		args = append(args, "%"+filters.Query+"%")
		argId++
	}

	if filters.ActorID != nil {
		conditions = append(conditions, fmt.Sprintf("actor_user_id = $%d", argId))
		args = append(args, *filters.ActorID)
		argId++
	}

	if filters.Action != "" {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argId))
		args = append(args, filters.Action)
		argId++
	}

	if filters.EntityType != "" {
		conditions = append(conditions, fmt.Sprintf("entity_type = $%d", argId))
		args = append(args, filters.EntityType)
		argId++
	}

	if filters.EntityID != nil {
		conditions = append(conditions, fmt.Sprintf("entity_id = $%d", argId))
		args = append(args, *filters.EntityID)
		argId++
	}

	if filters.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argId))
		args = append(args, *filters.DateFrom)
		argId++
	}

	if filters.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argId))
		args = append(args, *filters.DateTo)
		argId++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM audit_logs %s`, whereClause)
	var totalCount int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, actor_user_id, actor_email, actor_role, permission, action, 
		       entity_type, entity_id, request_id, ip, user_agent, metadata, created_at
		FROM audit_logs
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argId, argId+1)

	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(
			&l.ID,
			&l.ActorUserID,
			&l.ActorEmail,
			&l.ActorRole,
			&l.Permission,
			&l.Action,
			&l.EntityType,
			&l.EntityID,
			&l.RequestID,
			&l.IP,
			&l.UserAgent,
			&l.Metadata,
			&l.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if logs == nil {
		logs = []AuditLog{}
	}

	return logs, totalCount, nil
}
