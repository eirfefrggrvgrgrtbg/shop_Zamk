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

type ProcessFoundUnitRequest struct {
	UnitCode  string `json:"unitCode"`
	Condition string `json:"condition"` // "ok" | "damaged"
}

type ProcessFoundUnitResponse struct {
	UnitCode              string     `json:"unitCode"`
	InventoryUnitID       uuid.UUID  `json:"inventoryUnitId"`
	SupplyID              uuid.UUID  `json:"supplyId"`
	SupplyNumber          string     `json:"supplyNumber"`
	ReceivingSessionID    *uuid.UUID `json:"receivingSessionId,omitempty"`
	Condition             string     `json:"condition,omitempty"`
	SessionExpected       int        `json:"sessionExpected"`
	SessionScanned        int        `json:"sessionScanned"`
	SessionOk             int        `json:"sessionOk"`
	SessionDamaged        int        `json:"sessionDamaged"`
	SessionRemaining      int        `json:"sessionRemaining"`
	UnitStatus            string     `json:"unitStatus"`
	RecommendedNextAction string     `json:"recommendedNextAction"`
	ProductTitle          string     `json:"productTitle,omitempty"`
	ColorName             *string    `json:"colorName,omitempty"`
	SizeName              *string    `json:"sizeName,omitempty"`
	SellerSKU             *string    `json:"sellerSku,omitempty"`
	VariantBarcode        *string    `json:"variantBarcode,omitempty"`
	SellerName            *string    `json:"sellerName,omitempty"`
	BoxNumber             *string    `json:"boxNumber,omitempty"`
}

// SellerSupplyItemResponse represents a supply line item serialized to the Seller.
// It deliberately excludes warehouse-internal operational fields (such as receivingComment,
// operator IDs, and internal sessions) while exposing the physical receiving outcome.
type SellerSupplyItemResponse struct {
	ID                uuid.UUID `json:"id"`
	SupplyID          uuid.UUID `json:"supplyId"`
	VariantID         uuid.UUID `json:"variantId"`
	ExpectedQuantity  int       `json:"expectedQuantity"`
	AcceptedQuantity  int       `json:"acceptedQuantity"`
	DamagedQuantity   int       `json:"damagedQuantity"`
	MissingQuantity   int       `json:"missingQuantity"`
	ExtraQuantity     int       `json:"extraQuantity"`
	RemainingQuantity int       `json:"remainingQuantity"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`

	// Joins
	SKU          string  `json:"sku,omitempty"`
	SellerSKU    *string `json:"sellerSku,omitempty"`
	ProductTitle string  `json:"productTitle,omitempty"`
	ColorName    *string `json:"colorName,omitempty"`
	SizeName     *string `json:"sizeName,omitempty"`
	Barcode      *string `json:"barcode,omitempty"`
}

// SellerSupplyResponse represents the supply view returned to the Seller.
type SellerSupplyResponse struct {
	ID                          uuid.UUID                  `json:"id"`
	SupplyNumber                string                     `json:"supplyNumber"`
	SellerID                    uuid.UUID                  `json:"sellerId"`
	SellerName                  string                     `json:"sellerName,omitempty"`
	Status                      string                     `json:"status"`
	HandoffMethod               string                     `json:"handoffMethod"`
	CarrierName                 *string                    `json:"carrierName,omitempty"`
	TrackingNumber              *string                    `json:"trackingNumber,omitempty"`
	ExpectedArrivalDate         *time.Time                 `json:"expectedArrivalDate,omitempty"`
	QRToken                     *string                    `json:"qrToken,omitempty"`
	CreatedAt                   time.Time                  `json:"createdAt"`
	ShippedAt                   *time.Time                 `json:"shippedAt,omitempty"`
	ArrivedAt                   *time.Time                 `json:"arrivedAt,omitempty"`
	ReceivingStartedAt          *time.Time                 `json:"receivingStartedAt,omitempty"`
	CompletedAt                 *time.Time                 `json:"completedAt,omitempty"`
	UpdatedAt                   time.Time                  `json:"updatedAt"`

	TotalExpectedItems          int                        `json:"totalExpectedItems"`
	TotalAcceptedItems          int                        `json:"totalAcceptedItems"`
	TotalRemainingItems         int                        `json:"totalRemainingItems"`
	TotalExpectedBoxes          int                        `json:"totalExpectedBoxes"`
	SKUCount                    int                        `json:"skuCount"`
	DiscrepancyQuantity         int                        `json:"discrepancyQuantity"`
	DiscrepancyCount            int                        `json:"discrepancyCount"`
	IsReceivingComplete         bool                       `json:"isReceivingComplete"`
	AdditionalReceivingHappened bool                       `json:"additionalReceivingHappened"`

	Items []SellerSupplyItemResponse `json:"items,omitempty"`
	Boxes []SupplyBox                `json:"boxes,omitempty"`
}

