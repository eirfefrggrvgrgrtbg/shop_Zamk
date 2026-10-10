package vision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// EnsureVisionRun schedules or resolves a vision run from canonical parameters.
// All hashes and the idempotency key are derived internally and deterministically.
func (s *Store) EnsureVisionRun(ctx context.Context, params ScheduleVisionRunParams) (*VisionRun, bool, error) {
	run := BuildCanonicalVisionRun(params)
	return s.EnsureRawVisionRun(ctx, run)
}

// EnsureRawVisionRun persists a pre-built VisionRun while strictly validating
// that caller-provided hashes and idempotency keys match the canonical values.
// Rejects mismatched inputs with typed errors.
func (s *Store) EnsureRawVisionRun(ctx context.Context, run VisionRun) (*VisionRun, bool, error) {
	// 1. Verify snapshot hash integrity
	computedSnapHash := ComputeSnapshotHash(run.SnapshotJSON)
	if run.SnapshotHash != "" && run.SnapshotHash != computedSnapHash {
		return nil, false, ErrMismatchedSnapshotHash
	}
	run.SnapshotHash = computedSnapHash

	// 2. Verify taxonomy context hash integrity if context provided
	if run.TaxonomyContext != nil {
		computedTaxHash := ComputeTaxonomyContextHash(*run.TaxonomyContext)
		if run.TaxonomyContextHash != "" && run.TaxonomyContextHash != computedTaxHash {
			return nil, false, ErrMismatchedTaxonomyHash
		}
		run.TaxonomyContextHash = computedTaxHash
	}

	// 3. Verify idempotency key integrity
	computedKey := ComputeIdempotencyKey(run.Provider, run.ModelID, run.PromptVersion, run.SchemaVersion, run.TaxonomyContextHash, run.SnapshotHash)
	if run.IdempotencyKey != "" && run.IdempotencyKey != computedKey {
		return nil, false, ErrMismatchedIdempotencyKey
	}
	run.IdempotencyKey = computedKey

	snapBytes, err := json.Marshal(run.SnapshotJSON)
	if err != nil {
		return nil, false, fmt.Errorf("marshal snapshot_json: %w", err)
	}

	now := time.Now()
	query := `
		INSERT INTO product_vision_runs (
			id, product_id, product_revision_id, vision_content_version, provider, model_id, prompt_version, schema_version,
			vocabulary_version, taxonomy_context_hash, snapshot_hash, snapshot_json,
			idempotency_key, status, attempts, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 0, $15, $15
		)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id, product_id, product_revision_id, vision_content_version, provider, model_id, prompt_version, schema_version,
		          vocabulary_version, taxonomy_context_hash, snapshot_hash, snapshot_json,
		          idempotency_key, status, attempts, claimed_at, next_attempt_at, completed_at,
		          result_observation, result_evaluation, usage_metadata, latency_ms, cost_cents,
		          failure_code, error_message, created_at, updated_at
	`

	var r VisionRun
	var returnedSnapBytes []byte
	var obsBytes, evalBytes, usageBytes []byte

	err = s.pool.QueryRow(ctx, query,
		run.ID, run.ProductID, run.ProductRevisionID, run.VisionContentVersion, run.Provider, run.ModelID, run.PromptVersion, run.SchemaVersion,
		run.VocabularyVersion, run.TaxonomyContextHash, run.SnapshotHash, snapBytes,
		run.IdempotencyKey, RunStatusPending, now,
	).Scan(
		&r.ID, &r.ProductID, &r.ProductRevisionID, &r.VisionContentVersion, &r.Provider, &r.ModelID, &r.PromptVersion, &r.SchemaVersion,
		&r.VocabularyVersion, &r.TaxonomyContextHash, &r.SnapshotHash, &returnedSnapBytes,
		&r.IdempotencyKey, &r.Status, &r.Attempts, &r.ClaimedAt, &r.NextAttemptAt, &r.CompletedAt,
		&obsBytes, &evalBytes, &usageBytes, &r.LatencyMs, &r.CostCents,
		&r.FailureCode, &r.ErrorMessage, &r.CreatedAt, &r.UpdatedAt,
	)

	if err == nil {
		if len(returnedSnapBytes) > 0 {
			_ = json.Unmarshal(returnedSnapBytes, &r.SnapshotJSON)
		}
		return &r, true, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("insert vision run: %w", err)
	}

	// Conflict occurred: load existing row
	existingQuery := `
		SELECT id, product_id, product_revision_id, vision_content_version, provider, model_id, prompt_version, schema_version,
		       vocabulary_version, taxonomy_context_hash, snapshot_hash, snapshot_json,
		       idempotency_key, status, attempts, claimed_at, next_attempt_at, completed_at,
		       result_observation, result_evaluation, usage_metadata, latency_ms, cost_cents,
		       failure_code, error_message, created_at, updated_at
		FROM product_vision_runs
		WHERE idempotency_key = $1
	`
	err = s.pool.QueryRow(ctx, existingQuery, run.IdempotencyKey).Scan(
		&r.ID, &r.ProductID, &r.ProductRevisionID, &r.VisionContentVersion, &r.Provider, &r.ModelID, &r.PromptVersion, &r.SchemaVersion,
		&r.VocabularyVersion, &r.TaxonomyContextHash, &r.SnapshotHash, &returnedSnapBytes,
		&r.IdempotencyKey, &r.Status, &r.Attempts, &r.ClaimedAt, &r.NextAttemptAt, &r.CompletedAt,
		&obsBytes, &evalBytes, &usageBytes, &r.LatencyMs, &r.CostCents,
		&r.FailureCode, &r.ErrorMessage, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, false, fmt.Errorf("fetch existing vision run: %w", err)
	}

	if len(returnedSnapBytes) > 0 {
		_ = json.Unmarshal(returnedSnapBytes, &r.SnapshotJSON)
	}
	if len(obsBytes) > 0 {
		var obs ProductVisualObservationV1
		if err := json.Unmarshal(obsBytes, &obs); err == nil {
			r.ResultObservation = &obs
		}
	}
	if len(evalBytes) > 0 {
		var eval ModerationEvaluationV1
		if err := json.Unmarshal(evalBytes, &eval); err == nil {
			r.ResultEvaluation = &eval
		}
	}
	if len(usageBytes) > 0 {
		var usage UsageMetadata
		if err := json.Unmarshal(usageBytes, &usage); err == nil {
			r.UsageMetadata = &usage
		}
	}

	return &r, false, nil
}

