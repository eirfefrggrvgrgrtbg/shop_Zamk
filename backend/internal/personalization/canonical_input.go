package personalization

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	CanonicalEmbeddingSchemaVersion = 1
)

var (
	ErrProductNotFound = errors.New("product not found")
)

// CanonicalEmbeddingInput is the internal contract for a generated canonical product representation.
type CanonicalEmbeddingInput struct {
	SchemaVersion int    `json:"schemaVersion"`
	Text          string `json:"text"`
	ContentHash   string `json:"contentHash"`
}

// CanonicalAttribute represents a single attribute key-value pair.
type CanonicalAttribute struct {
	Key   string `json:"key"`   // attribute_definitions.code
	Value string `json:"value"` // canonical semantic human-readable value
}

// CanonicalProductInput captures all semantic catalog data needed to construct the canonical embedding text.
// Internal IDs, seller data, stock, prices, and variant sizing options are excluded.
type CanonicalProductInput struct {
	Title        string               `json:"title"`
	CategoryPath string               `json:"categoryPath"`
	Brand        *string              `json:"brand,omitempty"`
	Gender       *string              `json:"gender,omitempty"`
	Color        *string              `json:"color,omitempty"`
	Material     *string              `json:"material,omitempty"`
	Attributes   []CanonicalAttribute `json:"attributes,omitempty"`
	Description  *string              `json:"description,omitempty"`
}

// CategoryHierarchyNode represents a category node used to reconstruct hierarchy paths.
type CategoryHierarchyNode struct {
	ID       uuid.UUID  `json:"id"`
	ParentID *uuid.UUID `json:"parentId,omitempty"`
	Name     string     `json:"name"`
}

// escapeCanonicalValue normalizes and escapes field values for unambiguous serialization.
// Rules:
// 1. Trim leading and trailing whitespace.
// 2. Normalize newlines: CRLF (\r\n) and CR (\r) -> LF (\n).
// 3. Escape backslashes: \ -> \\
// 4. Encode embedded newlines: \n -> \n (literal backslash followed by 'n').
//
// This guarantees that the canonical text uses literal LF (0x0A) strictly as the structural
// delimiter between fields, preventing any field value from injecting structural lines.
func escapeCanonicalValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// cleanCanonicalKey trims and sanitizes attribute semantic keys.
func cleanCanonicalKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "=", "")
	return s
}

// ComputeContentHash computes the SHA-256 hash of the exact canonical UTF-8 text bytes,
// returning a lowercase hexadecimal string of exactly 64 characters.
func ComputeContentHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// FormatCanonicalEmbeddingText builds the deterministic canonical embedding string v1.
//
// Canonical logical order:
//
//	schema_version=1
//	title=...
//	category_path=...
//	brand=...
//	gender=...
//	color=...
//	material=...
//	attribute.<semantic_key>=...
//	description=...
//
// Rules:
// - Fixed newline separator \n (0x0A) between structural lines
// - Values have embedded newlines escaped as literal \n, and backslashes escaped as \\
// - Leading and trailing whitespace in values are trimmed
// - Empty or nil optional fields are omitted completely (no literal "null")
// - Attributes are sorted deterministically by semantic_key ASC, then by value ASC
// - Exact duplicate attribute (key, val) pairs are deduplicated
// - Internal IDs, prices, sellers, stock, and variant sizing are excluded
func FormatCanonicalEmbeddingText(input CanonicalProductInput) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("schema_version=%d", CanonicalEmbeddingSchemaVersion))

	if v := escapeCanonicalValue(input.Title); v != "" {
		lines = append(lines, "title="+v)
	}

	if v := escapeCanonicalValue(input.CategoryPath); v != "" {
		lines = append(lines, "category_path="+v)
	}

	if input.Brand != nil {
		if v := escapeCanonicalValue(*input.Brand); v != "" {
			lines = append(lines, "brand="+v)
		}
	}

	if input.Gender != nil {
		if v := escapeCanonicalValue(*input.Gender); v != "" {
			lines = append(lines, "gender="+v)
		}
	}

	if input.Color != nil {
		if v := escapeCanonicalValue(*input.Color); v != "" {
			lines = append(lines, "color="+v)
		}
	}

	if input.Material != nil {
		if v := escapeCanonicalValue(*input.Material); v != "" {
			lines = append(lines, "material="+v)
		}
	}

	if len(input.Attributes) > 0 {
		type attrItem struct {
			key string
			val string
		}
		var valid []attrItem
		for _, a := range input.Attributes {
			k := cleanCanonicalKey(a.Key)
			v := escapeCanonicalValue(a.Value)
			if k != "" && v != "" {
				valid = append(valid, attrItem{key: k, val: v})
			}
		}

		sort.Slice(valid, func(i, j int) bool {
			if valid[i].key != valid[j].key {
				return valid[i].key < valid[j].key
			}
			return valid[i].val < valid[j].val
		})

		// Deduplicate exact (key, val) duplicate pairs
		var deduped []attrItem
		for i, a := range valid {
			if i > 0 && a.key == valid[i-1].key && a.val == valid[i-1].val {
				continue
			}
			deduped = append(deduped, a)
		}

		for _, a := range deduped {
			lines = append(lines, "attribute."+a.key+"="+a.val)
		}
	}

	if input.Description != nil {
		if v := escapeCanonicalValue(*input.Description); v != "" {
			lines = append(lines, "description="+v)
		}
	}

	return strings.Join(lines, "\n")
}

