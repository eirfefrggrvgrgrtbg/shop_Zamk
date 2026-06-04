package storage

type StoredObject struct {
	ObjectURL string
	ObjectKey string
	Size      int64
}

type UploadOptions struct {
	IsMain    bool
	SortOrder int
	AltText   string
}
