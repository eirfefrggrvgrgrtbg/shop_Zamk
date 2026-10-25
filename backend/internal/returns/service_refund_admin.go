package returns

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
)

// BANK.REFUND.2: the admin view of bank refund operations. It reads refunds and derives, for each, the stage an
// operator sees, the actions the current state allows and why others are blocked. It holds no state of its own:
// actions go through SubmitRefund / ReconcileRefund. There is deliberately no action that settles a refund by hand.

// Bank refund stages shown to operators.
const (
	BankStagePrepared       = "prepared"        // reserved, not sent
	BankStageSending        = "sending"         // the submit is recorded, the call may be in flight
	BankStageAccepted       = "accepted"        // the bank took the request; no confirmed result
	BankStageUnknown        = "unknown"         // the submit ended without a usable answer
	BankStageReview         = "review"          // a bank answer could not be attributed: manual reconciliation
	BankStageConfirmed      = "confirmed"       // the bank confirmed this attempt
	BankStageRejected       = "rejected"        // the bank definitively refused this attempt
	BankStageLegacy         = "legacy"          // created before bank refunds: history
	BankStageSettledLocally = "settled_locally" // completed without a bank (dev simulator)
	BankStageFailedLocally  = "failed_locally"  // failed without a bank (dev simulator)
)

// Actions an operator may take on a bank refund.
const (
	BankActionSubmit    = "submit"
	BankActionReconcile = "reconcile"
	BankActionNote      = "note"
)

// bankStageSQL derives the stage from the stored row; it is the single definition used for listing and filtering.
const bankStageSQL = `CASE
	WHEN r.submit_outcome = 'legacy' THEN 'legacy'
	WHEN r.review_reason IS NOT NULL THEN 'review'
	WHEN r.submit_outcome = 'not_sent' AND r.status IN ('pending', 'processing') THEN 'prepared'
	WHEN r.submit_outcome IN ('sending', 'accepted', 'unknown', 'confirmed', 'rejected') THEN r.submit_outcome
	WHEN r.submit_outcome = 'not_sent' AND r.status IN ('succeeded', 'completed') THEN 'settled_locally'
	ELSE 'failed_locally' END`

var bankStages = map[string]bool{
	BankStagePrepared: true, BankStageSending: true, BankStageAccepted: true, BankStageUnknown: true,
	BankStageReview: true, BankStageConfirmed: true, BankStageRejected: true, BankStageLegacy: true,
	BankStageSettledLocally: true, BankStageFailedLocally: true,
}

// ErrInvalidBankStage: an unknown stage filter.
var ErrInvalidBankStage = errors.New("invalid bank refund stage")

// AdminBankRefund is one refund as an operator sees it. It carries no idempotency key, token, password, card data or
// raw provider payload.
type AdminBankRefund struct {
	ID                 uuid.UUID  `json:"id"`
	ReturnID           *uuid.UUID `json:"returnId"`
	OrderID            uuid.UUID  `json:"orderId"`
	OrderNumber        *string    `json:"orderNumber"`
	AmountCents        int64      `json:"amountCents"`
	Currency           string     `json:"currency"`
	Status             string     `json:"status"`
	SubmitOutcome      string     `json:"submitOutcome"`
	Stage              string     `json:"stage"`
	Provider           *string    `json:"provider"`
	ProviderStatus     *string    `json:"providerStatus"`
	FailureCode        *string    `json:"failureCode"`
	ReviewReason       *string    `json:"reviewReason"`
	// ReviewExplanation is the operator-facing (Russian) meaning of ReviewReason.
	ReviewExplanation *string `json:"reviewExplanation"`
	SubmitAttempts     int        `json:"submitAttempts"`
	PaymentAmountCents *int64     `json:"paymentAmountCents"`
	PaymentMethod      *string    `json:"paymentMethod"`
	IsPartial          bool       `json:"isPartial"`
	RetryOfRefundID    *uuid.UUID `json:"retryOfRefundId"`
	CreatedAt          time.Time  `json:"createdAt"`
	SubmittedAt        *time.Time `json:"submittedAt"`
	LastCheckedAt      *time.Time `json:"lastCheckedAt"`
	ProcessedAt        *time.Time `json:"processedAt"`
	FailedAt           *time.Time `json:"failedAt"`
	BlockingReason     *string    `json:"blockingReason"`
	AllowedActions     []string   `json:"allowedActions"`

	// for the derivation only
	paymentStatus     *string
	providerPaymentID *string
	integrationMode   *string
	idempotencyKey    *string
}

