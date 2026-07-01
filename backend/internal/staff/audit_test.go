package staff

import (
	"testing"
)

func TestSanitizeMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
		removed  []string
	}{
		{
			name:     "nil input returns empty map",
			input:    nil,
			expected: map[string]any{},
		},
		{
			name:     "removes password key",
			input:    map[string]any{"password": "secret123", "brandName": "ACME"},
			expected: map[string]any{"brandName": "ACME"},
			removed:  []string{"password"},
		},
		{
			name:     "removes token key",
			input:    map[string]any{"token": "abc", "ownerEmail": "x@y.com"},
			expected: map[string]any{"ownerEmail": "x@y.com"},
			removed:  []string{"token"},
		},
		{
			name:     "removes temporaryPassword key",
			input:    map[string]any{"temporaryPassword": "Tmp123!", "brandName": "B"},
			expected: map[string]any{"brandName": "B"},
			removed:  []string{"temporaryPassword"},
		},
		{
			name:     "removes access_token and refresh_token",
			input:    map[string]any{"access_token": "at", "refresh_token": "rt", "action": "login"},
			expected: map[string]any{"action": "login"},
			removed:  []string{"access_token", "refresh_token"},
		},
		{
			name:     "removes secret key",
			input:    map[string]any{"secret": "top_secret", "data": "ok"},
			expected: map[string]any{"data": "ok"},
			removed:  []string{"secret"},
		},
		{
			name:     "removes authorization key",
			input:    map[string]any{"authorization": "Bearer xyz", "id": 42},
			expected: map[string]any{"id": 42},
			removed:  []string{"authorization"},
		},
		{
			name:     "keeps non-sensitive keys intact",
			input:    map[string]any{"brandName": "Test", "ownerEmail": "a@b.com", "status": "active"},
			expected: map[string]any{"brandName": "Test", "ownerEmail": "a@b.com", "status": "active"},
		},
		{
			name:     "empty map returns empty map",
			input:    map[string]any{},
			expected: map[string]any{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeMetadata(tc.input)

			// Check removed keys are gone
			for _, key := range tc.removed {
				if _, exists := got[key]; exists {
					t.Errorf("key %q should have been removed but was found", key)
				}
			}

			// Check expected keys are present with correct values
			for k, v := range tc.expected {
				gotVal, exists := got[k]
				if !exists {
					t.Errorf("expected key %q not found in result", k)
					continue
				}
				if gotVal != v {
					t.Errorf("key %q: got %v, want %v", k, gotVal, v)
				}
			}

			// Check no extra keys were added
			if len(got) != len(tc.expected) {
				t.Errorf("result has %d keys, want %d", len(got), len(tc.expected))
			}
		})
	}
}

func TestSanitizeMetadataDoesNotMutateOriginal(t *testing.T) {
	original := map[string]any{"password": "secret", "name": "test"}
	result := SanitizeMetadata(original)

	if _, exists := result["password"]; exists {
		t.Error("sanitized result still contains 'password'")
	}
	if _, exists := original["password"]; !exists {
		t.Error("SanitizeMetadata mutated the original map")
	}
}
