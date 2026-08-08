package payments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// buildProblems checks all rules and returns an array of PaymentProblem
func buildProblems(p AdminPaymentDTO, oStatus string, oTotal int64, succeededCount int, hasInvalid bool, hasUnprocessed bool, isStuck bool) []PaymentProblem {
	var problems []PaymentProblem

	if oStatus == "paid" && succeededCount == 0 {
		problems = append(problems, PaymentProblem{Code: "PAID_ORDER_WITHOUT_SUCCEEDED_PAYMENT", Severity: "critical"})
	}
	if p.Status == "succeeded" && oStatus != "paid" {
		problems = append(problems, PaymentProblem{Code: "SUCCEEDED_PAYMENT_ORDER_NOT_PAID", Severity: "critical"})
	}
	if succeededCount > 1 && p.Status == "succeeded" {
		problems = append(problems, PaymentProblem{Code: "MULTIPLE_SUCCEEDED_PAYMENTS", Severity: "critical"})
	}
	if p.Status == "succeeded" && p.AmountCents != oTotal {
		problems = append(problems, PaymentProblem{Code: "AMOUNT_MISMATCH", Severity: "warning"})
	}
	if isStuck {
		problems = append(problems, PaymentProblem{Code: "STUCK_PENDING", Severity: "warning"})
	}
	if hasInvalid {
		problems = append(problems, PaymentProblem{Code: "INVALID_WEBHOOK_SIGNATURE", Severity: "warning"})
	}
	if hasUnprocessed {
		problems = append(problems, PaymentProblem{Code: "UNPROCESSED_WEBHOOK", Severity: "warning"})
	}

	if problems == nil {
		problems = make([]PaymentProblem, 0)
	}
	return problems
}

func calculateRefundFields(p *AdminPaymentDTO) {
	if p.Status == "succeeded" {
		p.PaidAmountCents = p.AmountCents
	} else {
		p.PaidAmountCents = 0
	}

	p.ReservedRefundAmountCents = p.SucceededRefundedAmountCents + p.PendingRefundAmountCents

	net := p.PaidAmountCents - p.SucceededRefundedAmountCents
	if net < 0 {
		net = 0
	}
	p.NetAmountCents = net

	avail := p.PaidAmountCents - p.ReservedRefundAmountCents
	if avail < 0 {
		avail = 0
	}
	p.AvailableToRefundCents = avail

	if p.SucceededRefundedAmountCents == 0 && p.PendingRefundAmountCents == 0 {
		p.RefundState = "none"
	} else if p.PendingRefundAmountCents > 0 && p.SucceededRefundedAmountCents == 0 {
		p.RefundState = "pending"
	} else if p.SucceededRefundedAmountCents > 0 && p.SucceededRefundedAmountCents < p.PaidAmountCents && p.PendingRefundAmountCents == 0 {
		p.RefundState = "partial"
	} else if p.PaidAmountCents > 0 && p.SucceededRefundedAmountCents >= p.PaidAmountCents {
		p.RefundState = "full"
	} else if p.SucceededRefundedAmountCents > 0 && p.PendingRefundAmountCents > 0 && p.ReservedRefundAmountCents < p.PaidAmountCents {
		p.RefundState = "partial_pending"
	} else if p.ReservedRefundAmountCents >= p.PaidAmountCents && p.SucceededRefundedAmountCents < p.PaidAmountCents {
		p.RefundState = "full_pending"
	} else {
		p.RefundState = "none"
	}
}

