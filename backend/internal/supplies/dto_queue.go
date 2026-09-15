package supplies

import (
	"time"

	"github.com/google/uuid"
)

type SupplyReceivingQueueItem struct {
	SupplyID                 uuid.UUID  `json:"supplyId"`
	SupplyNumber             string     `json:"supplyNumber"`
	Status                   string     `json:"status"`
	SellerID                 uuid.UUID  `json:"sellerId"`
	SellerName               string     `json:"sellerName"`
	ExpectedUnitsCount       int        `json:"expectedUnitsCount"`
	AcceptedUnitsCount       int        `json:"acceptedUnitsCount"`
	RemainingUnitsCount      int        `json:"remainingUnitsCount"`
	CargoPlacesCount         int        `json:"cargoPlacesCount"`
	ArrivedAt                *time.Time `json:"arrivedAt,omitempty"`
	ReceivingStartedAt       *time.Time `json:"receivingStartedAt,omitempty"`
	ActiveReceivingSessionID *uuid.UUID `json:"activeReceivingSessionId,omitempty"`
}
