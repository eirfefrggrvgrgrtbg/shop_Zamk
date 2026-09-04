package inventory

import (
	"time"

	"github.com/google/uuid"
)

type ReceiptRequest struct {
	ProductVariantID uuid.UUID `json:"productVariantId" validate:"required"`
	Quantity         int       `json:"quantity" validate:"required,gt=0"`
	Reason           *string   `json:"reason,omitempty"`
}

type AdjustmentRequest struct {
	ProductVariantID uuid.UUID `json:"productVariantId" validate:"required"`
	Quantity         int       `json:"quantity" validate:"required"` // Can be negative or positive
	Reason           string    `json:"reason" validate:"required"`
}

type WriteOffRequest struct {
	ProductVariantID uuid.UUID `json:"productVariantId" validate:"required"`
	Quantity         int       `json:"quantity" validate:"required,gt=0"` // Always positive, implies subtraction
	Reason           string    `json:"reason" validate:"required"`
}

type AggregateStock struct {
	Total     int `json:"total"`
	Reserved  int `json:"reserved"`
	Available int `json:"available"`
}

type PhysicalStock struct {
	Warehouse      int `json:"warehouse"`
	Allocated      int `json:"allocated"`
	Picked         int `json:"picked"`
	Free           int `json:"free"`
	Expected       int `json:"expected"`
	Damaged        int `json:"damaged"`
	WrittenOff     int `json:"writtenOff"`
	Shipped        int `json:"shipped"`
	StaleAllocated int `json:"staleAllocated,omitempty"`
}

type LegacyStock struct {
	OnHand    int `json:"onHand"`
	Reserved  int `json:"reserved"`
	Available int `json:"available"`
}

type InventoryHealth struct {
	Status string   `json:"status"` // 'healthy', 'warning', 'critical'
	Issues []string `json:"issues"` // 'physical_exceeds_aggregate', 'allocated_exceeds_physical', etc.
}

type ProductInfo struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	Slug         string    `json:"slug"`
	MainImageURL *string   `json:"mainImageUrl,omitempty"`
}

type VariantInfo struct {
	ID        uuid.UUID `json:"id"`
	SKU       string    `json:"sku"`
	SellerSKU string    `json:"sellerSku,omitempty"`
	Barcode   string    `json:"barcode,omitempty"`
	Size      string    `json:"size,omitempty"`
	Color     string    `json:"color,omitempty"`
	Label     string    `json:"label"`
}

type SellerInfo struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type AdminInventoryItem struct {
	ID               uuid.UUID `json:"id"`
	ProductID        uuid.UUID `json:"productId"`
	ProductVariantID uuid.UUID `json:"productVariantId"`
	ProductTitle     string    `json:"productTitle"`
	VariantLabel     string    `json:"variant"`
	SellerID         uuid.UUID `json:"sellerId"`
	SellerName       string    `json:"sellerName"`
	Source           string    `json:"source"` // 'auction_direct_sale' or 'seller'
	TotalStock       int       `json:"totalStock"`
	ReservedStock    int       `json:"reservedStock"`
	AvailableStock   int       `json:"availableStock"`
	CreatedAt        string    `json:"createdAt"`
	UpdatedAt        string    `json:"updatedAt"`

	// Canonical P0.1 Inventory Read Model
	Product        ProductInfo     `json:"product"`
	Variant        VariantInfo     `json:"variantInfo"`
	Seller         SellerInfo      `json:"seller"`
	Aggregate      AggregateStock  `json:"aggregate"`
	Physical       PhysicalStock   `json:"physical"`
	Legacy         LegacyStock     `json:"legacy"`
	AccountingMode string                       `json:"accountingMode"` // 'serialized', 'mixed', 'legacy'
	Health         InventoryHealth              `json:"health"`
	PhysicalUnits  []AdminInventoryPhysicalUnit `json:"physicalUnits,omitempty"`
}