func (r *Repository) ListAdminPayments(ctx context.Context, q, status, provider, method, mode, refundState, probCode, dateFrom, dateTo string, amountFrom, amountTo int64, hasProblem bool, sort, direction string, limit, offset, stuckMins int) ([]AdminPaymentDTO, int, error) {
	// Build the massive query. We will use a CTE to pre-calculate attempts, refunds, and order data so filtering can apply cleanly.
	cte := `
WITH payment_attempts AS (
	SELECT id, 
	ROW_NUMBER() OVER (PARTITION BY order_id ORDER BY created_at ASC, id ASC) as attempt_number,
	COUNT(*) OVER (PARTITION BY order_id) as attempts_count,
	SUM(CASE WHEN status = 'succeeded' THEN 1 ELSE 0 END) OVER (PARTITION BY order_id) as succeeded_payments_count
	FROM payments
),
payment_refunds AS (
	SELECT payment_id,
	COALESCE(SUM(amount_cents) FILTER (WHERE status = 'succeeded'), 0) as succeeded_amount,
	COALESCE(SUM(amount_cents) FILTER (WHERE status = 'pending'), 0) as pending_amount
	FROM refunds
	GROUP BY payment_id
),
payment_events_summary AS (
	SELECT payment_id,
	BOOL_OR(signature_valid = false) as has_invalid_webhook,
	BOOL_OR(signature_valid = true AND processed_at IS NULL) as has_unprocessed_webhook
	FROM payment_events
	GROUP BY payment_id
),
base_data AS (
	SELECT 
		p.id, p.order_id, p.provider, p.provider_payment_id, p.status, p.amount_cents, p.currency, p.payment_number, p.payment_method, p.integration_mode, p.created_at, p.updated_at, p.paid_at, p.failed_at, p.cancelled_at,
		o.order_number, o.status as order_status, o.total_price_cents as order_total_cents,
		u.id as customer_id, u.name as customer_name, u.email as customer_email, u.phone as customer_phone,
		pa.attempt_number, pa.attempts_count, pa.succeeded_payments_count,
		COALESCE(pr.succeeded_amount, 0) as succeeded_refunded,
		COALESCE(pr.pending_amount, 0) as pending_refunded,
		COALESCE(pes.has_invalid_webhook, false) as has_invalid_webhook,
		COALESCE(pes.has_unprocessed_webhook, false) as has_unprocessed_webhook,
		(p.status IN ('created', 'pending') AND p.created_at < now() - interval '1 minute' * $1) as is_stuck
	FROM payments p
	JOIN orders o ON p.order_id = o.id
	JOIN users u ON o.user_id = u.id
	LEFT JOIN payment_attempts pa ON p.id = pa.id
	LEFT JOIN payment_refunds pr ON p.id = pr.payment_id
	LEFT JOIN payment_events_summary pes ON p.id = pes.payment_id
)
SELECT * FROM base_data WHERE 1=1
`
	args := []any{stuckMins}
	argIdx := 2
	where := ""

	if q != "" {
		where += fmt.Sprintf(` AND (payment_number ILIKE $%d OR provider_payment_id ILIKE $%d OR order_number ILIKE $%d OR customer_email ILIKE $%d OR customer_name ILIKE $%d OR customer_phone ILIKE $%d OR id::text ILIKE $%d)`, argIdx, argIdx, argIdx, argIdx, argIdx, argIdx, argIdx)
		qLike := "%" + q + "%"
		args = append(args, qLike)
		argIdx++
	}

	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	if provider != "" {
		where += fmt.Sprintf(" AND provider = $%d", argIdx)
		args = append(args, provider)
		argIdx++
	}
	
	if method != "" {
		where += fmt.Sprintf(" AND payment_method = $%d", argIdx)
		args = append(args, method)
		argIdx++
	}

	if mode != "" {
		where += fmt.Sprintf(" AND integration_mode = $%d", argIdx)
		args = append(args, mode)
		argIdx++
	}

	if amountFrom > 0 {
		where += fmt.Sprintf(" AND amount_cents >= $%d", argIdx)
		args = append(args, amountFrom)
		argIdx++
	}
	if amountTo > 0 {
		where += fmt.Sprintf(" AND amount_cents <= $%d", argIdx)
		args = append(args, amountTo)
		argIdx++
	}

	if dateFrom != "" {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, dateFrom)
		argIdx++
	}
	if dateTo != "" {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, dateTo)
		argIdx++
	}

	// Dynamic calculation conditions for refund state
	if refundState != "" {
		// Calculate paid_amount internally
		paidCalc := "CASE WHEN status = 'succeeded' THEN amount_cents ELSE 0 END"
		resCalc := "succeeded_refunded + pending_refunded"
		
		switch refundState {
		case "none":
			where += " AND succeeded_refunded = 0 AND pending_refunded = 0"
		case "pending":
			where += " AND pending_refunded > 0 AND succeeded_refunded = 0"
		case "partial":
			where += fmt.Sprintf(" AND succeeded_refunded > 0 AND succeeded_refunded < %s AND pending_refunded = 0", paidCalc)
		case "full":
			where += fmt.Sprintf(" AND %s > 0 AND succeeded_refunded >= %s", paidCalc, paidCalc)
		case "partial_pending":
			where += fmt.Sprintf(" AND succeeded_refunded > 0 AND pending_refunded > 0 AND %s < %s", resCalc, paidCalc)
		case "full_pending":
			where += fmt.Sprintf(" AND %s >= %s AND succeeded_refunded < %s", resCalc, paidCalc, paidCalc)
		}
	}

	// Dynamic calculation for Problem Codes
	if hasProblem || probCode != "" {
		probConditions := []string{
			"(order_status = 'paid' AND succeeded_payments_count = 0)", // PAID_ORDER_WITHOUT_SUCCEEDED_PAYMENT
			"(status = 'succeeded' AND order_status != 'paid')", // SUCCEEDED_PAYMENT_ORDER_NOT_PAID
			"(succeeded_payments_count > 1 AND status = 'succeeded')", // MULTIPLE_SUCCEEDED_PAYMENTS
			"(status = 'succeeded' AND amount_cents != order_total_cents)", // AMOUNT_MISMATCH
			"is_stuck", // STUCK_PENDING
			"has_invalid_webhook", // INVALID_WEBHOOK_SIGNATURE
			"has_unprocessed_webhook", // UNPROCESSED_WEBHOOK
		}

		if probCode != "" {
			switch probCode {
			case "PAID_ORDER_WITHOUT_SUCCEEDED_PAYMENT": where += " AND " + probConditions[0]
			case "SUCCEEDED_PAYMENT_ORDER_NOT_PAID": where += " AND " + probConditions[1]
			case "MULTIPLE_SUCCEEDED_PAYMENTS": where += " AND " + probConditions[2]
			case "AMOUNT_MISMATCH": where += " AND " + probConditions[3]
			case "STUCK_PENDING": where += " AND " + probConditions[4]
			case "INVALID_WEBHOOK_SIGNATURE": where += " AND " + probConditions[5]
			case "UNPROCESSED_WEBHOOK": where += " AND " + probConditions[6]
			}
		} else if hasProblem {
			where += " AND (" + strings.Join(probConditions, " OR ") + ")"
		}
	}

	query := cte + where

	// Count Total
	countQuery := "WITH data AS (" + query + ") SELECT count(*) FROM data"
	var total int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Ordering
	orderClause := " ORDER BY "
	allowedSorts := map[string]string{
		"createdAt": "created_at",
		"updatedAt": "updated_at",
		"amount": "amount_cents",
		"paymentNumber": "payment_number",
		"status": "status",
	}
	dbSort, ok := allowedSorts[sort]
	if !ok {
		dbSort = "created_at"
	}
	dir := "DESC"
	if strings.ToLower(direction) == "asc" {
		dir = "ASC"
	}
	
	orderClause += fmt.Sprintf("%s %s, id %s", dbSort, dir, dir)

	query += orderClause + fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var payments []AdminPaymentDTO
	for rows.Next() {
		var p AdminPaymentDTO
		var oStatus string
		var oTotal int64
		var sCount int
		var hasInvalid, hasUnprocessed, isStuck bool

		p.Customer = &CustomerSummaryDTO{}

		var tCreatedAt, tUpdatedAt, tPaidAt, tFailedAt, tCancelledAt *time.Time

		err := rows.Scan(
			&p.PaymentID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentNumber, &p.PaymentMethod, &p.IntegrationMode, &tCreatedAt, &tUpdatedAt, &tPaidAt, &tFailedAt, &tCancelledAt,
			&p.OrderNumber, &oStatus, &oTotal,
			&p.Customer.ID, &p.Customer.Name, &p.Customer.Email, &p.Customer.Phone,
			&p.AttemptNumber, &p.AttemptsCount, &sCount,
			&p.SucceededRefundedAmountCents, &p.PendingRefundAmountCents,
			&hasInvalid, &hasUnprocessed, &isStuck,
		)
		if err != nil {
			return nil, 0, err
		}

		if tCreatedAt != nil { p.CreatedAt = tCreatedAt.Format(time.RFC3339) }
		if tUpdatedAt != nil { p.UpdatedAt = tUpdatedAt.Format(time.RFC3339) }
		if tPaidAt != nil { s := tPaidAt.Format(time.RFC3339); p.PaidAt = &s }
		if tFailedAt != nil { s := tFailedAt.Format(time.RFC3339); p.FailedAt = &s }
		if tCancelledAt != nil { s := tCancelledAt.Format(time.RFC3339); p.CancelledAt = &s }

		calculateRefundFields(&p)
		p.Problems = buildProblems(p, oStatus, oTotal, sCount, hasInvalid, hasUnprocessed, isStuck)

		payments = append(payments, p)
	}
	if payments == nil {
		payments = make([]AdminPaymentDTO, 0)
	}

	return payments, total, nil
}

