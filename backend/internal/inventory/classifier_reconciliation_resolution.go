package inventory

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

type RawResolutionFact struct {
	UnitID         uuid.UUID
	UnitCode       string
	VariantID      uuid.UUID
	ProductTitle   string
	VariantSize    string
	VariantColor   string
	VariantSKU     string
	VariantBarcode string
	SnapshotStatus *string // nil if not in expected units
	CurrentStatus  string  // from inventory_units
	Classification *string // nil if not scanned; 'expected_found', 'unexpected_found', etc.
	ScannedAt      *time.Time

	// Allocation context
	AllocationID            *uuid.UUID
	AllocationPickedAt      *time.Time
	AllocationReleasedAt    *time.Time
	AllocationReleaseReason *string
	OrderID                 *uuid.UUID
	OrderNumber             *string
	OrderStatus             *string
	FulfillmentID           *uuid.UUID
	FulfillmentStatus       *string
	ShipmentID              *uuid.UUID
	ShipmentStatus          *string
	ReturnID                *uuid.UUID
	ReturnStatus            *string

	// Supply context
	OriginSupplyID     *uuid.UUID
	OriginSupplyNumber *string
	OriginSupplyStatus *string
}

func formatRuStatus(status string) string {
	switch status {
	case "warehouse":
		return "На складе"
	case "expected":
		return "Ожидается"
	case "shipped":
		return "Отгружен"
	case "damaged":
		return "Брак"
	case "written_off":
		return "Списан"
	case "delivered":
		return "Доставлен"
	case "cancelled":
		return "Отменён"
	case "paid":
		return "Оплачен"
	case "assembling":
		return "Собирается"
	case "packed":
		return "Упакован"
	case "rejected":
		return "Отклонён"
	case "needs_info":
		return "Требует уточнения"
	case "approved":
		return "Одобрен"
	default:
		return "Статус не определён"
	}
}

func isTerminalOrder(status *string) bool {
	if status == nil {
		return false
	}
	switch *status {
	case "delivered", "cancelled", "returned", "refunded":
		return true
	}
	return false
}

func isTerminalFulfillment(status *string) bool {
	if status == nil {
		return false
	}
	switch *status {
	case "delivered", "cancelled", "returned", "refunded":
		return true
	}
	return false
}

