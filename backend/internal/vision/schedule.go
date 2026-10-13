package vision

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ScheduleCurrentProductVision reads the canonical state and taxonomy, and ensures a vision run exists.
func (s *Store) ScheduleCurrentProductVision(ctx context.Context, productID uuid.UUID) (*VisionRun, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin schedule tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Read Canonical Snapshot
	snap, err := s.ReadCurrentSnapshotTx(ctx, tx, productID)
	if err != nil {
		return nil, fmt.Errorf("read snapshot: %w", err)
	}

	// 2. Build Taxonomy Context
	var tc TaxonomyContext

	// Categories (deterministic ORDER BY)
	catRows, err := tx.Query(ctx, `SELECT id, name FROM categories WHERE is_active = true ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("read categories: %w", err)
	}
	defer catRows.Close()
	for catRows.Next() {
		var item TaxonomyItem
		if err := catRows.Scan(&item.ID, &item.Label); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		tc.Categories = append(tc.Categories, item)
	}

	// Dictionaries (e.g., Colors) (deterministic ORDER BY)
	dictRows, err := tx.Query(ctx, `SELECT id, name_ru FROM attribute_dictionary_values WHERE is_active = true ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("read dictionaries: %w", err)
	}
	defer dictRows.Close()
	for dictRows.Next() {
		var item TaxonomyItem
		if err := dictRows.Scan(&item.ID, &item.Label); err != nil {
			return nil, fmt.Errorf("scan dictionary: %w", err)
		}
		tc.Dictionaries = append(tc.Dictionaries, item)
	}

	tc.VocabularyRegistry = NewVocabularyRegistry()
	tc.PromptVersion = PromptVersion
	tc.SchemaVersion = SchemaVersion
	tc.VocabularyVersion = VocabularyVersion

	// 3. Build Canonical Run
	run := BuildCanonicalVisionRun(ScheduleVisionRunParams{
		Snapshot:        snap,
		TaxonomyContext: tc,
		Provider:        CanonicalProvider,
		ModelID:         CanonicalModelID,
	})

	// 4. Ensure Run (can be done inside or outside the tx, doing it outside is fine since idempotency key protects us)
	// We'll just execute it with s.EnsureRawVisionRun using a new conn or same pool
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit schedule tx: %w", err)
	}

	// Insert run
	insertedRun, _, err := s.EnsureRawVisionRun(ctx, run)
	if err != nil {
		return nil, fmt.Errorf("ensure raw vision run: %w", err)
	}

	return insertedRun, nil
}
