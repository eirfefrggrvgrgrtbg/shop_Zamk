package fulfillment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// FBO.DISPATCH.CHECKLIST.1: "is this parcel ready to be handed to the carrier, and if not, why". The backend is the
// single source of truth. The evaluator reads the same data DispatchFulfillmentTx locks and applies the same rules, so
// the screen shows exactly what the dispatch would decide; DispatchFulfillmentTx stays the authority and re-checks
// everything on fresh, locked data (a checklist loaded earlier never permits anything). No manual tick boxes, no new
// status, and no personal data in the result: only codes, statuses and counts.

type ReadinessSeverity string

const (
	// SeverityBlocking: dispatch refuses while this check fails.
	SeverityBlocking ReadinessSeverity = "blocking"
	// SeverityWarning: worth a look, never blocks.
	SeverityWarning ReadinessSeverity = "warning"
	// SeverityInfo: a fact for the operator (what is known, what will appear at dispatch).
	SeverityInfo ReadinessSeverity = "info"
)

const (
	CheckOrderState        = "ORDER_STATE"
	CheckNotShipped        = "NOT_ALREADY_SHIPPED"
	CheckPickingComplete   = "PICKING_COMPLETE"
	CheckPackingComplete   = "PACKING_COMPLETE"
	CheckComposition       = "COMPOSITION_CONSISTENT"
	CheckUnitsAndStock     = "UNITS_AND_STOCK"
	CheckShipmentState     = "SHIPMENT_STATE"
	CheckRecipient         = "RECIPIENT_PRESENT"
	CheckPhone             = "PHONE_PRESENT"
	CheckAddress           = "ADDRESS_PRESENT"
	CheckDeliveryMethod    = "DELIVERY_METHOD"
	CheckPostalCode        = "POSTAL_CODE"
	CheckCourierComment    = "COURIER_COMMENT"
	CheckCarrierAtDispatch = "CARRIER_AT_DISPATCH"
)

type ReadinessCheck struct {
	Code     string            `json:"code"`
	Severity ReadinessSeverity `json:"severity"`
	OK       bool              `json:"ok"`
	// Detail says what is wrong (or what is known) without any personal data.
	Detail string `json:"detail,omitempty"`
}

type DispatchReadiness struct {
	// Ready: no blocking check fails. It is information for the screen; the dispatch decides again by itself.
	Ready         bool             `json:"ready"`
	BlockingCount int              `json:"blockingCount"`
	WarningCount  int              `json:"warningCount"`
	Checks        []ReadinessCheck `json:"checks"`
}

var ErrDispatchDeliveryDataMissing = errors.New("dispatch_delivery_data_missing")

func blank(s *string) bool { return s == nil || strings.TrimSpace(*s) == "" }

// DeliveryDataMissing reports which of the recipient, phone and address an order lacks (a carrier cannot be given a
// parcel without them). Used by the checklist and, authoritatively, by the dispatch transaction.
func DeliveryDataMissing(name, phone, address *string) (recipient, phoneMissing, addressMissing bool) {
	return blank(name), blank(phone), blank(address)
}

type readinessInput struct {
	fulfillmentStatus string
	orderStatus       string
	name, phone, addr *string
	methodName        *string
	postal, comment   *string
	shipmentStatus    *string
	carrier, track    *string
	items             []readinessItem
	stock             map[uuid.UUID]stockLevel
}

type readinessItem struct {
	id, variantID uuid.UUID
	quantity      int
	legacyPicked  int
	allocs        []readinessAlloc
}

type readinessAlloc struct {
	picked     bool
	unitStatus string
	unitVar    uuid.UUID
}

type stockLevel struct{ total, reserved int }

// GetDispatchReadiness reads the parcel and evaluates its checklist.
func (r *Repository) GetDispatchReadiness(ctx context.Context, fulfillmentID uuid.UUID) (*DispatchReadiness, error) {
	return r.dispatchReadiness(ctx, r.db, fulfillmentID)
}

type readinessQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (r *Repository) dispatchReadiness(ctx context.Context, q readinessQuerier, fulfillmentID uuid.UUID) (*DispatchReadiness, error) {
	in := readinessInput{stock: map[uuid.UUID]stockLevel{}}
	var orderID uuid.UUID
	err := q.QueryRow(ctx, `
		SELECT f.status, f.order_id, o.status, o.customer_name, o.customer_phone, o.delivery_address, o.delivery_method_name,
		       o.delivery_postal_code, o.delivery_courier_comment
		FROM order_fulfillments f JOIN orders o ON o.id = f.order_id
		WHERE f.id = $1`, fulfillmentID).Scan(&in.fulfillmentStatus, &orderID, &in.orderStatus, &in.name, &in.phone, &in.addr, &in.methodName, &in.postal, &in.comment)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFulfillmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read dispatch readiness: %w", err)
	}
	if err := q.QueryRow(ctx, `SELECT status, carrier, tracking_number FROM shipments WHERE fulfillment_id = $1 LIMIT 1`, fulfillmentID).
		Scan(&in.shipmentStatus, &in.carrier, &in.track); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to read shipment for readiness: %w", err)
	}

	rows, err := q.Query(ctx, `SELECT id, product_variant_id, quantity, picked_quantity FROM order_items WHERE order_fulfillment_id = $1 ORDER BY created_at, id`, fulfillmentID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var it readinessItem
		if err := rows.Scan(&it.id, &it.variantID, &it.quantity, &it.legacyPicked); err != nil {
			rows.Close()
			return nil, err
		}
		in.items = append(in.items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range in.items {
		arows, err := q.Query(ctx, `
			SELECT a.picked_at IS NOT NULL, u.status, u.product_variant_id
			FROM order_item_allocations a JOIN inventory_units u ON u.id = a.inventory_unit_id
			WHERE a.order_item_id = $1 AND a.released_at IS NULL ORDER BY a.id`, in.items[i].id)
		if err != nil {
			return nil, err
		}
		for arows.Next() {
			var a readinessAlloc
			if err := arows.Scan(&a.picked, &a.unitStatus, &a.unitVar); err != nil {
				arows.Close()
				return nil, err
			}
			in.items[i].allocs = append(in.items[i].allocs, a)
		}
		arows.Close()
		if err := arows.Err(); err != nil {
			return nil, err
		}
		var sl stockLevel
		if err := q.QueryRow(ctx, `SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1`, in.items[i].variantID).Scan(&sl.total, &sl.reserved); err == nil {
			in.stock[in.items[i].variantID] = sl
		}
	}
	return evaluateReadiness(in), nil
}

// evaluateReadiness applies the dispatch rules (see DispatchFulfillmentTx) to what was read.
func evaluateReadiness(in readinessInput) *DispatchReadiness {
	var checks []ReadinessCheck
	add := func(code string, sev ReadinessSeverity, ok bool, detail string) {
		checks = append(checks, ReadinessCheck{Code: code, Severity: sev, OK: ok, Detail: detail})
	}

	// A. the order exists and its state allows a dispatch (assembling / packed, as DispatchFulfillmentTx requires)
	orderOK := in.orderStatus == "assembling" || in.orderStatus == "packed"
	add(CheckOrderState, SeverityBlocking, orderOK, orderStateDetail(in.orderStatus, orderOK))

	// I. not shipped yet
	shipped := in.fulfillmentStatus == "shipped" || in.fulfillmentStatus == "delivered"
	detail := ""
	if shipped {
		detail = "Посылка уже отгружена: повторная отгрузка невозможна."
	}
	add(CheckNotShipped, SeverityBlocking, !shipped, detail)

	// B. picking: every unit picked (serialized) or the legacy picked quantity equals the ordered quantity
	need, got := 0, 0
	compositionOK, compositionDetail := len(in.items) > 0, ""
	if len(in.items) == 0 {
		compositionDetail = "В посылке нет позиций."
	}
	unitsOK, unitsDetail := true, ""
	variantQty := map[uuid.UUID]int{}
	for _, it := range in.items {
		need += it.quantity
		switch {
		case it.quantity <= 0:
			compositionOK, compositionDetail = false, "У позиции посылки некорректное количество."
		case len(it.allocs) == it.quantity:
			for _, a := range it.allocs {
				if a.picked {
					got++
				}
				if a.unitStatus != "warehouse" && unitsOK {
					unitsOK, unitsDetail = false, "Физическая единица посылки уже не на складе (состояние не «warehouse»)."
				}
				if a.unitVar != it.variantID && compositionOK {
					compositionOK, compositionDetail = false, "Закреплённая единица не соответствует товару позиции."
				}
			}
			variantQty[it.variantID] += it.quantity
		case len(it.allocs) == 0:
			got += it.legacyPicked
			variantQty[it.variantID] += it.quantity
		default:
			if compositionOK {
				compositionOK, compositionDetail = false, fmt.Sprintf("Закреплено %d ед. при заказанном количестве %d: состав посылки не согласован со сборкой.", len(it.allocs), it.quantity)
			}
		}
		if len(it.allocs) == 0 && it.legacyPicked > it.quantity && compositionOK {
			compositionOK, compositionDetail = false, "Собрано больше, чем заказано."
		}
	}
	pickedOK := need > 0 && got == need
	pickDetail := ""
	if !pickedOK && need > 0 {
		pickDetail = fmt.Sprintf("Собрано %d из %d ед.", got, need)
	}
	add(CheckPickingComplete, SeverityBlocking, pickedOK, pickDetail)

	// C. packing finished: the fulfillment is «packed» (or already past it)
	packedOK := in.fulfillmentStatus == "packed" || shipped
	add(CheckPackingComplete, SeverityBlocking, packedOK, packingDetail(in.fulfillmentStatus, packedOK))

	// D. the parcel's composition agrees with what was picked / allocated
	add(CheckComposition, SeverityBlocking, compositionOK, compositionDetail)
	stockOK, stockDetail := true, ""
	for v, qty := range variantQty {
		sl, found := in.stock[v]
		if !found {
			stockOK, stockDetail = false, "Для товара посылки нет складской записи."
			break
		}
		if sl.total < qty || sl.reserved < qty {
			stockOK, stockDetail = false, "На складе недостаточно остатка или резерва для списания при отгрузке."
			break
		}
	}
	add(CheckUnitsAndStock, SeverityBlocking, unitsOK && stockOK, firstNonEmpty(unitsDetail, stockDetail))

	// an existing shipment must still be in a pre-dispatch state (DispatchFulfillmentTx: ErrShipmentContradictoryState)
	shipmentOK := true
	if in.shipmentStatus != nil && !shipped {
		switch *in.shipmentStatus {
		case "pending", "assembling", "packed":
		default:
			shipmentOK = false
		}
	}
	shipDetail := ""
	if !shipmentOK {
		shipDetail = "Запись отправления в противоречивом состоянии: отгрузка невозможна, обратитесь к администратору."
	}
	add(CheckShipmentState, SeverityBlocking, shipmentOK, shipDetail)

	// E–G. recipient, phone, address (the order's own snapshot)
	noName, noPhone, noAddr := DeliveryDataMissing(in.name, in.phone, in.addr)
	add(CheckRecipient, SeverityBlocking, !noName, ifMissing(noName, "В заказе не указан получатель."))
	add(CheckPhone, SeverityBlocking, !noPhone, ifMissing(noPhone, "В заказе не указан телефон получателя."))
	add(CheckAddress, SeverityBlocking, !noAddr, ifMissing(noAddr, "В заказе не указан адрес доставки."))

	// H. the delivery method: warning only (dispatch needs no carrier data; a service and a track appear at dispatch)
	methodOK := !blank(in.methodName)
	add(CheckDeliveryMethod, SeverityWarning, methodOK, ifMissing(!methodOK, "В заказе не указан способ доставки."))

	// information: optional data and what will be filled in later; none of this blocks
	add(CheckPostalCode, SeverityInfo, !blank(in.postal), ifMissing(blank(in.postal), "Индекс не указан (не обязателен)."))
	add(CheckCourierComment, SeverityInfo, !blank(in.comment), ifMissing(blank(in.comment), "Комментарий курьеру не указан (не обязателен)."))
	carrierKnown := !blank(in.carrier) || !blank(in.track)
	add(CheckCarrierAtDispatch, SeverityInfo, carrierKnown, ifMissing(!carrierKnown, "Служба доставки и трек-номер появятся при отгрузке."))

	res := &DispatchReadiness{Checks: checks}
	for _, c := range checks {
		if c.OK {
			continue
		}
		switch c.Severity {
		case SeverityBlocking:
			res.BlockingCount++
		case SeverityWarning:
			res.WarningCount++
		}
	}
	res.Ready = res.BlockingCount == 0
	return res
}

func ifMissing(missing bool, text string) string {
	if missing {
		return text
	}
	return ""
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func orderStateDetail(status string, ok bool) string {
	if ok {
		return ""
	}
	labels := map[string]string{"awaiting_payment": "ожидает оплаты", "paid": "оплачен, сборка не начата", "shipped": "уже отгружен", "delivered": "доставлен", "cancelled": "отменён"}
	if l, found := labels[status]; found {
		return "Заказ " + l + ": отгрузка невозможна."
	}
	return "Состояние заказа не допускает отгрузку."
}

func packingDetail(status string, ok bool) string {
	if ok {
		return ""
	}
	switch status {
	case "awaiting_payment":
		return "Заказ ещё не оплачен."
	case "paid":
		return "Сборка не начата, упаковка не выполнена."
	case "assembling":
		return "Упаковка не завершена: посылка ещё в сборке."
	case "cancelled":
		return "Посылка отменена."
	}
	return "Упаковка посылки не завершена."
}