// AdminBankRefundEvent is one operator action on a refund, from the staff audit log.
type AdminBankRefundEvent struct {
	Action     string    `json:"action"`
	ActorEmail *string   `json:"actorEmail"`
	Comment    *string   `json:"comment"`
	Escalated  bool      `json:"escalated"`
	Outcome    *string   `json:"outcome"`
	CreatedAt  time.Time `json:"createdAt"`
}

type AdminBankRefundDetail struct {
	AdminBankRefund
	ProviderConfigured bool                   `json:"providerConfigured"`
	ProviderName       *string                `json:"providerName"`
	History            []AdminBankRefundEvent `json:"history"`
}

type AdminBankRefundList struct {
	Items              []AdminBankRefund `json:"items"`
	TotalCount         int               `json:"totalCount"`
	StageCounts        map[string]int    `json:"stageCounts"`
	ProviderConfigured bool              `json:"providerConfigured"`
}

const adminBankRefundSelect = `
	SELECT r.id, r.return_id, r.order_id, o.order_number, r.amount_cents, r.currency, r.status, r.submit_outcome,
	       ` + bankStageSQL + ` AS stage,
	       r.provider, r.provider_status, r.failure_code, r.review_reason, r.submit_attempts,
	       p.amount_cents, p.payment_method, r.retry_of_refund_id, r.created_at, r.submitted_at, r.last_checked_at,
	       r.processed_at, r.failed_at, p.status, p.provider_payment_id, p.integration_mode, r.idempotency_key
	FROM refunds r
	JOIN orders o ON o.id = r.order_id
	LEFT JOIN payments p ON p.id = r.payment_id`

func scanAdminBankRefund(row pgx.Row) (*AdminBankRefund, error) {
	var a AdminBankRefund
	err := row.Scan(&a.ID, &a.ReturnID, &a.OrderID, &a.OrderNumber, &a.AmountCents, &a.Currency, &a.Status, &a.SubmitOutcome,
		&a.Stage, &a.Provider, &a.ProviderStatus, &a.FailureCode, &a.ReviewReason, &a.SubmitAttempts,
		&a.PaymentAmountCents, &a.PaymentMethod, &a.RetryOfRefundID, &a.CreatedAt, &a.SubmittedAt, &a.LastCheckedAt,
		&a.ProcessedAt, &a.FailedAt, &a.paymentStatus, &a.providerPaymentID, &a.integrationMode, &a.idempotencyKey)
	if err != nil {
		return nil, err
	}
	a.IsPartial = a.PaymentAmountCents != nil && a.AmountCents < *a.PaymentAmountCents
	if a.ReviewReason != nil {
		e := reviewReasonExplanation(*a.ReviewReason)
		a.ReviewExplanation = &e
	}
	return &a, nil
}