// ClaimPendingRuns atomically claims eligible pending runs for workers.
// Only claims runs that are pending and have next_attempt_at <= now() or NULL.
func (s *Store) ClaimPendingRuns(ctx context.Context, limit int) ([]VisionRun, error) {
	now := time.Now()
	query := `
		UPDATE product_vision_runs
		SET status = $1, claimed_at = $2, attempts = attempts + 1, updated_at = $2
		WHERE id IN (
			SELECT id FROM product_vision_runs
			WHERE status = $3
			  AND (next_attempt_at IS NULL OR next_attempt_at <= $2)
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $4
		)
		RETURNING id, product_id, product_revision_id, vision_content_version, provider, model_id, prompt_version, schema_version,
		          vocabulary_version, taxonomy_context_hash, snapshot_hash, snapshot_json,
		          idempotency_key, status, attempts, claimed_at, next_attempt_at, completed_at,
		          created_at, updated_at
	`
	rows, err := s.pool.Query(ctx, query, RunStatusProcessing, now, RunStatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("claim pending runs: %w", err)
	}
	defer rows.Close()

	var runs []VisionRun
	for rows.Next() {
		var r VisionRun
		var snapBytes []byte
		if err := rows.Scan(
			&r.ID, &r.ProductID, &r.ProductRevisionID, &r.VisionContentVersion, &r.Provider, &r.ModelID, &r.PromptVersion, &r.SchemaVersion,
			&r.VocabularyVersion, &r.TaxonomyContextHash, &r.SnapshotHash, &snapBytes,
			&r.IdempotencyKey, &r.Status, &r.Attempts, &r.ClaimedAt, &r.NextAttemptAt, &r.CompletedAt,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan claimed run: %w", err)
		}
		if len(snapBytes) > 0 {
			_ = json.Unmarshal(snapBytes, &r.SnapshotJSON)
		}
		runs = append(runs, r)
	}
	return runs, nil
}

