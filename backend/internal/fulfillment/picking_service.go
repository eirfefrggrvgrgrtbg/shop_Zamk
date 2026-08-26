package fulfillment

import (
	"context"
	"errors"
	"strings"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrPickingNotAllowed               = errors.New("picking_not_allowed")
	ErrInvariantViolation              = errors.New("invariant violation in allocation data")
	ErrUnitNotInWarehouse              = errors.New("unit_not_in_warehouse")
	ErrUnitNotAllocatedToFulfillment   = errors.New("unit_not_allocated_to_fulfillment")
	ErrUnitAllocatedToOtherOrder       = errors.New("unit_allocated_to_other_order")
	ErrAmbiguousPickingCode            = errors.New("ambiguous_picking_code")
	ErrCodeNotFound                    = errors.New("picking_code_not_found")
	ErrCannotPickSerializedWithBarcode = errors.New("cannot_pick_serialized_with_barcode")
)

// GetPickingOrder reads the persistent state of a picking fulfillment.
func (s *Service) GetPickingOrder(ctx context.Context, fulfillmentID uuid.UUID) (*PickingOrder, error) {
	return s.repo.GetPickingOrder(ctx, fulfillmentID)
}

func (s *Service) ScanPickingCode(ctx context.Context, adminID uuid.UUID, fulfillmentID uuid.UUID, code string) (*PickingScanResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrCodeNotFound
	}

	var res *PickingScanResult
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		var err error
		res, err = s.repo.ScanPickingCodeTx(ctx, tx, fulfillmentID, code)
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