// ListAdminBankRefunds lists refunds newest first, optionally of one stage, with the count of every stage.
func (s *Service) ListAdminBankRefunds(ctx context.Context, stage string, limit, offset int) (*AdminBankRefundList, error) {
	if stage != "" && !bankStages[stage] {
		return nil, ErrInvalidBankStage
	}
	rows, err := s.db.Pool.Query(ctx, adminBankRefundSelect+`
		WHERE ($1 = '' OR `+bankStageSQL+` = $1)
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2 OFFSET $3`, stage, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := &AdminBankRefundList{Items: []AdminBankRefund{}, StageCounts: map[string]int{}, ProviderConfigured: s.refundProvider != nil}
	for rows.Next() {
		a, err := scanAdminBankRefund(rows)
		if err != nil {
			return nil, err
		}
		s.deriveBankRefundActions(a)
		list.Items = append(list.Items, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	counts, err := s.db.Pool.Query(ctx, `SELECT `+bankStageSQL+` AS stage, count(*) FROM refunds r GROUP BY 1`)
	if err != nil {
		return nil, err
	}
	defer counts.Close()
	for counts.Next() {
		var st string
		var n int
		if err := counts.Scan(&st, &n); err != nil {
			return nil, err
		}
		list.StageCounts[st] = n
		if stage == "" || stage == st {
			list.TotalCount += n
		}
	}
	return list, counts.Err()
}

// GetAdminBankRefund returns one refund with its allowed actions and the operator history.
func (s *Service) GetAdminBankRefund(ctx context.Context, refundID uuid.UUID) (*AdminBankRefundDetail, error) {
	a, err := scanAdminBankRefund(s.db.Pool.QueryRow(ctx, adminBankRefundSelect+` WHERE r.id = $1`, refundID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefundNotFound
	}
	if err != nil {
		return nil, err
	}
	s.deriveBankRefundActions(a)
	d := &AdminBankRefundDetail{AdminBankRefund: *a, ProviderConfigured: s.refundProvider != nil, History: []AdminBankRefundEvent{}}
	if s.refundProvider != nil {
		name := s.refundProvider.Name()
		d.ProviderName = &name
	}

	rows, err := s.db.Pool.Query(ctx, `
		SELECT l.action, COALESCE(l.actor_email, u.email), l.metadata->>'comment',
		       COALESCE((l.metadata->>'escalated')::boolean, false), l.metadata->>'outcome', l.created_at
		FROM audit_logs l
		LEFT JOIN users u ON u.id = l.actor_user_id
		WHERE l.entity_type = 'refund' AND l.entity_id = $1 AND l.action LIKE 'refund.bank_%'
		ORDER BY l.created_at DESC
		LIMIT 50`, refundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e AdminBankRefundEvent
		if err := rows.Scan(&e.Action, &e.ActorEmail, &e.Comment, &e.Escalated, &e.Outcome, &e.CreatedAt); err != nil {
			return nil, err
		}
		d.History = append(d.History, e)
	}
	return d, rows.Err()
}

func (s *Service) deriveBankRefundActions(a *AdminBankRefund) {
	actions := []string{BankActionNote}
	var reason string
	switch a.Stage {
	case BankStagePrepared:
		switch {
		case a.Status != "pending":
			reason = "Статус возврата не позволяет отправку в банк."
		case s.refundProvider == nil:
			reason = "Отправка в банк выключена: провайдер возвратов не подключён. Возврат остаётся в резерве и не отправляется."
		case a.paymentStatus == nil || *a.paymentStatus != "succeeded" || a.providerPaymentID == nil || *a.providerPaymentID == "":
			reason = "Нет успешной оплаты с банковским идентификатором платежа: отправлять нечего."
		default:
			err := s.refundProvider.Preflight(payments.RefundSubmitInput{
				ProviderPaymentID:  *a.providerPaymentID,
				AmountCents:        a.AmountCents,
				Currency:           a.Currency,
				IdempotencyKey:     derefString(a.idempotencyKey) + keyPlaceholder(a.idempotencyKey),
				PaymentAmountCents: derefInt64(a.PaymentAmountCents),
				PaymentMethod:      derefString(a.PaymentMethod),
				IntegrationMode:    derefString(a.integrationMode),
			})
			if err != nil {
				reason = notSendableExplanation(err)
			} else {
				actions = append(actions, BankActionSubmit)
			}
		}
	case BankStageSending, BankStageAccepted, BankStageUnknown:
		switch a.Stage {
		case BankStageSending:
			reason = "Отправка начата: ответ банка ещё не записан. Повторная отправка запрещена, сумма в резерве."
		case BankStageAccepted:
			reason = "Банк принял запрос, но не подтвердил именно этот возврат. Сумма остаётся в резерве, удержание с продавца не выполнено."
		default:
			reason = "Исход отправки неизвестен (таймаут или нет ответа банка). Повторная отправка запрещена: банк мог выполнить возврат. Сумма остаётся в резерве до сверки."
		}
		if s.refundProvider == nil {
			reason += " Сверка недоступна: провайдер возвратов не подключён."
		} else {
			actions = append(actions, BankActionReconcile)
		}
	case BankStageReview:
		reason = "Требуется ручная сверка: ответ банка нельзя отнести к этой попытке. Автоматическое завершение и повторная отправка запрещены; сумма в резерве."
	case BankStageLegacy:
		reason = "Историческая запись до банковского контура. Её нужно проверить по истории операций банка; автоматическая отправка или повторный возврат денег запрещены."
	case BankStageRejected:
		reason = "Банк окончательно отказал в этой попытке. Новую попытку можно создать из карточки возврата, если причина устранена."
	}
	a.AllowedActions = actions
	if reason != "" {
		a.BlockingReason = &reason
	}
}

// keyPlaceholder lets Preflight judge a reservation made before keys existed (its key is assigned at submit).
func keyPlaceholder(key *string) string {
	if key == nil || *key == "" {
		return "assigned-at-submit"
	}
	return ""
}

func notSendableExplanation(err error) string {
	var ns *payments.RefundNotSendableError
	if !errors.As(err, &ns) {
		return "Возврат нельзя отправить в банк: проверка перед отправкой не пройдена."
	}
	switch ns.Code {
	case payments.NotSendableReceiptRequired:
		return "Частичный возврат нельзя отправить: при подключённой онлайн-кассе банку нужен чек (Receipt) на возвращаемые позиции, а фискальных данных чека в ZAMK нет. Подставлять условный чек запрещено."
	case payments.NotSendableMockPayment:
		return "Заказ оплачен через локальный эмулятор: в банке нет платежа, который можно вернуть."
	case payments.NotSendableSBP:
		return "Возврат по СБП не поддерживается: статус таких возвратов отслеживается отдельным методом GetQrState."
	case payments.NotSendableUnknownPaymentAmount, payments.NotSendableAmountOutOfRange:
		return "Сумма возврата не согласуется с суммой исходной оплаты."
	case payments.NotSendableNoPaymentID:
		return "У оплаты нет банковского идентификатора платежа."
	case payments.NotSendableCurrency:
		return "Возвращаются только платежи в рублях."
	default:
		return fmt.Sprintf("Возврат нельзя отправить в банк (%s).", ns.Code)
	}
}

// reviewReasonExplanation tells an operator, in Russian, why an attempt is on manual review.
func reviewReasonExplanation(reason string) string {
	switch {
	case strings.HasPrefix(reason, "unattributed provider state"):
		return "Банк сообщает только общий статус исходного платежа (например, PARTIAL_REFUNDED или REFUNDED). По нему нельзя доказать, что прошёл именно этот возврат: сверьте операцию по выписке или ответу банка."
	case strings.HasPrefix(reason, "provider answer is not attributed"):
		return "Ответ банка относится к другой операции возврата, не к этой попытке."
	case strings.HasPrefix(reason, "provider answer names another payment"):
		return "Банк ответил по другому платежу."
	case strings.HasPrefix(reason, "provider amount"):
		return "Сумма в ответе банка не совпадает с суммой этого возврата."
	default:
		return "Ответ банка нельзя однозначно отнести к этой попытке возврата."
	}
}
