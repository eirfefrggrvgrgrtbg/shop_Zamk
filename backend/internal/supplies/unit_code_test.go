package supplies

import (
	"strings"
	"testing"
)

func TestGenerateUnitCode(t *testing.T) {
	code, err := GenerateUnitCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(code, "ZMU-") {
		t.Errorf("expected prefix ZMU-, got %s", code)
	}

	if len(code) != 20 {
		t.Errorf("expected length 20, got %d", len(code))
	}

	// Verify alphabet
	alphabet := "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	for _, c := range code[4:] {
		if !strings.ContainsRune(alphabet, c) {
			t.Errorf("unexpected character %c in %s", c, code)
		}
	}

	// Uniqueness check (basic)
	set := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		c, _ := GenerateUnitCode()
		if set[c] {
			t.Fatalf("collision detected for code %s", c)
		}
		set[c] = true
	}
}
