package personalization

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrEmbeddingNotFound          = errors.New("product embedding not found")
	ErrEmbeddingNilProductID      = errors.New("product embedding: product_id must not be nil")
	ErrEmbeddingEmptyProvider     = errors.New("product embedding: provider must not be empty")
	ErrEmbeddingEmptyModel        = errors.New("product embedding: model must not be empty")
	ErrEmbeddingInvalidDimensions = errors.New("product embedding: dimensions must be 1536")
	ErrEmbeddingInvalidSchemaVer  = errors.New("product embedding: input_schema_version must be > 0")
	ErrEmbeddingInvalidHash       = errors.New("product embedding: content_hash must be a 64-character lowercase hex string")
	ErrEmbeddingInvalidVectorLen  = errors.New("product embedding: embedding vector length must be 1536")
	ErrEmbeddingNonFiniteVector   = errors.New("product embedding: vector contains non-finite element (NaN or Inf)")
)

// ProductEmbeddingMetadata represents the persisted embedding state without the vector column.
type ProductEmbeddingMetadata struct {
	ProductID          uuid.UUID `json:"productId"`
	Provider           string    `json:"provider"`
	Model              string    `json:"model"`
	Dimensions         int       `json:"dimensions"`
	InputSchemaVersion int       `json:"inputSchemaVersion"`
	ContentHash        string    `json:"contentHash"`
	GeneratedAt        time.Time `json:"generatedAt"`
}

// ProductEmbedding represents the full persisted product embedding, including the 1536 float32 vector.
type ProductEmbedding struct {
	ProductID          uuid.UUID `json:"productId"`
	Provider           string    `json:"provider"`
	Model              string    `json:"model"`
	Dimensions         int       `json:"dimensions"`
	InputSchemaVersion int       `json:"inputSchemaVersion"`
	ContentHash        string    `json:"contentHash"`
	Embedding          []float32 `json:"embedding"`
	GeneratedAt        time.Time `json:"generatedAt"`
}

// UpsertProductEmbeddingParams defines the inputs for atomically persisting or updating an embedding.
type UpsertProductEmbeddingParams struct {
	ProductID          uuid.UUID `json:"productId"`
	Provider           string    `json:"provider"`
	Model              string    `json:"model"`
	Dimensions         int       `json:"dimensions"`
	InputSchemaVersion int       `json:"inputSchemaVersion"`
	ContentHash        string    `json:"contentHash"`
	Embedding          []float32 `json:"embedding"`
}

// GetProductEmbeddingMetadata retrieves embedding metadata without loading or decoding the vector column.
func (r *Repository) GetProductEmbeddingMetadata(ctx context.Context, productID uuid.UUID) (*ProductEmbeddingMetadata, error) {
	if productID == uuid.Nil {
		return nil, ErrEmbeddingNilProductID
	}

	query := `
		SELECT product_id, provider, model, dimensions, input_schema_version, content_hash, generated_at
		FROM product_embeddings
		WHERE product_id = $1
	`

	var meta ProductEmbeddingMetadata
	err := r.db.QueryRow(ctx, query, productID).Scan(
		&meta.ProductID,
		&meta.Provider,
		&meta.Model,
		&meta.Dimensions,
		&meta.InputSchemaVersion,
		&meta.ContentHash,
		&meta.GeneratedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEmbeddingNotFound
		}
		return nil, fmt.Errorf("failed to get product embedding metadata: %w", err)
	}

	return &meta, nil
}