func (r *Repository) GetAdminPaymentDetail(ctx context.Context, id uuid.UUID, stuckMins int) (*AdminPaymentDetailDTO, error) {
	// 1. Get base payment and order info, similar to List but for a single ID
	cte := `
WITH payment_attempts AS (
	SELECT id, 
	ROW_NUMBER() OVER (PARTITION BY order_id ORDER BY created_at ASC, id ASC) as attempt_number,
	COUNT(*) OVER (PARTITION BY order_id) as attempts_count,
	SUM(CASE WHEN status = 'succeeded' THEN 1 ELSE 0 END) OVER (PARTITION BY order_id) as succeeded_payments_count
	FROM payments
),
payment_refunds AS (
	SELECT payment_id,
	COALESCE(SUM(amount_cents) FILTER (WHERE status = 'succeeded'), 0) as succeeded_amount,
	COALESCE(SUM(amount_cents) FILTER (WHERE status = 'pending'), 0) as pending_amount
	FROM refunds
	GROUP BY payment_id
),
payment_events_summary AS (
	SELECT payment_id,
	BOOL_OR(signature_valid = false) as has_invalid_webhook,
	BOOL_OR(signature_valid = true AND processed_at IS NULL) as has_unprocessed_webhook
	FROM payment_events
	GROUP BY payment_id
),
base_data AS (
	SELECT 
		p.id, p.order_id, p.provider, p.provider_payment_id, p.status, p.amount_cents, p.currency, p.payment_number, p.payment_method, p.integration_mode, p.created_at, p.updated_at, p.paid_at, p.failed_at, p.cancelled_at,
		o.order_number, o.status as order_status, o.total_price_cents as order_total_cents, o.created_at as order_created_at,
		u.id as customer_id, u.name as customer_name, u.email as customer_email, u.phone as customer_phone,
		pa.attempt_number, pa.attempts_count, pa.succeeded_payments_count,
		COALESCE(pr.succeeded_amount, 0) as succeeded_refunded,
		COALESCE(pr.pending_amount, 0) as pending_refunded,
		COALESCE(pes.has_invalid_webhook, false) as has_invalid_webhook,
		COALESCE(pes.has_unprocessed_webhook, false) as has_unprocessed_webhook,
		(p.status IN ('created', 'pending') AND p.created_at < now() - interval '1 minute' * $2) as is_stuck
	FROM payments p
	JOIN orders o ON p.order_id = o.id
	JOIN users u ON o.user_id = u.id
	LEFT JOIN payment_attempts pa ON p.id = pa.id
	LEFT JOIN payment_refunds pr ON p.id = pr.payment_id
	LEFT JOIN payment_events_summary pes ON p.id = pes.payment_id
)
SELECT * FROM base_data WHERE id = $1
`
	var p AdminPaymentDTO
	var oStatus string
	var oTotal int64
	var sCount int
	var hasInvalid, hasUnprocessed, isStuck bool
	var tCreatedAt, tUpdatedAt, tPaidAt, tFailedAt, tCancelledAt, oCreatedAt *time.Time

	p.Customer = &CustomerSummaryDTO{}
	var order OrderSummaryDTO

	err := r.db.QueryRow(ctx, cte, id, stuckMins).Scan(
		&p.PaymentID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentNumber, &p.PaymentMethod, &p.IntegrationMode, &tCreatedAt, &tUpdatedAt, &tPaidAt, &tFailedAt, &tCancelledAt,
		&order.OrderNumber, &oStatus, &oTotal, &oCreatedAt,
		&p.Customer.ID, &p.Customer.Name, &p.Customer.Email, &p.Customer.Phone,
		&p.AttemptNumber, &p.AttemptsCount, &sCount,
		&p.SucceededRefundedAmountCents, &p.PendingRefundAmountCents,
		&hasInvalid, &hasUnprocessed, &isStuck,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}

	if tCreatedAt != nil { p.CreatedAt = tCreatedAt.Format(time.RFC3339) }
	if tUpdatedAt != nil { p.UpdatedAt = tUpdatedAt.Format(time.RFC3339) }
	if tPaidAt != nil { s := tPaidAt.Format(time.RFC3339); p.PaidAt = &s }
	if tFailedAt != nil { s := tFailedAt.Format(time.RFC3339); p.FailedAt = &s }
	if tCancelledAt != nil { s := tCancelledAt.Format(time.RFC3339); p.CancelledAt = &s }
	
	order.OrderID = p.OrderID
	order.OrderStatus = oStatus
	order.OrderTotalCents = oTotal
	order.Customer = p.Customer
	if oCreatedAt != nil { order.CreatedAt = oCreatedAt.Format(time.RFC3339) }

	calculateRefundFields(&p)
	problems := buildProblems(p, oStatus, oTotal, sCount, hasInvalid, hasUnprocessed, isStuck)
	p.Problems = problems // also assign to parent detail DTO below

	// 2. Fetch Attempts
	attemptsQuery := `
		SELECT id, payment_number, status, provider, payment_method, amount_cents, provider_payment_id, created_at, paid_at, failed_at, cancelled_at,
		ROW_NUMBER() OVER (ORDER BY created_at ASC, id ASC) as attempt_number
		FROM payments
		WHERE order_id = $1
		ORDER BY created_at ASC, id ASC
	`
	rows, err := r.db.Query(ctx, attemptsQuery, p.OrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []AdminPaymentAttemptDTO
	for rows.Next() {
		var a AdminPaymentAttemptDTO
		var cAt *time.Time
		var pAt, fAt, cancAt *time.Time
		err := rows.Scan(&a.PaymentID, &a.PaymentNumber, &a.Status, &a.Provider, &a.PaymentMethod, &a.AmountCents, &a.ProviderPaymentID, &cAt, &pAt, &fAt, &cancAt, &a.AttemptNumber)
		if err != nil {
			return nil, err
		}
		if cAt != nil { a.CreatedAt = cAt.Format(time.RFC3339) }
		if pAt != nil {
			s := pAt.Format(time.RFC3339)
			a.TerminalAt = &s
		} else if fAt != nil {
			s := fAt.Format(time.RFC3339)
			a.TerminalAt = &s
		} else if cancAt != nil {
			s := cancAt.Format(time.RFC3339)
			a.TerminalAt = &s
		}
		attempts = append(attempts, a)
	}
	if attempts == nil {
		attempts = make([]AdminPaymentAttemptDTO, 0)
	}

	// 3. Fetch Refunds
	refundsQuery := `
		SELECT id, status, amount_cents, provider_refund_id, created_at, processed_at
		FROM refunds
		WHERE payment_id = $1
		ORDER BY created_at DESC
	`
	rRows, err := r.db.Query(ctx, refundsQuery, p.PaymentID)
	if err != nil {
		return nil, err
	}
	defer rRows.Close()

	var refunds []AdminRefundDTO
	for rRows.Next() {
		var rf AdminRefundDTO
		var rfCAt *time.Time
		var rfPAt *time.Time
		err := rRows.Scan(&rf.RefundID, &rf.Status, &rf.AmountCents, &rf.ProviderRefundID, &rfCAt, &rfPAt)
		if err != nil {
			return nil, err
		}
		if rfCAt != nil { rf.CreatedAt = rfCAt.Format(time.RFC3339) }
		if rfPAt != nil { 
			s := rfPAt.Format(time.RFC3339)
			rf.ProcessedAt = &s
		}
		refunds = append(refunds, rf)
	}
	if refunds == nil {
		refunds = make([]AdminRefundDTO, 0)
	}

	// 4. Fetch Events (Safe payload only!)
	eventsQuery := `
		SELECT id, event_type, signature_valid, processed_at, created_at, event_key, raw_payload
		FROM payment_events
		WHERE payment_id = $1
		ORDER BY created_at DESC
	`
	eRows, err := r.db.Query(ctx, eventsQuery, p.PaymentID)
	if err != nil {
		return nil, err
	}
	defer eRows.Close()

	var events []SafePaymentEventDTO
	for eRows.Next() {
		var e SafePaymentEventDTO
		var raw map[string]any
		var eCAt, ePAt *time.Time
		err := eRows.Scan(&e.EventID, &e.EventType, &e.SignatureValid, &ePAt, &eCAt, &e.EventKey, &raw)
		if err != nil {
			return nil, err
		}
		if eCAt != nil { e.CreatedAt = eCAt.Format(time.RFC3339) }
		if ePAt != nil {
			s := ePAt.Format(time.RFC3339)
			e.ProcessedAt = &s
		}
		
		// Build Safe Payload Summary
		safe := make(map[string]any)
		if val, ok := raw["PaymentId"]; ok { safe["PaymentId"] = val }
		if val, ok := raw["OrderId"]; ok { safe["OrderId"] = val }
		if val, ok := raw["Status"]; ok { safe["Status"] = val }
		if val, ok := raw["Success"]; ok { safe["Success"] = val }
		if val, ok := raw["ErrorCode"]; ok { safe["ErrorCode"] = val }
		if val, ok := raw["Amount"]; ok { safe["Amount"] = val }
		e.SafePayloadSummary = safe
		
		events = append(events, e)
	}
	if events == nil {
		events = make([]SafePaymentEventDTO, 0)
	}

	return &AdminPaymentDetailDTO{
		Payment:        p,
		Order:          order,
		Attempts:       attempts,
		ProviderEvents: events,
		Refunds:        refunds,
		Problems:       problems,
	}, nil
}
