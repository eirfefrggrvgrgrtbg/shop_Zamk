package storage

import "errors"

var (
	ErrInvalidMimeType  = errors.New("invalid mime type")
	ErrInvalidExtension = errors.New("invalid file extension")
	ErrFileTooLarge     = errors.New("file too large")
	ErrUploadFailed     = errors.New("upload failed")
	ErrProductNotOwned  = errors.New("product does not belong to seller")
	ErrProductNotDraft  = errors.New("product must be in draft or rejected status to modify images")
	ErrFileNotFound     = errors.New("file not found in request")
)
