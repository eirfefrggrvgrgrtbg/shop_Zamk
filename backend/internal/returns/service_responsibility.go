package returns

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SetReturnResponsibilityAllocationRequest struct {
	AllocationID          uuid.UUID  `json:"allocationId"`
	Quantity              int        `json:"quantity"`
	OrderItemAllocationID *uuid.UUID `json:"orderItemAllocationId"`
	Status                string     `json:"status"`
	ResponsibleParty      *string    `json:"responsibleParty"`
	ReasonCode            *string    `json:"reasonCode"`
	DecisionSource        *string    `json:"decisionSource"`
	InternalNote          *string    `json:"internalNote"`
	DecidedAt             *time.Time `json:"decidedAt"`
	ActorID               *uuid.UUID `json:"actorId"`
}

var validResponsibilityStatuses = map[string]bool{
	ReturnResponsibilityStatusPending:     true,
	ReturnResponsibilityStatusNotRequired: true,
	ReturnResponsibilityStatusResolved:    true,
}

var validResponsibleParties = map[string]bool{
	ReturnResponsiblePartySeller:   true,
	ReturnResponsiblePartyZamk:     true,
	ReturnResponsiblePartyCarrier:  true,
	ReturnResponsiblePartyCustomer: true,
}

var validResponsibilityReasons = map[string]bool{
	ReturnResponsibilityReasonCustomerChangeOfMind: true,
	ReturnResponsibilityReasonSellerProductDefect:  true,
	ReturnResponsibilityReasonZamkWarehouseDamage:  true,
	ReturnResponsibilityReasonZamkFulfillmentError: true,
	ReturnResponsibilityReasonCarrierDamage:        true,
	ReturnResponsibilityReasonCustomerDamage:       true,
	ReturnResponsibilityReasonFraudOrSubstitution:  true,
	ReturnResponsibilityReasonUnknown:              true,
}

var validResponsibilityDecisionSources = map[string]bool{
	ReturnResponsibilityDecisionSourceSystem:   true,
	ReturnResponsibilityDecisionSourceEmployee: true,
}

func ValidateResponsibilityAllocationParams(req SetReturnResponsibilityAllocationRequest) error {
	if req.Quantity <= 0 {
		return ErrInvalidQuantity
	}
	if req.OrderItemAllocationID != nil && req.Quantity != 1 {
		return ErrInvalidQuantity
	}

	if !validResponsibilityStatuses[req.Status] {
		return ErrInvalidResponsibilityStatus
	}

	if req.ResponsibleParty != nil {
		if !validResponsibleParties[*req.ResponsibleParty] {
			return ErrInvalidResponsibleParty
		}
	}

	if req.ReasonCode != nil {
		if !validResponsibilityReasons[*req.ReasonCode] {
			return ErrInvalidResponsibilityReason
		}
	}

	if req.DecisionSource != nil && !validResponsibilityDecisionSources[*req.DecisionSource] {
		return ErrInvalidResponsibilityDecisionSource
	}

	switch req.Status {
	case ReturnResponsibilityStatusPending:
		if req.ResponsibleParty != nil || req.ReasonCode != nil || req.DecisionSource != nil || req.ActorID != nil || req.DecidedAt != nil {
			return ErrResponsibilityInvariants
		}
	case ReturnResponsibilityStatusNotRequired:
		if req.ResponsibleParty != nil || req.ReasonCode == nil || req.DecisionSource == nil || req.DecidedAt == nil {
			return ErrResponsibilityInvariants
		}
	case ReturnResponsibilityStatusResolved:
		if req.ResponsibleParty == nil || req.ReasonCode == nil || req.DecisionSource == nil || req.DecidedAt == nil {
			return ErrResponsibilityInvariants
		}
	}

	if req.DecisionSource != nil {
		switch *req.DecisionSource {
		case ReturnResponsibilityDecisionSourceEmployee:
			if req.ActorID == nil {
				return ErrResponsibilityActorRequired
			}
		case ReturnResponsibilityDecisionSourceSystem:
			if req.ActorID != nil {
				return ErrResponsibilityActorForbidden
			}
		}
	}

	if err := validateResponsibilityAllocationReasonParty(req); err != nil {
		return err
	}

	return nil
}

