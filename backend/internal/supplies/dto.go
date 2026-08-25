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

type RecordSerializedScanRequest struct {
	UnitCode  string `json:"unitCode"`
	Condition string `json:"condition"`
}

type SerializedScanResponse struct {
	ScanID           uuid.UUID `json:"scanId"`
	UnitCode         string    `json:"unitCode"`
	Condition        string    `json:"condition"`
	ProductVariantID uuid.UUID `json:"productVariantId"`
	ProductTitle     string    `json:"productTitle"`
	ColorName        *string   `json:"colorName,omitempty"`
	SizeName         *string   `json:"sizeName,omitempty"`
	SellerSKU        *string   `json:"sellerSku,omitempty"`
	VariantBarcode   *string   `json:"variantBarcode,omitempty"`

	SessionExpected  int `json:"expected"`
	SessionScanned   int `json:"scanned"`
	SessionOk        int `json:"ok"`
	SessionDamaged   int `json:"damaged"`
	SessionRemaining int `json:"remaining"`
}

type SerializedRecentScanDTO struct {
	ScanID         uuid.UUID  `json:"scanId"`
	UnitCode       string     `json:"unitCode"`
	Condition      string     `json:"condition"`
	ScannedAt      time.Time  `json:"scannedAt"`
	VoidedAt       *time.Time `json:"voidedAt,omitempty"`
	ProductTitle   string     `json:"productTitle"`
	ColorName      *string    `json:"colorName,omitempty"`
	SizeName       *string    `json:"sizeName,omitempty"`
	SellerSKU      *string    `json:"sellerSku,omitempty"`
	VariantBarcode *string    `json:"variantBarcode,omitempty"`
}

type UndoSerializedScanResponse struct {
	ScanID           uuid.UUID `json:"scanId"`
	VoidedAt         time.Time `json:"voidedAt"`
	SessionExpected  int       `json:"expected"`
	SessionScanned   int       `json:"scanned"`
	SessionOk        int       `json:"ok"`
	SessionDamaged   int       `json:"damaged"`
	SessionRemaining int       `json:"remaining"`
}

type FinalizeReceivingRequest struct {
	// Any extra finalization notes
}

type SupplyUnitLabelsResponse struct {
	SupplyID     uuid.UUID              `json:"supplyId"`
	SupplyNumber string                 `json:"supplyNumber"`
	Serialized   bool                   `json:"serialized"`
	TotalUnits   int                    `json:"totalUnits"`
	Box          *SupplyUnitLabelBoxDTO `json:"box,omitempty"`
	Units        []SupplyUnitLabelDTO   `json:"units"`
}

type SupplyUnitLabelBoxDTO struct {
	ID        uuid.UUID `json:"id"`
	BoxNumber string    `json:"boxNumber"`
}

type SupplyUnitLabelDTO struct {
	InventoryUnitID  uuid.UUID `json:"inventoryUnitId"`
	UnitCode         string    `json:"unitCode"`
	UnitIndex        int       `json:"unitIndex"`
	SupplyItemID     uuid.UUID `json:"supplyItemId"`
	ProductVariantID uuid.UUID `json:"productVariantId"`
	ProductTitle     string    `json:"productTitle"`
	ColorName        *string   `json:"colorName,omitempty"`
	SizeName         *string   `json:"sizeName,omitempty"`
	SellerSKU        *string   `json:"sellerSku,omitempty"`
	VariantBarcode   *string   `json:"variantBarcode,omitempty"`
	BoxNumber        *string   `json:"boxNumber,omitempty"`
}

type ResolvedPhysicalUnit struct {
	InventoryUnitID   uuid.UUID  `json:"inventoryUnitId"`
	UnitCode          string     `json:"unitCode"`
	UnitStatus        string     `json:"unitStatus"`
	RecommendedAction string     `json:"recommendedAction"`
	Product           struct {
		Title string `json:"title"`
	} `json:"product"`
	Variant struct {
		Color     *string `json:"color,omitempty"`
		Size      *string `json:"size,omitempty"`
		SellerSKU *string `json:"sellerSku,omitempty"`
		Barcode   *string `json:"barcode,omitempty"`
	} `json:"variant"`
	Origin struct {
		SupplyID     uuid.UUID  `json:"supplyId"`
		SupplyNumber string     `json:"supplyNumber"`
		SupplyStatus string     `json:"supplyStatus"`
		SupplyItemID uuid.UUID  `json:"supplyItemId"`
		BoxNumber    *string    `json:"boxNumber,omitempty"`
		SellerName   *string    `json:"sellerName,omitempty"`
	} `json:"origin"`
	ReceivingState struct {
		ActiveReceivingSessionID *uuid.UUID `json:"activeReceivingSessionId,omitempty"`
	} `json:"receivingState"`
}
