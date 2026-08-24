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

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
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
		h.writeError(w, http.StatusBadRequest, "invalid_product_id", "invalid product id")
		return
	}

	sellerIDRaw := r.Context().Value("userID")
	userID, ok := sellerIDRaw.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user id not found in context")
		return
	}

	file, header, opts, err := h.parseUploadRequest(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request: "+err.Error())
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")

	resp, err := h.service.UploadSellerProductImage(r.Context(), userID, productID, file, header.Filename, header.Size, contentType, int64(h.cfg.UploadMaxSizeMB), opts)
	if err != nil {
		if err == ErrProductNotOwned || err == ErrProductNotDraft || err == products.ErrProductNotEditable {
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		if err == ErrProductMediaPortraitRequired {
			h.writeError(w, http.StatusBadRequest, "product_media_portrait_required", err.Error())
			return
		}
		if err == ErrProductMediaTooSmall {
			h.writeError(w, http.StatusBadRequest, "product_media_too_small", err.Error())
			return
		}
		if err == ErrInvalidMimeType || err == ErrInvalidExtension {
			h.writeError(w, http.StatusBadRequest, "invalid_file_type", err.Error())
			return
		}
		if err == ErrFileTooLarge {
			h.writeError(w, http.StatusBadRequest, "file_too_large", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "upload failed")
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) UploadAdminProductImage(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_product_id", "invalid product id")
		return
	}

	file, header, opts, err := h.parseUploadRequest(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request: "+err.Error())
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")

	resp, err := h.service.UploadAdminProductImage(r.Context(), productID, file, header.Filename, header.Size, contentType, int64(h.cfg.UploadMaxSizeMB), opts)
	if err != nil {
		if err == ErrInvalidMimeType || err == ErrInvalidExtension {
			h.writeError(w, http.StatusBadRequest, "invalid_file_type", err.Error())
			return
		}
		if err == ErrFileTooLarge {
			h.writeError(w, http.StatusBadRequest, "file_too_large", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "upload failed")
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) UploadAdminBrandLogo(w http.ResponseWriter, r *http.Request) {
	brandIDStr := chi.URLParam(r, "id")
	brandID, err := uuid.Parse(brandIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_brand_id", "invalid brand id")
		return
	}

	file, header, _, err := h.parseUploadRequest(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request: "+err.Error())
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")

	resp, err := h.service.UploadAdminBrandLogo(r.Context(), brandID, file, header.Filename, header.Size, contentType, int64(h.cfg.UploadMaxSizeMB))
	if err != nil {
		if err == ErrInvalidMimeType || err == ErrInvalidExtension {
			h.writeError(w, http.StatusBadRequest, "invalid_file_type", err.Error())
			return
		}
		if err == ErrFileTooLarge {
			h.writeError(w, http.StatusBadRequest, "file_too_large", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "upload failed")
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) UploadSellerProfileImage(w http.ResponseWriter, r *http.Request) {
	sellerIDRaw := r.Context().Value("userID")
	userID, ok := sellerIDRaw.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user id not found in context")
		return
	}

	file, header, _, err := h.parseUploadRequest(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request: "+err.Error())
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")

	resp, err := h.service.UploadSellerProfileImage(r.Context(), userID, file, header.Filename, header.Size, contentType, int64(h.cfg.UploadMaxSizeMB))
	if err != nil {
		if err == ErrInvalidMimeType || err == ErrInvalidExtension {
			h.writeError(w, http.StatusBadRequest, "invalid_file_type", err.Error())
			return
		}
		if err == ErrFileTooLarge {
			h.writeError(w, http.StatusBadRequest, "file_too_large", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "upload failed")
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteSellerProductImage(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_product_id", "invalid product id")
		return
	}

	imageIDStr := chi.URLParam(r, "imageId")
	imageID, err := uuid.Parse(imageIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_image_id", "invalid image id")
		return
	}

	sellerIDRaw := r.Context().Value("userID")
	userID, ok := sellerIDRaw.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user id not found in context")
		return
	}

	err = h.service.DeleteSellerProductImage(r.Context(), userID, productID, imageID)
	if err != nil {
		if err == ErrProductNotOwned || err == ErrProductNotDraft || err == products.ErrProductNotEditable {
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		if err.Error() == "image does not belong to product" || err == products.ErrProductNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "image not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete image: "+err.Error())
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
		h.writeError(w, http.StatusBadRequest, "invalid_product_id", "invalid product id")
		return
	}

	sellerIDRaw := r.Context().Value("userID")
	userID, ok := sellerIDRaw.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user id not found in context")
		return
	}

	var req ReorderImagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	err = h.service.ReorderSellerProductImages(r.Context(), userID, productID, req.ImageIDs)
	if err != nil {
		if err == ErrProductNotOwned || err == ErrProductNotDraft || err == products.ErrProductNotEditable {
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		if strings.Contains(err.Error(), "duplicate image ID") || strings.Contains(err.Error(), "does not belong") || strings.Contains(err.Error(), "missing images") {
			h.writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "failed to reorder images")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CropSellerProductImage(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_product_id", "invalid product id")
		return
	}

	imageIDStr := chi.URLParam(r, "imageId")
	imageID, err := uuid.Parse(imageIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_image_id", "invalid image id")
		return
	}

	sellerIDRaw := r.Context().Value("userID")
	userID, ok := sellerIDRaw.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user id not found in context")
		return
	}

	var req CropImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "product_media_invalid_crop", "invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		h.writeError(w, http.StatusBadRequest, "product_media_invalid_crop", "Не удалось сохранить кадр 4:5. Попробуйте выбрать область фотографии ещё раз.")
		return
	}

	resp, err := h.service.CropSellerProductImage(r.Context(), userID, productID, imageID, req)
	if err != nil {
		if err == ErrInvalidCropParameters || err == ErrInvalidCropAspect {
			h.writeError(w, http.StatusBadRequest, "product_media_invalid_crop", "Не удалось сохранить кадр 4:5. Попробуйте выбрать область фотографии ещё раз.")
			return
		}
		if err == ErrProductNotOwned || err == products.ErrProductNotEditable {
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		if err.Error() == "image does not belong to product" {
			h.writeError(w, http.StatusNotFound, "not_found", "image not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "failed to crop image: "+err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) SetMainProductImage(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_product_id", "invalid product id")
		return
	}

	imageIDStr := chi.URLParam(r, "imageId")
	imageID, err := uuid.Parse(imageIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_image_id", "invalid image id")
		return
	}

	sellerIDRaw := r.Context().Value("userID")
	userID, ok := sellerIDRaw.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user id not found in context")
		return
	}

	err = h.service.SetMainProductImage(r.Context(), userID, productID, imageID)
	if err != nil {
		if err == ErrProductNotOwned || err == products.ErrProductNotEditable {
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		if err.Error() == "image does not belong to product" {
			h.writeError(w, http.StatusNotFound, "not_found", "image not found")
			return
		}
		if err.Error() == "image must be cropped to 4:5 before it can be made main" || err.Error() == "selected image is not ready (missing rendition)" {
			h.writeError(w, http.StatusBadRequest, "product_media_not_ready", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "failed to set main image: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