// SaveSuccess transitions a run from 'processing' to 'succeeded' and checks DB-authoritative
// current product snapshot within the same transaction.
// If and only if the run's snapshot matches the DB's current snapshot, the current visual profile is updated.
// An older run completing later cannot regress a newer profile due to deterministic run_created_at precedence.
func (s *Store) SaveSuccess(ctx context.Context, runID uuid.UUID, obs ProductVisualObservationV1, eval ModerationEvaluationV1, usage *UsageMetadata, latencyMs *int, costCents *int) error {
	obsBytes, err := json.Marshal(obs)
	if err != nil {
		return fmt.Errorf("marshal observation: %w", err)
	}
	evalBytes, err := json.Marshal(eval)
	if err != nil {
		return fmt.Errorf("marshal evaluation: %w", err)
	}
	var usageBytes []byte
	if usage != nil {
		usageBytes, err = json.Marshal(usage)
		if err != nil {
			return fmt.Errorf("marshal usage metadata: %w", err)
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Lock and check run status
	var productID uuid.UUID
	var modelID, snapshotHash, taxonomyContextHash, currentStatus string
	var runCreatedAt time.Time
	var runVisionContentVersion int64
	err = tx.QueryRow(ctx, `
		SELECT product_id, model_id, snapshot_hash, taxonomy_context_hash, status, created_at, vision_content_version
		FROM product_vision_runs
		WHERE id = $1
		FOR UPDATE
	`, runID).Scan(&productID, &modelID, &snapshotHash, &taxonomyContextHash, &currentStatus, &runCreatedAt, &runVisionContentVersion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRunNotFound
		}
		return fmt.Errorf("lock vision run: %w", err)
	}

	if currentStatus != RunStatusProcessing {
		return fmt.Errorf("%w: cannot transition run %s from %s to %s", ErrInvalidRunTransition, runID, currentStatus, RunStatusSucceeded)
	}

	// 2. DB-authoritative current snapshot read
	currentSnap, err := s.ReadCurrentSnapshotTx(ctx, tx, productID)
	if err == nil {
		currentHash := ComputeSnapshotHash(currentSnap)
		// Only advance pointer if run snapshot matches DB-authoritative current snapshot
		// AND run vision_content_version matches current product version
		if snapshotHash == currentHash && runVisionContentVersion == currentSnap.VisionContentVersion {
			upsertProfileQuery := `
				INSERT INTO product_current_visual_profiles (
					product_id, run_id, run_created_at, vision_content_version, model_id, snapshot_hash,
					taxonomy_context_hash, observation, updated_at
				) VALUES (
					$1, $2, $3, $4, $5, $6, $7, $8, $9
				)
				ON CONFLICT (product_id) DO UPDATE SET
					run_id = EXCLUDED.run_id,
					run_created_at = EXCLUDED.run_created_at,
					vision_content_version = EXCLUDED.vision_content_version,
					model_id = EXCLUDED.model_id,
					snapshot_hash = EXCLUDED.snapshot_hash,
					taxonomy_context_hash = EXCLUDED.taxonomy_context_hash,
					observation = EXCLUDED.observation,
					updated_at = EXCLUDED.updated_at
				WHERE EXCLUDED.vision_content_version > product_current_visual_profiles.vision_content_version
				   OR (EXCLUDED.vision_content_version = product_current_visual_profiles.vision_content_version AND (
				       EXCLUDED.run_created_at > product_current_visual_profiles.run_created_at
				       OR (EXCLUDED.run_created_at = product_current_visual_profiles.run_created_at AND EXCLUDED.run_id > product_current_visual_profiles.run_id)
				   ))
			`
			now := time.Now()
			if _, err := tx.Exec(ctx, upsertProfileQuery,
				productID, runID, runCreatedAt, runVisionContentVersion, modelID, snapshotHash,
				taxonomyContextHash, obsBytes, now,
			); err != nil {
				return fmt.Errorf("upsert current visual profile: %w", err)
			}
		}
	}

	// 3. Update the run to succeeded
	now := time.Now()
	updateRunQuery := `
		UPDATE product_vision_runs
		SET status = $1, result_observation = $2, result_evaluation = $3,
		    usage_metadata = $4, latency_ms = $5, cost_cents = $6,
		    completed_at = $7, updated_at = $7
		WHERE id = $8 AND status = $9
	`
	tag, err := tx.Exec(ctx, updateRunQuery,
		RunStatusSucceeded, obsBytes, evalBytes,
		usageBytes, latencyMs, costCents,
		now, runID, RunStatusProcessing,
	)
	if err != nil {
		return fmt.Errorf("update run to succeeded: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: run %s was not in processing status", ErrInvalidRunTransition, runID)
	}

	return tx.Commit(ctx)
}

// SaveInvalidOutput marks a run as permanently failed due to invalid/corrupt model output.
// Transition: processing -> invalid_output ONLY.
func (s *Store) SaveInvalidOutput(ctx context.Context, runID uuid.UUID, failureCode, errorMsg string) error {
	now := time.Now()
	query := `
		UPDATE product_vision_runs
		SET status = $1, failure_code = $2, error_message = $3, completed_at = $4, updated_at = $4
		WHERE id = $5 AND status = $6
	`
	tag, err := s.pool.Exec(ctx, query, RunStatusInvalidOutput, failureCode, errorMsg, now, runID, RunStatusProcessing)
	if err != nil {
		return fmt.Errorf("save invalid output: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var currentStatus string
		if scanErr := s.pool.QueryRow(ctx, "SELECT status FROM product_vision_runs WHERE id = $1", runID).Scan(&currentStatus); scanErr != nil {
			if errors.Is(scanErr, pgx.ErrNoRows) {
				return ErrRunNotFound
			}
			return fmt.Errorf("check run status: %w", scanErr)
		}
		return fmt.Errorf("%w: cannot transition run %s from %s to %s", ErrInvalidRunTransition, runID, currentStatus, RunStatusInvalidOutput)
	}
	return nil
}

// SaveProviderFailure marks a run as failed due to a provider/network error.
// Transition: processing -> provider_failed ONLY.
func (s *Store) SaveProviderFailure(ctx context.Context, runID uuid.UUID, failureCode, errorMsg string, usage *UsageMetadata, latencyMs *int) error {
	var usageBytes []byte
	var err error
	if usage != nil {
		usageBytes, err = json.Marshal(usage)
		if err != nil {
			return fmt.Errorf("marshal usage metadata: %w", err)
		}
	}
	now := time.Now()
	query := `
		UPDATE product_vision_runs
		SET status = $1, failure_code = $2, error_message = $3, usage_metadata = $4, latency_ms = $5,
		    completed_at = $6, updated_at = $6
		WHERE id = $7 AND status = $8
	`
	tag, err := s.pool.Exec(ctx, query, RunStatusProviderFailed, failureCode, errorMsg, usageBytes, latencyMs, now, runID, RunStatusProcessing)
	if err != nil {
		return fmt.Errorf("save provider failure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var currentStatus string
		if scanErr := s.pool.QueryRow(ctx, "SELECT status FROM product_vision_runs WHERE id = $1", runID).Scan(&currentStatus); scanErr != nil {
			if errors.Is(scanErr, pgx.ErrNoRows) {
				return ErrRunNotFound
			}
			return fmt.Errorf("check run status: %w", scanErr)
		}
		return fmt.Errorf("%w: cannot transition run %s from %s to %s", ErrInvalidRunTransition, runID, currentStatus, RunStatusProviderFailed)
	}
	return nil
}

// RequeueProviderFailure resets a provider-failed or currently-processing run back to 'pending'
// with a specified next_attempt_at timestamp, enforcing the max attempts policy.
// If attempts >= maxAttempts, transitions to 'processing_failed'.
func (s *Store) RequeueProviderFailure(ctx context.Context, runID uuid.UUID, nextAttemptAt time.Time, maxAttempts int, failureCode, errorMsg string) error {
	now := time.Now()
	query := `
		UPDATE product_vision_runs
		SET status = CASE WHEN attempts >= $2 THEN $3 ELSE $4 END,
		    next_attempt_at = CASE WHEN attempts >= $2 THEN NULL::timestamptz ELSE $5::timestamptz END,
		    failure_code = $6,
		    error_message = $7,
		    completed_at = CASE WHEN attempts >= $2 THEN $8::timestamptz ELSE NULL::timestamptz END,
		    updated_at = $8
		WHERE id = $1 AND status IN ($9, $10)
	`
	tag, err := s.pool.Exec(ctx, query,
		runID, maxAttempts, RunStatusProcessingFailed, RunStatusPending,
		nextAttemptAt, failureCode, errorMsg, now,
		RunStatusProcessing, RunStatusProviderFailed,
	)
	if err != nil {
		return fmt.Errorf("requeue provider failure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var currentStatus string
		if scanErr := s.pool.QueryRow(ctx, "SELECT status FROM product_vision_runs WHERE id = $1", runID).Scan(&currentStatus); scanErr != nil {
			if errors.Is(scanErr, pgx.ErrNoRows) {
				return ErrRunNotFound
			}
			return fmt.Errorf("check run status: %w", scanErr)
		}
		return fmt.Errorf("%w: cannot requeue run %s from status %s", ErrInvalidRunTransition, runID, currentStatus)
	}
	return nil
}

// ResetStaleRuns performs dead-worker recovery. Moves stale 'processing' runs back to 'pending'
// or permanently transitions to 'processing_failed' if maxAttempts is exceeded.
func (s *Store) ResetStaleRuns(ctx context.Context, staleThreshold time.Duration, maxAttempts int) (int, error) {
	cutoff := time.Now().Add(-staleThreshold)
	now := time.Now()

	// 1. Mark exhausted runs as processing_failed
	failQuery := `
		UPDATE product_vision_runs
		SET status = $1, error_message = $2, completed_at = $3, updated_at = $3
		WHERE status = $4 AND claimed_at < $5 AND attempts >= $6
	`
	if _, err := s.pool.Exec(ctx, failQuery, RunStatusProcessingFailed, "Worker processing timed out, max attempts exceeded", now, RunStatusProcessing, cutoff, maxAttempts); err != nil {
		return 0, fmt.Errorf("fail exhausted runs: %w", err)
	}

	// 2. Requeue recoverable stale runs to pending
	resetQuery := `
		UPDATE product_vision_runs
		SET status = $1, next_attempt_at = $2, updated_at = $2
		WHERE status = $3 AND claimed_at < $4 AND attempts < $5
	`
	cmd, err := s.pool.Exec(ctx, resetQuery, RunStatusPending, now, RunStatusProcessing, cutoff, maxAttempts)
	if err != nil {
		return 0, fmt.Errorf("requeue stale runs: %w", err)
	}

	return int(cmd.RowsAffected()), nil
}

// GetCurrentVisualProfile returns the current active profile for a product.
// Fails closed if stored JSON is corrupt.
func (s *Store) GetCurrentVisualProfile(ctx context.Context, productID uuid.UUID) (*CurrentVisualProfile, error) {
	query := `
		SELECT cp.product_id, cp.run_id, cp.run_created_at, cp.vision_content_version,
		       cp.model_id, cp.snapshot_hash, cp.taxonomy_context_hash, cp.observation, cp.updated_at
		FROM product_current_visual_profiles cp
		JOIN products p ON cp.product_id = p.id AND cp.vision_content_version = p.vision_content_version
		WHERE cp.product_id = $1
	`
	var p CurrentVisualProfile
	var obsBytes []byte
	err := s.pool.QueryRow(ctx, query, productID).Scan(
		&p.ProductID, &p.RunID, &p.RunCreatedAt, &p.VisionContentVersion,
		&p.ModelID, &p.SnapshotHash, &p.TaxonomyContextHash, &obsBytes, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query current visual profile: %w", err)
	}

	if err := json.Unmarshal(obsBytes, &p.Observation); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptStoredJSON, err)
	}
	return &p, nil
}

// ReadCurrentSnapshotTx reads the DB-authoritative current product snapshot identity within an existing transaction.
func (s *Store) ReadCurrentSnapshotTx(ctx context.Context, tx pgx.Tx, productID uuid.UUID) (ProductVisionSnapshot, error) {
	var snap ProductVisionSnapshot
	snap.ProductID = productID

	// 1. Product details & live revision
	var catID *uuid.UUID
	var catName *string
	err := tx.QueryRow(ctx, `
		SELECT p.title, COALESCE(p.description, ''), p.category_id, c.name, p.live_revision_id, p.vision_content_version
		FROM products p
		LEFT JOIN categories c ON p.category_id = c.id
		WHERE p.id = $1
		FOR UPDATE OF p
	`, productID).Scan(&snap.Title, &snap.Description, &catID, &catName, &snap.ProductRevisionID, &snap.VisionContentVersion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return snap, ErrProductNotFound
		}
		return snap, fmt.Errorf("read product for snapshot: %w", err)
	}

	if catID != nil {
		snap.DeclaredCategoryID = *catID
	}
	if catName != nil {
		snap.DeclaredCategory = *catName
	}

	// 2. Images
	imgRows, err := tx.Query(ctx, `
		SELECT id, COALESCE(object_key, ''), COALESCE(rendition_object_key, ''),
		       crop_x, crop_y, crop_width, crop_height, sort_order, is_main, color_id
		FROM product_images
		WHERE product_id = $1
		ORDER BY sort_order ASC, id ASC
		FOR SHARE OF product_images
	`, productID)
	if err != nil {
		return snap, fmt.Errorf("read images for snapshot: %w", err)
	}
	defer imgRows.Close()

	for imgRows.Next() {
		var img SnapshotImage
		if err := imgRows.Scan(&img.ImageID, &img.ObjectKey, &img.RenditionObjectKey,
			&img.CropX, &img.CropY, &img.CropWidth, &img.CropHeight, &img.SortOrder, &img.IsMain, &img.ColorID); err != nil {
			return snap, fmt.Errorf("scan image for snapshot: %w", err)
		}
		snap.Images = append(snap.Images, img)
	}

	// 3. Declared Colors from active variants
	colorRows, err := tx.Query(ctx, `
		SELECT DISTINCT color_id
		FROM product_variants
		WHERE product_id = $1 AND is_active = true AND color_id IS NOT NULL
		ORDER BY color_id ASC
	`, productID)
	if err != nil {
		return snap, fmt.Errorf("read colors for snapshot: %w", err)
	}
	defer colorRows.Close()

	for colorRows.Next() {
		var colID uuid.UUID
		if err := colorRows.Scan(&colID); err != nil {
			return snap, fmt.Errorf("scan color for snapshot: %w", err)
		}
		snap.DeclaredColors = append(snap.DeclaredColors, colID)
	}

	// 4. Declared Options from active variants
	optRows, err := tx.Query(ctx, `
		SELECT option_values
		FROM product_variants
		WHERE product_id = $1 AND is_active = true AND option_values IS NOT NULL
	`, productID)
	if err != nil {
		return snap, fmt.Errorf("read options for snapshot: %w", err)
	}
	defer optRows.Close()

	snap.DeclaredOptions = make(map[string]string)
	for optRows.Next() {
		var opts map[string]interface{}
		if err := optRows.Scan(&opts); err == nil && opts != nil {
			for k, v := range opts {
				if strVal, ok := v.(string); ok && strVal != "" {
					snap.DeclaredOptions[k] = strVal
				}
			}
		}
	}

	return snap, nil
}
