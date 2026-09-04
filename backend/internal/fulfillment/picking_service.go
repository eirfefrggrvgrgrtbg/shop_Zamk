package fulfillment

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
)

var (
	ErrPickingNotAllowed                = errors.New("picking_not_allowed")
	ErrInvariantViolation               = errors.New("invariant violation in allocation data")
	ErrUnitNotInWarehouse               = errors.New("unit_not_in_warehouse")
	ErrUnitNotAllocatedToFulfillment    = errors.New("unit_not_allocated_to_fulfillment")
	ErrUnitAllocatedToOtherOrder        = errors.New("unit_allocated_to_other_order")
	ErrAmbiguousPickingCode             = errors.New("ambiguous_picking_code")
	ErrCodeNotFound                     = errors.New("picking_code_not_found")
	ErrCannotPickSerializedWithBarcode  = errors.New("cannot_pick_serialized_with_barcode")
	ErrUnitVariantMismatch              = errors.New("unit_variant_mismatch")
	ErrNoUnpickedAllocationForVariant   = errors.New("no_unpicked_allocation_for_variant")
	ErrItemNotSerialized                = errors.New("item_not_serialized")
	ErrItemAlreadyComplete              = errors.New("item_already_complete")
	ErrUnitAllocatedToOtherOrderItem    = errors.New("unit_allocated_to_other_order_item")
	ErrOrderItemRequiredForSubstitution = errors.New("order_item_required_for_substitution")
)

// GetPickingOrder reads the persistent state of a picking fulfillment.
func (s *Service) GetPickingOrder(ctx context.Context, fulfillmentID uuid.UUID) (*PickingOrder, error) {
	return s.repo.GetPickingOrder(ctx, fulfillmentID)
}

func (s *Service) ScanPickingCode(ctx context.Context, adminID uuid.UUID, fulfillmentID uuid.UUID, code string, orderItemID *uuid.UUID) (*PickingScanResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrCodeNotFound
	}

	var res *PickingScanResult
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		var err error
		res, err = s.repo.ScanPickingCodeTx(ctx, tx, fulfillmentID, code, orderItemID)
		if err != nil {
			return err
		}

		if res.ScanResult.NewlyPicked {
			f, errF := s.repo.GetAdminFulfillmentTx(ctx, tx, fulfillmentID)
			if errF != nil {
				return errF
			}
			if f.Status == "paid" {
				if err := s.repo.UpdateFulfillmentStatusTx(ctx, tx, fulfillmentID, "assembling"); err != nil {
					return err
				}
				if err := s.recalculateParentOrderStatusTx(ctx, tx, f.OrderID, adminID); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return res, nil
}

// CountActionablePicking returns the persistent count of fulfillments currently requiring picking.
func (s *Service) CountActionablePicking(ctx context.Context) (int, error) {
	return s.repo.CountActionablePicking(ctx)
}

// IsFulfillmentActionablePicking returns true if the specific fulfillment requires picking work.
func (s *Service) IsFulfillmentActionablePicking(ctx context.Context, fulfillmentID uuid.UUID) (bool, error) {
	return s.repo.IsFulfillmentActionablePicking(ctx, fulfillmentID)
}

func (s *Service) GetCompatibleUnits(ctx context.Context, fulfillmentID uuid.UUID, orderItemID uuid.UUID) ([]CompatibleUnit, error) {
	return s.repo.GetCompatibleUnits(ctx, fulfillmentID, orderItemID)
}
