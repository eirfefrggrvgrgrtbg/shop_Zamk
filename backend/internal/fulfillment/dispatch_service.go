package fulfillment

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
)

func (s *Service) DispatchFulfillment(ctx context.Context, adminID, fulfillmentID uuid.UUID) (*DispatchResult, error) {
	var result *DispatchResult
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		res, err := s.repo.DispatchFulfillmentTx(ctx, tx, adminID, fulfillmentID)
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
			// Customer notification
			var userID uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT user_id FROM orders WHERE id = $1`, res.OrderID).Scan(&userID); err == nil {
				notifC := notifications.Notification{
					RecipientUserID: &userID,
					RecipientKind:   notifications.RecipientKindCustomer,
					Type:            notifications.TypeCustomerShipmentShipped,
					Title:           "Статус отправления обновлен",
					Body:            "Ваша сборка отправлена.",
					EntityType:      "shipment",
					EntityID:        res.ShipmentID,
				}
				s.notifSvc.CreateNotificationTx(ctx, tx, notifC)
			}

			// Seller notification
			var sellerID uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT seller_id FROM order_fulfillments WHERE id = $1`, fulfillmentID).Scan(&sellerID); err == nil {
				notifS := notifications.Notification{
					RecipientSellerID: &sellerID,
					RecipientKind:     notifications.RecipientKindSeller,
					Type:              notifications.TypeSellerShipmentShipped,
					Title:             "Статус отправления обновлен",
					Body:              "Сборка отправлена.",
					EntityType:        "shipment",
					EntityID:          res.ShipmentID,
				}
				s.notifSvc.CreateNotificationTx(ctx, tx, notifS)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
