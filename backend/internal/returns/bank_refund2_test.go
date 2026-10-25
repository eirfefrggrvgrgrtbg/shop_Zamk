package returns_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// BANK.REFUND.2: the admin bank refund workflow through the real router, RBAC middleware and the dev fake refund
// provider (ZAMK_DEV_FAKE_REFUND_PROVIDER). No request leaves the process.

type adminBankStand struct {
	fix      *m51Fixture
	router   http.Handler
	tokens   *auth.TokenService
	viewer   uuid.UUID
	operator uuid.UUID
	noPerm   uuid.UUID
}

func newAdminBankStand(t *testing.T) *adminBankStand {
	t.Helper()
	fix := setupM51Fixture(t)
	ctx := context.Background()
	cfg := &config.Config{}
	cfg.JWT.AccessTokenSecret = "test-secret"
	cfg.JWT.AccessTokenTTLMinutes = 60
	cfg.RateLimit.Enabled = false
	cfg.Worker.ReturnWindowDays = 14
	cfg.TBank.RefundFakeProvider = true
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	router, cancel := app.BuildRouter(ctx, cfg, fix.client, &redis.Client{Client: goredis.NewClient(&goredis.Options{})}, logger)
	t.Cleanup(cancel)

	s := &adminBankStand{fix: fix, router: router,
		tokens: auth.NewTokenService(cfg.JWT.AccessTokenSecret, cfg.JWT.RefreshTokenSecret, cfg.JWT.AccessTokenTTLMinutes),
		viewer: uuid.New(), operator: uuid.New(), noPerm: uuid.New()}
	for user, perms := range map[uuid.UUID][]string{
		s.viewer:   {"refunds.read"},
		s.operator: {"refunds.read", "refunds.create"},
		s.noPerm:   {"returns.read"},
	} {
		_, err := fix.client.Pool.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role) VALUES ($1, 'Staff', $2, 'hash', 'admin')`,
			user, user.String()+"@staff.test")
		require.NoError(t, err)
		roleID := uuid.New()
		_, err = fix.client.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name, description, is_system) VALUES ($1, $2, 'R', '', false)`,
			roleID, "bank_refund_"+roleID.String()[:8])
		require.NoError(t, err)
		_, err = fix.client.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, user, roleID)
		require.NoError(t, err)
		require.NoError(t, testutil.GrantStaffAuthorizationState(ctx, fix.client.Pool, user, roleID, perms...))
	}
	return s
}

func (s *adminBankStand) call(t *testing.T, user uuid.UUID, method, path string, body any) (int, map[string]any, string) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, "/api/admin"+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if user != uuid.Nil {
		token, err := s.tokens.GenerateAccessToken(user, user.String()+"@staff.test", "admin")
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out, rr.Body.String()
}

