package supplies

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	repo              *Repository
	db                postgres.DBTX
	unitCodeGenerator func() (string, error)
}

func NewService(db postgres.DBTX, repo *Repository) *Service {
	return &Service{
		repo:              repo,
		db:                db,
		unitCodeGenerator: GenerateUnitCode,
	}
}

func (s *Service) SetUnitCodeGeneratorForTest(fn func() (string, error)) {
	s.unitCodeGenerator = fn
	if s.repo != nil {
		s.repo.SetUnitCodeGeneratorForTest(fn)
	}
}

func (s *Service) generateUnitCode() (string, error) {
	if s.unitCodeGenerator != nil {
		return s.unitCodeGenerator()
	}
	return GenerateUnitCode()
}

func (s *Service) CreateSupply(ctx context.Context, sellerID uuid.UUID, req CreateSupplyRequest) (*Supply, error) {
	if req.HandoffMethod == "" {
		req.HandoffMethod = "carrier_delivery"
	}

	if len(req.Items) == 0 {
		return nil, ErrInvalidQuantities
	}

	hasPositiveQuantity := false
	for _, item := range req.Items {
		if item.ExpectedQuantity > 0 {
			hasPositiveQuantity = true
			break
		}
	}
	if !hasPositiveQuantity {
		return nil, ErrInvalidQuantities
	}

	if req.HandoffMethod == "carrier_delivery" {
		if req.CarrierName == nil || strings.TrimSpace(*req.CarrierName) == "" {
			return nil, ErrCarrierRequired
		}
		carrierUpper := strings.ToUpper(strings.TrimSpace(*req.CarrierName))
		if carrierUpper != "СДЭК" && carrierUpper != "CDEK" {
			return nil, ErrCarrierUnsupported
		}
		canonicalCarrier := "СДЭК"
		req.CarrierName = &canonicalCarrier

		if req.TrackingNumber == nil || strings.TrimSpace(*req.TrackingNumber) == "" {
			return nil, ErrTrackingNumberRequired
		}
	}

	var variantIDs []uuid.UUID
	for _, item := range req.Items {
		if item.ExpectedQuantity > 0 {
			variantIDs = append(variantIDs, item.VariantID)
		}
	}

	err := s.repo.VerifyVariantsOwnership(ctx, sellerID, variantIDs)
	if err != nil {
		return nil, ErrUnauthorized
	}

	// Begin TX
	pool, ok := s.db.(*pgxpool.Pool)
	if !ok {
		return nil, errors.New("expected *pgxpool.Pool")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	repoTx := s.repo.WithTx(tx)

	supplyNumber, err := repoTx.GenerateSupplyNumber(ctx)
	if err != nil {
		return nil, err
	}

	supplyToken, err := generateRandomToken()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	supply := &Supply{
		ID:                  uuid.New(),
		SupplyNumber:        supplyNumber,
		SellerID:            sellerID,
		Status:              "ready_to_ship",
		HandoffMethod:       req.HandoffMethod,
		CarrierName:         req.CarrierName,
		TrackingNumber:      req.TrackingNumber,
		ExpectedArrivalDate: req.ExpectedArrivalDate,
		QRToken:             &supplyToken,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// Implicit V1 Box
	boxToken, err := generateRandomToken()
	if err != nil {
		return nil, err
	}
	defaultBox := SupplyBox{
		ID:        uuid.New(),
		SupplyID:  supply.ID,
		BoxNumber: fmt.Sprintf("%s-B1", supplyNumber),
		QRToken:   &boxToken,
		CreatedAt: now,
	}

	for _, reqItem := range req.Items {
		if reqItem.ExpectedQuantity <= 0 {
			continue // skip empty
		}
		item := SupplyItem{
			ID:               uuid.New(),
			SupplyID:         supply.ID,
			VariantID:        reqItem.VariantID,
			ExpectedQuantity: reqItem.ExpectedQuantity,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		supply.Items = append(supply.Items, item)
		
		bi := SupplyBoxItem{
			BoxID:        defaultBox.ID,
			SupplyItemID: item.ID,
			Quantity:     item.ExpectedQuantity,
		}
		defaultBox.Items = append(defaultBox.Items, bi)
			// Generate inventory units
			for i := 1; i <= reqItem.ExpectedQuantity; i++ {
				unitCode, err := s.generateUnitCode()
				if err != nil {
					return nil, err
				}
				unit := InventoryUnit{
					ID:                 uuid.New(),
					UnitCode:           unitCode,
					ProductVariantID:   reqItem.VariantID,
					OriginSupplyID:     supply.ID,
					OriginSupplyItemID: item.ID,
					OriginBoxID:        &defaultBox.ID,
					UnitIndex:          i,
					Status:             "expected",
					CreatedAt:          now,
					UpdatedAt:          now,
				}
				supply.InventoryUnits = append(supply.InventoryUnits, unit)
			}
	}

	supply.Boxes = append(supply.Boxes, defaultBox)

	err = repoTx.CreateSupply(ctx, supply)
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return s.repo.GetSupplyByID(ctx, supply.ID)
}

func (s *Service) MarkShipped(ctx context.Context, sellerID uuid.UUID, supplyID uuid.UUID) (*Supply, error) {
	supply, err := s.repo.GetSupplyByID(ctx, supplyID)
	if err != nil {
		return nil, err
	}
	if supply.SellerID != sellerID {
		return nil, ErrUnauthorized
	}
	if supply.Status != "ready_to_ship" {
		return nil, ErrInvalidStatus
	}

	err = s.repo.MarkShipped(ctx, supplyID)
	if err != nil {
		return nil, err
	}

	return s.repo.GetSupplyByID(ctx, supplyID)
}

func generateRandomToken() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
