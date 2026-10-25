package returns

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
)

// BANK.REFUND.2 admin endpoints for bank refund operations (routes under /api/admin/refunds):
//
//	GET  /bank                    list (refunds.read), ?stage= filter
//	GET  /{id}/bank               detail with allowed actions and operator history (refunds.read)
//	POST /{id}/bank/submit        hand a prepared refund to the provider once (refunds.create)
//	POST /{id}/bank/reconcile     ask the provider about an unresolved attempt; never sends (refunds.create)
//	POST /{id}/bank/notes         record an investigation note or escalation in the staff audit (refunds.create)
//
// No endpoint marks a refund succeeded, a return refunded, or writes a seller deduction by hand.

const maxRefundNoteLength = 2000

func (h *Handler) ListAdminBankRefunds(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	list, err := h.service.ListAdminBankRefunds(r.Context(), strings.TrimSpace(r.URL.Query().Get("stage")), page.Limit, page.Offset)
	if errors.Is(err, ErrInvalidBankStage) {
		h.writeError(w, http.StatusBadRequest, "invalid_stage", "Unknown bank refund stage")
		return
	}
	if err != nil {
		log.Printf("ListAdminBankRefunds failed: %v", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list bank refunds")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) GetAdminBankRefund(w http.ResponseWriter, r *http.Request) {
	refundID, ok := h.refundIDParam(w, r)
	if !ok {
		return
	}
	d, err := h.service.GetAdminBankRefund(r.Context(), refundID)
	if !h.writeBankRefundLookupError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) SubmitAdminBankRefund(w http.ResponseWriter, r *http.Request) {
	adminID := auth.GetUserID(r.Context())
	refundID, ok := h.refundIDParam(w, r)
	if !ok {
		return
	}
	before, err := h.service.GetAdminBankRefund(r.Context(), refundID)
	if !h.writeBankRefundLookupError(w, err) {
		return
	}
	if !hasAction(before.AllowedActions, BankActionSubmit) {
		code := "refund_not_submittable"
		switch before.Stage {
		case BankStageSending, BankStageAccepted, BankStageUnknown, BankStageConfirmed, BankStageRejected:
			code = "already_submitted"
		}
		h.writeError(w, http.StatusConflict, code, blockingMessage(before.BlockingReason, "Refund cannot be sent in its current state"))
		return
	}

	_, err = h.service.SubmitRefund(r.Context(), refundID)
	if err != nil {
		var status int
		var code, msg string
		switch {
		case errors.Is(err, ErrRefundProviderNotConfigured):
			status, code, msg = http.StatusServiceUnavailable, "refund_provider_disabled", "Refund provider is not configured"
		case errors.Is(err, payments.ErrRefundNotSendable):
			status, code, msg = http.StatusUnprocessableEntity, "refund_not_sendable", notSendableExplanation(err)
		case errors.Is(err, ErrRefundPaymentNotLinked):
			status, code, msg = http.StatusUnprocessableEntity, "payment_not_linked", "No succeeded payment with a bank payment id"
		case errors.Is(err, ErrRefundLegacyRecord):
			status, code, msg = http.StatusConflict, "legacy_refund", "A legacy refund is never sent to the bank"
		case errors.Is(err, ErrRefundUnderReview):
			status, code, msg = http.StatusConflict, "refund_under_review", "Refund is under manual review"
		case errors.Is(err, ErrRefundNotFound):
			status, code, msg = http.StatusNotFound, "not_found", "Refund not found"
		default:
			log.Printf("SubmitRefund failed for %s: %v", refundID, err)
			status, code, msg = http.StatusInternalServerError, "internal_error", "Failed to submit refund"
		}
		h.recordBankRefundAudit(r, adminID, refundID, "refund.bank_submit", map[string]any{"outcome": "refused:" + code})
		h.writeError(w, status, code, msg)
		return
	}

	after, err := h.service.GetAdminBankRefund(r.Context(), refundID)
	if !h.writeBankRefundLookupError(w, err) {
		return
	}
	h.recordBankRefundAudit(r, adminID, refundID, "refund.bank_submit", map[string]any{"outcome": after.Stage})
	writeJSON(w, http.StatusOK, after)
}

func (h *Handler) ReconcileAdminBankRefund(w http.ResponseWriter, r *http.Request) {
	adminID := auth.GetUserID(r.Context())
	refundID, ok := h.refundIDParam(w, r)
	if !ok {
		return
	}
	before, err := h.service.GetAdminBankRefund(r.Context(), refundID)
	if !h.writeBankRefundLookupError(w, err) {
		return
	}
	if !hasAction(before.AllowedActions, BankActionReconcile) {
		h.writeError(w, http.StatusConflict, "refund_not_reconcilable", blockingMessage(before.BlockingReason, "Refund has no unresolved bank attempt"))
		return
	}

	_, err = h.service.ReconcileRefund(r.Context(), refundID)
	if err != nil {
		switch {
		case errors.Is(err, ErrRefundUnderReview):
			h.writeError(w, http.StatusConflict, "refund_under_review", "Refund is under manual review")
		case errors.Is(err, ErrRefundProviderNotConfigured):
			h.writeError(w, http.StatusServiceUnavailable, "refund_provider_disabled", "Refund provider is not configured")
		default:
			// The provider did not answer usably: nothing was changed.
			log.Printf("ReconcileRefund failed for %s: %v", refundID, err)
			h.recordBankRefundAudit(r, adminID, refundID, "refund.bank_reconcile", map[string]any{"outcome": "provider_unavailable"})
			h.writeError(w, http.StatusBadGateway, "provider_unavailable", "The bank did not answer; the refund was left unchanged")
		}
		return
	}

	after, err := h.service.GetAdminBankRefund(r.Context(), refundID)
	if !h.writeBankRefundLookupError(w, err) {
		return
	}
	h.recordBankRefundAudit(r, adminID, refundID, "refund.bank_reconcile", map[string]any{"outcome": after.Stage})
	writeJSON(w, http.StatusOK, after)
}

type bankRefundNoteRequest struct {
	Comment  string `json:"comment"`
	Escalate bool   `json:"escalate"`
}

// AddAdminBankRefundNote records an investigation note (or an escalation) in the staff audit. It changes nothing
// about the refund itself.
func (h *Handler) AddAdminBankRefundNote(w http.ResponseWriter, r *http.Request) {
	adminID := auth.GetUserID(r.Context())
	refundID, ok := h.refundIDParam(w, r)
	if !ok {
		return
	}
	var req bankRefundNoteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	req.Comment = strings.TrimSpace(req.Comment)
	if req.Comment == "" || utf8.RuneCountInString(req.Comment) > maxRefundNoteLength {
		h.writeError(w, http.StatusBadRequest, "validation_error", "Comment must be 1-2000 characters")
		return
	}
	before, err := h.service.GetAdminBankRefund(r.Context(), refundID)
	if !h.writeBankRefundLookupError(w, err) {
		return
	}
	if h.auditRepo == nil {
		h.writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "Audit log is not available")
		return
	}
	action := "refund.bank_note"
	if req.Escalate {
		action = "refund.bank_escalated"
	}
	if err := h.auditRepo.RecordAudit(r.Context(), staff.AuditEvent{
		ActorUserID: adminID,
		Permission:  "refunds.create",
		Action:      action,
		EntityType:  "refund",
		EntityID:    &refundID,
		Metadata:    map[string]any{"comment": req.Comment, "escalated": req.Escalate, "outcome": before.Stage},
	}); err != nil {
		log.Printf("refund note audit failed for %s: %v", refundID, err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to record the note")
		return
	}
	after, err := h.service.GetAdminBankRefund(r.Context(), refundID)
	if !h.writeBankRefundLookupError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, after)
}

type fakeRefundScriptRequest struct {
	NextSubmit string `json:"nextSubmit"`
	RefundID   string `json:"refundId"`
	Answer     string `json:"answer"`
}

// ScriptDevFakeRefundProvider scripts the in-memory fake refund provider of a local stand. It exists only while
// the fake is attached (ZAMK_DEV_FAKE_REFUND_PROVIDER, never in production) and answers 404 otherwise.
func (h *Handler) ScriptDevFakeRefundProvider(w http.ResponseWriter, r *http.Request) {
	fake, ok := h.service.refundProvider.(*payments.FakeRefundProvider)
	if !ok || h.appEnv == "production" {
		h.writeError(w, http.StatusNotFound, "not_found", "Not found")
		return
	}
	var req fakeRefundScriptRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if req.NextSubmit != "" {
		b, ok := map[string]payments.FakeSubmitBehavior{
			"accept": payments.FakeSubmitAccept, "reject": payments.FakeSubmitReject,
			"timeout": payments.FakeSubmitTimeout, "lost": payments.FakeSubmitLostResponse,
		}[req.NextSubmit]
		if !ok {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "nextSubmit must be accept, reject, timeout or lost")
			return
		}
		fake.QueueSubmit(b)
		writeJSON(w, http.StatusOK, map[string]string{"queued": req.NextSubmit})
		return
	}
	switch req.Answer {
	case "unavailable":
		fake.FailStateQueries(errors.New("fake provider: bank unavailable"))
		writeJSON(w, http.StatusOK, map[string]string{"answer": req.Answer})
		return
	case "available":
		fake.FailStateQueries(nil)
		writeJSON(w, http.StatusOK, map[string]string{"answer": req.Answer})
		return
	}
	refundID, err := uuid.Parse(req.RefundID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "refundId required")
		return
	}
	key, err := h.service.refundIdempotencyKey(r.Context(), refundID)
	if err != nil || key == "" {
		h.writeError(w, http.StatusNotFound, "not_found", "Refund has no bank attempt")
		return
	}
	switch req.Answer {
	case "confirm":
		err = fake.Confirm(key)
	case "fail":
		err = fake.Fail(key, "FAKE_DECLINED")
	case "contradict":
		fake.OverrideState(key, payments.RefundStateResult{Outcome: payments.RefundStateSucceeded, IdempotencyKey: "another-attempt"})
	default:
		h.writeError(w, http.StatusBadRequest, "invalid_request", "answer must be confirm, fail, contradict, unavailable or available")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "The fake provider holds no operation for this refund")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"answer": req.Answer})
}

