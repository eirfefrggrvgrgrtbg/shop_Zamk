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

func mustPanicOnLoad(t *testing.T, name string) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: Load must refuse the configuration", name)
		}
	}()
	_, _ = Load()
}

// BANK.REFUND.2: final refusal codes cannot be configured, and the dev fake refund provider is refused on any stand
// that could reach a real bank.
func TestRefundConfigFailsClosed(t *testing.T) {
	t.Run("definitive_error_codes", func(t *testing.T) {
		t.Setenv("TBANK_REFUND_DEFINITIVE_ERROR_CODES", "1001,9999")
		mustPanicOnLoad(t, "definitive codes")
	})
	t.Run("fake_with_real_terminal", func(t *testing.T) {
		t.Setenv("APP_ENV", "development")
		t.Setenv("TBANK_TERMINAL_KEY", "REALTERMINAL")
		t.Setenv("TBANK_PASSWORD", "secret")
		t.Setenv("ZAMK_DEV_FAKE_REFUND_PROVIDER", "true")
		mustPanicOnLoad(t, "fake next to a real terminal")
	})
	t.Run("fake_on_staging", func(t *testing.T) {
		t.Setenv("APP_ENV", "staging")
		t.Setenv("TBANK_TERMINAL_KEY", "STUB")
		t.Setenv("ZAMK_DEV_FAKE_REFUND_PROVIDER", "true")
		mustPanicOnLoad(t, "fake on staging")
	})
	t.Run("defaults_send_nothing", func(t *testing.T) {
		t.Setenv("APP_ENV", "development")
		t.Setenv("TBANK_TERMINAL_KEY", "STUB")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.TBank.RefundsEnabled || cfg.TBank.RefundFakeProvider {
			t.Fatal("no refund provider may be enabled by default")
		}
	})
}
