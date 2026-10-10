package vision

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// Constants for Vision context identity
const (
	PromptVersion     = "vision-product-v1"
	SchemaVersion     = "1"
	VocabularyVersion = "1"
)

// Vision-owned experimental vocabulary (Version 1)
// These IDs are passed to the model and returned in ProductVisualObservationV1.

var (
	VocabularySilhouettes = []string{
		"vision.silhouette.regular",
		"vision.silhouette.oversized",
		"vision.silhouette.fitted",
		"vision.silhouette.relaxed",
		"vision.silhouette.boxy",
	}

	VocabularyPatterns = []string{
		"vision.pattern.solid",
		"vision.pattern.striped",
		"vision.pattern.plaid",
		"vision.pattern.floral",
		"vision.pattern.graphic",
		"vision.pattern.animal",
		"vision.pattern.abstract",
	}

	VocabularyTextures = []string{
		"vision.texture.smooth",
		"vision.texture.knitted",
		"vision.texture.woven",
		"vision.texture.leather_like",
		"vision.texture.fur_like",
		"vision.texture.denim",
		"vision.texture.fuzzy",
	}

	VocabularyQualityIssues = []string{
		"vision.quality.blur",
		"vision.quality.watermark_detected",
		"vision.quality.poor_lighting",
		"vision.quality.inappropriate_content",
	}
)

// VocabularyRegistry provides fast O(1) lookup for validation.
type VocabularyRegistry struct {
	ValidIDs map[string]bool
}

// NewVocabularyRegistry builds the allowed set.
func NewVocabularyRegistry() *VocabularyRegistry {
	reg := &VocabularyRegistry{
		ValidIDs: make(map[string]bool),
	}
	for _, id := range VocabularySilhouettes {
		reg.ValidIDs[id] = true
	}
	for _, id := range VocabularyPatterns {
		reg.ValidIDs[id] = true
	}
	for _, id := range VocabularyTextures {
		reg.ValidIDs[id] = true
	}
	for _, id := range VocabularyQualityIssues {
		reg.ValidIDs[id] = true
	}
	return reg
}

// TaxonomyItem represents a taxonomy entity with its canonical ID and human label as exposed to prompts.
type TaxonomyItem struct {
	ID    uuid.UUID `json:"id"`
	Label string    `json:"label"`
}

// TaxonomyContext encapsulates the runtime context used during inference.
type TaxonomyContext struct {
	PromptVersion      string
	SchemaVersion      string
	VocabularyVersion  string
	Categories         []TaxonomyItem
	Dictionaries       []TaxonomyItem
	OtherTaxonomyIDs   []uuid.UUID
	VocabularyRegistry *VocabularyRegistry
}

// ComputeTaxonomyContextHash deterministically hashes the prompt version, schema version,
// vocabulary version, runtime allowlists (with prompt-visible labels), and vision vocabulary.
func ComputeTaxonomyContextHash(tc TaxonomyContext) string {
	pv := tc.PromptVersion
	if pv == "" {
		pv = PromptVersion
	}
	sv := tc.SchemaVersion
	if sv == "" {
		sv = SchemaVersion
	}
	vv := tc.VocabularyVersion
	if vv == "" {
		vv = VocabularyVersion
	}

	// 1. Sort canonical Categories by ID (deterministic order)
	catItems := make([]TaxonomyItem, len(tc.Categories))
	copy(catItems, tc.Categories)
	sort.Slice(catItems, func(i, j int) bool {
		return catItems[i].ID.String() < catItems[j].ID.String()
	})
	catStrs := make([]string, len(catItems))
	for i, c := range catItems {
		catStrs[i] = c.ID.String() + ":" + c.Label
	}

	// 2. Sort canonical Dictionaries by ID
	dictItems := make([]TaxonomyItem, len(tc.Dictionaries))
	copy(dictItems, tc.Dictionaries)
	sort.Slice(dictItems, func(i, j int) bool {
		return dictItems[i].ID.String() < dictItems[j].ID.String()
	})
	dictStrs := make([]string, len(dictItems))
	for i, d := range dictItems {
		dictStrs[i] = d.ID.String() + ":" + d.Label
	}

	// 3. Sort Other Taxonomy IDs
	otherStrs := make([]string, len(tc.OtherTaxonomyIDs))
	for i, id := range tc.OtherTaxonomyIDs {
		otherStrs[i] = id.String()
	}
	sort.Strings(otherStrs)

	// 4. Sort Vision-owned vocabulary IDs
	vocab := tc.VocabularyRegistry
	if vocab == nil {
		vocab = NewVocabularyRegistry()
	}
	vocabStrs := make([]string, 0, len(vocab.ValidIDs))
	for id := range vocab.ValidIDs {
		vocabStrs = append(vocabStrs, id)
	}
	sort.Strings(vocabStrs)

	// 5. Build SHA-256 hash
	h := sha256.New()
	h.Write([]byte(pv + "\n"))
	h.Write([]byte(sv + "\n"))
	h.Write([]byte(vv + "\n"))
	h.Write([]byte(strings.Join(catStrs, ",") + "\n"))
	h.Write([]byte(strings.Join(dictStrs, ",") + "\n"))
	h.Write([]byte(strings.Join(otherStrs, ",") + "\n"))
	h.Write([]byte(strings.Join(vocabStrs, ",") + "\n"))

	return hex.EncodeToString(h.Sum(nil))
}
