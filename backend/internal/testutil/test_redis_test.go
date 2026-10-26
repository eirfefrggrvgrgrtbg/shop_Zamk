package testutil

import "testing"

func TestValidateIsolatedRedis(t *testing.T) {
	tok := "0123456789abcdef0123456789abcdef"
	if err := ValidateIsolatedRedis("127.0.0.1:55001", tok); err != nil {
		t.Fatalf("per-run loopback Redis must be accepted: %v", err)
	}
	for name, c := range map[string][2]string{
		"shared default localhost": {"localhost:6379", tok},
		"shared default 127.0.0.1": {"127.0.0.1:6379", tok},
		"empty address":            {"", tok},
		"empty token":              {"127.0.0.1:55001", ""},
		"short token":              {"127.0.0.1:55001", "abc"},
		"non-loopback host":        {"10.0.0.5:55001", tok},
		"no port":                  {"127.0.0.1", tok},
	} {
		if err := ValidateIsolatedRedis(c[0], c[1]); err == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
}
