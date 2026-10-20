package merchandising

import "testing"

func TestSlugValidationIsRegisteredAndStrict(t *testing.T) {
	h := NewHandler(nil)
	cases := map[string]bool{
		"summer-edit": true,
		"a1":          true,
		"Summer":      false,
		"two--dashes": false,
		"-leading":    false,
		"trailing-":   false,
		"with space":  false,
		"кириллица":   false,
		"":            false,
	}
	for slug, ok := range cases {
		err := h.validator.Struct(CreateCollectionRequest{Slug: slug, Title: "t"})
		if (err == nil) != ok {
			t.Errorf("slug %q: valid=%v, want %v (%v)", slug, err == nil, ok, err)
		}
	}
}
