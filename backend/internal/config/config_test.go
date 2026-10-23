package config

import (
	"os"
	"testing"
)

func TestProductionRejectsMockMode(t *testing.T) {
	os.Setenv("APP_ENV", "production")
	os.Setenv("TBANK_TPAY_MODE", "mock")
	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("TBANK_TPAY_MODE")
	}()

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected panic when APP_ENV=production and TBANK_TPAY_MODE=mock, but got none")
		} else {
			errMsg, ok := r.(string)
			if !ok || errMsg != "Cannot use mock TBank mode in production environment" {
				t.Errorf("unexpected panic message: %v", r)
			}
		}
	}()

	_, _ = Load()
}

// PAY.P1: production never runs on the default STUB terminal; an empty terminal is allowed (payments are then
// unavailable, not mocked).
func TestProductionRejectsStubTerminal(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("TBANK_TPAY_MODE", "quick_widget")
	t.Setenv("TBANK_TERMINAL_KEY", "STUB")

	defer func() {
		if r := recover(); r != "Cannot use STUB TBank terminal in production environment" {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	_, _ = Load()
}

func TestProductionAllowsEmptyTerminal(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("TBANK_TPAY_MODE", "quick_widget")
	t.Setenv("TBANK_TERMINAL_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TBank.TerminalKey != "" {
		t.Fatalf("expected the empty terminal to stay empty")
	}
}
