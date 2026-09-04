package fulfillment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/observability"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrPickingNotAllowed                = errors.New("picking_not_allowed")
	ErrInvariantViolation               = errors.New("invariant violation in allocation data")
	ErrUnitNotInWarehouse               = errors.New("unit_not_in_warehouse")
	ErrUnitNotAllocatedToFulfillment    = errors.New("unit_not_allocated_to_fulfillment")
	ErrUnitAllocatedToOtherOrder        = errors.New("unit_allocated_to_other_order")
	ErrAmbiguousPickingCode             = errors.New("ambiguous_picking_code")
	ErrCodeNotFound                     = errors.New("picking_code_not_found")
	ErrMalformedScannerCode             = fmt.Errorf("%w: malformed_scanner_code", ErrCodeNotFound)
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

	if !observability.IsCanonicalScannerCode(code) {
		observability.RecordPickingScan(ctx, "not_found")
		attrs := []slog.Attr{
			slog.String("fulfillment_id", fulfillmentID.String()),
			slog.String("result", "not_found"),
			slog.String("reason", "malformed_code"),
		}
		observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
			EventName:  "fulfillment.picking_unit_scanned",
			Domain:     "fulfillment",
			Action:     "scan_picking_unit",
			Result:     "not_found",
			ActorID:    adminID.String(),
			ActorRole:  "admin",
			Level:      slog.LevelWarn,
			Attributes: attrs,
		})
		return nil, ErrMalformedScannerCode
	}

	var res *PickingScanResult
	var pickingStarted bool
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
				pickingStarted = true
			}
		}

		return nil
	})

	if err != nil {
		scanResult, isExpected := mapPickingErrorToResult(err)
		if !isExpected {
			s.logger.ErrorContext(ctx, "picking scan internal failure",
				slog.Any("error", err),
				slog.String("fulfillment_id", fulfillmentID.String()),
			)
			return nil, err
		}

		isCanonical := observability.IsCanonicalScannerCode(code)
		if !isCanonical {
			scanResult = "not_found"
		}

		observability.RecordPickingScan(ctx, scanResult)
		attrs := []slog.Attr{
			slog.String("fulfillment_id", fulfillmentID.String()),
			slog.String("result", scanResult),
		}
		if isCanonical {
			if scanResult == "cannot_pick_serialized_with_barcode" {
				attrs = append(attrs, slog.String("barcode", code))
			} else {
				attrs = append(attrs, slog.String("zmu", code))
			}
		} else {
			attrs = append(attrs, slog.String("reason", "malformed_code"))
		}
		observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
			EventName:  "fulfillment.picking_unit_scanned",
			Domain:     "fulfillment",
			Action:     "scan_picking_unit",
			Result:     scanResult,
			ActorID:    adminID.String(),
			ActorRole:  "admin",
			Level:      slog.LevelWarn,
			Attributes: attrs,
		})
		return nil, err
	}

	// 1. Picking started event
	if pickingStarted {
		observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
			EventName: "fulfillment.picking_started",
			Domain:    "fulfillment",
			Action:    "start_picking",
			Result:    "success",
			ActorID:   adminID.String(),
			ActorRole: "admin",
			Level:     slog.LevelInfo,
			Attributes: []slog.Attr{
				slog.String("order_id", res.OrderID.String()),
				slog.String("order_number", res.OrderNumber),
				slog.String("actor_id", adminID.String()),
			},
		})
	}

	// 2. Unit scanned event & duplicate scan detection
	if res.ScanResult.AlreadyPicked || res.ScanResult.AlreadyComplete {
		warnAttrs := []any{
			slog.String("order_id", res.OrderID.String()),
			slog.String("order_number", res.OrderNumber),
		}
		if observability.IsCanonicalScannerCode(code) {
			warnAttrs = append(warnAttrs, slog.String("zmu", code))
		}
		s.logger.WarnContext(ctx, "duplicate picking scan", warnAttrs...)

		observability.RecordPickingScan(ctx, "already_picked")
		attrs := []slog.Attr{
			slog.String("order_id", res.OrderID.String()),
			slog.String("order_number", res.OrderNumber),
			slog.String("result", "already_picked"),
		}
		if observability.IsCanonicalScannerCode(code) {
			attrs = append(attrs, slog.String("zmu", code))
		}
		observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
			EventName:  "fulfillment.picking_unit_scanned",
			Domain:     "fulfillment",
			Action:     "scan_picking_unit",
			Result:     "already_picked",
			ActorID:    adminID.String(),
			ActorRole:  "admin",
			Level:      slog.LevelWarn,
			Attributes: attrs,
		})
	} else if res.ScanResult.NewlyPicked {
		observability.RecordPickingScan(ctx, "ok")
		attrs := []slog.Attr{
			slog.String("order_id", res.OrderID.String()),
			slog.String("order_number", res.OrderNumber),
			slog.String("result", "ok"),
		}
		if observability.IsCanonicalScannerCode(code) {
			attrs = append(attrs, slog.String("zmu", code))
		}
		observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
			EventName:  "fulfillment.picking_unit_scanned",
			Domain:     "fulfillment",
			Action:     "scan_picking_unit",
			Result:     "ok",
			ActorID:    adminID.String(),
			ActorRole:  "admin",
			Level:      slog.LevelInfo,
			Attributes: attrs,
		})
	}

	// 3. Picking completed event
	if res.FulfillmentProgress.IsComplete && res.ScanResult.NewlyPicked {
		observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
			EventName: "fulfillment.picking_completed",
			Domain:    "fulfillment",
			Action:    "complete_picking",
			Result:    "success",
			ActorID:   adminID.String(),
			ActorRole: "admin",
			Level:     slog.LevelInfo,
			Attributes: []slog.Attr{
				slog.String("order_id", res.OrderID.String()),
				slog.String("order_number", res.OrderNumber),
				slog.Int("items_count", res.FulfillmentProgress.TotalQuantity),
			},
		})
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

func mapPickingErrorToResult(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrCodeNotFound), errors.Is(err, ErrItemNotInFulfillment), errors.Is(err, ErrAmbiguousPickingCode):
		return "not_found", true
	case errors.Is(err, ErrUnitVariantMismatch):
		return "wrong_variant", true
	case errors.Is(err, ErrUnitAllocatedToOtherOrder), errors.Is(err, ErrUnitAllocatedToOtherOrderItem):
		return "allocated_to_other_order", true
	case errors.Is(err, ErrUnitNotAllocatedToFulfillment), errors.Is(err, ErrItemNotSerialized), errors.Is(err, ErrUnitNotInWarehouse), errors.Is(err, ErrOrderItemRequiredForSubstitution):
		return "not_allocated", true
	case errors.Is(err, ErrCannotPickSerializedWithBarcode):
		return "cannot_pick_serialized_with_barcode", true
	case errors.Is(err, ErrNoUnpickedAllocationForVariant), errors.Is(err, ErrItemAlreadyComplete):
		return "already_picked", true
	case errors.Is(err, ErrPickingNotAllowed), errors.Is(err, ErrFulfillmentNotFound):
		return "not_allocated", true
	default:
		return "", false
	}
}
