package payments

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type dummyProvider struct {
	mode string
}

func (d *dummyProvider) CreatePayment(ctx context.Context, input CreatePaymentInput) (ProviderCreatePaymentResult, error) {
	return ProviderCreatePaymentResult{
		ProviderPaymentID: "provider-pid-1",
		PaymentURL:        "https://stub.payment.url",
		Status:            "NEW",
	}, nil
}

func (d *dummyProvider) VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) error {
	return nil
}

func (d *dummyProvider) ParseWebhook(ctx context.Context, body []byte) (ProviderWebhookEvent, error) {
	return ProviderWebhookEvent{}, nil
}

func (d *dummyProvider) GetMode(method string) string {
	if method == "sbp" {
		return "unavailable"
	}
	if method == "tpay" {
		if d.mode != "" {
			return d.mode
		}
		return "mock"
	}
	if method == "card" {
		return "hosted_form"
	}
	return "unknown"
}

func TestTPayDisabled_ReturnsUnavailable(t *testing.T) {
	p := &dummyProvider{}
	if mode := p.GetMode("sbp"); mode != "unavailable" {
		t.Errorf("expected sbp mode to be unavailable, got %s", mode)
	}

	svc := &Service{provider: p}

	// SBP method should fail with ErrPaymentMethodUnavailable
	// We pass empty UUIDs since mode check happens first
	_, err := svc.CreatePayment(context.Background(), uuid.Nil, uuid.Nil, "sbp")
	if err != ErrPaymentMethodUnavailable {
		t.Errorf("expected ErrPaymentMethodUnavailable for sbp, got %v", err)
	}
}

func TestPaymentGetMode(t *testing.T) {
	p := &dummyProvider{mode: "mock"}
	if mode := p.GetMode("tpay"); mode != "mock" {
		t.Errorf("expected tpay mode mock, got %s", mode)
	}
	if mode := p.GetMode("card"); mode != "hosted_form" {
		t.Errorf("expected card mode hosted_form, got %s", mode)
	}
	if mode := p.GetMode("unknown_method"); mode != "unknown" {
		t.Errorf("expected unknown mode for invalid method, got %s", mode)
	}
}
