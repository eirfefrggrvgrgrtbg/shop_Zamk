package products

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

const (
	DefaultVisionRunsLimit = 20
	MaxVisionRunsLimit     = 100
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

// GetAdminProductVision retrieves the current Admin-safe Vision observation or run state.
func (s *Service) GetAdminProductVision(ctx context.Context, productID uuid.UUID, currentVersion int64) (*AdminProductVisionDTO, error) {
	if s.dbPool == nil || s.dbPool.Pool == nil {
		return nil, nil
	}

	// 1. Check current visual profile joined with its source vision run
	const queryProfile = `
		SELECT
			cp.vision_content_version,
			cp.snapshot_hash,
			cp.taxonomy_context_hash,
			cp.model_id,
			cp.observation,
			r.id,
			r.status,
			r.schema_version,
			r.snapshot_hash,
			r.taxonomy_context_hash,
			r.model_id,
			r.completed_at,
			r.result_evaluation,
			r.failure_code
		FROM product_current_visual_profiles cp
		JOIN product_vision_runs r ON cp.run_id = r.id
		WHERE cp.product_id = $1
	`
	var (
		cpVersion        int64
		cpSnapshotHash   string
		cpTaxonomyHash   string
		cpModelID        string
		obsJSON          []byte
		runID            uuid.UUID
		runStatus        string
		runSchemaVersion string
		runSnapshotHash  string
		runTaxonomyHash  string
		runModelID       string
		completedAt      *time.Time
		evalJSON         []byte
		failureCode      *string
	)

	err := s.dbPool.Pool.QueryRow(ctx, queryProfile, productID).Scan(
		&cpVersion,
		&cpSnapshotHash,
		&cpTaxonomyHash,
		&cpModelID,
		&obsJSON,
		&runID,
		&runStatus,
		&runSchemaVersion,
		&runSnapshotHash,
		&runTaxonomyHash,
		&runModelID,
		&completedAt,
		&evalJSON,
		&failureCode,
	)

	// Valid CURRENT visual profile predicate:
	// - Profile exists and matches product's DB-authoritative current version
	// - Run status is 'succeeded' and schema is canonical V2
	// - Identity parity: snapshot hash, taxonomy context hash, and model identity match between profile and run
	isValidCurrentProfile := (err == nil) &&
		(cpVersion == currentVersion) &&
		(runStatus == "succeeded") &&
		(runSchemaVersion == vision.SchemaVersion) &&
		(cpSnapshotHash != "" && cpSnapshotHash == runSnapshotHash) &&
		(cpTaxonomyHash != "" && cpTaxonomyHash == runTaxonomyHash) &&
		(cpModelID != "" && cpModelID == runModelID)

	if isValidCurrentProfile {
		return s.buildSucceededVisionDTO(ctx, obsJSON, evalJSON, completedAt)
	}

	// 2. No valid current profile exists. Inspect the latest run for CURRENT product version.
	// Hard rule: Bare historical succeeded runs MUST NOT be promoted into current truth.
	// If a run for the current version succeeded but no valid current profile exists, status is 'stale'.
	const queryLatestRun = `
		SELECT
			id,
			status,
			failure_code,
			schema_version,
			completed_at
		FROM product_vision_runs
		WHERE product_id = $1
		  AND vision_content_version = $2
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`
	var (
		latestRunID         uuid.UUID
		latestStatus        string
		latestFailureCode   *string
		latestSchemaVersion string
		latestCompletedAt   *time.Time
	)

	runErr := s.dbPool.Pool.QueryRow(ctx, queryLatestRun, productID, currentVersion).Scan(
		&latestRunID,
		&latestStatus,
		&latestFailureCode,
		&latestSchemaVersion,
		&latestCompletedAt,
	)

	if runErr != nil {
		// No run exists for the current content version
		return &AdminProductVisionDTO{
			Status: "not_scheduled",
		}, nil
	}

	switch latestStatus {
	case "pending":
		return &AdminProductVisionDTO{
			Status: "pending",
		}, nil
	case "processing":
		return &AdminProductVisionDTO{
			Status: "processing",
		}, nil
	case "succeeded":
		// A run for current version succeeded, but current profile is missing, stale, or identity-invalid.
		// Never present bare historical succeeded runs as current truth.
		if latestSchemaVersion != vision.SchemaVersion {
			return &AdminProductVisionDTO{
				Status: "not_scheduled",
			}, nil
		}
		return &AdminProductVisionDTO{
			Status: "stale",
		}, nil
	case "provider_failed", "invalid_output", "processing_failed", "failed":
		return &AdminProductVisionDTO{
			Status:      "failed",
			FailureCode: latestFailureCode,
		}, nil
	default:
		return &AdminProductVisionDTO{
			Status:      "failed",
			FailureCode: latestFailureCode,
		}, nil
	}
}

func (s *Service) buildSucceededVisionDTO(ctx context.Context, obsJSON, evalJSON []byte, completedAt *time.Time) (*AdminProductVisionDTO, error) {
	var obs vision.ProductVisualObservationV2
	if err := json.Unmarshal(obsJSON, &obs); err != nil {
		return nil, fmt.Errorf("unmarshal visual observation: %w", err)
	}

	dto := &AdminProductVisionDTO{
		Status:               "succeeded",
		AnalyzedAt:           completedAt,
		ObservedCategoryText: obs.ObservedCategoryText,
		DominantColors:       []AdminVisionColorDTO{},
		SecondaryColors:      []AdminVisionColorDTO{},
		Silhouette:           obs.SilhouetteID,
		FitAppearance:        obs.FitAppearanceID,
		Length:               obs.LengthID,
		Sleeves:              obs.SleeveIDs,
		Neckline:             obs.NecklineID,
		Closure:              obs.ClosureID,
		Pattern:              obs.PatternID,
		VisibleTexture:       obs.VisibleTextureID,
		DecorativeDetails:    obs.DecorativeDetailIDs,
		StyleDescriptors:     obs.StyleDescriptorIDs,
		Formality:            obs.FormalityID,
		SeasonalCues:         obs.SeasonalCueIDs,
		ImageSetConsistency:  obs.ImageSetConsistency,
		OCRText:              obs.OCRText,
		Confidence:           obs.Confidence,
		Evidence:             obs.Evidence,
	}

	if obs.CategoryConsistency != "" {
		consistencyStr := string(obs.CategoryConsistency)
		dto.CategoryConsistency = &consistencyStr
	}

	colorIDs := make([]uuid.UUID, 0, len(obs.DominantColorIDs)+len(obs.SecondaryColorIDs))
	colorIDs = append(colorIDs, obs.DominantColorIDs...)
	colorIDs = append(colorIDs, obs.SecondaryColorIDs...)

	resolvedColors, err := s.resolveColors(ctx, colorIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve colors: %w", err)
	}

	for _, id := range obs.DominantColorIDs {
		dto.DominantColors = append(dto.DominantColors, resolvedColors[id])
	}
	for _, id := range obs.SecondaryColorIDs {
		dto.SecondaryColors = append(dto.SecondaryColors, resolvedColors[id])
	}

	if len(evalJSON) > 0 {
		var eval vision.ModerationEvaluationV1
		if err := json.Unmarshal(evalJSON, &eval); err == nil && (eval.ModerationRecommendation != "" || len(eval.Contradictions) > 0 || len(eval.QualityIssues) > 0 || len(eval.OCRFindings) > 0) {
			dto.Moderation = &AdminVisionModerationDTO{
				Contradictions:       eval.Contradictions,
				QualityIssues:        eval.QualityIssues,
				OCRFindings:          eval.OCRFindings,
				Recommendation:       eval.ModerationRecommendation,
				RecommendationReason: eval.RecommendationReason,
			}
		}
	}

	return dto, nil
}

func (s *Service) resolveColors(ctx context.Context, colorIDs []uuid.UUID) (map[uuid.UUID]AdminVisionColorDTO, error) {
	result := make(map[uuid.UUID]AdminVisionColorDTO, len(colorIDs))
	if len(colorIDs) == 0 {
		return result, nil
	}

	uniqueIDs := make([]uuid.UUID, 0, len(colorIDs))
	seen := make(map[uuid.UUID]struct{}, len(colorIDs))
	for _, id := range colorIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	const query = `
		SELECT id, code, name_ru, hex
		FROM colors
		WHERE id = ANY($1)
	`
	rows, err := s.dbPool.Pool.Query(ctx, query, uniqueIDs)
	if err != nil {
		return nil, fmt.Errorf("query colors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id     uuid.UUID
			code   string
			nameRU string
			hex    *string
		)
		if err := rows.Scan(&id, &code, &nameRU, &hex); err != nil {
			return nil, fmt.Errorf("scan color: %w", err)
		}
		result[id] = AdminVisionColorDTO{
			ID:   id,
			Code: code,
			Name: nameRU,
			Hex:  hex,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate colors: %w", err)
	}

	for _, id := range uniqueIDs {
		if _, found := result[id]; !found {
			result[id] = AdminVisionColorDTO{
				ID:   id,
				Code: "unresolved",
				Name: "Неизвестный цвет",
				Hex:  nil,
			}
		}
	}

	return result, nil
}

// GetAdminProductVisionRuns retrieves the history of vision runs for a product in deterministic order.
func (s *Service) GetAdminProductVisionRuns(ctx context.Context, productID uuid.UUID, limit, offset int) (*AdminProductVisionRunsResponse, error) {
	if s.dbPool == nil || s.dbPool.Pool == nil {
		return &AdminProductVisionRunsResponse{
			Items:  []AdminProductVisionRunDTO{},
			Total:  0,
			Limit:  limit,
			Offset: offset,
		}, nil
	}

	// 1. Verify product exists and get its current vision_content_version
	var currentVersion int64
	err := s.dbPool.Pool.QueryRow(ctx, "SELECT vision_content_version FROM products WHERE id = $1", productID).Scan(&currentVersion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("check product exists: %w", err)
	}

	// 2. Check if a valid current profile exists for this product
	const queryProfile = `
		SELECT
			cp.run_id,
			cp.vision_content_version,
			cp.snapshot_hash,
			cp.taxonomy_context_hash,
			cp.model_id,
			r.status,
			r.schema_version,
			r.snapshot_hash,
			r.taxonomy_context_hash,
			r.model_id
		FROM product_current_visual_profiles cp
		JOIN product_vision_runs r ON cp.run_id = r.id
		WHERE cp.product_id = $1
	`
	var (
		cpRunID        uuid.UUID
		cpVersion      int64
		cpSnapshotHash string
		cpTaxonomyHash string
		cpModelID      string
		rStatus        string
		rSchemaVersion string
		rSnapshotHash  string
		rTaxonomyHash  string
		rModelID       string
	)
	profileErr := s.dbPool.Pool.QueryRow(ctx, queryProfile, productID).Scan(
		&cpRunID,
		&cpVersion,
		&cpSnapshotHash,
		&cpTaxonomyHash,
		&cpModelID,
		&rStatus,
		&rSchemaVersion,
		&rSnapshotHash,
		&rTaxonomyHash,
		&rModelID,
	)
	var currentRunID *uuid.UUID
	if profileErr == nil &&
		cpVersion == currentVersion &&
		rStatus == "succeeded" &&
		rSchemaVersion == vision.SchemaVersion &&
		cpSnapshotHash != "" && cpSnapshotHash == rSnapshotHash &&
		cpTaxonomyHash != "" && cpTaxonomyHash == rTaxonomyHash &&
		cpModelID != "" && cpModelID == rModelID {
		currentRunID = &cpRunID
	}

	// 3. Count total runs for this product
	var total int
	countErr := s.dbPool.Pool.QueryRow(ctx, "SELECT count(*) FROM product_vision_runs WHERE product_id = $1", productID).Scan(&total)
	if countErr != nil {
		return nil, fmt.Errorf("count vision runs: %w", countErr)
	}

	// Bounded limit
	if limit <= 0 {
		limit = DefaultVisionRunsLimit
	} else if limit > MaxVisionRunsLimit {
		limit = MaxVisionRunsLimit
	}
	if offset < 0 {
		offset = 0
	}

	items := make([]AdminProductVisionRunDTO, 0)
	if total == 0 {
		return &AdminProductVisionRunsResponse{
			Items:  items,
			Total:  0,
			Limit:  limit,
			Offset: offset,
		}, nil
	}

	// 4. Query runs with deterministic ordering: created_at DESC, id DESC
	const queryRuns = `
		SELECT
			id,
			product_id,
			vision_content_version,
			provider,
			model_id,
			prompt_version,
			schema_version,
			vocabulary_version,
			status,
			attempts,
			created_at,
			claimed_at,
			completed_at,
			latency_ms,
			cost_cents,
			failure_code,
			usage_metadata,
			result_observation,
			result_evaluation
		FROM product_vision_runs
		WHERE product_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.dbPool.Pool.Query(ctx, queryRuns, productID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query vision runs: %w", err)
	}
	defer rows.Close()

	type rawRun struct {
		id                   uuid.UUID
		productID            uuid.UUID
		visionContentVersion int64
		provider             string
		modelID              string
		promptVersion        string
		schemaVersion        string
		vocabularyVersion    string
		status               string
		attempts             int
		createdAt            time.Time
		claimedAt            *time.Time
		completedAt          *time.Time
		latencyMs            *int
		costCents            *int
		failureCode          *string
		usageJSON            []byte
		obsJSON              []byte
		evalJSON             []byte
	}

	var rawRuns []rawRun
	var allColorIDs []uuid.UUID

	for rows.Next() {
		var r rawRun
		if err := rows.Scan(
			&r.id,
			&r.productID,
			&r.visionContentVersion,
			&r.provider,
			&r.modelID,
			&r.promptVersion,
			&r.schemaVersion,
			&r.vocabularyVersion,
			&r.status,
			&r.attempts,
			&r.createdAt,
			&r.claimedAt,
			&r.completedAt,
			&r.latencyMs,
			&r.costCents,
			&r.failureCode,
			&r.usageJSON,
			&r.obsJSON,
			&r.evalJSON,
		); err != nil {
			return nil, fmt.Errorf("scan vision run: %w", err)
		}
		rawRuns = append(rawRuns, r)

		// Collect color IDs only for V2 observations
		if (r.schemaVersion == vision.SchemaVersionV2 || r.schemaVersion == "2") && len(r.obsJSON) > 0 {
			var obs vision.ProductVisualObservationV2
			if err := json.Unmarshal(r.obsJSON, &obs); err == nil {
				allColorIDs = append(allColorIDs, obs.DominantColorIDs...)
				allColorIDs = append(allColorIDs, obs.SecondaryColorIDs...)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate vision runs: %w", err)
	}

	resolvedColors, err := s.resolveColors(ctx, allColorIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve colors: %w", err)
	}

	for _, r := range rawRuns {
		isCurrent := (currentRunID != nil && r.id == *currentRunID)

		dto := AdminProductVisionRunDTO{
			ID:                   r.id,
			ProductID:            r.productID,
			VisionContentVersion: r.visionContentVersion,
			Status:               r.status,
			IsCurrent:            isCurrent,
			Provider:             r.provider,
			ModelID:              r.modelID,
			PromptVersion:        r.promptVersion,
			SchemaVersion:        r.schemaVersion,
			VocabularyVersion:    r.vocabularyVersion,
			Attempts:             r.attempts,
			CreatedAt:            r.createdAt,
			ClaimedAt:            r.claimedAt,
			CompletedAt:          r.completedAt,
			LatencyMs:            r.latencyMs,
			CostCents:            r.costCents,
			FailureCode:          r.failureCode,
		}

		// Usage metadata (promptTokens, completionTokens, totalTokens)
		if len(r.usageJSON) > 0 {
			var usage AdminVisionUsageDTO
			if err := json.Unmarshal(r.usageJSON, &usage); err == nil {
				dto.Usage = &usage
			}
		}

		// Observation and Moderation (V2 vs V1 safety)
		// For V2: decode observation and moderation
		// For V1: do NOT decode as V2, leave observation and moderation nil
		if (r.schemaVersion == vision.SchemaVersionV2 || r.schemaVersion == "2") && len(r.obsJSON) > 0 {
			var obs vision.ProductVisualObservationV2
			if err := json.Unmarshal(r.obsJSON, &obs); err == nil {
				obsDTO := &AdminVisionObservationSummaryDTO{
					ObservedCategoryText: obs.ObservedCategoryText,
					DominantColors:       []AdminVisionColorDTO{},
					SecondaryColors:      []AdminVisionColorDTO{},
					Silhouette:           obs.SilhouetteID,
					FitAppearance:        obs.FitAppearanceID,
					Length:               obs.LengthID,
					Sleeves:              obs.SleeveIDs,
					Neckline:             obs.NecklineID,
					Closure:              obs.ClosureID,
					Pattern:              obs.PatternID,
					VisibleTexture:       obs.VisibleTextureID,
					DecorativeDetails:    obs.DecorativeDetailIDs,
					StyleDescriptors:     obs.StyleDescriptorIDs,
					Formality:            obs.FormalityID,
					SeasonalCues:         obs.SeasonalCueIDs,
					ImageSetConsistency:  obs.ImageSetConsistency,
					OCRText:              obs.OCRText,
					Confidence:           obs.Confidence,
					Evidence:             obs.Evidence,
				}
				if obs.CategoryConsistency != "" {
					cStr := string(obs.CategoryConsistency)
					obsDTO.CategoryConsistency = &cStr
				}
				for _, cID := range obs.DominantColorIDs {
					if c, ok := resolvedColors[cID]; ok {
						obsDTO.DominantColors = append(obsDTO.DominantColors, c)
					}
				}
				for _, cID := range obs.SecondaryColorIDs {
					if c, ok := resolvedColors[cID]; ok {
						obsDTO.SecondaryColors = append(obsDTO.SecondaryColors, c)
					}
				}
				dto.Observation = obsDTO
			}
		}

		if len(r.evalJSON) > 0 {
			var eval vision.ModerationEvaluationV1
			if err := json.Unmarshal(r.evalJSON, &eval); err == nil && (eval.ModerationRecommendation != "" || len(eval.Contradictions) > 0 || len(eval.QualityIssues) > 0 || len(eval.OCRFindings) > 0) {
				dto.Moderation = &AdminVisionModerationDTO{
					Contradictions:       eval.Contradictions,
					QualityIssues:        eval.QualityIssues,
					OCRFindings:          eval.OCRFindings,
					Recommendation:       eval.ModerationRecommendation,
					RecommendationReason: eval.RecommendationReason,
				}
			}
		}

		items = append(items, dto)
	}

	return &AdminProductVisionRunsResponse{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}
