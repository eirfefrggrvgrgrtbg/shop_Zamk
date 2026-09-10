package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type Handler struct {
	service        *Service
	validator      *validator.Validate
	cookieDomain   string
	cookieSecure   bool
	cookieSameSite http.SameSite
	cookiePath     string
	refreshTTLDays int
}

type CookieConfig struct {
	Domain   string
	Secure   bool
	SameSite string
}

const (
	CookieShopSession   = "zamk_shop_session"
	CookieSellerSession = "zamk_seller_session"
	CookieAdminSession  = "zamk_admin_session"
	CookieLegacySession = "zamk_refresh_token"

	ScopeShop   = "shop"
	ScopeSeller = "seller"
	ScopeAdmin  = "admin"
)

var ErrInvalidAppScope = errors.New("invalid application scope")

func ResolveAppScope(r *http.Request) (string, error) {
	val := r.Header.Get("X-Zamk-App")
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return "", nil
	}
	switch strings.ToLower(trimmed) {
	case ScopeShop:
		return ScopeShop, nil
	case ScopeSeller:
		return ScopeSeller, nil
	case ScopeAdmin:
		return ScopeAdmin, nil
	default:
		return "", ErrInvalidAppScope
	}
}

func CookieNameForScope(scope string) string {
	switch scope {
	case ScopeShop:
		return CookieShopSession
	case ScopeSeller:
		return CookieSellerSession
	case ScopeAdmin:
		return CookieAdminSession
	default:
		return CookieLegacySession
	}
}

func getRefreshCookie(r *http.Request, scope string) (*http.Cookie, error) {
	targetName := CookieNameForScope(scope)
	cookie, err := r.Cookie(targetName)
	if err == nil && cookie.Value != "" {
		return cookie, nil
	}
	return nil, http.ErrNoCookie
}

func NewHandler(service *Service, refreshTTLDays int, cookieConfig CookieConfig) *Handler {
	return &Handler{
		service:        service,
		validator:      validator.New(),
		cookieDomain:   cookieConfig.Domain,
		cookieSecure:   cookieConfig.Secure,
		cookieSameSite: parseSameSite(cookieConfig.SameSite),
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

	if req.Password != req.PasswordConfirm {
		h.writeError(w, http.StatusBadRequest, "password_mismatch", "Пароли не совпадают.")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := ValidateNameFields(&req.FirstName, &req.LastName, &req.MiddleName); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_name", err.Error())
		return
	}

	cleanPhone := strings.ReplaceAll(req.Phone, " ", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, "(", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, ")", "")
	if cleanPhone == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_phone", "Номер телефона обязателен.")
		return
	}
	if !strings.HasPrefix(cleanPhone, "+") {
		h.writeError(w, http.StatusBadRequest, "invalid_phone", "Номер телефона должен начинаться с +.")
		return
	}
	if len(cleanPhone) < 11 || len(cleanPhone) > 16 {
		h.writeError(w, http.StatusBadRequest, "invalid_phone", "Некорректная длина номера телефона.")
		return
	}
	for _, c := range cleanPhone[1:] {
		if c < '0' || c > '9' {
			h.writeError(w, http.StatusBadRequest, "invalid_phone", "Номер телефона должен содержать только цифры после +.")
			return
		}
	}
	req.Phone = cleanPhone

	ip := r.RemoteAddr
	userAgent := r.UserAgent()

	resp, rawRefresh, err := h.service.RegisterCustomer(r.Context(), req, userAgent, ip)
	if err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			h.writeError(w, http.StatusConflict, "duplicate_email", err.Error())
			return
		}
		if IsPasswordError(err) {
			h.writeError(w, http.StatusBadRequest, "weak_password", err.Error())
			return
		}
		log.Printf("registration failed: %v", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to register")
		return
	}

	scope, err := ResolveAppScope(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_app_scope", "Invalid application scope")
		return
	}
	if scope != "" && scope != ScopeShop {
		h.writeError(w, http.StatusForbidden, "forbidden", "Customer registration is only allowed in shop scope")
		return
	}
	h.setRefreshCookie(w, rawRefresh, scope)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	scope, err := ResolveAppScope(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_app_scope", "Invalid application scope")
		return
	}

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

	// Validate role against application scope
	if scope == ScopeAdmin && resp.User.Role != "admin" {
		h.writeError(w, http.StatusForbidden, "forbidden", "This account does not have admin access")
		return
	}
	if scope == ScopeSeller && resp.User.Role != "seller" {
		h.writeError(w, http.StatusForbidden, "forbidden", "This account does not have seller access")
		return
	}
	if scope == ScopeShop && resp.User.Role != "customer" {
		h.writeError(w, http.StatusForbidden, "forbidden", "This account does not have customer access")
		return
	}

	h.setRefreshCookie(w, rawRefresh, scope)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	scope, err := ResolveAppScope(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_app_scope", "Invalid application scope")
		return
	}

	cookie, err := getRefreshCookie(r, scope)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing refresh token")
		return
	}

	ip := r.RemoteAddr
	userAgent := r.UserAgent()

	resp, newRawRefresh, err := h.service.Refresh(r.Context(), cookie.Value, userAgent, ip)
	if err != nil {
		h.clearRefreshCookie(w, scope)
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired session")
		return
	}

	// Ensure the session role matches the requested app scope
	if scope == ScopeAdmin && resp.User.Role != "admin" {
		h.clearRefreshCookie(w, scope)
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "This session does not have admin access")
		return
	}
	if scope == ScopeSeller && resp.User.Role != "seller" {
		h.clearRefreshCookie(w, scope)
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "This session does not have seller access")
		return
	}
	if scope == ScopeShop && resp.User.Role != "customer" {
		h.clearRefreshCookie(w, scope)
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "This session does not have customer access")
		return
	}

	h.setRefreshCookie(w, newRawRefresh, scope)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	scope, err := ResolveAppScope(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_app_scope", "Invalid application scope")
		return
	}

	cookie, err := getRefreshCookie(r, scope)
	if err == nil {
		_ = h.service.Logout(r.Context(), cookie.Value)
	}

	h.clearRefreshCookie(w, scope)

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
	scope, err := ResolveAppScope(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_app_scope", "Invalid application scope")
		return
	}

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

	h.clearRefreshCookie(w, scope)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	_ = h.service.ForgotPassword(r.Context(), req.Email)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Если аккаунт найден, заявка на восстановление создана.",
	})
}

func (h *Handler) setRefreshCookie(w http.ResponseWriter, token string, scope string) {
	targetName := CookieNameForScope(scope)
	http.SetCookie(w, &http.Cookie{
		Name:     targetName,
		Value:    token,
		Path:     h.cookiePath,
		Domain:   h.cookieDomain,
		MaxAge:   h.refreshTTLDays * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: h.cookieSameSite,
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter, scope string) {
	targetName := CookieNameForScope(scope)
	http.SetCookie(w, &http.Cookie{
		Name:     targetName,
		Value:    "",
		Path:     h.cookiePath,
		Domain:   h.cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: h.cookieSameSite,
	})
	if scope == "" {
		http.SetCookie(w, &http.Cookie{
			Name:     CookieLegacySession,
			Value:    "",
			Path:     h.cookiePath,
			Domain:   h.cookieDomain,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   h.cookieSecure,
			SameSite: h.cookieSameSite,
		})
	}
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax", "":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteLaxMode
	}
}

func (h *Handler) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := ErrorResponse{}
	resp.Error.Code = code
	resp.Error.Message = message
	json.NewEncoder(w).Encode(resp)
}
