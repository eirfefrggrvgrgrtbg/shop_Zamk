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

func TestTBankProvider_CheckOrder_GeneratesSignatureAndParsesPayments(t *testing.T) {
	var capturedPayload map[string]interface{}

	client := &http.Client{
		Transport: &mockRoundTripper{
			fn: func(req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				_ = json.Unmarshal(body, &capturedPayload)

				respData := map[string]interface{}{
					"Success":     true,
					"ErrorCode":   "0",
					"Message":     "OK",
					"TerminalKey": "TEST_TERMINAL",
					"OrderId":     "order-check-1",
					"Payments": []map[string]interface{}{
						{
							"PaymentId": 12345678,
							"Amount":    500000,
							"Status":    "CONFIRMED",
							"Success":   true,
						},
						{
							"PaymentId": "87654321",
							"Amount":    500000,
							"Status":    "AUTHORIZED",
							"Success":   true,
						},
					},
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

	provider := NewTBankProvider("TEST_TERMINAL", "TEST_SECRET_PASSWORD", "https://api.tbank.ru/v2", "", "", true, "O", "hosted_form")
	provider.client = client

	res, err := provider.CheckOrder(context.Background(), "order-check-1")
	if err != nil {
		t.Fatalf("unexpected CheckOrder error: %v", err)
	}

	if res.OrderID != "order-check-1" {
		t.Errorf("expected OrderID order-check-1, got %s", res.OrderID)
	}
	if len(res.Payments) != 2 {
		t.Fatalf("expected 2 payments, got %d", len(res.Payments))
	}
	if res.Payments[0].ProviderPaymentID != "12345678" || res.Payments[0].Status != "succeeded" || res.Payments[0].ProviderStatus != "CONFIRMED" {
		t.Errorf("unexpected payment 0: %+v", res.Payments[0])
	}
	if res.Payments[1].ProviderPaymentID != "87654321" || res.Payments[1].Status != "pending" || res.Payments[1].ProviderStatus != "AUTHORIZED" {
		t.Errorf("unexpected payment 1: %+v", res.Payments[1])
	}

	// Assert token signature in request payload
	if _, tokenExists := capturedPayload["Token"]; !tokenExists {
		t.Errorf("expected Token in CheckOrder request payload")
	}
	// Assert secret Password is NOT sent
	if _, passExists := capturedPayload["Password"]; passExists {
		t.Errorf("Password must NOT be present in CheckOrder request payload")
	}
}