func (s *adminBankStand) prepared(t *testing.T, accepted int) (bankRefundOrder, uuid.UUID, uuid.UUID) {
	t.Helper()
	o := newBankRefundOrder(t, s.fix, 3)
	retID := newReceivedReturn(t, s.fix, o, accepted)
	ref, err := s.fix.svc.CreateRefund(context.Background(), s.fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	return o, retID, ref.ID
}

func actions(m map[string]any) []string {
	var out []string
	for _, a := range m["allowedActions"].([]any) {
		out = append(out, a.(string))
	}
	return out
}

func TestBankRefund2_RBAC(t *testing.T) {
	s := newAdminBankStand(t)
	_, _, refundID := s.prepared(t, 3)
	id := refundID.String()

	for _, path := range []string{"/refunds/bank", "/refunds/" + id + "/bank"} {
		code, _, _ := s.call(t, uuid.Nil, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusUnauthorized, code, path)
		code, _, _ = s.call(t, s.noPerm, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusForbidden, code, path)
		code, _, _ = s.call(t, s.viewer, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusOK, code, path)
	}
	for _, path := range []string{"/refunds/" + id + "/bank/submit", "/refunds/" + id + "/bank/reconcile", "/refunds/" + id + "/bank/notes", "/dev/refund-provider"} {
		code, _, _ := s.call(t, s.viewer, http.MethodPost, path, map[string]any{"comment": "x"})
		assert.Equal(t, http.StatusForbidden, code, "viewing does not allow acting: "+path)
		code, _, _ = s.call(t, s.noPerm, http.MethodPost, path, map[string]any{"comment": "x"})
		assert.Equal(t, http.StatusForbidden, code, path)
	}
	assert.Equal(t, 0, readRefundRow(t, s.fix, refundID).Attempts, "a refused caller sent nothing")
}

// List, detail, submit through the fake, repeat refused, accepted without confirmation, reconcile, notes.
func TestBankRefund2_SubmitAcceptedAndNotes(t *testing.T) {
	s := newAdminBankStand(t)
	o, retID, refundID := s.prepared(t, 3)
	id := refundID.String()

	code, d, body := s.call(t, s.operator, http.MethodGet, "/refunds/"+id+"/bank", nil)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "prepared", d["stage"])
	assert.Equal(t, true, d["providerConfigured"])
	assert.Contains(t, actions(d), "submit")
	key := *readRefundRow(t, s.fix, refundID).Key
	assert.NotContains(t, body, key, "the idempotency key is not exposed")

	code, d, _ = s.call(t, s.operator, http.MethodPost, "/refunds/"+id+"/bank/submit", nil)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "accepted", d["stage"])
	assert.NotContains(t, actions(d), "submit")
	assert.Contains(t, actions(d), "reconcile")
	assert.Contains(t, d["blockingReason"], "не подтвердил")

	code, errBody, _ := s.call(t, s.operator, http.MethodPost, "/refunds/"+id+"/bank/submit", nil)
	assert.Equal(t, http.StatusConflict, code, "a sent refund is never sent again")
	assert.Equal(t, "already_submitted", errBody["error"].(map[string]any)["code"])
	assert.Equal(t, 1, readRefundRow(t, s.fix, refundID).Attempts)

	code, d, _ = s.call(t, s.operator, http.MethodPost, "/refunds/"+id+"/bank/reconcile", nil)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "accepted", d["stage"], "accepted without confirmation stays accepted")
	assertNoFinancialSuccess(t, s.fix, o, refundID, retID)

	code, _, _ = s.call(t, s.operator, http.MethodPost, "/refunds/"+id+"/bank/notes", map[string]any{"comment": ""})
	assert.Equal(t, http.StatusBadRequest, code)
	code, d, _ = s.call(t, s.operator, http.MethodPost, "/refunds/"+id+"/bank/notes", map[string]any{"comment": "Запросили выписку у банка", "escalate": true})
	require.Equal(t, http.StatusCreated, code)
	history := d["history"].([]any)
	var sawNote, sawSubmit bool
	for _, h := range history {
		e := h.(map[string]any)
		if e["action"] == "refund.bank_escalated" && e["comment"] == "Запросили выписку у банка" && e["escalated"] == true {
			sawNote = true
		}
		if e["action"] == "refund.bank_submit" && e["outcome"] == "accepted" {
			sawSubmit = true
		}
	}
	assert.True(t, sawNote, "the escalation is in the audit history")
	assert.True(t, sawSubmit, "the submit is audited with its outcome")

	code, list, _ := s.call(t, s.viewer, http.MethodGet, "/refunds/bank?stage=accepted", nil)
	require.Equal(t, http.StatusOK, code)
	found := false
	for _, it := range list["items"].([]any) {
		item := it.(map[string]any)
		assert.Equal(t, "accepted", item["stage"], "the stage filter returns only that stage")
		if item["id"] == id {
			found = true
		}
	}
	assert.True(t, found)
	code, _, _ = s.call(t, s.viewer, http.MethodGet, "/refunds/bank?stage=bogus", nil)
	assert.Equal(t, http.StatusBadRequest, code)
}

