package returns

import (
	"context"
	"sync"
)

type FakeLogisticsProvider struct {
	mu       sync.Mutex
	Statuses map[string]string
}

func NewFakeLogisticsProvider() *FakeLogisticsProvider {
	return &FakeLogisticsProvider{
		Statuses: make(map[string]string),
	}
}

func (p *FakeLogisticsProvider) ListOffices(ctx context.Context) ([]Office, error) {
	return []Office{
		{Code: "MSK1", Address: "Moscow, 1", Name: "Office 1"},
		{Code: "MSK2", Address: "Moscow, 2", Name: "Office 2"},
	}, nil
}

func (p *FakeLogisticsProvider) CreateShipment(ctx context.Context, req ProviderShipmentRequest) (*ProviderShipmentResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := "FAKE-CDEK-" + req.ReturnID.String()
	p.Statuses[id] = "awaiting_handover"
	return &ProviderShipmentResult{
		ProviderShipmentID: id,
		TrackingNumber:     "TRACK-" + req.ReturnID.String(),
		Status:             "awaiting_handover",
	}, nil
}

func (p *FakeLogisticsProvider) GetShipmentStatus(ctx context.Context, providerShipmentID string) (*ProviderShipmentStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	status, ok := p.Statuses[providerShipmentID]
	if !ok {
		status = "unknown"
	}
	return &ProviderShipmentStatus{Status: status}, nil
}
