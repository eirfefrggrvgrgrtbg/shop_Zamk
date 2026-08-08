package supplies

import (
	"time"

	"github.com/google/uuid"
)

type CreateSupplyRequest struct {
	HandoffMethod       string                   `json:"handoffMethod"`
	CarrierName         *string                  `json:"carrierName,omitempty"`
	TrackingNumber      *string                  `json:"trackingNumber,omitempty"`
	ExpectedArrivalDate *time.Time               `json:"expectedArrivalDate,omitempty"`
	Items               []CreateSupplyItemRequest `json:"items"`
	Boxes               []CreateSupplyBoxRequest  `json:"boxes"`
}

type CreateSupplyItemRequest struct {
	VariantID        uuid.UUID `json:"variantId"`
	ExpectedQuantity int       `json:"expectedQuantity"`
}

type CreateSupplyBoxRequest struct {
	BoxNumber string                           `json:"boxNumber"`
	Items     []CreateSupplyBoxItemRequest     `json:"items"`
}

type CreateSupplyBoxItemRequest struct {
	VariantID uuid.UUID `json:"variantId"` // used to map to supply item
	Quantity  int       `json:"quantity"`
}

type UpdateSupplyRequest struct {
	HandoffMethod       *string    `json:"handoffMethod,omitempty"`
	CarrierName         *string    `json:"carrierName,omitempty"`
	TrackingNumber      *string    `json:"trackingNumber,omitempty"`
	ExpectedArrivalDate *time.Time `json:"expectedArrivalDate,omitempty"`
}

type StartReceivingRequest struct {
	SupplyID uuid.UUID `json:"supplyId"`
}

type RecordReceivingScanRequest struct {
	VariantID uuid.UUID `json:"variantId"`
	Quantity  int       `json:"quantity"`
	IsDamage  bool      `json:"isDamage"`
}

type FinalizeReceivingRequest struct {
	// Any extra finalization notes
}
