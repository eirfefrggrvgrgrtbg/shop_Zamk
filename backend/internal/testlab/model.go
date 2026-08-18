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
}
