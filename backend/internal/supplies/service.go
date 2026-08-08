package supplies

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	repo *Repository
	db   postgres.DBTX
}

func NewService(db postgres.DBTX, repo *Repository) *Service {
	return &Service{
		repo: repo,
		db:   db,
	}
}

func (s *Service) CreateSupply(ctx context.Context, sellerID uuid.UUID, req CreateSupplyRequest) (*Supply, error) {
	// Validate total quantities match box quantities
	variantExpected := make(map[uuid.UUID]int)
	var variantIDs []uuid.UUID
	for _, item := range req.Items {
		if _, ok := variantExpected[item.VariantID]; !ok {
			variantIDs = append(variantIDs, item.VariantID)
		}
		variantExpected[item.VariantID] += item.ExpectedQuantity
	}

	err := s.repo.VerifyVariantsOwnership(ctx, sellerID, variantIDs)
	if err != nil {
		return nil, ErrUnauthorized
	}

	variantBoxed := make(map[uuid.UUID]int)
	for _, box := range req.Boxes {
		for _, bi := range box.Items {
			variantBoxed[bi.VariantID] += bi.Quantity
		}
	}

	for vID, exp := range variantExpected {
		if boxed := variantBoxed[vID]; exp != boxed {
			return nil, ErrInvalidQuantities
		}
	}

	for vID, boxed := range variantBoxed {
		if exp := variantExpected[vID]; exp != boxed {
			return nil, ErrInvalidQuantities
		}
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
		Status:              "draft", // Initially draft
		HandoffMethod:       req.HandoffMethod,
		CarrierName:         req.CarrierName,
		TrackingNumber:      req.TrackingNumber,
		ExpectedArrivalDate: req.ExpectedArrivalDate,
		QRToken:             &supplyToken,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// Build Items
	variantToSupplyItemID := make(map[uuid.UUID]uuid.UUID)
	for _, reqItem := range req.Items {
		item := SupplyItem{
			ID:               uuid.New(),
			SupplyID:         supply.ID,
			VariantID:        reqItem.VariantID,
			ExpectedQuantity: reqItem.ExpectedQuantity,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		supply.Items = append(supply.Items, item)
		variantToSupplyItemID[item.VariantID] = item.ID
	}

	// Build Boxes
	for _, reqBox := range req.Boxes {
		boxToken, err := generateRandomToken()
		if err != nil {
			return nil, err
		}
		box := SupplyBox{
			ID:        uuid.New(),
			SupplyID:  supply.ID,
			BoxNumber: reqBox.BoxNumber,
			QRToken:   &boxToken,
			CreatedAt: now,
		}
		for _, reqBoxItem := range reqBox.Items {
			bi := SupplyBoxItem{
				BoxID:        box.ID,
				SupplyItemID: variantToSupplyItemID[reqBoxItem.VariantID],
				Quantity:     reqBoxItem.Quantity,
			}
			box.Items = append(box.Items, bi)
		}
		supply.Boxes = append(supply.Boxes, box)
	}

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

func (s *Service) MarkShipped(ctx context.Context, sellerID uuid.UUID, supplyID uuid.UUID) error {
	supply, err := s.repo.GetSupplyByID(ctx, supplyID)
	if err != nil {
		return err
	}
	if supply.SellerID != sellerID {
		return ErrUnauthorized
	}
	if supply.Status != "draft" && supply.Status != "ready_to_ship" {
		return ErrInvalidStatus
	}

	return s.repo.MarkShipped(ctx, supplyID)
}

func generateRandomToken() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
