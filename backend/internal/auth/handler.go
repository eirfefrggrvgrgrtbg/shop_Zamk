package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type Handler struct {
	service        *Service
	validator      *validator.Validate
	cookieDomain   string
	cookieSecure   bool
	cookiePath     string
	refreshTTLDays int
}

func NewHandler(service *Service, refreshTTLDays int) *Handler {
	return &Handler{
		service:        service,
		validator:      validator.New(),
		cookieDomain:   "", // local
		cookieSecure:   false, // local
		cookiePath:     "/api/auth",
		refreshTTLDays: refreshTTLDays,
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	ip := r.RemoteAddr
	userAgent := r.UserAgent()

	resp, rawRefresh, err := h.service.RegisterCustomer(r.Context(), req, userAgent, ip)
	if err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			h.writeError(w, http.StatusConflict, "duplicate_email", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to register")
		return
	}

	h.setRefreshCookie(w, rawRefresh)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	ip := r.RemoteAddr
	userAgent := r.UserAgent()

	resp, rawRefresh, err := h.service.Login(r.Context(), req, userAgent, ip)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			h.writeError(w, http.StatusUnauthorized, "invalid_credentials", err.Error())
			return
		}
		if errors.Is(err, ErrUserBlocked) || errors.Is(err, ErrUserDeleted) {
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to login")
		return
	}

	h.setRefreshCookie(w, rawRefresh)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("zamk_refresh_token")
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing refresh token")
		return
	}

	ip := r.RemoteAddr
	userAgent := r.UserAgent()

	resp, newRawRefresh, err := h.service.Refresh(r.Context(), cookie.Value, userAgent, ip)
	if err != nil {
		h.clearRefreshCookie(w)
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired session")
		return
	}

	h.setRefreshCookie(w, newRawRefresh)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("zamk_refresh_token")
	if err == nil {
		_ = h.service.Logout(r.Context(), cookie.Value)
	}

	h.clearRefreshCookie(w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	// Require userID from context
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}

	userID, ok := val.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid user context")
		return
	}

	resp, err := h.service.Me(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get profile")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}

	userID, ok := val.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid user context")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.service.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	h.clearRefreshCookie(w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "zamk_refresh_token",
		Value:    token,
		Path:     h.cookiePath,
		Domain:   h.cookieDomain,
		MaxAge:   h.refreshTTLDays * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   h.cookieSecure, // TODO: set to true in production via config
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "zamk_refresh_token",
		Value:    "",
		Path:     h.cookiePath,
		Domain:   h.cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := ErrorResponse{}
	resp.Error.Code = code
	resp.Error.Message = message
	json.NewEncoder(w).Encode(resp)
}
