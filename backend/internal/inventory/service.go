package inventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
)

type Service struct {
	repo       *Repository
	sellerRepo *sellers.Repository
	dbPool     *postgres.Client
}

func NewService(repo *Repository, sellerRepo *sellers.Repository, dbPool *postgres.Client) *Service {
	return &Service{
		repo:       repo,
		sellerRepo: sellerRepo,
		dbPool:     dbPool,
	}
}

// ---------------------------------------------------------
// Helper: Resolve Product Variant Info
// ---------------------------------------------------------

type productVariantInfo struct {
	ProductID uuid.UUID
	SellerID  uuid.UUID
}

func (s *Service) getVariantInfo(ctx context.Context, variantID uuid.UUID) (*productVariantInfo, error) {
	query := `
		SELECT p.id, p.seller_id
		FROM product_variants pv
		JOIN products p ON pv.product_id = p.id
		WHERE pv.id = $1
	`
	var info productVariantInfo
	err := s.dbPool.Pool.QueryRow(ctx, query, variantID).Scan(&info.ProductID, &info.SellerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductVariantNotFound
		}
		return nil, fmt.Errorf("failed to get product variant info: %w", err)
	}
	return &info, nil
}

func (s *Service) getSellerForUser(ctx context.Context, userID uuid.UUID) (*sellers.Seller, error) {
	seller, _, err := s.sellerRepo.GetSellerByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sellers.ErrSellerUserNotFound) {
			return nil, ErrSellerMismatch
		}
		return nil, err
	}
	return seller, nil
}

// ---------------------------------------------------------
// Admin Operations
// ---------------------------------------------------------

func (s *Service) ListAdminInventory(ctx context.Context, limit, offset int) (InventoryListResponse, error) {
	items, err := s.repo.ListInventory(ctx, limit, offset)
	if err != nil {
		return InventoryListResponse{}, err
	}
	if items == nil {
		items = []Item{}
	}
	return InventoryListResponse{Items: items, TotalCount: len(items)}, nil
}

func (s *Service) GetAdminInventoryItem(ctx context.Context, id uuid.UUID) (Item, error) {
	i, err := s.repo.GetItemByID(ctx, id)
	if err != nil {
		return Item{}, err
	}
	return *i, nil
}

func (s *Service) ListMovements(ctx context.Context, itemID uuid.UUID, limit, offset int) (StockMovementsListResponse, error) {
	movs, err := s.repo.ListMovementsByInventoryItemID(ctx, itemID, limit, offset)
	if err != nil {
		return StockMovementsListResponse{}, err
	}
	if movs == nil {
		movs = []StockMovement{}
	}
	return StockMovementsListResponse{Items: movs, TotalCount: len(movs)}, nil
}

func (s *Service) ReceiveStock(ctx context.Context, adminUserID uuid.UUID, req ReceiptRequest) (Item, error) {
	info, err := s.getVariantInfo(ctx, req.ProductVariantID)
	if err != nil {
		return Item{}, err
	}

	var resultingItem Item

	err = s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		item, err := txRepo.GetItemForUpdateByVariant(ctx, req.ProductVariantID)
		if err != nil && !errors.Is(err, ErrInventoryItemNotFound) {
			return err
		}

		now := time.Now()
		isNew := false
		if errors.Is(err, ErrInventoryItemNotFound) {
			isNew = true
			item = &Item{
				ID:               uuid.New(),
				ProductID:        info.ProductID,
				ProductVariantID: req.ProductVariantID,
				SellerID:         info.SellerID,
				TotalStock:       0,
				ReservedStock:    0,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
		}

		item.TotalStock += req.Quantity
		item.UpdatedAt = now

		if isNew {
			if err := txRepo.CreateItem(ctx, item); err != nil {
				return err
			}
		} else {
			if err := txRepo.UpdateItemStock(ctx, item); err != nil {
				return err
			}
		}

		mov := &StockMovement{
			ID:               uuid.New(),
			InventoryItemID:  item.ID,
			ProductID:        item.ProductID,
			ProductVariantID: item.ProductVariantID,
			SellerID:         item.SellerID,
			Type:             MovementTypeReceipt,
			Quantity:         req.Quantity,
			Reason:           req.Reason,
			ActorUserID:      &adminUserID,
			CreatedAt:        now,
		}
		if err := txRepo.RecordMovement(ctx, mov); err != nil {
			return err
		}

		item.ComputeAvailable()
		resultingItem = *item
		return nil
	})

	if err != nil {
		return Item{}, err
	}
	return resultingItem, nil
}

