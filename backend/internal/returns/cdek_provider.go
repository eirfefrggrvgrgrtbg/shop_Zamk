package returns

import (
	"context"
	"errors"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
)

var ErrCDEKNotConfigured = errors.New("cdek_not_configured")

type cdekProvider struct {
	cfg config.CDEKConfig
}

func NewCDEKProvider(cfg config.CDEKConfig) ReturnLogisticsProvider {
	return &cdekProvider{cfg: cfg}
}

func (p *cdekProvider) isConfigured() bool {
	return p.cfg.ClientID != "" && p.cfg.ClientSecret != ""
}

func (p *cdekProvider) ListOffices(ctx context.Context) ([]Office, error) {
	if !p.isConfigured() {
		return nil, ErrCDEKNotConfigured
	}
	// TODO: implement real HTTP request
	return nil, errors.New("not implemented")
}

func (p *cdekProvider) CreateShipment(ctx context.Context, req ProviderShipmentRequest) (*ProviderShipmentResult, error) {
	if !p.isConfigured() {
		return nil, ErrCDEKNotConfigured
	}
	// TODO: implement real HTTP request
	return nil, errors.New("not implemented")
}

func (p *cdekProvider) GetShipmentStatus(ctx context.Context, providerShipmentID string) (*ProviderShipmentStatus, error) {
	if !p.isConfigured() {
		return nil, ErrCDEKNotConfigured
	}
	// TODO: implement real HTTP request
	return nil, errors.New("not implemented")
}