// GetProductEmbedding retrieves the full product embedding record, including the decoded 1536 float32 vector.
func (r *Repository) GetProductEmbedding(ctx context.Context, productID uuid.UUID) (*ProductEmbedding, error) {
	if productID == uuid.Nil {
		return nil, ErrEmbeddingNilProductID
	}

	query := `
		SELECT product_id, provider, model, dimensions, input_schema_version, content_hash, (embedding)::text, generated_at
		FROM product_embeddings
		WHERE product_id = $1
	`

	var emb ProductEmbedding
	var vecText string
	err := r.db.QueryRow(ctx, query, productID).Scan(
		&emb.ProductID,
		&emb.Provider,
		&emb.Model,
		&emb.Dimensions,
		&emb.InputSchemaVersion,
		&emb.ContentHash,
		&vecText,
		&emb.GeneratedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEmbeddingNotFound
		}
		return nil, fmt.Errorf("failed to get product embedding: %w", err)
	}

	vec, err := parseVectorLiteral(vecText, emb.Dimensions)
	if err != nil {
		return nil, fmt.Errorf("failed to parse product embedding vector: %w", err)
	}
	emb.Embedding = vec

	return &emb, nil
}

// UpsertProductEmbedding validates inputs before SQL and atomically inserts or updates the embedding row.
func (r *Repository) UpsertProductEmbedding(ctx context.Context, params UpsertProductEmbeddingParams) error {
	if err := validateUpsertParams(params); err != nil {
		return err
	}

	vecLiteral := formatVectorLiteral(params.Embedding)

	query := `
		INSERT INTO product_embeddings (
			product_id,
			provider,
			model,
			dimensions,
			input_schema_version,
			content_hash,
			embedding,
			generated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7::vector, NOW())
		ON CONFLICT (product_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			model = EXCLUDED.model,
			dimensions = EXCLUDED.dimensions,
			input_schema_version = EXCLUDED.input_schema_version,
			content_hash = EXCLUDED.content_hash,
			embedding = EXCLUDED.embedding,
			generated_at = NOW()
	`

	_, err := r.db.Exec(
		ctx,
		query,
		params.ProductID,
		strings.TrimSpace(params.Provider),
		strings.TrimSpace(params.Model),
		params.Dimensions,
		params.InputSchemaVersion,
		params.ContentHash,
		vecLiteral,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert product embedding: %w", err)
	}

	return nil
}

func validateUpsertParams(params UpsertProductEmbeddingParams) error {
	if params.ProductID == uuid.Nil {
		return ErrEmbeddingNilProductID
	}
	if strings.TrimSpace(params.Provider) == "" {
		return ErrEmbeddingEmptyProvider
	}
	if strings.TrimSpace(params.Model) == "" {
		return ErrEmbeddingEmptyModel
	}
	if params.Dimensions != 1536 {
		return ErrEmbeddingInvalidDimensions
	}
	if params.InputSchemaVersion <= 0 {
		return ErrEmbeddingInvalidSchemaVer
	}
	if len(params.ContentHash) != 64 {
		return ErrEmbeddingInvalidHash
	}
	for i := 0; i < len(params.ContentHash); i++ {
		c := params.ContentHash[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return ErrEmbeddingInvalidHash
	}
	if len(params.Embedding) != 1536 {
		return ErrEmbeddingInvalidVectorLen
	}
	for i, v := range params.Embedding {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return fmt.Errorf("%w at index %d", ErrEmbeddingNonFiniteVector, i)
		}
	}
	return nil
}

func formatVectorLiteral(vec []float32) string {
	var b strings.Builder
	b.Grow(len(vec) * 10)
	b.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(v), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

func parseVectorLiteral(s string, expectedDim int) ([]float32, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, errors.New("malformed vector literal: missing brackets")
	}
	s = s[1 : len(s)-1]
	if s == "" {
		if expectedDim == 0 {
			return []float32{}, nil
		}
		return nil, errors.New("empty vector literal")
	}

	parts := strings.Split(s, ",")
	if expectedDim > 0 && len(parts) != expectedDim {
		return nil, fmt.Errorf("vector literal dimension mismatch: expected %d, got %d", expectedDim, len(parts))
	}

	res := make([]float32, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("malformed vector literal at index %d: %w", i, err)
		}
		f32 := float32(v)
		if math.IsNaN(float64(f32)) || math.IsInf(float64(f32), 0) {
			return nil, fmt.Errorf("non-finite vector element at index %d", i)
		}
		res[i] = f32
	}
	return res, nil
}
