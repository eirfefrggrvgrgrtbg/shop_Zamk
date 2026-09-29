package products_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func makeVector(dim int) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < dim; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("0.0")
	}
	sb.WriteString("]")
	return sb.String()
}

func TestEmbeddingsSchema(t *testing.T) {
	ctx := context.Background()

	// Safe test connection string: only zamk_test is allowed
	const testConnStr = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"
	pool, err := pgxpool.New(ctx, testConnStr)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}

	// 1. TEST DB SAFETY GUARD
	// Verify current database is exact zamk_test BEFORE any mutation
	var currentDB string
	if err := pool.QueryRow(ctx, "SELECT current_database();").Scan(&currentDB); err != nil {
		t.Fatalf("failed to query current_database: %v", err)
	}
	if currentDB != "zamk_test" {
		t.Fatalf("FATAL DB SAFETY VIOLATION: expected 'zamk_test', got %q", currentDB)
	}

	// Fixture UUIDs defined upfront
	sellerUserID := uuid.New()
	sellerID := uuid.New()
	catID := uuid.New()

	product1ID := uuid.New() // Valid embedding & cascade test
	product2ID := uuid.New() // Vector dimension mismatch test (dimensions=1536, vector=100)
	product3ID := uuid.New() // Metadata dimension CHECK test (dimensions=100, vector=1536)
	product4ID := uuid.New() // NOT NULL proof test
	product5ID := uuid.New() // Duplicate PK test

	allProductIDs := []uuid.UUID{product1ID, product2ID, product3ID, product4ID, product5ID}

	// Register scoped cleanup upfront before any INSERT
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

	// Seed prerequisite parent rows
	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, status, role, name, first_name) VALUES ($1, $2, 'hash', 'active', 'seller', 'Name', 'Name')`,
		sellerUserID, "test_seller_"+sellerUserID.String()+"@test.com")
	if err != nil {
		t.Fatalf("failed to insert prerequisite user: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, status) VALUES ($1, 'TestBrand', $2, 'active')`,
		sellerID, "test-brand-"+sellerID.String())
	if err != nil {
		t.Fatalf("failed to insert prerequisite seller: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO categories (id, name, slug) VALUES ($1, 'TestCategory', $2)`,
		catID, "test-cat-"+catID.String())
	if err != nil {
		t.Fatalf("failed to insert prerequisite category: %v", err)
	}

	// Insert all fixture products
	for _, pid := range allProductIDs {
		_, err = pool.Exec(ctx, `
			INSERT INTO products (id, seller_id, category_id, title, slug, status, source, price_cents, currency)
			VALUES ($1, $2, $3, 'Test Product', $4, 'published', 'api', 1000, 'RUB')
		`, pid, sellerID, catID, "test-prod-"+pid.String())
		if err != nil {
			t.Fatalf("failed to insert fixture product %s: %v", pid, err)
		}
	}

	validHash := strings.Repeat("a", 64)
	vec1536 := makeVector(1536)
	vec100 := makeVector(100)

	// Helper to insert into product_embeddings
	insertEmbedding := func(pID uuid.UUID, provider, model string, dim, schemaVer int, hash, vecStr string) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO product_embeddings (product_id, provider, model, dimensions, input_schema_version, content_hash, embedding)
			VALUES ($1, $2, $3, $4, $5, $6, $7::vector)
		`, pID, provider, model, dim, schemaVer, hash, vecStr)
		return err
	}

	// B. Accepts valid VECTOR(1536) on product1
	if err := insertEmbedding(product1ID, "openai", "text-embedding-3-small", 1536, 1, validHash, vec1536); err != nil {
		t.Fatalf("expected to accept valid embedding, got: %v", err)
	}

	// 2. REAL VECTOR DIMENSION PROOF
	// Separate fixture product (product2ID) without existing embedding.
	// metadata dimensions = 1536, but embedding vector has only 100 elements.
	// MUST be rejected specifically because column type is VECTOR(1536), NOT by CHECK or PK.
	err = insertEmbedding(product2ID, "openai", "text-embedding-3-small", 1536, 1, validHash, vec100)
	if err == nil {
		t.Fatalf("expected vector literal with 100 elements to be rejected for VECTOR(1536) column, got nil")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "22000" || !strings.Contains(err.Error(), "expected 1536 dimensions") {
		t.Fatalf("expected pgvector dimension error (SQLSTATE 22000, 'expected 1536 dimensions'), got: %v (code: %s)", err, pgErr.Code)
	}

	// METADATA DIMENSION CHECK PROOF
	// Separate fixture product (product3ID) without existing embedding.
	// metadata dimensions = 100, but actual vector = 1536 elements.
	// MUST be rejected by product_embeddings_dimensions_check.
	err = insertEmbedding(product3ID, "openai", "text-embedding-3-small", 100, 1, validHash, vec1536)
	if err == nil {
		t.Fatalf("expected metadata dimension=100 to be rejected by CHECK constraint, got nil")
	}
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || !strings.Contains(err.Error(), "product_embeddings_dimensions_check") {
		t.Fatalf("expected CHECK constraint violation (SQLSTATE 23514, 'product_embeddings_dimensions_check'), got: %v (code: %s)", err, pgErr.Code)
	}

	// D. DUPLICATE PK PROOF
	// product1ID already has an embedding row. Attempting to insert another must fail on PK.
	err = insertEmbedding(product1ID, "openai", "text-embedding-3-small", 1536, 1, validHash, vec1536)
	if err == nil {
		t.Fatalf("expected duplicate product_id to be rejected by PK, got nil")
	}
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || !strings.Contains(err.Error(), "product_embeddings_pkey") {
		t.Fatalf("expected unique violation on PK (SQLSTATE 23505, 'product_embeddings_pkey'), got: %v (code: %s)", err, pgErr.Code)
	}

	// E. NONEXISTENT PRODUCT_ID (FK) PROOF
	nonexistentID := uuid.New()
	err = insertEmbedding(nonexistentID, "openai", "text-embedding-3-small", 1536, 1, validHash, vec1536)
	if err == nil {
		t.Fatalf("expected nonexistent product_id to be rejected by FK, got nil")
	}
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" || !strings.Contains(err.Error(), "product_embeddings_product_id_fkey") {
		t.Fatalf("expected FK violation (SQLSTATE 23503, 'product_embeddings_product_id_fkey'), got: %v (code: %s)", err, pgErr.Code)
	}

	// 3. NOT NULL PROOF
	// Separate fixture product (product4ID) with NO existing embedding row.
	// Attempt INSERT with missing required metadata fields (provider, model, content_hash, embedding, etc.).
	// Must fail specifically on NOT NULL (SQLSTATE 23502), NOT on duplicate PK or other errors.
	_, err = pool.Exec(ctx, `INSERT INTO product_embeddings (product_id, provider) VALUES ($1, 'openai')`, product4ID)
	if err == nil {
		t.Fatalf("expected NOT NULL violation when omitting required metadata, got nil")
	}
	if !errors.As(err, &pgErr) || pgErr.Code != "23502" || !strings.Contains(err.Error(), "violates not-null constraint") {
		t.Fatalf("expected NOT NULL constraint violation (SQLSTATE 23502), got: %v (code: %s)", err, pgErr.Code)
	}

	// 4. CASCADE PROOF
	// Verify embedding exists for product1ID
	var embCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM product_embeddings WHERE product_id = $1`, product1ID).Scan(&embCount); err != nil {
		t.Fatalf("failed to query product_embeddings count: %v", err)
	}
	if embCount != 1 {
		t.Fatalf("expected 1 embedding row before cascade delete, got %d", embCount)
	}

	// Delete exact fixture product
	if _, err := pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, product1ID); err != nil {
		t.Fatalf("failed to delete fixture product1: %v", err)
	}

	// Verify embedding row was cascaded
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM product_embeddings WHERE product_id = $1`, product1ID).Scan(&embCount); err != nil {
		t.Fatalf("failed to query product_embeddings count after delete: %v", err)
	}
	if embCount != 0 {
		t.Fatalf("expected 0 embedding rows after cascade delete, got %d", embCount)
	}

	// 5. NO ANN INDEX PROOF
	// Verify product_embeddings has NO HNSW or IVFFlat index.
	// Primary key btree index is allowed and required.
	rows, err := pool.Query(ctx, `
		SELECT c.relname, am.amname
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indexrelid
		JOIN pg_class t ON t.oid = i.indrelid
		JOIN pg_am am ON am.oid = c.relam
		WHERE t.relname = 'product_embeddings';
	`)
	if err != nil {
		t.Fatalf("failed to query indexes on product_embeddings: %v", err)
	}
	defer rows.Close()

	var indexCount int
	for rows.Next() {
		var idxName, amName string
		if err := rows.Scan(&idxName, &amName); err != nil {
			t.Fatalf("failed to scan index row: %v", err)
		}
		indexCount++
		if amName == "hnsw" || amName == "ivfflat" {
			t.Fatalf("found forbidden ANN index %q with access method %q on product_embeddings", idxName, amName)
		}
		if amName != "btree" {
			t.Fatalf("unexpected index access method %q for index %q", amName, idxName)
		}
	}
	if rows.Err() != nil {
		t.Fatalf("error iterating index rows: %v", rows.Err())
	}
	if indexCount == 0 {
		t.Fatalf("expected at least PK btree index on product_embeddings, found none")
	}
}
