package config

import "testing"

// BANK.REFUND.0: the refund simulator is allowed only outside production and only without a real T-Bank terminal.
func TestRefundSimulatorAllowed(t *testing.T) {
	cases := []struct {
		name     string
		env      string
		key      string
		password string
		want     bool
	}{
		{"local_stub", "development", "STUB", "STUB", true},
		{"tests_without_terminal", "", "", "", true},
		{"key_without_password", "staging", "TERMINAL", "", true},
		{"staging_real_terminal", "staging", "TERMINAL", "secret", false},
		{"development_real_terminal", "development", "TERMINAL", "secret", false},
		{"production_stub", "production", "STUB", "STUB", false},
		{"production_real_terminal", "production", "TERMINAL", "secret", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &Config{App: AppConfig{Env: c.env}, TBank: TBankConfig{TerminalKey: c.key, Password: c.password}}
			if got := cfg.RefundSimulatorAllowed(); got != c.want {
				t.Fatalf("RefundSimulatorAllowed() = %v, want %v", got, c.want)
			}
		})
	}
}
