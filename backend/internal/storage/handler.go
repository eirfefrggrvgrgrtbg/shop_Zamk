package storage

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
	cfg     *config.S3Config
}

func NewHandler(service *Service, cfg *config.S3Config) *Handler {
	return &Handler{
		service: service,
		cfg:     cfg,
	}
}

func (h *Handler) writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) parseUploadRequest(r *http.Request) (fileReader multipart.File, header *multipart.FileHeader, opts UploadOptions, err error) {
	maxMemory := int64(h.cfg.UploadMaxSizeMB) * 1024 * 1024
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return nil, nil, opts, err
	}

	fileReader, header, err = r.FormFile("image")
	if err != nil {
		fileReader, header, err = r.FormFile("logo")
		if err != nil {
			return nil, nil, opts, ErrFileNotFound
		}
	}

	opts.IsMain = r.FormValue("isMain") == "true"
	opts.AltText = r.FormValue("altText")
	if sortOrderStr := r.FormValue("sortOrder"); sortOrderStr != "" {
		if sortOrder, err := strconv.Atoi(sortOrderStr); err == nil {
			opts.SortOrder = sortOrder
		}
	}

	return fileReader, header, opts, nil
}

func (h *Handler) UploadSellerProductImage(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	sellerIDRaw := r.Context().Value("userID")
	userID, ok := sellerIDRaw.(uuid.UUID)
	if !ok {
		h.writeJSONError(w, http.StatusUnauthorized, "user id not found in context")
		return
	}

	file, header, opts, err := h.parseUploadRequest(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")

	resp, err := h.service.UploadSellerProductImage(r.Context(), userID, productID, file, header.Filename, header.Size, contentType, int64(h.cfg.UploadMaxSizeMB), opts)
	if err != nil {
		if err == ErrProductNotOwned || err == ErrProductNotDraft || err == products.ErrProductNotEditable {
			h.writeJSONError(w, http.StatusForbidden, err.Error())
			return
		}
		if err == ErrInvalidMimeType || err == ErrInvalidExtension || err == ErrFileTooLarge {
			h.writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeJSONError(w, http.StatusInternalServerError, "upload failed")
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) UploadAdminProductImage(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	file, header, opts, err := h.parseUploadRequest(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")

	resp, err := h.service.UploadAdminProductImage(r.Context(), productID, file, header.Filename, header.Size, contentType, int64(h.cfg.UploadMaxSizeMB), opts)
	if err != nil {
		if err == ErrInvalidMimeType || err == ErrInvalidExtension || err == ErrFileTooLarge {
			h.writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeJSONError(w, http.StatusInternalServerError, "upload failed")
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) UploadAdminBrandLogo(w http.ResponseWriter, r *http.Request) {
	brandIDStr := chi.URLParam(r, "id")
	brandID, err := uuid.Parse(brandIDStr)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid brand id")
		return
	}

	file, header, _, err := h.parseUploadRequest(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")

	resp, err := h.service.UploadAdminBrandLogo(r.Context(), brandID, file, header.Filename, header.Size, contentType, int64(h.cfg.UploadMaxSizeMB))
	if err != nil {
		if err == ErrInvalidMimeType || err == ErrInvalidExtension || err == ErrFileTooLarge {
			h.writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeJSONError(w, http.StatusInternalServerError, "upload failed")
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) UploadSellerProfileImage(w http.ResponseWriter, r *http.Request) {
	sellerIDRaw := r.Context().Value("userID")
	userID, ok := sellerIDRaw.(uuid.UUID)
	if !ok {
		h.writeJSONError(w, http.StatusUnauthorized, "user id not found in context")
		return
	}

	file, header, _, err := h.parseUploadRequest(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")

	resp, err := h.service.UploadSellerProfileImage(r.Context(), userID, file, header.Filename, header.Size, contentType, int64(h.cfg.UploadMaxSizeMB))
	if err != nil {
		if err == ErrInvalidMimeType || err == ErrInvalidExtension || err == ErrFileTooLarge {
			h.writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeJSONError(w, http.StatusInternalServerError, "upload failed")
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteSellerProductImage(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	imageIDStr := chi.URLParam(r, "imageId")
	imageID, err := uuid.Parse(imageIDStr)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid image id")
		return
	}

	sellerIDRaw := r.Context().Value("userID")
	userID, ok := sellerIDRaw.(uuid.UUID)
	if !ok {
		h.writeJSONError(w, http.StatusUnauthorized, "user id not found in context")
		return
	}

	err = h.service.DeleteSellerProductImage(r.Context(), userID, productID, imageID)
	if err != nil {
		if err == ErrProductNotOwned || err == ErrProductNotDraft || err == products.ErrProductNotEditable {
			h.writeJSONError(w, http.StatusForbidden, err.Error())
			return
		}
		if err.Error() == "image does not belong to product" || err == products.ErrProductNotFound {
			h.writeJSONError(w, http.StatusNotFound, "image not found")
			return
		}
		h.writeJSONError(w, http.StatusInternalServerError, "failed to delete image: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type ReorderImagesRequest struct {
	ImageIDs []uuid.UUID `json:"imageIds"`
}

func (h *Handler) ReorderSellerProductImages(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	sellerIDRaw := r.Context().Value("userID")
	userID, ok := sellerIDRaw.(uuid.UUID)
	if !ok {
		h.writeJSONError(w, http.StatusUnauthorized, "user id not found in context")
		return
	}

	var req ReorderImagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err = h.service.ReorderSellerProductImages(r.Context(), userID, productID, req.ImageIDs)
	if err != nil {
		if err == ErrProductNotOwned || err == ErrProductNotDraft || err == products.ErrProductNotEditable {
			h.writeJSONError(w, http.StatusForbidden, err.Error())
			return
		}
		if strings.Contains(err.Error(), "duplicate image ID") || strings.Contains(err.Error(), "does not belong") || strings.Contains(err.Error(), "missing images") {
			h.writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeJSONError(w, http.StatusInternalServerError, "failed to reorder images")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
