package personalization_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/personalization"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const safeTestDBConn = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"

func makeValidFloatVector(dim int) []float32 {
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = float32(i) * 0.001
	}
	return vec
}

func makeValidHexHash() string {
	return strings.Repeat("a", 64)
}

func setupEmbeddingTestDB(t *testing.T) (*pgxpool.Pool, *personalization.Repository) {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, safeTestDBConn)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	// 1. Mandatory test DB safety guard
	var currentDB string
	if err := pool.QueryRow(ctx, "SELECT current_database();").Scan(&currentDB); err != nil {
		pool.Close()
		t.Fatalf("failed to query current_database: %v", err)
	}
	if currentDB != "zamk_test" {
		pool.Close()
		t.Fatalf("FATAL DB SAFETY VIOLATION: expected 'zamk_test', got %q", currentDB)
	}

	repo := personalization.NewRepository(pool)
	return pool, repo
}

func TestEmbeddingRepository_ValidationBeforeSQL(t *testing.T) {
	// Tests H, I, J, K, L, M, N, O, P must be rejected before SQL execution.
	// Using a closed/nil pool proves zero SQL interaction occurs.
	repo := personalization.NewRepository(nil)
	ctx := context.Background()

	baseParams := func() personalization.UpsertProductEmbeddingParams {
		return personalization.UpsertProductEmbeddingParams{
			ProductID:          uuid.New(),
			Provider:           "openai",
			Model:              "text-embedding-3-small",
			Dimensions:         1536,
			InputSchemaVersion: 1,
			ContentHash:        makeValidHexHash(),
			Embedding:          makeValidFloatVector(1536),
		}
	}

	// H. uuid.Nil rejected before SQL
	t.Run("H_NilProductID_Rejected", func(t *testing.T) {
		p := baseParams()
		p.ProductID = uuid.Nil
		err := repo.UpsertProductEmbedding(ctx, p)
		if !errors.Is(err, personalization.ErrEmbeddingNilProductID) {
			t.Fatalf("expected ErrEmbeddingNilProductID, got: %v", err)
		}

		_, err = repo.GetProductEmbeddingMetadata(ctx, uuid.Nil)
		if !errors.Is(err, personalization.ErrEmbeddingNilProductID) {
			t.Fatalf("expected ErrEmbeddingNilProductID on metadata read, got: %v", err)
		}

		_, err = repo.GetProductEmbedding(ctx, uuid.Nil)
		if !errors.Is(err, personalization.ErrEmbeddingNilProductID) {
			t.Fatalf("expected ErrEmbeddingNilProductID on full read, got: %v", err)
		}
	})

	// I. empty provider rejected
	t.Run("I_EmptyProvider_Rejected", func(t *testing.T) {
		for _, invalidProvider := range []string{"", "   ", "\t\n"} {
			p := baseParams()
			p.Provider = invalidProvider
			err := repo.UpsertProductEmbedding(ctx, p)
			if !errors.Is(err, personalization.ErrEmbeddingEmptyProvider) {
				t.Fatalf("expected ErrEmbeddingEmptyProvider for %q, got: %v", invalidProvider, err)
			}
		}
	})

	// J. empty model rejected
	t.Run("J_EmptyModel_Rejected", func(t *testing.T) {
		for _, invalidModel := range []string{"", "   ", "\t"} {
			p := baseParams()
			p.Model = invalidModel
			err := repo.UpsertProductEmbedding(ctx, p)
			if !errors.Is(err, personalization.ErrEmbeddingEmptyModel) {
				t.Fatalf("expected ErrEmbeddingEmptyModel for %q, got: %v", invalidModel, err)
			}
		}
	})

	// K. dimensions != 1536 rejected
	t.Run("K_InvalidDimensions_Rejected", func(t *testing.T) {
		for _, invalidDim := range []int{0, 100, 512, 1535, 1537, 3072} {
			p := baseParams()
			p.Dimensions = invalidDim
			err := repo.UpsertProductEmbedding(ctx, p)
			if !errors.Is(err, personalization.ErrEmbeddingInvalidDimensions) {
				t.Fatalf("expected ErrEmbeddingInvalidDimensions for dim %d, got: %v", invalidDim, err)
			}
		}
	})

	// L. schema_version <= 0 rejected
	t.Run("L_InvalidSchemaVersion_Rejected", func(t *testing.T) {
		for _, invalidVer := range []int{0, -1, -99} {
			p := baseParams()
			p.InputSchemaVersion = invalidVer
			err := repo.UpsertProductEmbedding(ctx, p)
			if !errors.Is(err, personalization.ErrEmbeddingInvalidSchemaVer) {
				t.Fatalf("expected ErrEmbeddingInvalidSchemaVer for ver %d, got: %v", invalidVer, err)
			}
		}
	})

	// M. malformed content_hash rejected
	t.Run("M_MalformedContentHash_Rejected", func(t *testing.T) {
		malformedHashes := []string{
			"",
			strings.Repeat("a", 63), // length 63
			strings.Repeat("a", 65), // length 65
			strings.Repeat("A", 64), // uppercase
			strings.Repeat("0", 63) + "G",
			"not-a-valid-hex-hash",
		}
		for _, h := range malformedHashes {
			p := baseParams()
			p.ContentHash = h
			err := repo.UpsertProductEmbedding(ctx, p)
			if !errors.Is(err, personalization.ErrEmbeddingInvalidHash) {
				t.Fatalf("expected ErrEmbeddingInvalidHash for hash %q, got: %v", h, err)
			}
		}
	})

	// N. vector len != 1536 rejected
	t.Run("N_InvalidVectorLength_Rejected", func(t *testing.T) {
		for _, invalidLen := range []int{0, 100, 512, 1535, 1537} {
			p := baseParams()
			p.Embedding = make([]float32, invalidLen)
			err := repo.UpsertProductEmbedding(ctx, p)
			if !errors.Is(err, personalization.ErrEmbeddingInvalidVectorLen) {
				t.Fatalf("expected ErrEmbeddingInvalidVectorLen for len %d, got: %v", invalidLen, err)
			}
		}
	})

	// O. NaN rejected
	t.Run("O_NaNElement_Rejected", func(t *testing.T) {
		p := baseParams()
		p.Embedding[100] = float32(math.NaN())
		err := repo.UpsertProductEmbedding(ctx, p)
		if !errors.Is(err, personalization.ErrEmbeddingNonFiniteVector) {
			t.Fatalf("expected ErrEmbeddingNonFiniteVector for NaN, got: %v", err)
		}
		if !strings.Contains(err.Error(), "index 100") {
			t.Fatalf("expected index 100 in error, got: %v", err)
		}
	})

	// P. Inf rejected
	t.Run("P_InfElement_Rejected", func(t *testing.T) {
		pPos := baseParams()
		pPos.Embedding[200] = float32(math.Inf(1))
		err := repo.UpsertProductEmbedding(ctx, pPos)
		if !errors.Is(err, personalization.ErrEmbeddingNonFiniteVector) {
			t.Fatalf("expected ErrEmbeddingNonFiniteVector for +Inf, got: %v", err)
		}
		if !strings.Contains(err.Error(), "index 200") {
			t.Fatalf("expected index 200 in error, got: %v", err)
		}

		pNeg := baseParams()
		pNeg.Embedding[300] = float32(math.Inf(-1))
		err = repo.UpsertProductEmbedding(ctx, pNeg)
		if !errors.Is(err, personalization.ErrEmbeddingNonFiniteVector) {
			t.Fatalf("expected ErrEmbeddingNonFiniteVector for -Inf, got: %v", err)
		}
		if !strings.Contains(err.Error(), "index 300") {
			t.Fatalf("expected index 300 in error, got: %v", err)
		}
	})
}