// BuildCanonicalEmbeddingInput constructs the canonical embedding text and its SHA-256 content hash.
func BuildCanonicalEmbeddingInput(input CanonicalProductInput) CanonicalEmbeddingInput {
	text := FormatCanonicalEmbeddingText(input)
	hash := ComputeContentHash(text)
	return CanonicalEmbeddingInput{
		SchemaVersion: CanonicalEmbeddingSchemaVersion,
		Text:          text,
		ContentHash:   hash,
	}
}

// BuildCategoryPath constructs a full root-to-leaf hierarchy breadcrumb path (e.g., "Root > Middle > Leaf").
// It handles missing parents gracefully and prevents infinite loops/cycles by safely terminating
// before repeated nodes.
func BuildCategoryPath(nodes []CategoryHierarchyNode, leafID uuid.UUID) string {
	if len(nodes) == 0 {
		return ""
	}

	lookup := make(map[uuid.UUID]CategoryHierarchyNode, len(nodes))
	for _, n := range nodes {
		lookup[n.ID] = n
	}

	curr, ok := lookup[leafID]
	if !ok {
		return ""
	}

	var path []string
	visited := make(map[uuid.UUID]bool, len(nodes))
	const maxDepth = 50

	for depth := 0; depth < maxDepth; depth++ {
		if visited[curr.ID] {
			// Cycle detected, stop safely before repeated node
			break
		}
		visited[curr.ID] = true

		if name := strings.TrimSpace(curr.Name); name != "" {
			path = append(path, name)
		}

		if curr.ParentID == nil {
			break
		}

		parent, found := lookup[*curr.ParentID]
		if !found {
			// Missing parent, stop and use ancestors found so far
			break
		}
		curr = parent
	}

	if len(path) == 0 {
		return ""
	}

	// Reverse to achieve root -> leaf ordering
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return strings.Join(path, " > ")
}

// GetCategoryHierarchyPath loads the full category path from the database using a recursive CTE with
// cycle detection (path_ids tracking).
// Returns root -> leaf ordering separated by " > ", cleanly terminating without repeating nodes if a cycle exists.
func (r *Repository) GetCategoryHierarchyPath(ctx context.Context, categoryID uuid.UUID) (string, error) {
	query := `
		WITH RECURSIVE cat_tree AS (
			SELECT id, parent_id, name, 1 as depth, ARRAY[id] as path_ids
			FROM categories
			WHERE id = $1
			UNION ALL
			SELECT c.id, c.parent_id, c.name, ct.depth + 1, ct.path_ids || c.id
			FROM categories c
			JOIN cat_tree ct ON c.id = ct.parent_id
			WHERE ct.depth < 20 AND NOT (c.id = ANY(ct.path_ids))
		)
		SELECT id, name FROM cat_tree ORDER BY depth DESC;
	`
	rows, err := r.db.Query(ctx, query, categoryID)
	if err != nil {
		return "", fmt.Errorf("failed to query category hierarchy: %w", err)
	}
	defer rows.Close()

	var parts []string
	visited := make(map[uuid.UUID]bool)
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return "", fmt.Errorf("failed to scan category row: %w", err)
		}
		if visited[id] {
			// Defensively skip any duplicate category in case of corruption
			continue
		}
		visited[id] = true
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error reading category hierarchy rows: %w", err)
	}

	return strings.Join(parts, " > "), nil
}

