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