type AdminInventoryAllocationInfo struct {
	ID                uuid.UUID  `json:"id"`
	OrderID           uuid.UUID  `json:"orderId"`
	OrderNumber       string     `json:"orderNumber"`
	OrderStatus       string     `json:"orderStatus"`
	FulfillmentID     *uuid.UUID `json:"fulfillmentId,omitempty"`
	FulfillmentStatus *string    `json:"fulfillmentStatus,omitempty"`
	PickedAt          *time.Time `json:"pickedAt,omitempty"`
}

type AdminInventorySupplyLineage struct {
	SupplyID     uuid.UUID  `json:"supplyId"`
	SupplyNumber string     `json:"supplyNumber"`
	SupplyStatus string     `json:"supplyStatus"`
	ReceivedAt   *time.Time `json:"receivedAt,omitempty"`
}

type AdminInventoryPhysicalUnit struct {
	ID                uuid.UUID                     `json:"id"`
	UnitCode          string                        `json:"unitCode"`
	Status            string                        `json:"status"` // 'expected', 'warehouse', 'damaged', 'written_off', 'shipped'
	CreatedAt         time.Time                     `json:"createdAt"`
	Availability      string                        `json:"availability"` // 'free', 'allocated', 'picked', 'unavailable_expected', 'unavailable_damaged', 'unavailable_written_off', 'unavailable_shipped'
	IsStaleAllocation bool                          `json:"isStaleAllocation"`
	LiveAllocation    *AdminInventoryAllocationInfo `json:"liveAllocation,omitempty"`
	StaleAllocation   *AdminInventoryAllocationInfo `json:"staleAllocation,omitempty"`
	SupplyLineage     *AdminInventorySupplyLineage  `json:"supplyLineage,omitempty"`
}

type AdminInventoryUnitTraceability struct {
	Identity          AdminInventoryUnitIdentity        `json:"identity"`
	CurrentState      AdminInventoryUnitCurrentState    `json:"currentState"`
	Origin            *AdminInventorySupplyLineage      `json:"origin,omitempty"`
	CurrentContext    AdminInventoryUnitContext         `json:"currentContext"`
	Timeline          []AdminInventoryUnitTimelineEvent `json:"timeline"`
	HasPartialHistory bool                              `json:"hasPartialHistory"`
}

type AdminInventoryUnitIdentity struct {
	ID           uuid.UUID `json:"id"`
	UnitCode     string    `json:"unitCode"`
	VariantID    uuid.UUID `json:"variantId"`
	ProductID    uuid.UUID `json:"productId"`
	ProductTitle string    `json:"productTitle"`
	VariantName  string    `json:"variantName"`
	SKU          string    `json:"sku"`
	Barcode      string    `json:"barcode"`
	Size         string    `json:"size"`
	Color        string    `json:"color"`
	SellerID     uuid.UUID `json:"sellerId"`
	SellerName   string    `json:"sellerName"`
	Source       string    `json:"source"`
}

type AdminInventoryUnitCurrentState struct {
	Status            string `json:"status"`            // "warehouse", "expected", "damaged", "written_off", "shipped"
	Availability      string `json:"availability"`      // "free", "allocated", "picked", "unavailable_expected", ...
	Location          string `json:"location"`          // "Не ведётся"
	IsStaleAllocation bool   `json:"isStaleAllocation"`
	HealthIssue       string `json:"healthIssue,omitempty"` // "stale_active_allocation"
}

type AdminInventoryUnitContext struct {
	LiveAllocation  *AdminInventoryAllocationInfo `json:"liveAllocation,omitempty"`
	StaleAllocation *AdminInventoryAllocationInfo `json:"staleAllocation,omitempty"`
}

