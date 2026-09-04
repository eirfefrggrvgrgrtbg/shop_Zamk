package observability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/middleware"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/observability"
)

func TestActorContext_RealAuthWiringAndPIIShield(t *testing.T) {
	// Simulate the exact context populated by internal/http/middleware/auth.go:
	// ctx = context.WithValue(r.Context(), "userID", userID)
	// ctx = context.WithValue(ctx, "email", email)
	// ctx = context.WithValue(ctx, "role", role)
	adminID := uuid.New()
	customerEmail := "superadmin@zamk.local"
	role := "admin"

	ctx := context.Background()
	ctx = context.WithValue(ctx, "userID", adminID)
	ctx = context.WithValue(ctx, "email", customerEmail)
	ctx = context.WithValue(ctx, "role", role)
	ctx = context.WithValue(ctx, "token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.secret")

	// 1. Verify ActorFromContext extracts actor_id and actor_role
	extractedID, extractedRole := observability.ActorFromContext(ctx)
	assert.Equal(t, adminID.String(), extractedID)
	assert.Equal(t, role, extractedRole)

	// 2. Verify EmitBusinessEvent with a capturing JSON logger
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	logger := slog.New(jsonHandler)

	observability.EmitBusinessEvent(ctx, logger, observability.BusinessEvent{
		EventName: "warehouse.reconciliation_started",
		Domain:    "warehouse",
		Action:    "start_reconciliation",
		Result:    "success",
		Attributes: []slog.Attr{
			slog.String("reconciliation_session_id", uuid.New().String()),
		},
	})

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	// Must contain actor_id and actor_role
	assert.Equal(t, adminID.String(), logEntry["actor_id"])
	assert.Equal(t, "admin", logEntry["actor_role"])

	// Must NEVER contain email, token, password, or PII
	assert.Nil(t, logEntry["email"], "email must not be extracted or emitted")
	assert.Nil(t, logEntry["token"], "token must not be extracted or emitted")
	assert.Nil(t, logEntry["password"], "password must not be extracted or emitted")

	rawLog := buf.String()
	assert.NotContains(t, rawLog, customerEmail)
	assert.NotContains(t, rawLog, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
}

func TestActorContext_AuthMiddlewarePipeline_PIIShield(t *testing.T) {
	tokenSvc := auth.NewTokenService("access-secret-12345", "refresh-secret-12345", 60)
	adminID := uuid.New()
	adminEmail := "ops_admin@zamk.local"
	role := "admin"

	token, err := tokenSvc.GenerateAccessToken(adminID, adminEmail, role)
	require.NoError(t, err)

	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	logger := slog.New(jsonHandler)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observability.EmitBusinessEvent(r.Context(), logger, observability.BusinessEvent{
			EventName: "warehouse.reconciliation_completed",
			Domain:    "warehouse",
			Action:    "complete_reconciliation",
			Result:    "success",
			Attributes: []slog.Attr{
				slog.String("session_id", "recon-session-123"),
			},
		})
		w.WriteHeader(http.StatusOK)
	})

	authMw := middleware.AuthMiddleware(tokenSvc)
	wrappedHandler := authMw(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/reconciliation/complete", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	// Actor credentials must propagate seamlessly from AuthMiddleware into business events
	assert.Equal(t, adminID.String(), logEntry["actor_id"])
	assert.Equal(t, "admin", logEntry["actor_role"])
	assert.Equal(t, "warehouse.reconciliation_completed", logEntry["event_name"])

	// PII and secrets must never be leaked
	assert.Nil(t, logEntry["email"], "email must not be extracted or emitted")
	assert.Nil(t, logEntry["token"], "token must not be extracted or emitted")
	assert.Nil(t, logEntry["password"], "password must not be extracted or emitted")
	assert.Nil(t, logEntry["phone"], "phone must not be extracted or emitted")

	rawLog := buf.String()
	assert.NotContains(t, rawLog, adminEmail)
	assert.NotContains(t, rawLog, token)
}
