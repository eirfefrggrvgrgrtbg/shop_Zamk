package products

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxShadeNameRunes = 60

// normalizeShadeName trims the optional shade description; an empty value means "no shade".
func normalizeShadeName(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// validateVariantColors checks the primary color of every submitted variant against the canonical `colors`
// dictionary (independent of the category schema): the id must exist and be active, the shade name must be a
// short plain label, and one (color, size) combination may appear only once. Variants without color_id
// (legacy free-text color) stay valid: color is not made mandatory here.
func (s *Service) validateVariantColors(ctx context.Context, vReqs []ProductVariantRequest) error {
	seenCombo := map[string]bool{}
	checked := map[uuid.UUID]bool{}
	for i, v := range vReqs {
		if v.ShadeName != nil {
			t := strings.TrimSpace(*v.ShadeName)
			if utf8.RuneCountInString(t) > maxShadeNameRunes {
				return fmt.Errorf("variant validation failed: shade name on variant %d is longer than %d characters", i, maxShadeNameRunes)
			}
			for _, r := range t {
				if unicode.IsControl(r) || r == '<' || r == '>' {
					return fmt.Errorf("variant validation failed: shade name on variant %d contains invalid characters", i)
				}
			}
		}
		if v.ColorID == nil {
			continue
		}
		if !checked[*v.ColorID] {
			var active bool
			err := s.dbPool.Pool.QueryRow(ctx, "SELECT is_active FROM colors WHERE id = $1", *v.ColorID).Scan(&active)
			if err != nil || !active {
				return fmt.Errorf("color %s is invalid or inactive; not valid for this category", *v.ColorID)
			}
			checked[*v.ColorID] = true
		}
		size := ""
		if v.SizeValueID != nil {
			size = v.SizeValueID.String()
		}
		key := v.ColorID.String() + "|" + size
		if seenCombo[key] {
			return fmt.Errorf("variant validation failed: duplicate color/size combination on variant %d", i)
		}
		seenCombo[key] = true
	}
	return nil
}