func (s *Service) AdjustStock(ctx context.Context, adminUserID uuid.UUID, req AdjustmentRequest) (Item, error) {
	if req.Quantity == 0 {
		return Item{}, ErrInvalidQuantity
	}

	var resultingItem Item

	err := s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		item, err := txRepo.GetItemForUpdateByVariant(ctx, req.ProductVariantID)
		if err != nil {
			return err
		}

		newTotal := item.TotalStock + req.Quantity
		if newTotal < 0 {
			return ErrNegativeStock
		}
		if newTotal < item.ReservedStock {
			return ErrStockBelowReserved
		}

		item.TotalStock = newTotal
		item.UpdatedAt = time.Now()

		if err := txRepo.UpdateItemStock(ctx, item); err != nil {
			return err
		}

		qty := req.Quantity
		if qty < 0 {
			qty = -qty
		}

		mov := &StockMovement{
			ID:               uuid.New(),
			InventoryItemID:  item.ID,
			ProductID:        item.ProductID,
			ProductVariantID: item.ProductVariantID,
			SellerID:         item.SellerID,
			Type:             MovementTypeAdjustment,
			Quantity:         qty,
			Reason:           &req.Reason,
			ActorUserID:      &adminUserID,
			CreatedAt:        time.Now(),
		}
		if err := txRepo.RecordMovement(ctx, mov); err != nil {
			return err
		}

		item.ComputeAvailable()
		resultingItem = *item
		return nil
	})

	if err != nil {
		return Item{}, err
	}
	return resultingItem, nil
}

func (s *Service) WriteOffStock(ctx context.Context, adminUserID uuid.UUID, req WriteOffRequest) (Item, error) {
	var resultingItem Item

	err := s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		item, err := txRepo.GetItemForUpdateByVariant(ctx, req.ProductVariantID)
		if err != nil {
			return err
		}

		newTotal := item.TotalStock - req.Quantity
		if newTotal < 0 {
			return ErrNegativeStock
		}
		if newTotal < item.ReservedStock {
			return ErrStockBelowReserved
		}

		item.TotalStock = newTotal
		item.UpdatedAt = time.Now()

		if err := txRepo.UpdateItemStock(ctx, item); err != nil {
			return err
		}

		mov := &StockMovement{
			ID:               uuid.New(),
			InventoryItemID:  item.ID,
			ProductID:        item.ProductID,
			ProductVariantID: item.ProductVariantID,
			SellerID:         item.SellerID,
			Type:             MovementTypeWriteOff,
			Quantity:         req.Quantity,
			Reason:           &req.Reason,
			ActorUserID:      &adminUserID,
			CreatedAt:        time.Now(),
		}
		if err := txRepo.RecordMovement(ctx, mov); err != nil {
			return err
		}

		item.ComputeAvailable()
		resultingItem = *item
		return nil
	})

	if err != nil {
		return Item{}, err
	}
	return resultingItem, nil
}

// ---------------------------------------------------------
// Seller Operations
// ---------------------------------------------------------

func (s *Service) ListSellerInventory(ctx context.Context, currentUserID uuid.UUID, limit, offset int) (InventoryListResponse, error) {
	seller, err := s.getSellerForUser(ctx, currentUserID)
	if err != nil {
		return InventoryListResponse{}, err
	}

	items, err := s.repo.ListInventoryBySeller(ctx, seller.ID, limit, offset)
	if err != nil {
		return InventoryListResponse{}, err
	}
	if items == nil {
		items = []Item{}
	}
	return InventoryListResponse{Items: items, TotalCount: len(items)}, nil
}

func (s *Service) GetSellerInventoryItem(ctx context.Context, currentUserID uuid.UUID, id uuid.UUID) (Item, error) {
	seller, err := s.getSellerForUser(ctx, currentUserID)
	if err != nil {
		return Item{}, err
	}

	i, err := s.repo.GetItemByID(ctx, id)
	if err != nil {
		return Item{}, err
	}
	if i.SellerID != seller.ID {
		return Item{}, ErrSellerMismatch
	}
	return *i, nil
}

func (s *Service) ListSellerMovements(ctx context.Context, currentUserID uuid.UUID, itemID uuid.UUID, limit, offset int) (StockMovementsListResponse, error) {
	seller, err := s.getSellerForUser(ctx, currentUserID)
	if err != nil {
		return StockMovementsListResponse{}, err
	}

	i, err := s.repo.GetItemByID(ctx, itemID)
	if err != nil {
		return StockMovementsListResponse{}, err
	}
	if i.SellerID != seller.ID {
		return StockMovementsListResponse{}, ErrSellerMismatch
	}

	movs, err := s.repo.ListMovementsByInventoryItemID(ctx, itemID, limit, offset)
	if err != nil {
		return StockMovementsListResponse{}, err
	}
	if movs == nil {
		movs = []StockMovement{}
	}
	return StockMovementsListResponse{Items: movs, TotalCount: len(movs)}, nil
}