// ClassifyResolutionFact evaluates a raw resolution fact against the canonical rules A-K.
// Returns nil if the fact is not a discrepancy (e.g. cleanly expected_found, or excluded wrong_variant/duplicate/unknown_code).
func ClassifyResolutionFact(fact RawResolutionFact) *ReconciliationResolutionCase {
	// Exclude non-actionable or excluded scans:
	if fact.Classification != nil {
		switch *fact.Classification {
		case "wrong_variant", "unknown_code", "duplicate":
			return nil
		}
	}

	// Build common variant context
	varCtx := ReconciliationVariantContext{
		ProductTitle: fact.ProductTitle,
		Size:         fact.VariantSize,
		Color:        fact.VariantColor,
		SKU:          fact.VariantSKU,
		Barcode:      fact.VariantBarcode,
	}

	// Build historical context
	var histCtx *ReconciliationHistoricalContext
	if fact.OrderID != nil || fact.AllocationID != nil || fact.OriginSupplyID != nil {
		ordNum := ""
		if fact.OrderNumber != nil {
			ordNum = *fact.OrderNumber
		}
		ordStat := ""
		if fact.OrderStatus != nil {
			ordStat = *fact.OrderStatus
		}
		fulStat := ""
		if fact.FulfillmentStatus != nil {
			fulStat = *fact.FulfillmentStatus
		}
		shipStat := ""
		if fact.ShipmentStatus != nil {
			shipStat = *fact.ShipmentStatus
		}
		retStat := ""
		if fact.ReturnStatus != nil {
			retStat = *fact.ReturnStatus
		}
		supNum := ""
		if fact.OriginSupplyNumber != nil {
			supNum = *fact.OriginSupplyNumber
		}
		relReason := ""
		if fact.AllocationReleaseReason != nil {
			relReason = *fact.AllocationReleaseReason
		}

		histCtx = &ReconciliationHistoricalContext{
			OrderID:           fact.OrderID,
			OrderNumber:       ordNum,
			OrderStatus:       ordStat,
			FulfillmentID:     fact.FulfillmentID,
			FulfillmentStatus: fulStat,
			ShipmentID:        fact.ShipmentID,
			ShipmentStatus:    shipStat,
			ReturnID:          fact.ReturnID,
			ReturnStatus:      retStat,
			SupplyID:          fact.OriginSupplyID,
			SupplyNumber:      supNum,
			AllocationID:      fact.AllocationID,
			PickedAt:          fact.AllocationPickedAt,
			ReleasedAt:        fact.AllocationReleasedAt,
			ReleaseReason:     relReason,
		}
	}

	snapStatusStr := ""
	if fact.SnapshotStatus != nil {
		snapStatusStr = *fact.SnapshotStatus
	}

	baseCase := ReconciliationResolutionCase{
		UnitID:            fact.UnitID,
		UnitCode:          fact.UnitCode,
		VariantID:         fact.VariantID,
		Variant:           varCtx,
		SnapshotStatus:    snapStatusStr,
		CurrentStatus:     fact.CurrentStatus,
		HistoricalContext: histCtx,
		AllowedActions:    []ReconciliationResolutionAction{},
	}

	if fact.OrderNumber != nil && *fact.OrderNumber != "" {
		ordStat := ""
		if fact.OrderStatus != nil {
			ordStat = *fact.OrderStatus
		}
		if ordStat != "" {
			baseCase.CurrentAllocationCtx = fmt.Sprintf("Заказ %s · %s", *fact.OrderNumber, formatRuStatus(ordStat))
		} else {
			baseCase.CurrentAllocationCtx = fmt.Sprintf("Заказ %s", *fact.OrderNumber)
		}
	}
	if fact.OriginSupplyNumber != nil && *fact.OriginSupplyNumber != "" {
		baseCase.LineageCtx = fmt.Sprintf("Поставка %s", *fact.OriginSupplyNumber)
	}

	// 1. RULE H: changed_during_count takes PRECEDENCE over missing/unexpected.
	// If snapshot status was recorded, and current status in DB differs:
	if fact.SnapshotStatus != nil && *fact.SnapshotStatus != fact.CurrentStatus {
		baseCase.CaseType = CaseTypeChangedDuringCount
		baseCase.Title = "Состояние изменилось во время проверки"
		baseCase.Severity = SeverityWarning
		baseCase.Explanation = fmt.Sprintf(
			"При фиксации остатков единица имела статус «%s», но сейчас её статус в системе — «%s». Изменение произошло в процессе работы склада.",
			formatRuStatus(*fact.SnapshotStatus),
			formatRuStatus(fact.CurrentStatus),
		)
		baseCase.AllowedActions = []ReconciliationResolutionAction{
			{
				ID:          ActionIDNoActionStateChanged,
				SafetyLevel: ActionSafetyNavigation,
				Label:       "Изменено в процессе работы",
				Route:       "/inventory",
				Enabled:     true,
			},
			{
				ID:          ActionIDOpenUnitHistory,
				SafetyLevel: ActionSafetyWorkflowHandoff,
				Label:       "История движения",
				Route:       "/inventory",
				Enabled:     true,
			},
		}
		return &baseCase
	}

	// Check if this was an expected_found scan where status did NOT change:
	// Clean match -> not a resolution discrepancy.
	if fact.Classification != nil && *fact.Classification == "expected_found" {
		return nil
	}

	// Analyze allocation status: LIVE, STALE, PICKED_NOT_SHIPPED
	hasUnreleasedAlloc := fact.AllocationID != nil && fact.AllocationReleasedAt == nil
	allocIsStale := hasUnreleasedAlloc && (isTerminalOrder(fact.OrderStatus) || isTerminalFulfillment(fact.FulfillmentStatus))
	allocIsLive := hasUnreleasedAlloc && !allocIsStale
	allocIsPicked := allocIsLive && fact.AllocationPickedAt != nil

	// 2. Unexpected physically scanned units (Case D, E, F, G, or unexpected_free)
	if fact.Classification != nil && *fact.Classification == "unexpected_found" {
		switch fact.CurrentStatus {
		case "expected":
			// Case D: expected_found
			baseCase.CaseType = CaseTypeExpectedFound
			baseCase.Title = "Приёмка единицы не завершена"
			baseCase.Severity = SeverityInfo
			baseCase.Explanation = "Единица найдена, но её приёмка ещё не завершена."
			baseCase.AllowedActions = []ReconciliationResolutionAction{
				{
					ID:          ActionIDOpenReceiving,
					SafetyLevel: ActionSafetyWorkflowHandoff,
					Label:       "Перейти в приёмку",
					Route:       fmt.Sprintf("/warehouse/free-scan?unitCode=%s", fact.UnitCode),
					Enabled:     true,
				},
			}
			if fact.OriginSupplyID != nil {
				supNum := ""
				if fact.OriginSupplyNumber != nil {
					supNum = " " + *fact.OriginSupplyNumber
				}
				baseCase.AllowedActions = append(baseCase.AllowedActions, ReconciliationResolutionAction{
					ID:          ActionIDOpenSupply,
					SafetyLevel: ActionSafetyWorkflowHandoff,
					Label:       "Открыть поставку" + supNum,
					Route:       "/supplies/receiving",
					Enabled:     true,
				})
			}
			return &baseCase

		case "shipped":
			// Case E: shipped_found
			baseCase.CaseType = CaseTypeShippedFound
			baseCase.Title = "Найдена, хотя числится отгруженной"
			baseCase.Severity = SeverityCritical
			ordNum := ""
			if fact.OrderNumber != nil && *fact.OrderNumber != "" {
				ordNum = " по заказу " + *fact.OrderNumber
			}
			baseCase.Explanation = fmt.Sprintf("Единица числится отгруженной%s, однако была физически обнаружена на складе.", ordNum)
			baseCase.BlockedReason = "Требуется ручная проверка отгрузки. Автоматическое исправление недоступно."
			baseCase.AllowedActions = []ReconciliationResolutionAction{
				{
					ID:            ActionIDInvestigateShippedFound,
					SafetyLevel:   ActionSafetyBlocked,
					Label:         "Требуется ручная проверка отгрузки",
					BlockedReason: "Автоматическое исправление недоступно.",
					Enabled:       false,
				},
				{
					ID:          ActionIDOpenUnitHistory,
					SafetyLevel: ActionSafetyWorkflowHandoff,
					Label:       "История движения",
					Route:       "/inventory",
					Enabled:     true,
				},
			}
			if fact.OrderID != nil {
				orderLabel := "Открыть заказ"
				if fact.OrderNumber != nil && *fact.OrderNumber != "" {
					orderLabel += " " + *fact.OrderNumber
				}
				baseCase.AllowedActions = append([]ReconciliationResolutionAction{
					{
						ID:          ActionIDOpenOrder,
						SafetyLevel: ActionSafetyWorkflowHandoff,
						Label:       orderLabel,
						Route:       fmt.Sprintf("/orders/%s", fact.OrderID.String()),
						Enabled:     true,
					},
				}, baseCase.AllowedActions...)
			}

			return &baseCase

		case "damaged":
			// Case F: damaged_found
			baseCase.CaseType = CaseTypeDamagedFound
			baseCase.Title = "Найдена бракованная единица"
			baseCase.Severity = SeverityWarning
			baseCase.Explanation = "Единица числится браком и была физически найдена."
			baseCase.AllowedActions = []ReconciliationResolutionAction{
				{
					ID:          ActionIDOpenUnitHistory,
					SafetyLevel: ActionSafetyWorkflowHandoff,
					Label:       "История движения",
					Route:       "/inventory",
					Enabled:     true,
				},
			}
			return &baseCase

		default:
			// Unit on warehouse
			if allocIsStale {
				// Case G: stale_allocation
				baseCase.CaseType = CaseTypeStaleAllocation
				baseCase.Title = "Найдена со старым назначением"
				baseCase.Severity = SeverityWarning
				ordNum := ""
				if fact.OrderNumber != nil {
					ordNum = " " + *fact.OrderNumber
				}
				baseCase.Explanation = fmt.Sprintf("Единица фактически найдена на складе, но за ней числится неснятое назначение по завершённому или отменённому заказу%s.", ordNum)
				baseCase.AllowedActions = []ReconciliationResolutionAction{
					{
						ID:            ActionIDCloseStaleAllocation,
						SafetyLevel:   ActionSafetyMutationRequiresConfirmation,
						Label:         "Освободить зависшее назначение",
						BlockedReason: "Автоматическое освобождение аллокации будет доступно в P2.2B",
						Enabled:       false,
					},
					{
						ID:          ActionIDOpenUnitHistory,
						SafetyLevel: ActionSafetyWorkflowHandoff,
						Label:       "История движения",
						Route:       "/inventory",
						Enabled:     true,
					},
				}
				if fact.OrderID != nil {
					baseCase.AllowedActions = append([]ReconciliationResolutionAction{
						{
							ID:          ActionIDOpenOrder,
							SafetyLevel: ActionSafetyWorkflowHandoff,
							Label:       "Открыть заказ" + ordNum,
							Route:       fmt.Sprintf("/orders/%s", fact.OrderID.String()),
							Enabled:     true,
						},
					}, baseCase.AllowedActions...)
				}
				return &baseCase
			}

			// Unexpected free unit
			baseCase.CaseType = CaseTypeUnexpectedFree
			baseCase.Title = "Неожиданно найден свободный остаток"
			baseCase.Severity = SeverityInfo
			baseCase.Explanation = "Единица фактически найдена на складе, но не входила в список ожидаемых для данной проверки."
			baseCase.AllowedActions = []ReconciliationResolutionAction{
				{
					ID:          ActionIDOpenUnitHistory,
					SafetyLevel: ActionSafetyWorkflowHandoff,
					Label:       "История движения",
					Route:       "/inventory",
					Enabled:     true,
				},
			}
			return &baseCase
		}
	}

	// 3. Missing units (expected, but not scanned)
	if fact.Classification == nil {
		if allocIsStale {
			// Case G: stale_allocation on missing unit
			baseCase.CaseType = CaseTypeStaleAllocation
			baseCase.Title = "Не найдена — старое назначение"
			baseCase.Severity = SeverityHigh
			ordNum := ""
			if fact.OrderNumber != nil {
				ordNum = " " + *fact.OrderNumber
			}
			baseCase.Explanation = fmt.Sprintf("Единица не найдена при проверке. За ней числится неснятое назначение по завершённому или отменённому заказу%s.", ordNum)
			baseCase.AllowedActions = []ReconciliationResolutionAction{
				{
					ID:          ActionIDRecount,
					SafetyLevel: ActionSafetyWorkflowHandoff,
					Label:       "Перепроверить ZMU",
					Route:       fmt.Sprintf("/warehouse/free-scan?unitCode=%s", fact.UnitCode),
					Enabled:     true,
				},
				{
					ID:            ActionIDCloseStaleAllocation,
					SafetyLevel:   ActionSafetyMutationRequiresConfirmation,
					Label:         "Освободить зависшее назначение",
					BlockedReason: "Автоматическое освобождение аллокации будет доступно в P2.2B",
					Enabled:       false,
				},
				{
					ID:            ActionIDConfirmMissing,
					SafetyLevel:   ActionSafetyMutationRequiresConfirmation,
					Label:         "Списать недостачу",
					BlockedReason: "Списание недостачи будет доступно в P2.2B",
					Enabled:       false,
				},
			}
			if fact.OrderID != nil {
				baseCase.AllowedActions = append([]ReconciliationResolutionAction{
					{
						ID:          ActionIDOpenOrder,
						SafetyLevel: ActionSafetyWorkflowHandoff,
						Label:       "Открыть заказ" + ordNum,
						Route:       fmt.Sprintf("/orders/%s", fact.OrderID.String()),
						Enabled:     true,
					},
				}, baseCase.AllowedActions...)
			}
			return &baseCase
		}

		if allocIsPicked {
			// Case C: missing_picked_not_shipped
			baseCase.CaseType = CaseTypeMissingPickedNotShipped
			baseCase.Title = "Не найдена — уже собрана в заказ"
			baseCase.Severity = SeverityCritical
			ordNum := ""
			if fact.OrderNumber != nil {
				ordNum = " " + *fact.OrderNumber
			}
			baseCase.Explanation = fmt.Sprintf("Единица была отобрана при сборке, но не найдена при инвентаризации. Заказ%s ожидает упаковки или отгрузки.", ordNum)
			baseCase.AllowedActions = []ReconciliationResolutionAction{
				{
					ID:          ActionIDRecount,
					SafetyLevel: ActionSafetyWorkflowHandoff,
					Label:       "Перепроверить ZMU",
					Route:       fmt.Sprintf("/warehouse/free-scan?unitCode=%s", fact.UnitCode),
					Enabled:     true,
				},
				{
					ID:            ActionIDConfirmMissing,
					SafetyLevel:   ActionSafetyMutationRequiresConfirmation,
					Label:         "Списать недостачу",
					BlockedReason: "Списание недостачи будет доступно в P2.2B",
					Enabled:       false,
				},
			}
			if fact.OrderID != nil {
				baseCase.AllowedActions = append([]ReconciliationResolutionAction{
					{
						ID:          ActionIDOpenOrder,
						SafetyLevel: ActionSafetyWorkflowHandoff,
						Label:       "Открыть заказ" + ordNum,
						Route:       fmt.Sprintf("/orders/%s", fact.OrderID.String()),
						Enabled:     true,
					},
				}, baseCase.AllowedActions...)
			}
			if fact.FulfillmentID != nil {
				baseCase.AllowedActions = append(baseCase.AllowedActions, ReconciliationResolutionAction{
					ID:          ActionIDInspectAllocation,
					SafetyLevel: ActionSafetyWorkflowHandoff,
					Label:       "Перейти в сборку",
					Route:       fmt.Sprintf("/fulfillment/picking/%s", fact.FulfillmentID.String()),
					Enabled:     true,
				})
			}
			return &baseCase
		}

		if allocIsLive {
			// Case B: missing_live_allocated
			baseCase.CaseType = CaseTypeMissingLiveAllocated
			baseCase.Title = "Не найдена — назначена заказу"
			baseCase.Severity = SeverityHigh
			ordNum := ""
			if fact.OrderNumber != nil {
				ordNum = " " + *fact.OrderNumber
			}
			baseCase.Explanation = fmt.Sprintf("Единица не найдена, но назначена активному заказу%s. Заказ не может быть скомплектован.", ordNum)
			baseCase.AllowedActions = []ReconciliationResolutionAction{
				{
					ID:          ActionIDRecount,
					SafetyLevel: ActionSafetyWorkflowHandoff,
					Label:       "Перепроверить ZMU",
					Route:       fmt.Sprintf("/warehouse/free-scan?unitCode=%s", fact.UnitCode),
					Enabled:     true,
				},
				{
					ID:            ActionIDConfirmMissing,
					SafetyLevel:   ActionSafetyMutationRequiresConfirmation,
					Label:         "Списать недостачу",
					BlockedReason: "Списание недостачи будет доступно в P2.2B",
					Enabled:       false,
				},
			}
			if fact.OrderID != nil {
				baseCase.AllowedActions = append([]ReconciliationResolutionAction{
					{
						ID:          ActionIDOpenOrder,
						SafetyLevel: ActionSafetyWorkflowHandoff,
						Label:       "Открыть заказ" + ordNum,
						Route:       fmt.Sprintf("/orders/%s", fact.OrderID.String()),
						Enabled:     true,
					},
				}, baseCase.AllowedActions...)
			}
			return &baseCase
		}

		// Case A: missing_free
		baseCase.CaseType = CaseTypeMissingFree
		baseCase.Title = "Единица не найдена"
		baseCase.Severity = SeverityWarning
		baseCase.Explanation = "Ожидаемая единица товара не найдена на складе."
		baseCase.AllowedActions = []ReconciliationResolutionAction{
			{
				ID:          ActionIDRecount,
				SafetyLevel: ActionSafetyWorkflowHandoff,
				Label:       "Перепроверить ZMU",
				Route:       fmt.Sprintf("/warehouse/free-scan?unitCode=%s", fact.UnitCode),
				Enabled:     true,
			},
			{
				ID:            ActionIDConfirmMissing,
				SafetyLevel:   ActionSafetyMutationRequiresConfirmation,
				Label:         "Списать недостачу",
				BlockedReason: "Списание недостачи будет доступно в P2.2B",
				Enabled:       false,
			},
		}
		return &baseCase
	}

	return nil
}
