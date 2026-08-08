package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

type mockRoundTripper struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

func TestTPayInitPayload_PayTypeIsTopLevel(t *testing.T) {
	var capturedPayload map[string]interface{}

	client := &http.Client{
		Transport: &mockRoundTripper{
			fn: func(req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				_ = json.Unmarshal(body, &capturedPayload)

				respData := initResponse{
					Success:    true,
					PaymentId:  "123456",
					PaymentURL: "https://tbank.ru/pay/123456",
					Status:     "NEW",
				}
				respBytes, _ := json.Marshal(respData)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer(respBytes)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	provider := NewTBankProvider("TEST_TERMINAL", "TEST_SECRET_PASSWORD", "https://api.tbank.ru/v2", "http://success", "http://fail", true, "O", "quick_widget")
	provider.client = client

	input := CreatePaymentInput{
		OrderID:         "order-123",
		AmountCents:     500000,
		Currency:        "RUB",
		IdempotencyKey:  "key-123",
		Description:     "Test Payment",
		Method:          "tpay",
		IntegrationMode: "quick_widget",
	}

	res, err := provider.CreatePayment(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ProviderPaymentID != "123456" {
		t.Errorf("expected payment ID 123456, got %s", res.ProviderPaymentID)
	}

	// Assert PayType is at root level
	payTypeVal, exists := capturedPayload["PayType"]
	if !exists {
		t.Errorf("expected PayType to be at root level of JSON payload")
	} else if payTypeVal != "O" {
		t.Errorf("expected PayType to be 'O', got %v", payTypeVal)
	}

	// Assert Amount comes from Order input
	amountVal, ok := capturedPayload["Amount"].(float64)
	if !ok || int64(amountVal) != 500000 {
		t.Errorf("expected Amount to be 500000, got %v", capturedPayload["Amount"])
	}

	// Assert secret Password is NOT in payload
	if _, passwordExists := capturedPayload["Password"]; passwordExists {
		t.Errorf("secret Password must NOT be included in public wire JSON payload")
	}

	// Assert DATA does NOT contain PayType
	if dataMap, ok := capturedPayload["DATA"].(map[string]interface{}); ok {
		if _, payTypeInData := dataMap["PayType"]; payTypeInData {
			t.Errorf("PayType must NOT be placed inside DATA map")
		}
	}
}

func TestTPayWidgetInit_HasConnectionTypeWidget(t *testing.T) {
	var capturedPayload map[string]interface{}

	client := &http.Client{
		Transport: &mockRoundTripper{
			fn: func(req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				_ = json.Unmarshal(body, &capturedPayload)

				respData := initResponse{
					Success:    true,
					PaymentId:  "999888",
					PaymentURL: "https://tbank.ru/widget/999888",
					Status:     "NEW",
				}
				respBytes, _ := json.Marshal(respData)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer(respBytes)),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	provider := NewTBankProvider("TEST_TERMINAL", "TEST_SECRET_PASSWORD", "https://api.tbank.ru/v2", "http://success", "http://fail", true, "O", "quick_widget")
	provider.client = client

	input := CreatePaymentInput{
		OrderID:         "order-456",
		AmountCents:     150000,
		Currency:        "RUB",
		IdempotencyKey:  "key-456",
		Description:     "Widget Test",
		Method:          "tpay",
		IntegrationMode: "quick_widget",
	}

	_, err := provider.CreatePayment(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dataMap, ok := capturedPayload["DATA"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected DATA object in JSON payload")
	}

	connType, exists := dataMap["connection_type"]
	if !exists || connType != "Widget" {
		t.Errorf("expected DATA.connection_type = 'Widget', got %v", connType)
	}
}
