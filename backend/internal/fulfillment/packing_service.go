package fulfillment

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
)

func (s *Service) PackFulfillment(ctx context.Context, adminID, fulfillmentID uuid.UUID) (*PackResult, error) {
	var result *PackResult
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		res, err := s.repo.PackFulfillmentTx(ctx, tx, fulfillmentID)
		if err != nil {
			return err
		}

		if err := s.recalculateParentOrderStatusTx(ctx, tx, res.OrderID, adminID); err != nil {
			return err
		}

		// Read updated parent order status
		var orderStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, res.OrderID).Scan(&orderStatus); err != nil {
			return err
		}
		res.OrderStatus = orderStatus
		result = res

		if s.notifSvc != nil {
			var userID uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT user_id FROM orders WHERE id = $1`, res.OrderID).Scan(&userID); err == nil {
				notifC := notifications.Notification{
					RecipientUserID: &userID,
					RecipientKind:   notifications.RecipientKindCustomer,
					Type:            notifications.TypeCustomerFulfillmentPacked,
					Title:           "Сборка готова к отправке",
					Body:            "Заказ упакован на складе и ожидает отправления.",
					EntityType:      "fulfillment",
					EntityID:        fulfillmentID,
				}
				s.notifSvc.CreateNotificationTx(ctx, tx, notifC)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