type AdminInventoryUnitTimelineEvent struct {
	ID              string     `json:"id"`
	Type            string     `json:"type"`                      // "inbound_created", "received", "allocation_created", "picked", "packed", "shipped", "delivered", "return_requested", "return_approved", "return_receiving_started", "return_unit_scanned", "return_received", "return_damaged", "allocation_released", "damaged", "written_off"
	Category        string     `json:"category"`                  // "physical", "commitment", "operation", "order_lifecycle", "diagnostic"
	EventName       string     `json:"eventName"`                 // Human Russian event name
	Description     string     `json:"description"`               // Short explanation
	Timestamp       time.Time  `json:"timestamp"`
	SourceEntity    string     `json:"sourceEntity"`              // "seller_supplies", "orders", "order_fulfillments", "shipments", "returns", "order_item_allocations"
	ReferenceNumber string     `json:"referenceNumber,omitempty"` // "SUP-001197", "ORD-100193", "RET-..."
	ReferenceID     *uuid.UUID `json:"referenceId,omitempty"`
	ActorRole       string     `json:"actorRole,omitempty"`       // "system", "staff", "customer", "seller"
	ActorName       string     `json:"actorName,omitempty"`       // Staff name or email
	Link            string     `json:"link,omitempty"`            // Deep link path
}

type PhysicalUnitContext struct {
	UnitCode     string    `json:"unitCode"`
	Status       string    `json:"status"`
	StatusLabel  string    `json:"statusLabel"`
	ProductTitle string    `json:"productTitle"`
	VariantLabel string    `json:"variant"`
	SKU          string    `json:"sku"`
	ProductID    uuid.UUID `json:"productId"`
	VariantID    uuid.UUID `json:"variantId"`
}

type AdminInventoryListResponse struct {
	Items       []AdminInventoryItem `json:"items"`
	TotalCount  int                  `json:"totalCount"`
	IssuesCount int                  `json:"issuesCount"`
	UnitContext *PhysicalUnitContext `json:"unitContext,omitempty"`
}

type InventoryListResponse struct {
	Items      []Item `json:"items"`
	TotalCount int    `json:"totalCount"`
}

type SellerInventoryItem struct {
	VariantID          uuid.UUID              `json:"variantId"`
	ProductID          uuid.UUID              `json:"productId"`
	ProductTitle       string                 `json:"productTitle"`
	Image              *string                `json:"image,omitempty"`
	OptionValues       map[string]interface{} `json:"optionValues,omitempty"`
	SKU                string                 `json:"sku"`
	OnHand             int                    `json:"onHand"`
	Reserved           int                    `json:"reserved"`
	Available          int                    `json:"available"`
	Inbound            int                    `json:"inbound"`
	AvailabilityStatus string                 `json:"availabilityStatus"`
}

type SellerInventoryListResponse struct {
	Items      []SellerInventoryItem `json:"items"`
	TotalCount int                   `json:"totalCount"`
}

type StockMovementsListResponse struct {
	Items      []StockMovement `json:"items"`
	TotalCount int             `json:"totalCount"`
}

type UnifiedAdjustmentRequest struct {
	Type      string `json:"type" validate:"required,oneof=receipt adjustment write_off"`
	Quantity  int    `json:"quantity" validate:"required,gt=0"`
	Reason    string `json:"reason" validate:"required"`
	Reference string `json:"reference,omitempty"`
}

type AdminInventoryReservationResponse struct {
	Items []interface{} `json:"items"`
}

func EvaluateInventoryHealth(agg AggregateStock, phys PhysicalStock, leg LegacyStock) (string, InventoryHealth) {
	var issues []string
	status := "healthy"

	if phys.StaleAllocated > 0 {
		issues = append(issues, "stale_active_allocation")
		if status != "critical" {
			status = "warning"
		}
	}
	if phys.Allocated > phys.Warehouse {
		issues = append(issues, "allocated_exceeds_physical")
		status = "critical"
	}
	if phys.Picked > phys.Allocated {
		issues = append(issues, "picked_exceeds_allocated")
		status = "critical"
	}
	if phys.Warehouse > agg.Total {
		issues = append(issues, "physical_exceeds_aggregate")
		if status != "critical" {
			status = "warning"
		}
	}
	if phys.Allocated > agg.Reserved {
		issues = append(issues, "serialized_allocations_exceed_reserved")
		if status != "critical" {
			status = "warning"
		}
	}
	if agg.Reserved > agg.Total {
		issues = append(issues, "reserved_exceeds_total")
		status = "critical"
	}
	if agg.Available < 0 {
		issues = append(issues, "aggregate_available_negative")
		status = "critical"
	}
	if leg.OnHand < 0 {
		issues = append(issues, "legacy_projection_negative")
		if status != "critical" {
			status = "warning"
		}
	}
	if leg.Reserved < 0 {
		issues = append(issues, "legacy_reserved_negative")
		if status != "critical" {
			status = "warning"
		}
	}
	if leg.Available < 0 {
		issues = append(issues, "legacy_available_negative")
		if status != "critical" {
			status = "warning"
		}
	}

	mode := "legacy"
	if phys.Warehouse > 0 && leg.OnHand <= 0 {
		mode = "serialized"
	} else if phys.Warehouse > 0 && leg.OnHand > 0 {
		mode = "mixed"
	} else {
		mode = "legacy"
	}

	if len(issues) == 0 {
		issues = []string{}
	}

	return mode, InventoryHealth{
		Status: status,
		Issues: issues,
	}
}