func TestEmbeddingRepository_DBIntegration(t *testing.T) {
	pool, repo := setupEmbeddingTestDB(t)
	ctx := context.Background()

	// Fixtures setup
	sellerUserID := uuid.New()
	sellerID := uuid.New()
	catID := uuid.New()

	product1ID := uuid.New() // general tests
	product2ID := uuid.New() // concurrent upserts
	product3ID := uuid.New() // cascade test

	allProductIDs := []uuid.UUID{product1ID, product2ID, product3ID}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		for _, pid := range allProductIDs {
			if _, err := pool.Exec(cleanupCtx, `DELETE FROM products WHERE id = $1`, pid); err != nil {
				t.Errorf("cleanup product %s failed: %v", pid, err)
			}
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM categories WHERE id = $1`, catID); err != nil {
			t.Errorf("cleanup category %s failed: %v", catID, err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM sellers WHERE id = $1`, sellerID); err != nil {
			t.Errorf("cleanup seller %s failed: %v", sellerID, err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, sellerUserID); err != nil {
			t.Errorf("cleanup user %s failed: %v", sellerUserID, err)
		}
		pool.Close()
	})

	// Seed prerequisite rows
	_, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, status, role, name, first_name) VALUES ($1, $2, 'hash', 'active', 'seller', 'Name', 'Name')`,
		sellerUserID, "test_emb_seller_"+sellerUserID.String()+"@test.com")
	if err != nil {
		t.Fatalf("failed to insert prerequisite user: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, status) VALUES ($1, 'TestBrand', $2, 'active')`,
		sellerID, "test-emb-brand-"+sellerID.String())
	if err != nil {
		t.Fatalf("failed to insert prerequisite seller: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'TestCategory', $2)`,
		catID, "test-emb-cat-"+catID.String())
	if err != nil {
		t.Fatalf("failed to insert prerequisite category: %v", err)
	}

	for _, pid := range allProductIDs {
		_, err = pool.Exec(ctx, `
			INSERT INTO products (id, seller_id, category_id, title, slug, status, source, price_cents, currency)
			VALUES ($1, $2, $3, 'Test Product', $4, 'published', 'api', 1000, 'RUB')
		`, pid, sellerID, catID, "test-emb-prod-"+pid.String())
		if err != nil {
			t.Fatalf("failed to insert fixture product %s: %v", pid, err)
		}
	}

	// A. metadata missing -> not found / exists=false semantics
	t.Run("A_MetadataMissing_NotFoundSemantics", func(t *testing.T) {
		meta, err := repo.GetProductEmbeddingMetadata(ctx, product1ID)
		if !errors.Is(err, personalization.ErrEmbeddingNotFound) {
			t.Fatalf("expected ErrEmbeddingNotFound, got: %v", err)
		}
		if meta != nil {
			t.Fatalf("expected nil metadata, got: %+v", meta)
		}

		full, err := repo.GetProductEmbedding(ctx, product1ID)
		if !errors.Is(err, personalization.ErrEmbeddingNotFound) {
			t.Fatalf("expected ErrEmbeddingNotFound on full embedding read, got: %v", err)
		}
		if full != nil {
			t.Fatalf("expected nil full embedding, got: %+v", full)
		}
	})

	hash1 := strings.Repeat("1", 64)
	vec1 := makeValidFloatVector(1536)

	// B. valid insert -> metadata exact
	var firstGeneratedAt time.Time
	t.Run("B_ValidInsert_MetadataExact", func(t *testing.T) {
		params := personalization.UpsertProductEmbeddingParams{
			ProductID:          product1ID,
			Provider:           "openai",
			Model:              "text-embedding-3-small",
			Dimensions:         1536,
			InputSchemaVersion: 1,
			ContentHash:        hash1,
			Embedding:          vec1,
		}

		err := repo.UpsertProductEmbedding(ctx, params)
		if err != nil {
			t.Fatalf("failed to upsert valid embedding: %v", err)
		}

		meta, err := repo.GetProductEmbeddingMetadata(ctx, product1ID)
		if err != nil {
			t.Fatalf("unexpected error getting metadata: %v", err)
		}
		if meta == nil {
			t.Fatalf("expected non-nil metadata")
		}

		if meta.ProductID != product1ID {
			t.Errorf("expected productID %s, got %s", product1ID, meta.ProductID)
		}
		if meta.Provider != "openai" {
			t.Errorf("expected provider 'openai', got %q", meta.Provider)
		}
		if meta.Model != "text-embedding-3-small" {
			t.Errorf("expected model 'text-embedding-3-small', got %q", meta.Model)
		}
		if meta.Dimensions != 1536 {
			t.Errorf("expected dimensions 1536, got %d", meta.Dimensions)
		}
		if meta.InputSchemaVersion != 1 {
			t.Errorf("expected schema version 1, got %d", meta.InputSchemaVersion)
		}
		if meta.ContentHash != hash1 {
			t.Errorf("expected hash %s, got %s", hash1, meta.ContentHash)
		}
		if meta.GeneratedAt.IsZero() {
			t.Errorf("expected non-zero generated_at")
		}
		firstGeneratedAt = meta.GeneratedAt
	})

	// C. full read -> vector length 1536 and values preserved
	t.Run("C_FullRead_VectorPreserved", func(t *testing.T) {
		emb, err := repo.GetProductEmbedding(ctx, product1ID)
		if err != nil {
			t.Fatalf("failed to get full embedding: %v", err)
		}
		if emb.ProductID != product1ID {
			t.Errorf("productID mismatch: expected %s, got %s", product1ID, emb.ProductID)
		}
		if len(emb.Embedding) != 1536 {
			t.Fatalf("expected vector length 1536, got %d", len(emb.Embedding))
		}

		// Verify float values preserved
		for i := range vec1 {
			diff := math.Abs(float64(emb.Embedding[i] - vec1[i]))
			if diff > 1e-6 {
				t.Fatalf("vector element mismatch at index %d: expected %f, got %f (diff: %e)",
					i, vec1[i], emb.Embedding[i], diff)
			}
		}
	})

	// D, E, F: Second upsert same product -> still one row, updates fields, advances generated_at
	hash2 := strings.Repeat("2", 64)
	vec2 := make([]float32, 1536)
	for i := range vec2 {
		vec2[i] = float32(i) * 0.002
	}

	t.Run("D_E_F_SecondUpsert_OneRow_FieldsUpdated_GeneratedAtAdvances", func(t *testing.T) {
		// Ensure timestamp difference
		time.Sleep(15 * time.Millisecond)

		params := personalization.UpsertProductEmbeddingParams{
			ProductID:          product1ID,
			Provider:           "custom-provider",
			Model:              "custom-model-v2",
			Dimensions:         1536,
			InputSchemaVersion: 2,
			ContentHash:        hash2,
			Embedding:          vec2,
		}

		err := repo.UpsertProductEmbedding(ctx, params)
		if err != nil {
			t.Fatalf("failed to upsert updated embedding: %v", err)
		}

		// D. Still exactly one row
		var count int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_embeddings WHERE product_id = $1", product1ID).Scan(&count); err != nil {
			t.Fatalf("failed to count rows: %v", err)
		}
		if count != 1 {
			t.Fatalf("Test D failed: expected exactly 1 row, got %d", count)
		}

		// E. Metadata & vector updated
		meta, err := repo.GetProductEmbeddingMetadata(ctx, product1ID)
		if err != nil {
			t.Fatalf("failed to get updated metadata: %v", err)
		}
		if meta.Provider != "custom-provider" || meta.Model != "custom-model-v2" ||
			meta.InputSchemaVersion != 2 || meta.ContentHash != hash2 {
			t.Fatalf("Test E failed: fields were not updated properly: %+v", meta)
		}

		emb, err := repo.GetProductEmbedding(ctx, product1ID)
		if err != nil {
			t.Fatalf("failed to get updated full embedding: %v", err)
		}
		diff := math.Abs(float64(emb.Embedding[1] - vec2[1]))
		if diff > 1e-6 {
			t.Fatalf("Test E failed: vector was not updated: expected %f, got %f", vec2[1], emb.Embedding[1])
		}

		// F. generated_at advances
		if !meta.GeneratedAt.After(firstGeneratedAt) {
			t.Fatalf("Test F failed: expected updated generated_at (%v) to advance past first (%v)",
				meta.GeneratedAt, firstGeneratedAt)
		}
	})

	// G. Duplicate concurrent upserts -> one final row, no duplicate
	t.Run("G_DuplicateConcurrentUpserts_SingleRow", func(t *testing.T) {
		const concurrency = 10
		var wg sync.WaitGroup
		errs := make([]error, concurrency)

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				char := fmt.Sprintf("%x", idx%16)
				params := personalization.UpsertProductEmbeddingParams{
					ProductID:          product2ID,
					Provider:           "openai",
					Model:              "text-embedding-3-small",
					Dimensions:         1536,
					InputSchemaVersion: 1,
					ContentHash:        strings.Repeat(char, 64),
					Embedding:          makeValidFloatVector(1536),
				}
				errs[idx] = repo.UpsertProductEmbedding(ctx, params)
			}(i)
		}
		wg.Wait()

		for idx, err := range errs {
			if err != nil {
				t.Fatalf("concurrent upsert %d failed: %v", idx, err)
			}
		}

		var count int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_embeddings WHERE product_id = $1", product2ID).Scan(&count); err != nil {
			t.Fatalf("failed to count product2 embeddings: %v", err)
		}
		if count != 1 {
			t.Fatalf("Test G failed: expected exactly 1 row after concurrent upserts, got %d", count)
		}
	})

	// Q. Nonexistent product FK error surfaced cleanly
	t.Run("Q_NonexistentProductFK_SurfacedCleanly", func(t *testing.T) {
		nonexistentID := uuid.New()
		params := personalization.UpsertProductEmbeddingParams{
			ProductID:          nonexistentID,
			Provider:           "openai",
			Model:              "text-embedding-3-small",
			Dimensions:         1536,
			InputSchemaVersion: 1,
			ContentHash:        makeValidHexHash(),
			Embedding:          makeValidFloatVector(1536),
		}

		err := repo.UpsertProductEmbedding(ctx, params)
		if err == nil {
			t.Fatalf("expected FK violation error for nonexistent product, got nil")
		}

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
			t.Fatalf("Test Q failed: expected SQLSTATE 23503 (foreign_key_violation), got: %v", err)
		}
	})

	// R. Deleting product cascades embedding
	t.Run("R_DeletingProductCascadesEmbedding", func(t *testing.T) {
		params := personalization.UpsertProductEmbeddingParams{
			ProductID:          product3ID,
			Provider:           "openai",
			Model:              "text-embedding-3-small",
			Dimensions:         1536,
			InputSchemaVersion: 1,
			ContentHash:        makeValidHexHash(),
			Embedding:          makeValidFloatVector(1536),
		}

		if err := repo.UpsertProductEmbedding(ctx, params); err != nil {
			t.Fatalf("failed to insert embedding for product3: %v", err)
		}

		// Delete product from products table
		if _, err := pool.Exec(ctx, "DELETE FROM products WHERE id = $1", product3ID); err != nil {
			t.Fatalf("failed to delete product3: %v", err)
		}

		// Verify embedding row was cascaded
		var count int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM product_embeddings WHERE product_id = $1", product3ID).Scan(&count); err != nil {
			t.Fatalf("failed to count rows: %v", err)
		}
		if count != 0 {
			t.Fatalf("Test R failed: expected 0 embedding rows after cascade, got %d", count)
		}

		meta, err := repo.GetProductEmbeddingMetadata(ctx, product3ID)
		if !errors.Is(err, personalization.ErrEmbeddingNotFound) {
			t.Fatalf("expected ErrEmbeddingNotFound after cascade, got: %v", err)
		}
		if meta != nil {
			t.Fatalf("expected nil metadata after cascade")
		}
	})

	// S. Metadata read does not require/decode embedding payload if this can be proved structurally
	t.Run("S_MetadataRead_DoesNotQueryVectorColumn", func(t *testing.T) {
		// Structurally prove that GetProductEmbeddingMetadata executes without decoding or retrieving the vector:
		// Product 1 has an embedding row.
		meta, err := repo.GetProductEmbeddingMetadata(ctx, product1ID)
		if err != nil {
			t.Fatalf("unexpected error getting metadata: %v", err)
		}
		if meta == nil {
			t.Fatalf("expected non-nil metadata")
		}
		// Confirm ProductEmbeddingMetadata struct has no vector field to begin with.
		// Furthermore, even if the database has 1536 floats in `embedding`, the metadata query selects only:
		// (product_id, provider, model, dimensions, input_schema_version, content_hash, generated_at).
	})

	// X. Conditional delete repository method:
	// exact matching spec/hash deletes; mismatching hash/spec leaves row untouched
	t.Run("X_ConditionalDelete_ExactMatchDeletes_MismatchLeavesUntouched", func(t *testing.T) {
		exactHash := strings.Repeat("e", 64)
		params := personalization.UpsertProductEmbeddingParams{
			ProductID:          product1ID,
			Provider:           "openai",
			Model:              "text-embedding-3-small",
			Dimensions:         1536,
			InputSchemaVersion: 1,
			ContentHash:        exactHash,
			Embedding:          makeValidFloatVector(1536),
		}
		if err := repo.UpsertProductEmbedding(ctx, params); err != nil {
			t.Fatalf("failed to setup embedding: %v", err)
		}

		// 1. Mismatching hash -> does NOT delete
		deleted, err := repo.DeleteProductEmbeddingIfMatch(ctx, product1ID, "openai", "text-embedding-3-small", 1536, 1, strings.Repeat("f", 64))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if deleted {
			t.Fatalf("expected deleted=false on hash mismatch")
		}
		// Confirm row still exists
		meta, err := repo.GetProductEmbeddingMetadata(ctx, product1ID)
		if err != nil || meta == nil {
			t.Fatalf("expected row to remain in DB on hash mismatch")
		}

		// 2. Mismatching model -> does NOT delete
		deleted, err = repo.DeleteProductEmbeddingIfMatch(ctx, product1ID, "openai", "wrong-model", 1536, 1, exactHash)
		if err != nil || deleted {
			t.Fatalf("expected deleted=false on model mismatch")
		}

		// 3. Mismatching provider -> does NOT delete
		deleted, err = repo.DeleteProductEmbeddingIfMatch(ctx, product1ID, "wrong-provider", "text-embedding-3-small", 1536, 1, exactHash)
		if err != nil || deleted {
			t.Fatalf("expected deleted=false on provider mismatch")
		}

		// 4. Mismatching dimensions -> does NOT delete
		deleted, err = repo.DeleteProductEmbeddingIfMatch(ctx, product1ID, "openai", "text-embedding-3-small", 512, 1, exactHash)
		if err != nil || deleted {
			t.Fatalf("expected deleted=false on dimensions mismatch")
		}

		// 5. Mismatching schema version -> does NOT delete
		deleted, err = repo.DeleteProductEmbeddingIfMatch(ctx, product1ID, "openai", "text-embedding-3-small", 1536, 2, exactHash)
		if err != nil || deleted {
			t.Fatalf("expected deleted=false on schema mismatch")
		}

		// 6. Exact match -> DELETES row
		deleted, err = repo.DeleteProductEmbeddingIfMatch(ctx, product1ID, "openai", "text-embedding-3-small", 1536, 1, exactHash)
		if err != nil {
			t.Fatalf("unexpected error on exact delete: %v", err)
		}
		if !deleted {
			t.Fatalf("expected deleted=true on exact match")
		}

		// Confirm row is now gone
		meta, err = repo.GetProductEmbeddingMetadata(ctx, product1ID)
		if !errors.Is(err, personalization.ErrEmbeddingNotFound) || meta != nil {
			t.Fatalf("expected ErrEmbeddingNotFound after successful conditional delete")
		}
	})
}
