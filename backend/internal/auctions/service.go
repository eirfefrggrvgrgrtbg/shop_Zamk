package auctions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/ratelimit"
)

var (
	ErrAuctionNotStarted    = errors.New("Аукцион ещё не начался")
	ErrAuctionEnded         = errors.New("Аукцион уже завершён")
	ErrBiddingDisabled      = errors.New("Ставки временно приостановлены")
	ErrLotUnavailable       = errors.New("Лот недоступен для ставок")
	ErrAlreadyLeading       = errors.New("Вы уже лидируете по этому лоту")
	ErrInvalidBidAmount     = errors.New("Ставка устарела. Обновите данные")
	ErrTooManyBids          = errors.New("Слишком много ставок. Попробуйте чуть позже")
	ErrDuplicateIdempotency = errors.New("Конфликтующий запрос со старым ключом идемпотентности")
)

type Service struct {
	repo          *Repository
	notifications *notifications.Service
	rateLimiter   *ratelimit.Limiter
	hub           *SSEHub
}

func NewService(repo *Repository, notifs *notifications.Service, limiter *ratelimit.Limiter, hub *SSEHub) *Service {
	return &Service{
		repo:          repo,
		notifications: notifs,
		rateLimiter:   limiter,
		hub:           hub,
	}
}

func (s *Service) PlaceBid(ctx context.Context, lotID, userID uuid.UUID, req BidRequest) (*BidResponse, error) {
	// 1. Quick check idempotency without locking
	if req.IdempotencyKey != nil {
		existing, err := s.repo.CheckIdempotencyKey(ctx, lotID, userID, *req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			if req.AmountCents != nil && *req.AmountCents != existing.AmountCents {
				return nil, ErrDuplicateIdempotency
			}
			return &BidResponse{Success: true, NewCurrentBid: existing.AmountCents}, nil
		}
	}

	// Rate limiting logic
	rlKey := fmt.Sprintf("auction_bid:%s:%s", lotID.String(), userID.String())
	allowed, err := s.rateLimiter.Allow(ctx, rlKey, 10, time.Minute)
	if err == nil && !allowed.Allowed {
		s.logSuspicious(ctx, nil, &lotID, &userID, "rate_limit_exceeded", nil)
		return nil, ErrTooManyBids
	}

	var resp BidResponse
	var auctionID uuid.UUID

	err = s.repo.ExecTx(ctx, func(tx pgx.Tx) error {
		// 2. Lock lot
		lot, err := s.repo.GetLotForUpdate(ctx, tx, lotID)
		if err != nil {
			return err
		}
		if lot == nil {
			return ErrLotUnavailable
		}

		auctionID = lot.AuctionID

		if lot.Status != LotStatusActive {
			return ErrLotUnavailable
		}

		// 3. Lock event
		event, err := s.repo.GetEventForUpdate(ctx, tx, lot.AuctionID)
		if err != nil {
			return err
		}
		if event == nil {
			return ErrLotUnavailable
		}

		now := time.Now()

		if event.Status != AuctionStatusLive {
			return ErrBiddingDisabled
		}
		if !event.BiddingEnabled {
			return ErrBiddingDisabled
		}
		if now.Before(event.StartsAt) {
			return ErrAuctionNotStarted
		}
		if now.After(event.EndsAt) {
			return ErrAuctionEnded
		}

		if lot.CurrentWinnerUserID != nil && *lot.CurrentWinnerUserID == userID {
			return ErrAlreadyLeading
		}

		// 4. Calculate expected next bid
		expectedBid := lot.StartPriceCents
		if lot.CurrentBidCents != nil {
			expectedBid = *lot.CurrentBidCents + lot.BidStepCents
		}

		if req.AmountCents != nil && *req.AmountCents != expectedBid {
			return ErrInvalidBidAmount
		}

		// 5. Insert Bid
		bidID := uuid.New()
		bid := &AuctionBid{
			ID:             bidID,
			AuctionID:      event.ID,
			LotID:          lot.ID,
			UserID:         userID,
			AmountCents:    expectedBid,
			IdempotencyKey: req.IdempotencyKey,
			CreatedAt:      now,
		}
		if err := s.repo.InsertBidTx(ctx, tx, bid); err != nil {
			return err
		}

		// 6. Update Lot
		if err := s.repo.UpdateLotBidTx(ctx, tx, lot.ID, expectedBid, userID); err != nil {
			return err
		}

		// 7. Insert Log
		meta, _ := json.Marshal(map[string]interface{}{
			"bidId":       bidID,
			"amountCents": expectedBid,
		})
		logEntry := &AuctionLog{
			ID:          uuid.New(),
			AuctionID:   &event.ID,
			LotID:       &lot.ID,
			ActorUserID: &userID,
			Action:      "bid_placed",
			Metadata:    meta,
			CreatedAt:   now,
		}
		if err := s.repo.InsertLogTx(ctx, tx, logEntry); err != nil {
			return err
		}

		// 8. Anti-sniping extension
		extensionApplied := false
		newEndsAt := event.EndsAt
		if event.AntiSnipingEnabled {
			timeLeft := event.EndsAt.Sub(now)
			if timeLeft <= time.Duration(event.AntiSnipingTriggerSeconds)*time.Second {
				newEndsAt = event.EndsAt.Add(time.Duration(event.AntiSnipingExtensionSeconds) * time.Second)
				if err := s.repo.ExtendAuctionTx(ctx, tx, event.ID, newEndsAt); err != nil {
					return err
				}
				extensionApplied = true
				
				extMeta, _ := json.Marshal(map[string]interface{}{
					"triggerBidId": bidID,
					"newEndsAt":    newEndsAt,
				})
				extLog := &AuctionLog{
					ID:          uuid.New(),
					AuctionID:   &event.ID,
					ActorUserID: nil,
					Action:      "auction_extended",
					Metadata:    extMeta,
					CreatedAt:   now,
				}
				_ = s.repo.InsertLogTx(ctx, tx, extLog)
			}
		}

		// 9. Send notifications
		metaNotif := map[string]interface{}{
			"lotId":       lot.ID.String(),
			"amountCents": expectedBid,
		}
		acceptedNotif := notifications.Notification{
			ID:              uuid.New(),
			RecipientUserID: &userID,
			RecipientKind:   notifications.RecipientKindCustomer,
			Type:            "auction_bid_accepted",
			Title:           "Ставка принята",
			Body:            "Ваша ставка успешно принята.",
			EntityType:      "auction_lot",
			EntityID:        lot.ID,
			Metadata:        metaNotif,
			CreatedAt:       now,
		}
		_ = s.notifications.CreateNotificationTx(ctx, tx, acceptedNotif)

		if lot.CurrentWinnerUserID != nil {
			outbidMeta := map[string]interface{}{
				"lotId":          lot.ID.String(),
				"newAmountCents": expectedBid,
			}
			outbidNotif := notifications.Notification{
				ID:              uuid.New(),
				RecipientUserID: lot.CurrentWinnerUserID,
				RecipientKind:   notifications.RecipientKindCustomer,
				Type:            "auction_outbid",
				Title:           "Вашу ставку перебили",
				Body:            "Появилась ставка выше вашей.",
				EntityType:      "auction_lot",
				EntityID:        lot.ID,
				Metadata:        outbidMeta,
				CreatedAt:       now,
			}
			_ = s.notifications.CreateNotificationTx(ctx, tx, outbidNotif)
		}

		resp.Success = true
		resp.NewCurrentBid = expectedBid
		resp.IsLeading = true
		resp.LotStatus = lot.Status
		resp.EndsAt = newEndsAt
		resp.ExtensionApplied = extensionApplied

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 10. Broadcast Realtime Event (after transaction commit)
	s.hub.Broadcast(auctionID, AuctionRealtimeEvent{
		EventType:       "bid_accepted",
		AuctionID:       auctionID,
		LotID:           &lotID,
		BidID:           nil, // we could extract it if needed, but client mainly needs the updated amounts
		CurrentBidCents: &resp.NewCurrentBid,
		EndsAt:          &resp.EndsAt,
		LotStatus:       &resp.LotStatus,
	})

	if resp.ExtensionApplied {
		s.hub.Broadcast(auctionID, AuctionRealtimeEvent{
			EventType:       "lot_extended",
			AuctionID:       auctionID,
			LotID:           &lotID,
			CurrentBidCents: &resp.NewCurrentBid,
			EndsAt:          &resp.EndsAt,
			LotStatus:       &resp.LotStatus,
		})
	}

	return &resp, nil
}

func (s *Service) logSuspicious(ctx context.Context, auctionID, lotID, userID *uuid.UUID, reason string, metadata map[string]interface{}) {
	metaRaw, _ := json.Marshal(metadata)
	if metaRaw == nil {
		metaRaw = []byte("{}")
	}
	ev := &AuctionSuspiciousEvent{
		ID:        uuid.New(),
		AuctionID: auctionID,
		LotID:     lotID,
		UserID:    userID,
		Reason:    reason,
		Metadata:  metaRaw,
		CreatedAt: time.Now(),
	}
	_ = s.repo.LogSuspiciousEvent(ctx, ev)
}

func (s *Service) FinalizeAuction(ctx context.Context, auctionID uuid.UUID, adminID uuid.UUID) error {
	err := s.repo.ExecTx(ctx, func(tx pgx.Tx) error {
		event, err := s.repo.GetEventForUpdate(ctx, tx, auctionID)
		if err != nil {
			return err
		}
		if event == nil {
			return errors.New("auction not found")
		}
		if event.Status == AuctionStatusEnded || event.Status == AuctionStatusCancelled {
			return errors.New("auction already ended or cancelled")
		}

		// Update auction status
		_, err = tx.Exec(ctx, "UPDATE auction_events SET status = $1, updated_at = now() WHERE id = $2", AuctionStatusEnded, auctionID)
		if err != nil {
			return err
		}

		// Finalize lots
		rows, err := tx.Query(ctx, "SELECT id, current_bid_cents, current_winner_user_id, status FROM auction_lots WHERE auction_id = $1 FOR UPDATE", auctionID)
		if err != nil {
			return err
		}
		defer rows.Close()

		var lots []AuctionLot
		for rows.Next() {
			var l AuctionLot
			if err := rows.Scan(&l.ID, &l.CurrentBidCents, &l.CurrentWinnerUserID, &l.Status); err != nil {
				return err
			}
			lots = append(lots, l)
		}

		for _, lot := range lots {
			if lot.Status != LotStatusActive {
				continue
			}

			if lot.CurrentWinnerUserID == nil {
				_, _ = tx.Exec(ctx, "UPDATE auction_lots SET status = $1, updated_at = now() WHERE id = $2", LotStatusEndedNoBids, lot.ID)
			} else {
				deadline := time.Now().Add(time.Duration(24) * time.Hour) // Use a default since event.PaymentDeadlineHours isn't joined
				_, _ = tx.Exec(ctx, "UPDATE auction_lots SET status = $1, payment_deadline_at = $2, updated_at = now() WHERE id = $3", LotStatusWonPendingPayment, deadline, lot.ID)
				
				// Notify winner
				metaNotif := map[string]interface{}{"lotId": lot.ID.String()}
				wonNotif := notifications.Notification{
					ID:              uuid.New(),
					RecipientUserID: lot.CurrentWinnerUserID,
					RecipientKind:   notifications.RecipientKindCustomer,
					Type:            "auction_won",
					Title:           "Вы выиграли лот",
					Body:            "Ожидается оплата лота.",
					EntityType:      "auction_lot",
					EntityID:        lot.ID,
					Metadata:        metaNotif,
					CreatedAt:       time.Now(),
				}
				_ = s.notifications.CreateNotificationTx(ctx, tx, wonNotif)
			}
		}

		meta, _ := json.Marshal(map[string]interface{}{"adminId": adminID})
		logEntry := &AuctionLog{
			ID:          uuid.New(),
			AuctionID:   &auctionID,
			ActorUserID: &adminID,
			Action:      "auction_finalized",
			Metadata:    meta,
			CreatedAt:   time.Now(),
		}
		return s.repo.InsertLogTx(ctx, tx, logEntry)
	})

	if err == nil {
		status := AuctionStatusEnded
		s.hub.Broadcast(auctionID, AuctionRealtimeEvent{
			EventType:     "auction_status_changed",
			AuctionID:     auctionID,
			AuctionStatus: &status,
		})
	}

	return err
}

func (s *Service) UpdateEventAdmin(ctx context.Context, id uuid.UUID, req AdminUpdateAuctionRequest) error {
	event, err := s.repo.GetEventByID(ctx, id)
	if err != nil {
		return err
	}
	if event == nil {
		return errors.New("auction not found")
	}

	if req.Title != nil { event.Title = *req.Title }
	if req.Description != nil { event.Description = req.Description }
	if req.StartsAt != nil { event.StartsAt = *req.StartsAt }
	if req.EndsAt != nil { event.EndsAt = *req.EndsAt }
	if req.BidStepCents != nil { event.BidStepCents = *req.BidStepCents }
	if req.PaymentDeadlineHours != nil { event.PaymentDeadlineHours = *req.PaymentDeadlineHours }
	if req.AntiSnipingEnabled != nil { event.AntiSnipingEnabled = *req.AntiSnipingEnabled }
	if req.AntiSnipingTriggerSeconds != nil { event.AntiSnipingTriggerSeconds = *req.AntiSnipingTriggerSeconds }
	if req.AntiSnipingExtensionSeconds != nil { event.AntiSnipingExtensionSeconds = *req.AntiSnipingExtensionSeconds }
	if req.MaxBidsPerUserPerLotPerMinute != nil { event.MaxBidsPerUserPerLotPerMinute = *req.MaxBidsPerUserPerLotPerMinute }
	if req.MaxRejectedBidsPerUserPerMinute != nil { event.MaxRejectedBidsPerUserPerMinute = *req.MaxRejectedBidsPerUserPerMinute }
	if req.NoBidsPolicy != nil { event.NoBidsPolicy = *req.NoBidsPolicy }
	if req.UnpaidWinnerPolicy != nil { event.UnpaidWinnerPolicy = *req.UnpaidWinnerPolicy }
	if req.IsPublic != nil { event.IsPublic = *req.IsPublic }
	if req.ShowOnHomepage != nil { event.ShowOnHomepage = *req.ShowOnHomepage }
	if req.HighlightInNav != nil { event.HighlightInNav = *req.HighlightInNav }
	if req.BiddingEnabled != nil { event.BiddingEnabled = *req.BiddingEnabled }

	return s.repo.UpdateEvent(ctx, event)
}

func (s *Service) UpdateEventStatus(ctx context.Context, id uuid.UUID, status AuctionStatus) error {
	err := s.repo.UpdateEventStatus(ctx, id, status)
	if err == nil {
		s.hub.Broadcast(id, AuctionRealtimeEvent{
			EventType:     "auction_status_changed",
			AuctionID:     id,
			AuctionStatus: &status,
		})
	}
	return err
}

func (s *Service) CancelAuction(ctx context.Context, id uuid.UUID, adminID uuid.UUID) error {
	err := s.repo.ExecTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "UPDATE auction_events SET status = $1, updated_at = now() WHERE id = $2", AuctionStatusCancelled, id)
		if err != nil { return err }

		_, err = tx.Exec(ctx, "UPDATE auction_lots SET status = $1, updated_at = now() WHERE auction_id = $2 AND status IN ($3, $4)", LotStatusCancelled, id, LotStatusDraft, LotStatusActive)
		if err != nil { return err }

		meta, _ := json.Marshal(map[string]interface{}{"adminId": adminID})
		logEntry := &AuctionLog{
			ID:          uuid.New(),
			AuctionID:   &id,
			ActorUserID: &adminID,
			Action:      "auction_cancelled",
			Metadata:    meta,
			CreatedAt:   time.Now(),
		}
		return s.repo.InsertLogTx(ctx, tx, logEntry)
	})

	if err == nil {
		status := AuctionStatusCancelled
		s.hub.Broadcast(id, AuctionRealtimeEvent{
			EventType:     "auction_status_changed",
			AuctionID:     id,
			AuctionStatus: &status,
		})
	}

	return err
}

