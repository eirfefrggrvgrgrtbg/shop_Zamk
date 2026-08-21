package testlab

import (
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/selleranalytics"
)

type ScenarioPreset string

const (
	PresetBasicSales          ScenarioPreset = "BASIC_SALES"
	PresetNeverSold           ScenarioPreset = "NEVER_SOLD"
	PresetZeroCurrentPeriod   ScenarioPreset = "ZERO_CURRENT_PERIOD"
	PresetInventoryAndInbound ScenarioPreset = "INVENTORY_AND_INBOUND"
)

type ScenarioConfig struct {
	Preset   ScenarioPreset `json:"preset"`
	Timezone string         `json:"timezone"`
}

type ScenarioRun struct {
	RunID          string                           `json:"runId"`
	SellerID       uuid.UUID                        `json:"sellerId"`
	Period         selleranalytics.TimePeriod       `json:"period"`
	ExpectedResult selleranalytics.OverviewResponse `json:"expectedResult"`
	// AuxUserIDs holds the exact UUIDs of all auxiliary Test Lab users created
	// for this run (e.g. canonical buyer accounts). These are tracked here so
	// that CleanupRun can delete them precisely without relying on email
	// patterns or broad timestamp filters.
	AuxUserIDs []uuid.UUID `json:"auxUserIds,omitempty"`
}
