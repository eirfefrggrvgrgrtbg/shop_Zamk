package supplies

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		svc:    svc,
		logger: logger,
	}
}

func (h *Handler) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func (h *Handler) ListSupplies(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	if !okRole || role != "seller" {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing seller role")
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}

	sellerID, err := h.svc.repo.GetSellerIDByUserID(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller profile not found")
		return
	}

	supplies, err := h.svc.repo.GetSuppliesBySeller(r.Context(), sellerID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(supplies)
}

func (h *Handler) GetSupply(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okRole || !okUser {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid supply ID")
		return
	}

	supply, err := h.svc.repo.GetSupplyByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrSupplyNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Supply not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	sellerID, err := h.svc.repo.GetSellerIDByUserID(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller profile not found")
		return
	}

	if role == "seller" && supply.SellerID != sellerID {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(supply)
}

func (h *Handler) GetUnitLabels(w http.ResponseWriter, r *http.Request) {
	role, okRole := r.Context().Value("role").(string)
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okRole || !okUser || role != "seller" {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid supply ID")
		return
	}

	sellerID, err := h.svc.repo.GetSellerIDByUserID(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller profile not found")
		return
	}

	res, err := h.svc.GetSupplyUnitLabels(r.Context(), sellerID, id)
	if err != nil {
		if errors.Is(err, ErrSupplyNotFound) {
			h.writeError(w, http.StatusNotFound, "supply_not_found", "Supply not found")
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			h.writeError(w, http.StatusForbidden, "supply_forbidden", "Access denied")
			return
		}
		if errors.Is(err, ErrSupplyUnitIdentityMismatch) {
			h.writeError(w, http.StatusUnprocessableEntity, "supply_unit_identity_mismatch", "Supply unit count does not match expected quantity")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *Handler) CreateSupply(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.GetReqID(r.Context())
	role, okRole := r.Context().Value("role").(string)
	if !okRole || role != "seller" {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	sellerID, err := h.svc.repo.GetSellerIDByUserID(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller not found")
		return
	}

	var req CreateSupplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("create supply request decode failed",
			"event", "create_supply_failed",
			"request_id", reqID,
			"seller_id", sellerID.String(),
			"http_status", http.StatusBadRequest,
			"error_code", "invalid_request",
			"error", err.Error(),
		)
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	supply, err := h.svc.CreateSupply(r.Context(), sellerID, req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "internal_error"
		errorMsg := err.Error()

		if errors.Is(err, ErrCarrierRequired) {
			statusCode = http.StatusBadRequest
			errorCode = "supply_carrier_required"
			errorMsg = "carrier is required for carrier delivery"
		} else if errors.Is(err, ErrCarrierUnsupported) {
			statusCode = http.StatusBadRequest
			errorCode = "supply_carrier_unsupported"
			errorMsg = "unsupported carrier, currently only CDEK is supported"
		} else if errors.Is(err, ErrTrackingNumberRequired) {
			statusCode = http.StatusBadRequest
			errorCode = "supply_tracking_number_required"
			errorMsg = "tracking number is required for carrier delivery"
		} else if errors.Is(err, ErrInvalidQuantities) {
			statusCode = http.StatusBadRequest
			errorCode = "supply_items_required"
			errorMsg = "at least one item with quantity > 0 is required"
		} else if errors.Is(err, ErrUnauthorized) {
			statusCode = http.StatusForbidden
			errorCode = "forbidden"
			errorMsg = "one or more variants do not belong to the seller"
		}

		h.logger.Warn("create supply failed",
			"event", "create_supply_failed",
			"request_id", reqID,
			"seller_id", sellerID.String(),
			"http_status", statusCode,
			"error_code", errorCode,
			"error", errorMsg,
			"handoff_method", req.HandoffMethod,
			"item_count", len(req.Items),
			"carrier_present", req.CarrierName != nil && strings.TrimSpace(*req.CarrierName) != "",
			"tracking_present", req.TrackingNumber != nil && strings.TrimSpace(*req.TrackingNumber) != "",
		)

		h.writeError(w, statusCode, errorCode, errorMsg)
		return
	}

	h.logger.Info("create supply success",
		"event", "create_supply_success",
		"request_id", reqID,
		"seller_id", sellerID.String(),
		"supply_id", supply.ID.String(),
		"supply_number", supply.SupplyNumber,
		"item_count", len(supply.Items),
		"total_expected_units", supply.TotalExpectedItems,
		"box_count", supply.TotalExpectedBoxes,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(supply)
}

func (h *Handler) MarkShipped(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.GetReqID(r.Context())
	role, okRole := r.Context().Value("role").(string)
	if !okRole || role != "seller" {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID, okUser := r.Context().Value("userID").(uuid.UUID)
	if !okUser {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid supply ID")
		return
	}

	sellerID, err := h.svc.repo.GetSellerIDByUserID(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller not found")
		return
	}

	updatedSupply, err := h.svc.MarkShipped(r.Context(), sellerID, id)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "internal_error"
		errorMsg := err.Error()

		if errors.Is(err, ErrUnauthorized) {
			statusCode = http.StatusForbidden
			errorCode = "forbidden"
			errorMsg = "access denied"
		} else if errors.Is(err, ErrInvalidStatus) {
			statusCode = http.StatusBadRequest
			errorCode = "supply_invalid_status"
			errorMsg = "supply must be in ready_to_ship status to mark shipped"
		} else if errors.Is(err, ErrSupplyNotFound) {
			statusCode = http.StatusNotFound
			errorCode = "not_found"
			errorMsg = "supply not found"
		}

		h.logger.Warn("mark supply shipped failed",
			"event", "supply_mark_shipped_failed",
			"request_id", reqID,
			"seller_id", sellerID.String(),
			"supply_id", id.String(),
			"http_status", statusCode,
			"error_code", errorCode,
			"error", errorMsg,
		)

		h.writeError(w, statusCode, errorCode, errorMsg)
		return
	}

	h.logger.Info("mark supply shipped success",
		"event", "supply_mark_shipped_success",
		"request_id", reqID,
		"seller_id", sellerID.String(),
		"supply_id", updatedSupply.ID.String(),
		"supply_number", updatedSupply.SupplyNumber,
		"old_status", "ready_to_ship",
		"new_status", "shipped_by_seller",
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedSupply)
}