// GetCanonicalProductInput loads all semantic fields for a product from the database
// into a CanonicalProductInput struct.
func (r *Repository) GetCanonicalProductInput(ctx context.Context, productID uuid.UUID) (*CanonicalProductInput, error) {
	prodQuery := `
		SELECT
			p.title,
			p.description,
			p.gender,
			p.color,
			p.material,
			p.category_id,
			b.name
		FROM products p
		LEFT JOIN brands b ON p.brand_id = b.id
		WHERE p.id = $1
	`
	var input CanonicalProductInput
	var catID *uuid.UUID
	var brandName *string
	err := r.db.QueryRow(ctx, prodQuery, productID).Scan(
		&input.Title,
		&input.Description,
		&input.Gender,
		&input.Color,
		&input.Material,
		&catID,
		&brandName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to get product for canonical embedding: %w", err)
	}
	input.Brand = brandName

	if catID != nil {
		path, err := r.GetCategoryHierarchyPath(ctx, *catID)
		if err != nil {
			return nil, err
		}
		input.CategoryPath = path
	}

	attrQuery := `
		SELECT
			ad.code,
			pav.text_value,
			pav.number_value,
			pav.bool_value,
			adv.name_ru,
			adv.code
		FROM product_attribute_values pav
		JOIN attribute_definitions ad ON pav.attribute_definition_id = ad.id
		LEFT JOIN attribute_dictionary_values adv ON pav.enum_value_id = adv.id
		WHERE pav.product_id = $1 AND ad.is_active = true
	`
	attrRows, err := r.db.Query(ctx, attrQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to query product attributes: %w", err)
	}
	defer attrRows.Close()

	for attrRows.Next() {
		var code string
		var textVal *string
		var numVal *float64
		var boolVal *bool
		var enumNameRu *string
		var enumCode *string
		if err := attrRows.Scan(&code, &textVal, &numVal, &boolVal, &enumNameRu, &enumCode); err != nil {
			return nil, fmt.Errorf("failed to scan product attribute: %w", err)
		}

		var val string
		if enumNameRu != nil && strings.TrimSpace(*enumNameRu) != "" {
			val = strings.TrimSpace(*enumNameRu)
		} else if enumCode != nil && strings.TrimSpace(*enumCode) != "" {
			val = strings.TrimSpace(*enumCode)
		} else if textVal != nil && strings.TrimSpace(*textVal) != "" {
			val = strings.TrimSpace(*textVal)
		} else if numVal != nil {
			val = strconv.FormatFloat(*numVal, 'f', -1, 64)
		} else if boolVal != nil {
			if *boolVal {
				val = "true"
			} else {
				val = "false"
			}
		}

		if val != "" {
			input.Attributes = append(input.Attributes, CanonicalAttribute{
				Key:   code,
				Value: val,
			})
		}
	}
	if err := attrRows.Err(); err != nil {
		return nil, fmt.Errorf("error reading attribute rows: %w", err)
	}

	return &input, nil
}

// GetProductEmbeddingInput loads catalog data and computes the canonical embedding input and content hash.
func (r *Repository) GetProductEmbeddingInput(ctx context.Context, productID uuid.UUID) (*CanonicalEmbeddingInput, error) {
	input, err := r.GetCanonicalProductInput(ctx, productID)
	if err != nil {
		return nil, err
	}
	res := BuildCanonicalEmbeddingInput(*input)
	return &res, nil
}
