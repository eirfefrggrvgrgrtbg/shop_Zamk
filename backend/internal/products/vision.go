package products

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

// ScheduleCurrentProductVision schedules a vision run for the DB-authoritative current product state.
func (s *Service) ScheduleCurrentProductVision(ctx context.Context, productID uuid.UUID) error {
	if s.vision == nil {
		// Vision scheduler disabled
		return nil
	}

	_, err := s.vision.ScheduleCurrentProductVision(ctx, productID)
	return err
}

// ReconcileMissingVisionRuns finds eligible products (pending_moderation or published)
// that lack a product_vision_runs record for the CURRENT canonical inference identity,
// and schedules one with bounded, deterministic ordering.
func (s *Service) ReconcileMissingVisionRuns(ctx context.Context, limit int) (int, error) {
	if s.vision == nil {
		return 0, nil
	}

	query := `
		SELECT p.id
		FROM products p
		WHERE p.status IN ('pending_moderation', 'published')
		  AND NOT EXISTS (
		      SELECT 1 FROM product_vision_runs v
		      WHERE v.product_id = p.id
		        AND v.vision_content_version = p.vision_content_version
		        AND v.provider = $2
		        AND v.model_id = $3
		        AND v.prompt_version = $4
		        AND v.schema_version = $5
		        AND v.vocabulary_version = $6
		  )
		ORDER BY p.id ASC
		LIMIT $1
	`
	rows, err := s.dbPool.Pool.Query(ctx, query,
		limit,
		vision.CanonicalProvider,
		vision.CanonicalModelID,
		vision.PromptVersion,
		vision.SchemaVersion,
		vision.VocabularyVersion,
	)
	if err != nil {
		return 0, fmt.Errorf("query missing vision runs: %w", err)
	}
	defer rows.Close()

	var productIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scan product id: %w", err)
		}
		productIDs = append(productIDs, id)
	}
	rows.Close() // Explicitly close before we start iterating

	successCount := 0
	for _, id := range productIDs {
		if err := s.ScheduleCurrentProductVision(ctx, id); err != nil {
			s.logger.Error("failed to reconcile vision run", "product_id", id, "error", err)
		} else {
			successCount++
		}
	}

	return successCount, nil
}
