package fulfillment

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
)

func (s *Service) DeliverShipment(ctx context.Context, adminID, shipmentID uuid.UUID, req DeliverShipmentRequest) (*DeliveryResult, error) {
	var result *DeliveryResult
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		res, err := s.repo.DeliverShipmentTx(ctx, tx, adminID, shipmentID, req.Comment)
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

		if s.payouts != nil {
			if err := s.payouts.CreatePendingSalesForFulfillmentTx(ctx, tx, res.FulfillmentID); err != nil {
				return fmt.Errorf("payouts.CreatePendingSalesForFulfillmentTx: %w", err)
			}
		}


		if s.notifSvc != nil {
			// Customer notification
			var userID uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT user_id FROM orders WHERE id = $1`, res.OrderID).Scan(&userID); err == nil {
				notifC := notifications.Notification{
					RecipientUserID: &userID,
					RecipientKind:   notifications.RecipientKindCustomer,
					Type:            notifications.TypeCustomerShipmentDelivered,
					Title:           "Заказ доставлен",
					Body:            "Ваш заказ успешно доставлен.",
					EntityType:      "shipment",
					EntityID:        res.ShipmentID,
				}
				_ = s.notifSvc.CreateNotificationTx(ctx, tx, notifC)
			}

			// Seller notification
			var sellerID uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT seller_id FROM order_fulfillments WHERE id = $1`, res.FulfillmentID).Scan(&sellerID); err == nil {
				notifS := notifications.Notification{
					RecipientSellerID: &sellerID,
					RecipientKind:     notifications.RecipientKindSeller,
					Type:              notifications.TypeSellerShipmentDelivered,
					Title:             "Отправление доставлено",
					Body:              "Отправление успешно доставлено покупателю.",
					EntityType:        "shipment",
					EntityID:          res.ShipmentID,
				}
				_ = s.notifSvc.CreateNotificationTx(ctx, tx, notifS)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
