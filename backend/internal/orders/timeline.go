package orders

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var orderEventTypePriority = map[string]int{
	"order.created":              10,
	"order.reserved":             15,
	"order.reservation_released": 16,
	"order.paid":                 20,
	"order.picking_started":      25,
	"order.unit_picked":          30,
	"order.packed":               40,
	"order.shipped":              50,
	"order.delivered":            60,
	"order.cancelled":            70,
}

// AssembleAdminOrderTimeline is the single canonical assembler for Order timelines.
func AssembleAdminOrderTimeline(ctx context.Context, db *pgxpool.Pool, orderID uuid.UUID) (*TimelineResponse, error) {
	var orderNumber *string
	var createdAt time.Time
	var cancelledAt *time.Time

	err := db.QueryRow(ctx, `
		SELECT order_number, created_at, cancelled_at
		FROM orders
		WHERE id = $1
	`, orderID).Scan(&orderNumber, &createdAt, &cancelledAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to query order: %w", err)
	}

	canonicalOrderNum := "Неизвестно"
	if orderNumber != nil && *orderNumber != "" {
		canonicalOrderNum = *orderNumber
	}

	var events []TimelineEvent

	// 1. order.created
	events = append(events, TimelineEvent{
		ID:          fmt.Sprintf("order-created-%s", orderID),
		Type:        "order.created",
		OccurredAt:  createdAt,
		Title:       "Заказ создан",
		Description: fmt.Sprintf("Оформлен заказ %s", canonicalOrderNum),
		ActorType:   "system",
		ActorLabel:  "Система",
	})

	// 2. order.reserved & order.reservation_released
	resRows, err := db.Query(ctx, `
		SELECT r.id, r.quantity, r.status, r.created_at, r.released_at
		FROM reservations r
		WHERE r.order_id = $1
		   OR r.id IN (SELECT reservation_id FROM order_reservations WHERE order_id = $1)
		ORDER BY r.created_at ASC, r.id ASC
	`, orderID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to query reservations: %w", err)
	}
	if err == nil {
		defer resRows.Close()
		for resRows.Next() {
			var rID uuid.UUID
			var qty int
			var status string
			var rCreatedAt time.Time
			var releasedAt *time.Time
			if err := resRows.Scan(&rID, &qty, &status, &rCreatedAt, &releasedAt); err != nil {
				return nil, fmt.Errorf("failed to scan reservation: %w", err)
			}
			events = append(events, TimelineEvent{
				ID:          fmt.Sprintf("order-reserved-%s", rID),
				Type:        "order.reserved",
				OccurredAt:  rCreatedAt,
				Title:       "Товар зарезервирован",
				Description: fmt.Sprintf("Товар зарезервирован для заказа %s", canonicalOrderNum),
				ActorType:   "system",
				ActorLabel:  "Система",
			})
			if releasedAt != nil {
				events = append(events, TimelineEvent{
					ID:          fmt.Sprintf("order-reservation-released-%s", rID),
					Type:        "order.reservation_released",
					OccurredAt:  *releasedAt,
					Title:       "Резерв отменен",
					Description: fmt.Sprintf("Резерв товара отменен для заказа %s", canonicalOrderNum),
					ActorType:   "system",
					ActorLabel:  "Система",
				})
			}
		}
		if err := resRows.Err(); err != nil {
			return nil, fmt.Errorf("reservation rows error: %w", err)
		}
	}

	// 3. order.paid
	pRows, err := db.Query(ctx, `
		SELECT id, status, payment_number, paid_at
		FROM payments
		WHERE order_id = $1 AND status IN ('succeeded', 'paid') AND paid_at IS NOT NULL
		ORDER BY paid_at ASC, id ASC
	`, orderID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to query payments: %w", err)
	}
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var pID uuid.UUID
			var status string
			var pNumber string
			var paidAt *time.Time
			if err := pRows.Scan(&pID, &status, &pNumber, &paidAt); err != nil {
				return nil, fmt.Errorf("failed to scan payment: %w", err)
			}
			if paidAt != nil {
				events = append(events, TimelineEvent{
					ID:          fmt.Sprintf("order-paid-%s", pID),
					Type:        "order.paid",
					OccurredAt:  *paidAt,
					Title:       "Заказ оплачен",
					Description: fmt.Sprintf("Оплата %s", pNumber),
					ActorType:   "system",
					ActorLabel:  "Система",
				})
			}
		}
		if err := pRows.Err(); err != nil {
			return nil, fmt.Errorf("payment rows error: %w", err)
		}
	}

	// 4. order_status_history (order.picking_started & order.cancelled fallback)
	hRows, err := db.Query(ctx, `
		SELECT id, from_status, to_status, comment, created_at
		FROM order_status_history
		WHERE order_id = $1
		ORDER BY created_at ASC, id ASC
	`, orderID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to query status history: %w", err)
	}
	hasCancelledInHistory := false
	if err == nil {
		defer hRows.Close()
		for hRows.Next() {
			var hID uuid.UUID
			var fromStatus, toStatus, comment *string
			var hCreatedAt time.Time
			if err := hRows.Scan(&hID, &fromStatus, &toStatus, &comment, &hCreatedAt); err != nil {
				return nil, fmt.Errorf("failed to scan status history: %w", err)
			}
			if toStatus != nil {
				switch *toStatus {
				case "assembling":
					events = append(events, TimelineEvent{
						ID:          fmt.Sprintf("order-picking-started-%s", hID),
						Type:        "order.picking_started",
						OccurredAt:  hCreatedAt,
						Title:       "Сборка начата",
						Description: fmt.Sprintf("Заказ %s передан в сборку", canonicalOrderNum),
						ActorType:   "warehouse",
						ActorLabel:  "Склад ZAMK",
					})
				case "cancelled":
					hasCancelledInHistory = true
					events = append(events, TimelineEvent{
						ID:          fmt.Sprintf("order-cancelled-%s", hID),
						Type:        "order.cancelled",
						OccurredAt:  hCreatedAt,
						Title:       "Заказ отменен",
						Description: "Заказ был отменен",
						ActorType:   "admin",
						ActorLabel:  "Администратор",
					})
				}
			}
		}
		if err := hRows.Err(); err != nil {
			return nil, fmt.Errorf("status history rows error: %w", err)
		}
	}

	// 5. order.unit_picked
	allocRows, err := db.Query(ctx, `
		SELECT a.id, u.unit_code, a.picked_at, COALESCE(oi.title, '')
		FROM order_item_allocations a
		JOIN order_items oi ON a.order_item_id = oi.id
		LEFT JOIN inventory_units u ON a.inventory_unit_id = u.id
		WHERE oi.order_id = $1 AND a.picked_at IS NOT NULL
		ORDER BY a.picked_at ASC, a.id ASC
	`, orderID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to query allocations: %w", err)
	}
	if err == nil {
		defer allocRows.Close()
		for allocRows.Next() {
			var aID uuid.UUID
			var code, title string
			var pickedAt time.Time
			if err := allocRows.Scan(&aID, &code, &pickedAt, &title); err != nil {
				return nil, fmt.Errorf("failed to scan allocation: %w", err)
			}
			desc := fmt.Sprintf("Собран юнит %s", code)
			if title != "" {
				desc = fmt.Sprintf("%s · %s", code, title)
			}
			events = append(events, TimelineEvent{
				ID:          fmt.Sprintf("order-unit-picked-%s", aID),
				Type:        "order.unit_picked",
				OccurredAt:  pickedAt,
				Title:       "Сборка товара",
				Description: desc,
				ActorType:   "warehouse",
				ActorLabel:  "Склад ZAMK",
			})
		}
		if err := allocRows.Err(); err != nil {
			return nil, fmt.Errorf("allocation rows error: %w", err)
		}
	}

	// 6. order.packed
	fRows, err := db.Query(ctx, `
		SELECT id, packed_at
		FROM order_fulfillments
		WHERE order_id = $1 AND packed_at IS NOT NULL
		ORDER BY packed_at ASC, id ASC
	`, orderID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to query fulfillments: %w", err)
	}
	if err == nil {
		defer fRows.Close()
		for fRows.Next() {
			var fID uuid.UUID
			var packedAt time.Time
			if err := fRows.Scan(&fID, &packedAt); err != nil {
				return nil, fmt.Errorf("failed to scan fulfillment: %w", err)
			}
			events = append(events, TimelineEvent{
				ID:          fmt.Sprintf("order-packed-%s", fID),
				Type:        "order.packed",
				OccurredAt:  packedAt,
				Title:       "Заказ упакован",
				Description: fmt.Sprintf("Заказ %s собран и упакован", canonicalOrderNum),
				ActorType:   "warehouse",
				ActorLabel:  "Склад ZAMK",
			})
		}
		if err := fRows.Err(); err != nil {
			return nil, fmt.Errorf("fulfillment rows error: %w", err)
		}
	}

	// 7. order.shipped & order.delivered
	sRows, err := db.Query(ctx, `
		SELECT id, status, tracking_number, shipped_at, delivered_at
		FROM shipments
		WHERE order_id = $1
		ORDER BY created_at ASC, id ASC
	`, orderID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to query shipments: %w", err)
	}
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var sID uuid.UUID
			var status string
			var tracking *string
			var shippedAt, deliveredAt *time.Time
			if err := sRows.Scan(&sID, &status, &tracking, &shippedAt, &deliveredAt); err != nil {
				return nil, fmt.Errorf("failed to scan shipment: %w", err)
			}
			trackingStr := ""
			if tracking != nil && *tracking != "" {
				trackingStr = fmt.Sprintf(" (трек: %s)", *tracking)
			}
			if shippedAt != nil {
				events = append(events, TimelineEvent{
					ID:          fmt.Sprintf("order-shipped-%s", sID),
					Type:        "order.shipped",
					OccurredAt:  *shippedAt,
					Title:       "Заказ передан в доставку",
					Description: fmt.Sprintf("Отправлен%s", trackingStr),
					ActorType:   "system",
					ActorLabel:  "Служба доставки",
				})
			}
			if deliveredAt != nil {
				events = append(events, TimelineEvent{
					ID:          fmt.Sprintf("order-delivered-%s", sID),
					Type:        "order.delivered",
					OccurredAt:  *deliveredAt,
					Title:       "Заказ доставлен",
					Description: fmt.Sprintf("Доставлен%s", trackingStr),
					ActorType:   "system",
					ActorLabel:  "Служба доставки",
				})
			}
		}
		if err := sRows.Err(); err != nil {
			return nil, fmt.Errorf("shipment rows error: %w", err)
		}
	}

	// 8. order.cancelled fallback if not present in order_status_history
	if cancelledAt != nil && !hasCancelledInHistory {
		events = append(events, TimelineEvent{
			ID:          fmt.Sprintf("order-cancelled-%s", orderID),
			Type:        "order.cancelled",
			OccurredAt:  *cancelledAt,
			Title:       "Заказ отменен",
			Description: "Заказ был отменен",
			ActorType:   "admin",
			ActorLabel:  "Администратор",
		})
	}

	// Deterministic tie-breaker sorting
	sort.Slice(events, func(i, j int) bool {
		if !events[i].OccurredAt.Equal(events[j].OccurredAt) {
			return events[i].OccurredAt.Before(events[j].OccurredAt)
		}
		pI := orderEventTypePriority[events[i].Type]
		pJ := orderEventTypePriority[events[j].Type]
		if pI != pJ {
			return pI < pJ
		}
		if events[i].Type != events[j].Type {
			return events[i].Type < events[j].Type
		}
		return events[i].ID < events[j].ID
	})

	return &TimelineResponse{
		EntityType:          "order",
		EntityID:            orderID,
		CanonicalIdentifier: canonicalOrderNum,
		Events:              events,
	}, nil
}

func (s *Service) GetAdminOrderTimeline(ctx context.Context, orderID uuid.UUID) (*TimelineResponse, error) {
	return AssembleAdminOrderTimeline(ctx, s.db.Pool, orderID)
}