// ToSellerSupplyResponse converts a domain Supply model into a sanitized SellerSupplyResponse.
func ToSellerSupplyResponse(s *Supply) *SellerSupplyResponse {
	if s == nil {
		return nil
	}
	resp := &SellerSupplyResponse{
		ID:                          s.ID,
		SupplyNumber:                s.SupplyNumber,
		SellerID:                    s.SellerID,
		SellerName:                  s.SellerName,
		Status:                      s.Status,
		HandoffMethod:               s.HandoffMethod,
		CarrierName:                 s.CarrierName,
		TrackingNumber:              s.TrackingNumber,
		ExpectedArrivalDate:         s.ExpectedArrivalDate,
		QRToken:                     s.QRToken,
		CreatedAt:                   s.CreatedAt,
		ShippedAt:                   s.ShippedAt,
		ArrivedAt:                   s.ArrivedAt,
		ReceivingStartedAt:          s.ReceivingStartedAt,
		CompletedAt:                 s.CompletedAt,
		UpdatedAt:                   s.UpdatedAt,
		TotalExpectedItems:          s.TotalExpectedItems,
		TotalAcceptedItems:          s.TotalAcceptedItems,
		TotalRemainingItems:         s.TotalRemainingItems,
		TotalExpectedBoxes:          s.TotalExpectedBoxes,
		SKUCount:                    s.SKUCount,
		DiscrepancyQuantity:         s.DiscrepancyQuantity,
		DiscrepancyCount:            s.DiscrepancyCount,
		IsReceivingComplete:         s.IsReceivingComplete,
		AdditionalReceivingHappened: s.AdditionalReceivingHappened,
		Boxes:                       s.Boxes,
	}
	if s.Items != nil {
		resp.Items = make([]SellerSupplyItemResponse, len(s.Items))
		for i, it := range s.Items {
			resp.Items[i] = SellerSupplyItemResponse{
				ID:                it.ID,
				SupplyID:          it.SupplyID,
				VariantID:         it.VariantID,
				ExpectedQuantity:  it.ExpectedQuantity,
				AcceptedQuantity:  it.AcceptedQuantity,
				DamagedQuantity:   it.DamagedQuantity,
				MissingQuantity:   it.MissingQuantity,
				ExtraQuantity:     it.ExtraQuantity,
				RemainingQuantity: it.RemainingQuantity,
				CreatedAt:         it.CreatedAt,
				UpdatedAt:         it.UpdatedAt,
				SKU:               it.SKU,
				SellerSKU:         it.SellerSKU,
				ProductTitle:      it.ProductTitle,
				ColorName:         it.ColorName,
				SizeName:          it.SizeName,
				Barcode:           it.Barcode,
			}
		}
	}
	return resp
}

// ToSellerSupplyListResponse converts a list of domain Supply models into sanitized SellerSupplyResponses.
func ToSellerSupplyListResponse(supplies []Supply) []SellerSupplyResponse {
	res := make([]SellerSupplyResponse, len(supplies))
	for i := range supplies {
		res[i] = *ToSellerSupplyResponse(&supplies[i])
	}
	return res
}