func validateResponsibilityAllocationReasonParty(req SetReturnResponsibilityAllocationRequest) error {
	if req.ReasonCode == nil {
		return nil
	}

	requireResolvedParty := func(expectedParty string) error {
		if req.Status != ReturnResponsibilityStatusResolved || req.ResponsibleParty == nil || *req.ResponsibleParty != expectedParty {
			return ErrResponsibilityReasonPartyMismatch
		}
		return nil
	}

	switch *req.ReasonCode {
	case ReturnResponsibilityReasonSellerProductDefect:
		return requireResolvedParty(ReturnResponsiblePartySeller)
	case ReturnResponsibilityReasonZamkWarehouseDamage, ReturnResponsibilityReasonZamkFulfillmentError:
		return requireResolvedParty(ReturnResponsiblePartyZamk)
	case ReturnResponsibilityReasonCarrierDamage:
		return requireResolvedParty(ReturnResponsiblePartyCarrier)
	case ReturnResponsibilityReasonCustomerDamage:
		return requireResolvedParty(ReturnResponsiblePartyCustomer)
	case ReturnResponsibilityReasonCustomerChangeOfMind:
		if req.Status != ReturnResponsibilityStatusNotRequired || req.ResponsibleParty != nil {
			return ErrResponsibilityReasonPartyMismatch
		}
	}

	return nil
}

func isSameAllocation(current *ReturnResponsibilityAllocation, req SetReturnResponsibilityAllocationRequest) bool {
	if current.Quantity != req.Quantity {
		return false
	}
	if !equalUUIDPtr(current.OrderItemAllocationID, req.OrderItemAllocationID) {
		return false
	}
	if current.Status != req.Status {
		return false
	}
	if !equalStringPtr(current.ResponsibleParty, req.ResponsibleParty) {
		return false
	}
	if !equalStringPtr(current.ReasonCode, req.ReasonCode) {
		return false
	}
	if !equalStringPtr(current.DecisionSource, req.DecisionSource) {
		return false
	}
	if !equalStringPtr(current.InternalNote, req.InternalNote) {
		return false
	}
	if !equalUUIDPtr(current.ActorID, req.ActorID) {
		return false
	}
	if !equalTimePtr(current.DecidedAt, req.DecidedAt) {
		return false
	}
	return true
}

func equalStringPtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalUUIDPtr(a, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalTimePtr(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func (s *Service) GetReturnResponsibilityAllocations(ctx context.Context, returnID uuid.UUID) ([]ReturnResponsibilityAllocation, error) {
	return s.repo.GetReturnResponsibilityAllocationsByReturnID(ctx, returnID)
}

func (s *Service) GetReturnResponsibilityAllocationHistory(ctx context.Context, allocationID uuid.UUID) ([]ReturnResponsibilityAllocationHistory, error) {
	return s.repo.GetReturnResponsibilityAllocationHistory(ctx, allocationID)
}

func (s *Service) SetReturnResponsibilityAllocation(ctx context.Context, req SetReturnResponsibilityAllocationRequest) (*ReturnResponsibilityAllocation, error) {
	if err := ValidateResponsibilityAllocationParams(req); err != nil {
		return nil, err
	}

	var resultAlloc *ReturnResponsibilityAllocation
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// 1. Resolve return_item_id
		returnItemID, err := s.repo.GetAllocationReturnItemIDTx(ctx, tx, req.AllocationID)
		if err != nil {
			return err
		}

		// 2. Lock return_item parent first to ensure deterministic lock ordering
		returnItem, err := s.repo.LockReturnItemTx(ctx, tx, returnItemID)
		if err != nil {
			return err
		}

		// 3. Lock source responsibility allocation
		sourceAlloc, err := s.repo.GetReturnResponsibilityAllocationTx(ctx, tx, req.AllocationID)
		if err != nil {
			return err
		}

		if req.Quantity > sourceAlloc.Quantity {
			return ErrInvalidQuantity
		}

		// 4. Physical binding immutability checks
		if sourceAlloc.OrderItemAllocationID != nil {
			// Source is already bound to a physical unit:
			// Must not be cleared to NULL, and must not be changed to another unit.
			if req.OrderItemAllocationID == nil || *req.OrderItemAllocationID != *sourceAlloc.OrderItemAllocationID {
				return ErrPhysicalBindingImmutable
			}
		} else {
			// Source is unbound:
			if req.OrderItemAllocationID != nil {
				// Binding requested:
				// Must have quantity == 1 (already verified by ValidateResponsibilityAllocationParams).
				// Verify belongs to same order_item and not already bound elsewhere.
				if err := s.repo.ValidateOrderItemAllocationForReturnItemTx(ctx, tx, sourceAlloc.ReturnItemID, *req.OrderItemAllocationID, sourceAlloc.ID); err != nil {
					return err
				}
			}
		}

		// 5. Semantic idempotency check:
		// If exact same quantity and decision, no-op (no update, no history, updated_at unchanged).
		// A split (req.Quantity < sourceAlloc.Quantity) is never identical.
		if isSameAllocation(sourceAlloc, req) {
			resultAlloc = sourceAlloc
			return nil
		}

		// 6. Split or full update
		if req.Quantity < sourceAlloc.Quantity {
			// Cannot split an allocation that has a physical unit bound (quantity is 1 for bound allocations).
			if sourceAlloc.OrderItemAllocationID != nil {
				return ErrInvalidQuantity
			}

			// Reduce source allocation quantity
			sourceAlloc.Quantity -= req.Quantity
			if err := s.repo.UpdateReturnResponsibilityAllocationTx(ctx, tx, sourceAlloc); err != nil {
				return err
			}

			// Create new allocation
			newAlloc := &ReturnResponsibilityAllocation{
				ID:                    uuid.New(),
				ReturnItemID:          sourceAlloc.ReturnItemID,
				OrderItemAllocationID: req.OrderItemAllocationID,
				Quantity:              req.Quantity,
				Status:                req.Status,
				ResponsibleParty:      req.ResponsibleParty,
				ReasonCode:            req.ReasonCode,
				DecisionSource:        req.DecisionSource,
				InternalNote:          req.InternalNote,
				DecidedAt:             req.DecidedAt,
				ActorID:               req.ActorID,
			}
			if err := s.repo.InsertReturnResponsibilityAllocationTx(ctx, tx, newAlloc); err != nil {
				return err
			}
			resultAlloc = newAlloc
		} else {
			// Full update
			sourceAlloc.OrderItemAllocationID = req.OrderItemAllocationID
			sourceAlloc.Status = req.Status
			sourceAlloc.ResponsibleParty = req.ResponsibleParty
			sourceAlloc.ReasonCode = req.ReasonCode
			sourceAlloc.DecisionSource = req.DecisionSource
			sourceAlloc.InternalNote = req.InternalNote
			sourceAlloc.DecidedAt = req.DecidedAt
			sourceAlloc.ActorID = req.ActorID
			if err := s.repo.UpdateReturnResponsibilityAllocationTx(ctx, tx, sourceAlloc); err != nil {
				return err
			}
			resultAlloc = sourceAlloc
		}

		// 7. Verify post-operation coverage invariant
		totalQty, err := s.repo.GetTotalAllocationsQuantityTx(ctx, tx, returnItemID)
		if err != nil {
			return err
		}
		if totalQty != returnItem.Quantity {
			return ErrResponsibilityCoverageInvariant
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return resultAlloc, nil
}