func (s *Service) UpdateLotAdmin(ctx context.Context, id uuid.UUID, req AdminUpdateLotRequest) error {
	lot, err := s.repo.GetLotByID(ctx, id)
	if err != nil {
		return err
	}
	if lot == nil {
		return errors.New("lot not found")
	}

	if req.Title != nil { lot.Title = *req.Title }
	if req.Description != nil { lot.Description = req.Description }
	if req.StartPriceCents != nil { lot.StartPriceCents = *req.StartPriceCents }
	if req.BidStepCents != nil { lot.BidStepCents = *req.BidStepCents }
	if req.CanRelaunch != nil { lot.CanRelaunch = *req.CanRelaunch }
	if req.CanMoveToDirectSale != nil { lot.CanMoveToDirectSale = *req.CanMoveToDirectSale }
	if req.DirectSalePriceCents != nil { lot.DirectSalePriceCents = req.DirectSalePriceCents }
	if req.AdminNote != nil { lot.AdminNote = req.AdminNote }

	return s.repo.UpdateLot(ctx, lot)
}

func (s *Service) UpdateLotStatus(ctx context.Context, id uuid.UUID, status LotStatus) error {
	err := s.repo.UpdateLotStatus(ctx, id, status)
	if err == nil {
		// Need auctionID to broadcast
		lot, _ := s.repo.GetLotByID(ctx, id)
		if lot != nil {
			s.hub.Broadcast(lot.AuctionID, AuctionRealtimeEvent{
				EventType: "lot_status_changed",
				AuctionID: lot.AuctionID,
				LotID:     &id,
				LotStatus: &status,
			})
		}
	}
	return err
}

func (s *Service) MoveLotToDirectSale(ctx context.Context, id uuid.UUID, adminID uuid.UUID) error {
	return s.repo.ExecTx(ctx, func(tx pgx.Tx) error {
		lot, err := s.repo.GetLotForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if lot == nil {
			return errors.New("lot not found")
		}
		
		if lot.Status != LotStatusEndedNoBids && lot.Status != LotStatusUnpaidManualReview {
			return errors.New("lot cannot be moved to direct sale from current status")
		}

		_, err = tx.Exec(ctx, "UPDATE auction_lots SET status = $1, updated_at = now() WHERE id = $2", LotStatusMovedToDirectSale, id)
		if err != nil {
			return err
		}

		meta, _ := json.Marshal(map[string]interface{}{"adminId": adminID})
		logEntry := &AuctionLog{
			ID:          uuid.New(),
			LotID:       &id,
			ActorUserID: &adminID,
			Action:      "lot_moved_to_direct_sale",
			Metadata:    meta,
			CreatedAt:   time.Now(),
		}
		return s.repo.InsertLogTx(ctx, tx, logEntry)
	})
}