// ---------------------------------------------------------
// Reservation Foundation (Internal/Helper)
// ---------------------------------------------------------

func (s *Service) CreateReservation(ctx context.Context, userID uuid.UUID, variantID uuid.UUID, quantity int, ttl time.Duration) (Reservation, error) {
	var resultingRes Reservation

	err := s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		item, err := txRepo.GetItemForUpdateByVariant(ctx, variantID)
		if err != nil {
			return err
		}

		if item.TotalStock-item.ReservedStock < quantity {
			return ErrInsufficientStock
		}

		item.ReservedStock += quantity
		item.UpdatedAt = time.Now()

		if err := txRepo.UpdateItemStock(ctx, item); err != nil {
			return err
		}

		now := time.Now()
		res := &Reservation{
			ID:               uuid.New(),
			InventoryItemID:  item.ID,
			ProductID:        item.ProductID,
			ProductVariantID: item.ProductVariantID,
			UserID:           &userID,
			Quantity:         quantity,
			Status:           ReservationStatusActive,
			ExpiresAt:        now.Add(ttl),
			CreatedAt:        now,
		}

		if err := txRepo.CreateReservation(ctx, res); err != nil {
			return err
		}

		mov := &StockMovement{
			ID:               uuid.New(),
			InventoryItemID:  item.ID,
			ProductID:        item.ProductID,
			ProductVariantID: item.ProductVariantID,
			SellerID:         item.SellerID,
			Type:             MovementTypeReservationCreated,
			Quantity:         quantity,
			ActorUserID:      &userID,
			ReferenceType:    func(s string) *string { return &s }("reservation"),
			ReferenceID:      &res.ID,
			CreatedAt:        now,
		}
		if err := txRepo.RecordMovement(ctx, mov); err != nil {
			return err
		}

		resultingRes = *res
		return nil
	})

	if err != nil {
		return Reservation{}, err
	}
	return resultingRes, nil
}

func (s *Service) CreateReservationTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, variantID uuid.UUID, quantity int, ttl time.Duration) (Reservation, error) {
	txRepo := s.repo.WithTx(tx)

	item, err := txRepo.GetItemForUpdateByVariant(ctx, variantID)
	if err != nil {
		return Reservation{}, err
	}

	if item.TotalStock-item.ReservedStock < quantity {
		return Reservation{}, ErrInsufficientStock
	}

	item.ReservedStock += quantity
	item.UpdatedAt = time.Now()

	if err := txRepo.UpdateItemStock(ctx, item); err != nil {
		return Reservation{}, err
	}

	now := time.Now()
	res := &Reservation{
		ID:               uuid.New(),
		InventoryItemID:  item.ID,
		ProductID:        item.ProductID,
		ProductVariantID: item.ProductVariantID,
		UserID:           &userID,
		Quantity:         quantity,
		Status:           ReservationStatusActive,
		ExpiresAt:        now.Add(ttl),
		CreatedAt:        now,
	}

	if err := txRepo.CreateReservation(ctx, res); err != nil {
		return Reservation{}, err
	}

	mov := &StockMovement{
		ID:               uuid.New(),
		InventoryItemID:  item.ID,
		ProductID:        item.ProductID,
		ProductVariantID: item.ProductVariantID,
		SellerID:         item.SellerID,
		Type:             MovementTypeReservationCreated,
		Quantity:         quantity,
		ActorUserID:      &userID,
		ReferenceType:    func(s string) *string { return &s }("reservation"),
		ReferenceID:      &res.ID,
		CreatedAt:        now,
	}
	if err := txRepo.RecordMovement(ctx, mov); err != nil {
		return Reservation{}, err
	}

	return *res, nil
}

func (s *Service) ReleaseReservation(ctx context.Context, reservationID uuid.UUID) error {
	return s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		res, err := txRepo.GetReservationByIDForUpdate(ctx, reservationID)
		if err != nil {
			return err
		}

		if res.Status != ReservationStatusActive {
			return ErrReservationNotActive
		}

		item, err := txRepo.GetItemForUpdateByVariant(ctx, res.ProductVariantID)
		if err != nil {
			return err
		}

		item.ReservedStock -= res.Quantity
		if item.ReservedStock < 0 {
			item.ReservedStock = 0 // Safety bounds
		}
		item.UpdatedAt = time.Now()

		if err := txRepo.UpdateItemStock(ctx, item); err != nil {
			return err
		}

		now := time.Now()
		res.Status = ReservationStatusReleased
		res.ReleasedAt = &now

		if err := txRepo.UpdateReservationStatus(ctx, res); err != nil {
			return err
		}

		mov := &StockMovement{
			ID:               uuid.New(),
			InventoryItemID:  item.ID,
			ProductID:        item.ProductID,
			ProductVariantID: item.ProductVariantID,
			SellerID:         item.SellerID,
			Type:             MovementTypeReservationReleased,
			Quantity:         res.Quantity,
			ReferenceType:    func(s string) *string { return &s }("reservation"),
			ReferenceID:      &res.ID,
			CreatedAt:        now,
		}
		return txRepo.RecordMovement(ctx, mov)
	})
}