// Timeout -> unknown: no resend offered or accepted; bank unavailable leaves it unchanged; a contradictory answer
// sends it to review, which stays in the queue and cannot be reconciled or settled automatically.
func TestBankRefund2_UnknownUnavailableAndReview(t *testing.T) {
	s := newAdminBankStand(t)
	o, retID, refundID := s.prepared(t, 3)
	id := refundID.String()

	code, _, _ := s.call(t, s.operator, http.MethodPost, "/dev/refund-provider", map[string]any{"nextSubmit": "lost"})
	require.Equal(t, http.StatusOK, code)
	code, d, _ := s.call(t, s.operator, http.MethodPost, "/refunds/"+id+"/bank/submit", nil)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "unknown", d["stage"])
	assert.NotContains(t, actions(d), "submit", "no resend is offered for an unknown outcome")
	assert.Contains(t, d["blockingReason"], "Повторная отправка запрещена")
	code, _, _ = s.call(t, s.operator, http.MethodPost, "/refunds/"+id+"/bank/submit", nil)
	assert.Equal(t, http.StatusConflict, code)

	code, _, _ = s.call(t, s.operator, http.MethodPost, "/dev/refund-provider", map[string]any{"answer": "unavailable"})
	require.Equal(t, http.StatusOK, code)
	code, _, _ = s.call(t, s.operator, http.MethodPost, "/refunds/"+id+"/bank/reconcile", nil)
	assert.Equal(t, http.StatusBadGateway, code)
	assert.Equal(t, "unknown", readRefundRow(t, s.fix, refundID).SubmitOutcome, "an unreachable bank changes nothing")
	_, _, _ = s.call(t, s.operator, http.MethodPost, "/dev/refund-provider", map[string]any{"answer": "available"})

	code, _, _ = s.call(t, s.operator, http.MethodPost, "/dev/refund-provider", map[string]any{"refundId": id, "answer": "contradict"})
	require.Equal(t, http.StatusOK, code)
	code, d, _ = s.call(t, s.operator, http.MethodPost, "/refunds/"+id+"/bank/reconcile", nil)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "review", d["stage"])
	assert.Equal(t, []string{"note"}, actions(d), "under review only notes are possible")
	assert.NotNil(t, d["reviewReason"])
	code, _, _ = s.call(t, s.operator, http.MethodPost, "/refunds/"+id+"/bank/reconcile", nil)
	assert.Equal(t, http.StatusConflict, code)

	code, list, _ := s.call(t, s.viewer, http.MethodGet, "/refunds/bank?stage=review", nil)
	require.Equal(t, http.StatusOK, code)
	inQueue := false
	for _, it := range list["items"].([]any) {
		if it.(map[string]any)["id"] == id {
			inQueue = true
		}
	}
	assert.True(t, inQueue, "a review stays in the queue")
	assertNoFinancialSuccess(t, s.fix, o, refundID, retID)
}

// The dev script exists only while the fake is attached.
func TestBankRefund2_DevScriptAbsentWithoutFake(t *testing.T) {
	data := SetupRealRouterAuthFixture(t)
	token, err := data.TokenService.GenerateAccessToken(data.AdminWithPermID, "a@test.com", "admin")
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/refund-provider", strings.NewReader(`{"nextSubmit":"accept"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	data.Router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/admin/refunds/bank", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	data.Router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code, "refunds.create alone does not grant viewing")
}

// A partial refund needing a Receipt is explained and not offered for sending (T-Bank adapter on a local server).
func TestBankRefund2_ReceiptExplained(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	bank := newR1Bank(t)
	fix.svc.WithRefundProvider(bank.provider(t, true, 2*time.Second))
	o := newBankRefundOrder(t, fix, 3)
	setProviderPaymentID(t, fix, &o)
	retID := newReceivedReturn(t, fix, o, 1)
	ref, err := fix.svc.CreateRefund(ctx, fix.userID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)

	d, err := fix.svc.GetAdminBankRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.Equal(t, "prepared", d.Stage)
	assert.True(t, d.IsPartial)
	assert.Equal(t, []string{"note"}, d.AllowedActions)
	require.NotNil(t, d.BlockingReason)
	assert.Contains(t, *d.BlockingReason, "Receipt")
	assert.Equal(t, 0, bank.count("Cancel"))

	fix.svc.WithRefundProvider(nil)
	d, err = fix.svc.GetAdminBankRefund(ctx, ref.ID)
	require.NoError(t, err)
	assert.False(t, d.ProviderConfigured)
	assert.Contains(t, *d.BlockingReason, "провайдер возвратов не подключён")
}