func (s *Service) refundIdempotencyKey(ctx context.Context, refundID uuid.UUID) (string, error) {
	var key *string
	if err := s.db.Pool.QueryRow(ctx, `SELECT idempotency_key FROM refunds WHERE id = $1`, refundID).Scan(&key); err != nil {
		return "", err
	}
	return derefString(key), nil
}

func (h *Handler) refundIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid refund ID")
		return uuid.Nil, false
	}
	return id, true
}

// writeBankRefundLookupError answers a failed lookup and reports whether the handler may continue.
func (h *Handler) writeBankRefundLookupError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, ErrRefundNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", "Refund not found")
	default:
		log.Printf("bank refund lookup failed: %v", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load refund")
	}
	return false
}

func (h *Handler) recordBankRefundAudit(r *http.Request, adminID, refundID uuid.UUID, action string, meta map[string]any) {
	if h.auditRepo == nil {
		return
	}
	if err := h.auditRepo.RecordAudit(r.Context(), staff.AuditEvent{
		ActorUserID: adminID,
		Permission:  "refunds.create",
		Action:      action,
		EntityType:  "refund",
		EntityID:    &refundID,
		Metadata:    meta,
	}); err != nil {
		log.Printf("refund audit %s failed for %s: %v", action, refundID, err)
	}
}

func hasAction(actions []string, a string) bool {
	for _, x := range actions {
		if x == a {
			return true
		}
	}
	return false
}

func blockingMessage(reason *string, fallback string) string {
	if reason != nil && *reason != "" {
		return *reason
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
