package returns

import (
	"context"

	"github.com/google/uuid"
)

type Office struct {
	Code         string
	Address      string
	Name         string
	WorkingHours string
}

type ProviderShipmentResult struct {
	ProviderShipmentID string
	TrackingNumber     string
	Status             string
}

type ProviderShipmentStatus struct {
	Status string
}

type ProviderShipmentRequest struct {
	ReturnID       uuid.UUID
	CustomerName   string
	CustomerPhone  string
	Method         string
	CDEKOfficeCode string
	PickupAddress  *PickupAddressDTO
}

type ReturnLogisticsProvider interface {
	ListOffices(ctx context.Context) ([]Office, error)
	CreateShipment(ctx context.Context, req ProviderShipmentRequest) (*ProviderShipmentResult, error)
	GetShipmentStatus(ctx context.Context, providerShipmentID string) (*ProviderShipmentStatus, error)
}
