package returns

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var returnEventTypePriority = map[string]int{
	"return.requested":         10,
	"return.info_requested":    20,
	"return.customer_replied":  25,
	"return.approved":          30,
	"return.rejected":          30,
	"return.logistics_created": 35,
	"return.receiving_started": 40,
	"return.unit_scanned":      50,
	"return.refunded":          60,
}

func (s *Service) GetAdminTimeline(ctx context.Context, returnID uuid.UUID) (*TimelineResponse, error) {
	var retID, orderID uuid.UUID
	var status, orderNumber string
	var createdAt time.Time
	var approvedAt, rejectedAt, receivingStartedAt, completedAt *time.Time

	queryReturn := `
		SELECT r.id, r.order_id, r.status, r.created_at, r.approved_at, r.rejected_at, r.receiving_started_at, r.completed_at, o.order_number
		FROM returns r
		JOIN orders o ON r.order_id = o.id
		WHERE r.id = $1
	`
	err := s.db.Pool.QueryRow(ctx, queryReturn, returnID).Scan(
		&retID, &orderID, &status, &createdAt, &approvedAt, &rejectedAt, &receivingStartedAt, &completedAt, &orderNumber,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrReturnNotFound
		}
		return nil, fmt.Errorf("failed to query return: %w", err)
	}

	var events []TimelineEvent

	// 1. return.requested
	events = append(events, TimelineEvent{
		ID:          fmt.Sprintf("return-requested-%s", retID),
		Type:        "return.requested",
		OccurredAt:  createdAt,
		Title:       "Заявка создана",
		Description: fmt.Sprintf("Заявка на возврат по заказу %s", orderNumber),
		ActorType:   "customer",
		ActorLabel:  "Покупатель",
	})

	// 2. return.approved
	if approvedAt != nil {
		events = append(events, TimelineEvent{
			ID:          fmt.Sprintf("return-approved-%s", retID),
			Type:        "return.approved",
			OccurredAt:  *approvedAt,
			Title:       "Возврат одобрен",
			Description: "Заявка на возврат была одобрена",
			ActorType:   "admin",
			ActorLabel:  "Администратор",
		})
	}

	// 3. return.rejected
	if rejectedAt != nil {
		events = append(events, TimelineEvent{
			ID:          fmt.Sprintf("return-rejected-%s", retID),
			Type:        "return.rejected",
			OccurredAt:  *rejectedAt,
			Title:       "Возврат отклонен",
			Description: "Заявка на возврат была отклонена",
			ActorType:   "admin",
			ActorLabel:  "Администратор",
		})
	}

	// 4. return.logistics_created from return_shipments
	sRows, err := s.db.Pool.Query(ctx, `
		SELECT id, provider, method, tracking_number, created_at
		FROM return_shipments
		WHERE return_id = $1
		ORDER BY created_at ASC, id ASC
	`, returnID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to query return shipments: %w", err)
	}
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var sID uuid.UUID
			var provider, method string
			var tracking *string
			var sCreatedAt time.Time
			if err := sRows.Scan(&sID, &provider, &method, &tracking, &sCreatedAt); err != nil {
				return nil, fmt.Errorf("failed to scan return shipment: %w", err)
			}
			var methodDesc string
			switch method {
			case "cdek_courier":
				methodDesc = "Способ возврата: СДЭК — курьер"
			case "cdek_office":
				methodDesc = "Способ возврата: СДЭК — отделение"
			default:
				methodDesc = fmt.Sprintf("Способ возврата: %s", method)
			}
			events = append(events, TimelineEvent{
				ID:          fmt.Sprintf("return-logistics-%s", sID),
				Type:        "return.logistics_created",
				OccurredAt:  sCreatedAt,
				Title:       "Оформлена доставка возврата",
				Description: methodDesc,
				ActorType:   "customer",
				ActorLabel:  "Покупатель",
			})
		}
		if err := sRows.Err(); err != nil {
			return nil, fmt.Errorf("return shipment rows error: %w", err)
		}
	}

	// 5. return.receiving_started
	if receivingStartedAt != nil {
		events = append(events, TimelineEvent{
			ID:          fmt.Sprintf("return-receiving-started-%s", retID),
			Type:        "return.receiving_started",
			OccurredAt:  *receivingStartedAt,
			Title:       "Начата приемка",
			Description: "Сотрудник склада начал приемку возврата",
			ActorType:   "warehouse",
			ActorLabel:  "Склад ZAMK",
		})
	}

	// 6. Scanned physical units (single bounded query)
	uRows, err := s.db.Pool.Query(ctx, `
		SELECT riu.id, iu.unit_code, oi.title, riu.scanned_at
		FROM return_item_units riu
		JOIN return_items ri ON ri.id = riu.return_item_id
		JOIN order_item_allocations oia ON riu.order_item_allocation_id = oia.id
		JOIN inventory_units iu ON oia.inventory_unit_id = iu.id
		JOIN order_items oi ON oia.order_item_id = oi.id
		WHERE ri.return_id = $1 AND riu.scanned_at IS NOT NULL
		ORDER BY riu.scanned_at ASC, riu.id ASC
	`, returnID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to query return scanned units: %w", err)
	}
	if err == nil {
		defer uRows.Close()
		for uRows.Next() {
			var uID uuid.UUID
			var unitCode, productTitle string
			var scannedAt time.Time
			if err := uRows.Scan(&uID, &unitCode, &productTitle, &scannedAt); err != nil {
				return nil, fmt.Errorf("failed to scan return unit: %w", err)
			}
			events = append(events, TimelineEvent{
				ID:          fmt.Sprintf("return-unit-scanned-%s", uID),
				Type:        "return.unit_scanned",
				OccurredAt:  scannedAt,
				Title:       "Товар принят на складе",
				Description: fmt.Sprintf("%s · %s", unitCode, productTitle),
				ActorType:   "warehouse",
				ActorLabel:  "Склад ZAMK",
			})
		}
		if err := uRows.Err(); err != nil {
			return nil, fmt.Errorf("return unit rows error: %w", err)
		}
	}

	// 7. Workflow messages (info_requested & customer_replied)
	mRows, err := s.db.Pool.Query(ctx, `
		SELECT id, sender_role, message_type, created_at
		FROM return_messages
		WHERE return_id = $1
		ORDER BY created_at ASC, id ASC
	`, returnID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to query return messages: %w", err)
	}
	if err == nil {
		defer mRows.Close()
		waitingForReply := false
		for mRows.Next() {
			var mID uuid.UUID
			var senderRole, messageType string
			var msgCreatedAt time.Time
			if err := mRows.Scan(&mID, &senderRole, &messageType, &msgCreatedAt); err != nil {
				return nil, fmt.Errorf("failed to scan return message: %w", err)
			}
			if messageType == ReturnMessageTypeInfoRequest {
				events = append(events, TimelineEvent{
					ID:          fmt.Sprintf("return-info-requested-%s", mID),
					Type:        "return.info_requested",
					OccurredAt:  msgCreatedAt,
					Title:       "Запрошено уточнение",
					Description: "Сотрудник запросил дополнительную информацию",
					ActorType:   "admin",
					ActorLabel:  "Администратор",
				})
				waitingForReply = true
			} else if waitingForReply && senderRole == ReturnMessageSenderRoleCustomer {
				events = append(events, TimelineEvent{
					ID:          fmt.Sprintf("return-customer-replied-%s", mID),
					Type:        "return.customer_replied",
					OccurredAt:  msgCreatedAt,
					Title:       "Получен ответ покупателя",
					Description: "Покупатель предоставил запрошенную информацию",
					ActorType:   "customer",
					ActorLabel:  "Покупатель",
				})
				waitingForReply = false
			}
		}
		if err := mRows.Err(); err != nil {
			return nil, fmt.Errorf("return message rows error: %w", err)
		}
	}

	// 8. return.refunded from refunds (strictly when status is succeeded/completed and processed_at is not null)
	refRows, err := s.db.Pool.Query(ctx, `
		SELECT id, amount_cents, status, processed_at
		FROM refunds
		WHERE return_id = $1 AND status IN ('succeeded', 'completed') AND processed_at IS NOT NULL
		ORDER BY processed_at ASC, id ASC
	`, returnID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to query refunds: %w", err)
	}
	if err == nil {
		defer refRows.Close()
		for refRows.Next() {
			var refID uuid.UUID
			var amtCents int64
			var refStatus string
			var processedAt *time.Time
			if err := refRows.Scan(&refID, &amtCents, &refStatus, &processedAt); err != nil {
				return nil, fmt.Errorf("failed to scan refund: %w", err)
			}
			if processedAt == nil {
				continue
			}
			events = append(events, TimelineEvent{
				ID:          fmt.Sprintf("return-refunded-%s", refID),
				Type:        "return.refunded",
				OccurredAt:  *processedAt,
				Title:       "Возврат средств выполнен",
				Description: fmt.Sprintf("Возврат средств покупателю (%d ₽)", amtCents/100),
				ActorType:   "system",
				ActorLabel:  "Система",
			})
		}
		if err := refRows.Err(); err != nil {
			return nil, fmt.Errorf("refund rows error: %w", err)
		}
	}

	// Deterministic sorting
	sort.Slice(events, func(i, j int) bool {
		if !events[i].OccurredAt.Equal(events[j].OccurredAt) {
			return events[i].OccurredAt.Before(events[j].OccurredAt)
		}
		pI := returnEventTypePriority[events[i].Type]
		pJ := returnEventTypePriority[events[j].Type]
		if pI != pJ {
			return pI < pJ
		}
		if events[i].Type != events[j].Type {
			return events[i].Type < events[j].Type
		}
		return events[i].ID < events[j].ID
	})

	return &TimelineResponse{
		EntityType:          "return",
		EntityID:            retID,
		CanonicalIdentifier: orderNumber,
		Events:              events,
	}, nil
}