type StartReconciliationRequest struct {
	VariantID uuid.UUID `json:"variantId"`
}

type ReconciliationSessionDTO struct {
	ID        uuid.UUID  `json:"id"`
	VariantID uuid.UUID  `json:"variantId"`
	VariantTitle string `json:"variantTitle"`
	VariantSize string `json:"variantSize"`
	VariantColor string `json:"variantColor"`
	VariantSKU string `json:"variantSKU"`
	VariantBarcode string `json:"variantBarcode"`
	AccountingMode string `json:"accountingMode"`
	LegacyOnHand int `json:"legacyOnHand"`

	Status    string     `json:"status"` // 'in_progress', 'completed', 'cancelled'
	StartedBy uuid.UUID  `json:"startedBy"`
	StartedAt time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	CompletedBy *uuid.UUID `json:"completedBy,omitempty"`
	CancelledAt *time.Time `json:"cancelledAt,omitempty"`
	CancelledBy *uuid.UUID `json:"cancelledBy,omitempty"`
	Notes       string     `json:"notes,omitempty"`

	// Live counters for active session
	ExpectedCount     int `json:"expectedCount"`
	FoundExpectedCount int `json:"foundExpectedCount"`
	UnexpectedCount   int `json:"unexpectedCount"`
	ProblemsCount     int `json:"problemsCount"`
}

type ScanReconciliationRequest struct {
	RawCode string `json:"rawCode"`
}

type ReconciliationScanUnitContext struct {
	UnitCode     string `json:"unitCode"`
	ProductTitle string `json:"productTitle"`
	Size         string `json:"size,omitempty"`
	Color        string `json:"color,omitempty"`
	SKU          string `json:"sku,omitempty"`
	Barcode      string `json:"barcode,omitempty"`
	Status       string `json:"status"`
}

type ScanReconciliationResponse struct {
	Classification string                         `json:"classification"`
	Unit           *AdminInventoryPhysicalUnit    `json:"unit,omitempty"`
	UnitContext    *ReconciliationScanUnitContext `json:"unitContext,omitempty"`
	Session        ReconciliationSessionDTO       `json:"session"`
}

type ListReconciliationSessionsResponse struct {
	Items []ReconciliationSessionDTO `json:"items"`
}

type ReconciliationReviewItemDTO struct {
	UnitID         uuid.UUID  `json:"unitId"`
	UnitCode       string     `json:"unitCode"`
	SnapshotStatus string     `json:"snapshotStatus"`
	CurrentStatus  string     `json:"currentStatus"`
	Classification string     `json:"classification"`
	ScannedAt      *time.Time `json:"scannedAt"`
}

type ReconciliationReviewDTO struct {
	ExpectedFound      []ReconciliationReviewItemDTO `json:"expectedFound"`
	Missing            []ReconciliationReviewItemDTO `json:"missing"`
	UnexpectedFound    []ReconciliationReviewItemDTO `json:"unexpectedFound"`
	ChangedDuringCount []ReconciliationReviewItemDTO `json:"changedDuringCount"`
}
