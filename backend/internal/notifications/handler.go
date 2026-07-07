package notifications

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"log/slog"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) getPagination(r *http.Request) (int, int) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	
	limit := 20
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	
	offset := 0
	if o, err := strconv.Atoi(offsetStr); err == nil && o > 0 {
		offset = o
	}
	
	return limit, offset
}

func (h *Handler) ListCustomerNotifications(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)

	limit, offset := h.getPagination(r)
	notifications, total, err := h.svc.ListForCustomer(r.Context(), userID, limit, offset)
	if err != nil {
		h.log.Error("failed to list customer notifications", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if notifications == nil {
		notifications = []Notification{}
	}

	json.NewEncoder(w).Encode(PaginatedNotifications{Items: notifications, TotalCount: total})
}

func (h *Handler) ListSellerNotifications(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)

	limit, offset := h.getPagination(r)
	notifications, total, err := h.svc.ListForSeller(r.Context(), userID, limit, offset)
	if err != nil {
		h.log.Error("failed to list seller notifications", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if notifications == nil {
		notifications = []Notification{}
	}

	json.NewEncoder(w).Encode(PaginatedNotifications{Items: notifications, TotalCount: total})
}

func (h *Handler) ListAdminNotifications(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)

	limit, offset := h.getPagination(r)
	notifications, total, err := h.svc.ListForStaff(r.Context(), userID, limit, offset)
	if err != nil {
		h.log.Error("failed to list admin notifications", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if notifications == nil {
		notifications = []Notification{}
	}

	json.NewEncoder(w).Encode(PaginatedNotifications{Items: notifications, TotalCount: total})
}

func (h *Handler) ReadCustomerNotification(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.svc.MarkReadCustomer(r.Context(), id, userID); err != nil {
		if err.Error() == "no rows in result set" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to mark customer notification read", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ReadSellerNotification(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.svc.MarkReadSeller(r.Context(), id, userID); err != nil {
		if err.Error() == "no rows in result set" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to mark seller notification read", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ReadAdminNotification(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.svc.MarkReadStaff(r.Context(), id, userID); err != nil {
		if err.Error() == "no rows in result set" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to mark admin notification read", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ReadAllCustomer(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)
	if err := h.svc.MarkAllReadCustomer(r.Context(), userID); err != nil {
		h.log.Error("failed to mark all customer notifications read", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ReadAllSeller(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)
	if err := h.svc.MarkAllReadSeller(r.Context(), userID); err != nil {
		h.log.Error("failed to mark all seller notifications read", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ReadAllAdmin(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)

	if err := h.svc.MarkAllReadStaff(r.Context(), userID); err != nil {
		h.log.Error("failed to mark all admin notifications read", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) UnreadCountCustomer(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)
	count, err := h.svc.CountUnreadCustomer(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to count unread customer notifications", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(UnreadCountResponse{UnreadCount: count})
}

func (h *Handler) UnreadCountSeller(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)
	count, err := h.svc.CountUnreadSeller(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to count unread seller notifications", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(UnreadCountResponse{UnreadCount: count})
}

func (h *Handler) UnreadCountAdmin(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)

	count, err := h.svc.CountUnreadStaff(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to count unread admin notifications", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(UnreadCountResponse{UnreadCount: count})
}
