package payments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/http"
	"sort"
	"strconv"
)

type TBankProvider struct {
	terminalKey string
	password    string
	apiBaseURL  string
	successURL  string
	failURL     string
	client      *http.Client
}

func NewTBankProvider(terminalKey, password, apiBaseURL, successURL, failURL string) *TBankProvider {
	if apiBaseURL == "" {
		apiBaseURL = "https://securepay.tinkoff.ru/v2"
	}
	return &TBankProvider{
		terminalKey: terminalKey,
		password:    password,
		apiBaseURL:  apiBaseURL,
		successURL:  successURL,
		failURL:     failURL,
		client:      &http.Client{},
	}
}

type initRequest struct {
	TerminalKey string `json:"TerminalKey"`
	Amount      int64  `json:"Amount"` // in kopecks
	OrderId     string `json:"OrderId"`
	Description string `json:"Description,omitempty"`
	Token       string `json:"Token"`
	SuccessURL  string `json:"SuccessURL,omitempty"`
	FailURL     string `json:"FailURL,omitempty"`
}

type initResponse struct {
	Success    bool   `json:"Success"`
	ErrorCode  string `json:"ErrorCode"`
	Message    string `json:"Message"`
	Details    string `json:"Details"`
	PaymentURL string `json:"PaymentURL"`
	PaymentId  string `json:"PaymentId"`
	Status     string `json:"Status"`
}

func (p *TBankProvider) generateToken(params map[string]string) string {
	params["Password"] = p.password
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb bytes.Buffer
	for _, k := range keys {
		sb.WriteString(params[k])
	}

	hash := sha256.Sum256(sb.Bytes())
	return fmt.Sprintf("%x", hash)
}

func (p *TBankProvider) CreatePayment(ctx context.Context, input CreatePaymentInput) (ProviderCreatePaymentResult, error) {
	// For sandbox/stub mode, if terminalKey is "STUB", mock the response
	if p.terminalKey == "STUB" {
		hash := crc32.ChecksumIEEE([]byte(input.IdempotencyKey))
		pid := fmt.Sprintf("%d", hash)
		return ProviderCreatePaymentResult{
			ProviderPaymentID: pid,
			PaymentURL:        "https://stub.payment.url/pay/" + pid,
			Status:            "NEW",
		}, nil
	}

	reqData := initRequest{
		TerminalKey: p.terminalKey,
		Amount:      input.AmountCents,
		OrderId:     input.OrderID,
		Description: input.Description,
		SuccessURL:  p.successURL,
		FailURL:     p.failURL,
	}

	paramsMap := map[string]string{
		"TerminalKey": p.terminalKey,
		"Amount":      strconv.FormatInt(input.AmountCents, 10),
		"OrderId":     input.OrderID,
		"Description": input.Description,
	}
	if p.successURL != "" {
		paramsMap["SuccessURL"] = p.successURL
	}
	if p.failURL != "" {
		paramsMap["FailURL"] = p.failURL
	}

	reqData.Token = p.generateToken(paramsMap)

	body, err := json.Marshal(reqData)
	if err != nil {
		return ProviderCreatePaymentResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiBaseURL+"/Init", bytes.NewBuffer(body))
	if err != nil {
		return ProviderCreatePaymentResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return ProviderCreatePaymentResult{}, err
	}
	defer resp.Body.Close()

	var tResp initResponse
	if err := json.NewDecoder(resp.Body).Decode(&tResp); err != nil {
		return ProviderCreatePaymentResult{}, err
	}

	if !tResp.Success {
		return ProviderCreatePaymentResult{}, fmt.Errorf("tbank init failed: %s - %s", tResp.Message, tResp.Details)
	}

	return ProviderCreatePaymentResult{
		ProviderPaymentID: tResp.PaymentId,
		PaymentURL:        tResp.PaymentURL,
		Status:            tResp.Status,
	}, nil
}

type webhookPayload struct {
	TerminalKey string `json:"TerminalKey"`
	OrderId     string `json:"OrderId"`
	Success     bool   `json:"Success"`
	Status      string `json:"Status"`
	PaymentId   int64  `json:"PaymentId"`
	ErrorCode   string `json:"ErrorCode"`
	Amount      int64  `json:"Amount"`
	RebillId    int64  `json:"RebillId"`
	CardId      int64  `json:"CardId"`
	Pan         string `json:"Pan"`
	ExpDate     string `json:"ExpDate"`
	Token       string `json:"Token"`
}

func (p *TBankProvider) VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) error {
	if p.terminalKey == "STUB" {
		return nil
	}

	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return err
	}

	paramsMap := make(map[string]string)
	for k, v := range data {
		if k == "Token" || k == "Receipt" || k == "DATA" {
			continue
		}
		switch val := v.(type) {
		case string:
			paramsMap[k] = val
		case float64:
			paramsMap[k] = strconv.FormatInt(int64(val), 10)
		case bool:
			if val {
				paramsMap[k] = "true"
			} else {
				paramsMap[k] = "false"
			}
		}
	}

	expectedToken := p.generateToken(paramsMap)
	if payload.Token != expectedToken {
		return ErrInvalidSignature
	}

	return nil
}

func (p *TBankProvider) ParseWebhook(ctx context.Context, body []byte) (ProviderWebhookEvent, error) {
	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return ProviderWebhookEvent{}, err
	}

	status := "failed"
	if payload.Status == "CONFIRMED" || payload.Status == "AUTHORIZED" {
		status = "succeeded"
	} else if payload.Status == "CANCELED" || payload.Status == "REJECTED" || payload.Status == "DEADLINE_EXPIRED" {
		status = "cancelled"
	} else if payload.Status == "NEW" || payload.Status == "FORMSHOWED" {
		status = "pending"
	}

	return ProviderWebhookEvent{
		ProviderPaymentID: strconv.FormatInt(payload.PaymentId, 10),
		OrderID:           payload.OrderId,
		Status:            status,
		AmountCents:       payload.Amount,
		RawPayload:        body,
	}, nil
}
