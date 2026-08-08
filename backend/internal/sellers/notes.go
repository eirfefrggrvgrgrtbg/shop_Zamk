package sellers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

func (r *Repository) ListSellerNotes(ctx context.Context, sellerID uuid.UUID) ([]SellerNoteDTO, error) {
	query := `
		SELECT n.id, n.seller_id, n.author_id, u.name as author_name, n.note_type, n.content, n.deadline, n.is_archived, n.created_at, n.updated_at
		FROM seller_notes n
		LEFT JOIN admin_users u ON n.author_id = u.id
		WHERE n.seller_id = $1
		ORDER BY n.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}
	defer rows.Close()

	var notes []SellerNoteDTO
	for rows.Next() {
		var n SellerNoteDTO
		var authorID *uuid.UUID
		if err := rows.Scan(
			&n.ID, &n.SellerID, &authorID, &n.AuthorName, &n.NoteType, &n.Content, &n.Deadline, &n.IsArchived, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan note: %w", err)
		}
		if authorID != nil {
			s := authorID.String()
			n.AuthorID = &s
		}
		notes = append(notes, n)
	}
	return notes, nil
}

func (r *Repository) CreateSellerNote(ctx context.Context, sellerID uuid.UUID, authorID *uuid.UUID, req CreateSellerNoteRequest) (SellerNoteDTO, error) {
	var n SellerNoteDTO
	var returnedAuthorID *uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO seller_notes (seller_id, author_id, note_type, content, deadline)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, seller_id, author_id, note_type, content, deadline, is_archived, created_at, updated_at
	`, sellerID, authorID, req.NoteType, req.Content, req.Deadline).Scan(
		&n.ID, &n.SellerID, &returnedAuthorID, &n.NoteType, &n.Content, &n.Deadline, &n.IsArchived, &n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		return n, fmt.Errorf("failed to create note: %w", err)
	}
	if returnedAuthorID != nil {
		s := returnedAuthorID.String()
		n.AuthorID = &s
	}
	return n, nil
}

func (r *Repository) ListImprovementPlans(ctx context.Context, sellerID uuid.UUID) ([]ImprovementPlanDTO, error) {
	query := `
		SELECT p.id, p.seller_id, p.assignee_id, a.name as assignee_name, p.creator_id, c.name as creator_name, p.status, p.reason, p.actions, p.internal_comment, p.deadline, p.created_at, p.updated_at, p.completed_at
		FROM seller_improvement_plans p
		LEFT JOIN admin_users a ON p.assignee_id = a.id
		LEFT JOIN admin_users c ON p.creator_id = c.id
		WHERE p.seller_id = $1
		ORDER BY p.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list plans: %w", err)
	}
	defer rows.Close()

	var plans []ImprovementPlanDTO
	for rows.Next() {
		var p ImprovementPlanDTO
		var assigneeID, creatorID *uuid.UUID
		var actionsJSON []byte
		if err := rows.Scan(
			&p.ID, &p.SellerID, &assigneeID, &p.AssigneeName, &creatorID, &p.CreatorName, &p.Status, &p.Reason, &actionsJSON, &p.InternalComment, &p.Deadline, &p.CreatedAt, &p.UpdatedAt, &p.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan plan: %w", err)
		}
		if assigneeID != nil {
			s := assigneeID.String()
			p.AssigneeID = &s
		}
		if creatorID != nil {
			s := creatorID.String()
			p.CreatorID = &s
		}
		if len(actionsJSON) > 0 {
			_ = json.Unmarshal(actionsJSON, &p.Actions)
		}
		plans = append(plans, p)
	}
	return plans, nil
}

func (r *Repository) CreateImprovementPlan(ctx context.Context, sellerID uuid.UUID, creatorID *uuid.UUID, req CreateImprovementPlanRequest) (ImprovementPlanDTO, error) {
	var p ImprovementPlanDTO
	var returnedAssignee, returnedCreator *uuid.UUID
	var actionsJSON []byte
	
	b, _ := json.Marshal(req.Actions)
	if req.Actions == nil {
		b = []byte("[]")
	}

	err := r.db.QueryRow(ctx, `
		INSERT INTO seller_improvement_plans (seller_id, assignee_id, creator_id, reason, actions, internal_comment, deadline)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, seller_id, assignee_id, creator_id, status, reason, actions, internal_comment, deadline, created_at, updated_at, completed_at
	`, sellerID, req.AssigneeID, creatorID, req.Reason, b, req.InternalComment, req.Deadline).Scan(
		&p.ID, &p.SellerID, &returnedAssignee, &returnedCreator, &p.Status, &p.Reason, &actionsJSON, &p.InternalComment, &p.Deadline, &p.CreatedAt, &p.UpdatedAt, &p.CompletedAt,
	)
	if err != nil {
		return p, fmt.Errorf("failed to create plan: %w", err)
	}
	if returnedAssignee != nil {
		s := returnedAssignee.String()
		p.AssigneeID = &s
	}
	if returnedCreator != nil {
		s := returnedCreator.String()
		p.CreatorID = &s
	}
	if len(actionsJSON) > 0 {
		_ = json.Unmarshal(actionsJSON, &p.Actions)
	}
	return p, nil
}

func (r *Repository) UpdateImprovementPlanStatus(ctx context.Context, planID uuid.UUID, status string) error {
	res, err := r.db.Exec(ctx, `
		UPDATE seller_improvement_plans
		SET status = $1, 
			completed_at = CASE WHEN $1 = 'completed' THEN NOW() ELSE completed_at END,
			updated_at = NOW()
		WHERE id = $2
	`, status, planID)
	if err != nil {
		return fmt.Errorf("failed to update plan status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("plan not found")
	}
	return nil
}

// --- Service Methods ---

func (s *Service) ListSellerNotes(ctx context.Context, sellerID uuid.UUID) ([]SellerNoteDTO, error) {
	return s.repo.ListSellerNotes(ctx, sellerID)
}

func (s *Service) CreateSellerNote(ctx context.Context, sellerID uuid.UUID, authorID *uuid.UUID, req CreateSellerNoteRequest) (SellerNoteDTO, error) {
	return s.repo.CreateSellerNote(ctx, sellerID, authorID, req)
}

func (s *Service) ListImprovementPlans(ctx context.Context, sellerID uuid.UUID) ([]ImprovementPlanDTO, error) {
	return s.repo.ListImprovementPlans(ctx, sellerID)
}

func (s *Service) CreateImprovementPlan(ctx context.Context, sellerID uuid.UUID, creatorID *uuid.UUID, req CreateImprovementPlanRequest) (ImprovementPlanDTO, error) {
	return s.repo.CreateImprovementPlan(ctx, sellerID, creatorID, req)
}

func (s *Service) UpdateImprovementPlanStatus(ctx context.Context, planID uuid.UUID, status string) error {
	return s.repo.UpdateImprovementPlanStatus(ctx, planID, status)
}

func (r *Repository) ArchiveSellerNote(ctx context.Context, noteID uuid.UUID, isArchived bool) error {
	res, err := r.db.Exec(ctx, `
		UPDATE seller_notes
		SET is_archived = $1, updated_at = NOW()
		WHERE id = $2
	`, isArchived, noteID)
	if err != nil {
		return fmt.Errorf("failed to archive note: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("note not found")
	}
	return nil
}

func (s *Service) ArchiveSellerNote(ctx context.Context, noteID uuid.UUID, isArchived bool) error {
	return s.repo.ArchiveSellerNote(ctx, noteID, isArchived)
}