func (s *Service) ConvertReservationToSaleTx(ctx context.Context, tx pgx.Tx, reservationID uuid.UUID) error {
	txRepo := s.repo.WithTx(tx)

	res, err := txRepo.GetReservationByIDForUpdate(ctx, reservationID)
	if err != nil {
		return err
	}

	if res.Status != ReservationStatusActive {
		return ErrReservationNotActive
	}

	item, err := txRepo.GetItemForUpdateByVariant(ctx, res.ProductVariantID)
	if err != nil {
		return err
	}

	// Decrease total and reserved stock
	item.TotalStock -= res.Quantity
	if item.TotalStock < 0 {
		item.TotalStock = 0
	}
	item.ReservedStock -= res.Quantity
	if item.ReservedStock < 0 {
		item.ReservedStock = 0
	}
	item.UpdatedAt = time.Now()

	if err := txRepo.UpdateItemStock(ctx, item); err != nil {
		return err
	}

	now := time.Now()
	res.Status = ReservationStatusConverted
	res.ReleasedAt = &now // Using released_at to mark when it was converted/closed

	if err := txRepo.UpdateReservationStatus(ctx, res); err != nil {
		return err
	}

	mov := &StockMovement{
		ID:               uuid.New(),
		InventoryItemID:  item.ID,
		ProductID:        item.ProductID,
		ProductVariantID: item.ProductVariantID,
		SellerID:         item.SellerID,
		Type:             MovementTypeSale,
		Quantity:         res.Quantity,
		ReferenceType:    func(s string) *string { return &s }("order"),
		ReferenceID:      res.OrderID,
		CreatedAt:        now,
	}
	return txRepo.RecordMovement(ctx, mov)
}

func (s *Service) ReleaseReservationTx(ctx context.Context, tx pgx.Tx, reservationID uuid.UUID) error {
	txRepo := s.repo.WithTx(tx)

	res, err := txRepo.GetReservationByIDForUpdate(ctx, reservationID)
	if err != nil {
		return err
	}

	if res.Status != ReservationStatusActive {
		return ErrReservationNotActive
	}

	item, err := txRepo.GetItemForUpdateByVariant(ctx, res.ProductVariantID)
	if err != nil {
		return err
	}

	item.ReservedStock -= res.Quantity
	if item.ReservedStock < 0 {
		item.ReservedStock = 0 // Safety bounds
	}
	item.UpdatedAt = time.Now()

	if err := txRepo.UpdateItemStock(ctx, item); err != nil {
		return err
	}

	now := time.Now()
	res.Status = ReservationStatusReleased
	res.ReleasedAt = &now

	if err := txRepo.UpdateReservationStatus(ctx, res); err != nil {
		return err
	}

	mov := &StockMovement{
		ID:               uuid.New(),
		InventoryItemID:  item.ID,
		ProductID:        item.ProductID,
		ProductVariantID: item.ProductVariantID,
		SellerID:         item.SellerID,
		Type:             MovementTypeReservationReleased,
		Quantity:         res.Quantity,
		ReferenceType:    func(s string) *string { return &s }("reservation"),
		ReferenceID:      &res.ID,
		CreatedAt:        now,
	}
	return txRepo.RecordMovement(ctx, mov)
}

func (s *Service) ProcessRestockTx(ctx context.Context, tx pgx.Tx, variantID uuid.UUID, quantity int, returnID *uuid.UUID) error {
	txRepo := s.repo.WithTx(tx)

	item, err := txRepo.GetItemForUpdateByVariant(ctx, variantID)
	if err != nil {
		return err // Could be ErrInventoryItemNotFound, but we assume it exists if it was ordered
	}

	item.TotalStock += quantity
	item.UpdatedAt = time.Now()

	if err := txRepo.UpdateItemStock(ctx, item); err != nil {
		return err
	}

	mov := &StockMovement{
		ID:               uuid.New(),
		InventoryItemID:  item.ID,
		ProductID:        item.ProductID,
		ProductVariantID: item.ProductVariantID,
		SellerID:         item.SellerID,
		Type:             MovementTypeReturn,
		Quantity:         quantity,
		ReferenceType:    func(s string) *string { return &s }("return"),
		ReferenceID:      returnID,
		CreatedAt:        time.Now(),
	}
	return txRepo.RecordMovement(ctx, mov)
}
