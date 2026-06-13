package payments

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type Service struct {
	repo         *Repository
	ordersRepo   *orders.Repository
	inventorySvc *inventory.Service
	provider     Provider
	db           *postgres.Client
}

func NewService(repo *Repository, ordersRepo *orders.Repository, inventorySvc *inventory.Service, provider Provider, db *postgres.Client) *Service {
	return &Service{
		repo:         repo,
		ordersRepo:   ordersRepo,
		inventorySvc: inventorySvc,
		provider:     provider,
		db:           db,
	}
}

func (s *Service) CreatePayment(ctx context.Context, userID, orderID uuid.UUID) (*CreatePaymentResponse, error) {
	order, err := s.ordersRepo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, ErrOrderNotFound
	}
	if order.Status != "awaiting_payment" {
		return nil, ErrOrderNotAwaitingPayment
	}

	// Check for active pending/created payment
	existingPayment, err := s.repo.GetActivePaymentForOrder(ctx, orderID)
	if err != nil && !errors.Is(err, ErrPaymentNotFound) {
		return nil, err
	}

	if existingPayment != nil {
		return &CreatePaymentResponse{
			PaymentID:   existingPayment.ID,
			Provider:    existingPayment.Provider,
			Status:      existingPayment.Status,
			AmountCents: existingPayment.AmountCents,
			Currency:    existingPayment.Currency,
			PaymentURL:  *existingPayment.PaymentURL,
		}, nil
	}

	idempotencyKey := uuid.New().String()
	input := CreatePaymentInput{
		OrderID:        orderID.String(),
		AmountCents:    order.TotalPriceCents,
		Currency:       order.Currency,
		IdempotencyKey: idempotencyKey,
		Description:    "Payment for order " + orderID.String(),
	}

	res, err := s.provider.CreatePayment(ctx, input)
	if err != nil {
		return nil, err
	}

	payment := &Payment{
		ID:                uuid.New(),
		OrderID:           orderID,
		Provider:          "tbank", // hardcoded provider name for now
		ProviderPaymentID: &res.ProviderPaymentID,
		Status:            "pending",
		AmountCents:       order.TotalPriceCents,
		Currency:          order.Currency,
		PaymentURL:        &res.PaymentURL,
		IdempotencyKey:    idempotencyKey,
	}

	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		return nil, err
	}

	return &CreatePaymentResponse{
		PaymentID:   payment.ID,
		Provider:    payment.Provider,
		Status:      payment.Status,
		AmountCents: payment.AmountCents,
		Currency:    payment.Currency,
		PaymentURL:  *payment.PaymentURL,
	}, nil
}

func (s *Service) HandleWebhook(ctx context.Context, headers map[string]string, body []byte) error {
	if err := s.provider.VerifyWebhook(ctx, headers, body); err != nil {
		return err
	}

	event, err := s.provider.ParseWebhook(ctx, body)
	if err != nil {
		return err
	}

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		payment, err := s.repo.GetPaymentByProviderIDForUpdate(ctx, tx, "tbank", event.ProviderPaymentID)
		if err != nil {
			return err
		}

		if payment.Status == "succeeded" || payment.Status == "failed" || payment.Status == "cancelled" {
			// Idempotency check: Already processed safely
			return ErrPaymentAlreadyProcessed
		}

		now := time.Now()
		var pEvent PaymentEvent
		pEvent.ID = uuid.New()
		pEvent.PaymentID = &payment.ID
		pEvent.Provider = "tbank"
		pEvent.ProviderPaymentID = &event.ProviderPaymentID
		pEvent.EventType = event.Status
		pEvent.RawPayload = event.RawPayload
		pEvent.SignatureValid = true
		pEvent.ProcessedAt = &now

		if err := s.repo.CreatePaymentEventTx(ctx, tx, &pEvent); err != nil {
			return err
		}

		if event.Status == "succeeded" {
			order, err := s.ordersRepo.GetOrderForUpdateTx(ctx, tx, payment.OrderID)
			if err != nil {
				return err
			}

			if order.Status != "awaiting_payment" {
				return nil // If order is already paid or cancelled, skip converting. Webhook processed safely.
			}

			// 1. Update Payment
			payment.Status = "succeeded"
			payment.PaidAt = &now
			if err := s.repo.UpdatePaymentStatusTx(ctx, tx, payment); err != nil {
				return err
			}

			// 2. Update Order
			if err := s.ordersRepo.UpdateOrderStatusTx(ctx, tx, order.ID, "paid"); err != nil {
				return err
			}
			if err := s.ordersRepo.MarkOrderFulfillmentsStatusTx(ctx, tx, order.ID, "awaiting_payment", "paid"); err != nil {
				return err
			}

			history := &orders.OrderStatusHistory{
				ID:         uuid.New(),
				OrderID:    order.ID,
				FromStatus: &order.Status,
				ToStatus:   "paid",
			}
			if err := s.ordersRepo.CreateOrderStatusHistoryTx(ctx, tx, history); err != nil {
				return err
			}

			// 3. Convert Reservations to Sale
			resIDs, err := s.ordersRepo.GetOrderReservations(ctx, order.ID)
			if err != nil {
				return err
			}

			for _, rid := range resIDs {
				if err := s.inventorySvc.ConvertReservationToSaleTx(ctx, tx, rid); err != nil {
					return err
				}
			}

		} else if event.Status == "failed" {
			payment.Status = "failed"
			payment.FailedAt = &now
			if err := s.repo.UpdatePaymentStatusTx(ctx, tx, payment); err != nil {
				return err
			}
		} else if event.Status == "cancelled" {
			payment.Status = "cancelled"
			payment.CancelledAt = &now
			if err := s.repo.UpdatePaymentStatusTx(ctx, tx, payment); err != nil {
				return err
			}
		}

		return nil
	})
}
