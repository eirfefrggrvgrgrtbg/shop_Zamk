package payments

import (
	"errors"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
)

// RefundProviderFromConfig builds the refund provider the configuration allows (BANK.REFUND.2):
//   - TBANK_REFUNDS_ENABLED=true: the real T-Bank adapter (an error without a real terminal);
//   - ZAMK_DEV_FAKE_REFUND_PROVIDER=true: the in-memory fake, which config.Load refuses in production, on staging
//     and next to a real terminal or enabled real refunds;
//   - otherwise none (nil): refunds are reserved but never sent.
func RefundProviderFromConfig(cfg *config.Config) (RefundProvider, error) {
	switch {
	case cfg.TBank.RefundsEnabled && cfg.TBank.RefundFakeProvider:
		return nil, errors.New("real and fake refund providers cannot both be enabled")
	case cfg.TBank.RefundsEnabled:
		return NewTBankRefundProvider(TBankRefundConfig{
			TerminalKey:     cfg.TBank.TerminalKey,
			Password:        cfg.TBank.Password,
			APIBaseURL:      cfg.TBank.APIBaseURL,
			Timeout:         time.Duration(cfg.TBank.RefundTimeoutSeconds) * time.Second,
			ReceiptRequired: cfg.TBank.RefundReceiptRequired,
		})
	case cfg.TBank.RefundFakeProvider:
		return NewFakeRefundProvider(), nil
	}
	return nil, nil
}
