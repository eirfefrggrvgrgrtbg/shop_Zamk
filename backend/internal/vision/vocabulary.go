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

// Hashes the allowed categories, dictionaries, and vocabulary to create
// a deterministic taxonomy context identity.
func ComputeTaxonomyContextHash(allowedCategoryIDs []uuid.UUID, allowedDictionaryIDs []uuid.UUID, vocab *VocabularyRegistry) string {
	// 1. Sort canonical IDs to guarantee determinism
	catStrs := make([]string, len(allowedCategoryIDs))
	for i, id := range allowedCategoryIDs {
		catStrs[i] = id.String()
	}
	sort.Strings(catStrs)

	dictStrs := make([]string, len(allowedDictionaryIDs))
	for i, id := range allowedDictionaryIDs {
		dictStrs[i] = id.String()
	}
	sort.Strings(dictStrs)

	// 2. Sort vision vocabulary
	vocabStrs := make([]string, 0, len(vocab.ValidIDs))
	for id := range vocab.ValidIDs {
		vocabStrs = append(vocabStrs, id)
	}
	sort.Strings(vocabStrs)

	// 3. Hash them all
	h := sha256.New()
	h.Write([]byte(PromptVersion + "\n"))
	h.Write([]byte(SchemaVersion + "\n"))
	h.Write([]byte(VocabularyVersion + "\n"))

	h.Write([]byte(strings.Join(catStrs, ",") + "\n"))
	h.Write([]byte(strings.Join(dictStrs, ",") + "\n"))
	h.Write([]byte(strings.Join(vocabStrs, ",") + "\n"))

	return hex.EncodeToString(h.Sum(nil))
}
