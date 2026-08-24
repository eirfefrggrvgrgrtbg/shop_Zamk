package storage

import "errors"

var (
	ErrInvalidMimeType  = errors.New("invalid mime type")
	ErrInvalidExtension = errors.New("invalid file extension")
	ErrFileTooLarge     = errors.New("file too large")
	ErrUploadFailed     = errors.New("upload failed")
	ErrProductNotOwned              = errors.New("product does not belong to seller")
	ErrProductNotDraft              = errors.New("product must be in draft or rejected status to modify images")
	ErrFileNotFound                 = errors.New("file not found in request")
	ErrProductMediaPortraitRequired = errors.New("Для товара нужны вертикальные фотографии. Загрузите изображение в вертикальном формате.")
	ErrProductMediaTooSmall         = errors.New("Изображение слишком маленькое. Минимальный размер — 800×1000 пикселей.")
	ErrInvalidCropParameters        = errors.New("invalid crop parameters")
	ErrInvalidCropAspect            = errors.New("crop aspect ratio must be 4:5")
)