// Concurrent reconciliation runs: one confirmation, one deduction; the backoff skips freshly checked attempts.
func TestBankRefund2_ConcurrentReconciliationAndBackoff(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	fake := withFakeRefundProvider(fix)
	o := newBankRefundOrder(t, fix, 3)
	ret1 := newReceivedReturn(t, fix, o, 1)
	ret2 := newReceivedReturn(t, fix, o, 1)
	ref1, err := fix.svc.CreateRefund(ctx, fix.userID, ret1, returns.CreateRefundRequest{})
	require.NoError(t, err)
	ref2, err := fix.svc.CreateRefund(ctx, fix.userID, ret2, returns.CreateRefundRequest{})
	require.NoError(t, err)
	for _, id := range []uuid.UUID{ref1.ID, ref2.ID} {
		_, err := fix.svc.SubmitRefund(ctx, id)
		require.NoError(t, err)
	}
	require.NoError(t, fake.Confirm(*readRefundRow(t, fix, ref1.ID).Key)) // only the first partial is confirmed

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = fix.svc.ReconcileUnresolvedRefunds(ctx, returns.RefundReconcileBatch{Limit: 50})
		}()
	}
	wg.Wait()
	assert.Equal(t, "succeeded", readRefundRow(t, fix, ref1.ID).Status)
	assert.Equal(t, "pending", readRefundRow(t, fix, ref2.ID).Status, "the other partial refund is not confirmed by the first")
	n, sum := returnDeductions(t, fix, o.ord.orderID)
	assert.Equal(t, 1, n, "concurrent runs post the deduction once")
	assert.Equal(t, int64(-1000), sum)
	assert.Equal(t, 1, fake.SubmitCalls(*readRefundRow(t, fix, ref1.ID).Key), "reconciliation never resends")

	checked, err := fix.svc.ReconcileUnresolvedRefunds(ctx, returns.RefundReconcileBatch{Limit: 50, RecheckAfter: time.Hour})
	require.NoError(t, err)
	assert.Equal(t, 0, checked, "an attempt checked within the backoff is skipped")
}

// A refund from before migration 000120 is listed as legacy history: never sent, never reconciled automatically.
func TestBankRefund2_LegacyIsHistoryOnly(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewDisposableDatabase(t)
	testutil.MustBeDisposableDatabase(t, pool)
	_, err := pool.Exec(ctx, readMigration(t, "000120_refund_bank_submission.down.sql"))
	require.NoError(t, err)
	userID, orderID, payID, refundID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users (id, name, phone, email, password_hash) VALUES ($1, 'C', '+79990000001', $2, 'h')`, []any{userID, userID.String() + "@t.test"}},
		{`INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
			VALUES ($1, $2, 'ORD-LEGACY-1', 'delivered', 3000, 'RUB', 'A', 'C', 0, 'N', 'e@t.test', '+79990000001')`, []any{orderID, userID}},
		{`INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key)
			VALUES ($1, $2, 'tbank', '700000777', 'succeeded', 3000, 'RUB', $3)`, []any{payID, orderID, uuid.New().String()}},
		{`INSERT INTO refunds (id, payment_id, order_id, status, amount_cents, currency) VALUES ($1, $2, $3, 'pending', 3000, 'RUB')`, []any{refundID, payID, orderID}},
	} {
		_, err := pool.Exec(ctx, q.sql, q.args...)
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx, readMigration(t, "000120_refund_bank_submission.up.sql"))
	require.NoError(t, err)

	svc := returns.NewService(returns.NewRepository(pool), nil, nil, &postgres.Client{Pool: pool}, nil, nil, 14, nil, nil, nil, nil).
		WithRefundProvider(payments.NewFakeRefundProvider())
	d, err := svc.GetAdminBankRefund(ctx, refundID)
	require.NoError(t, err)
	assert.Equal(t, "legacy", d.Stage)
	assert.Equal(t, []string{"note"}, d.AllowedActions)
	assert.Contains(t, *d.BlockingReason, "Историческая запись")
	_, err = svc.SubmitRefund(ctx, refundID)
	assert.ErrorIs(t, err, returns.ErrRefundLegacyRecord)
	list, err := svc.ListAdminBankRefunds(ctx, "legacy", 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, 1, list.StageCounts["legacy"])
	checked, err := svc.ReconcileUnresolvedRefunds(ctx, returns.RefundReconcileBatch{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, checked, "legacy refunds are never reconciled automatically")
}
