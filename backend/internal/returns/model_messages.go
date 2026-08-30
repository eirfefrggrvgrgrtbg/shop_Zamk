package returns

import (
	"time"

	"github.com/google/uuid"
)

const (
	ReturnMessageSenderRoleCustomer = "customer"
	ReturnMessageSenderRoleAdmin    = "admin"

	ReturnMessageTypeMessage     = "message"
	ReturnMessageTypeInfoRequest = "info_request"
)

type ReturnMessage struct {
	ID           uuid.UUID `json:"id"`
	ReturnID     uuid.UUID `json:"returnId"`
	SenderUserID uuid.UUID `json:"senderUserId"`
	SenderRole   string    `json:"senderRole"`
	MessageType  string    `json:"messageType"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ReturnMessageAttachment struct {
	ID               uuid.UUID `json:"id"`
	MessageID        uuid.UUID `json:"messageId"`
	StorageKey       string    `json:"storageKey"`
	ContentType      string    `json:"contentType"`
	SizeBytes        int64     `json:"sizeBytes"`
	OriginalFilename *string   `json:"originalFilename"`
	SortOrder        int       `json:"sortOrder"`
	CreatedAt        time.Time `json:"createdAt"`
}

type ReturnStagedMessageAttachment struct {
	ID               uuid.UUID `json:"id"`
	ReturnID         uuid.UUID `json:"returnId"`
	UploaderUserID   uuid.UUID `json:"uploaderUserId"`
	StorageKey       string    `json:"storageKey"`
	ContentType      string    `json:"contentType"`
	SizeBytes        int64     `json:"sizeBytes"`
	OriginalFilename *string   `json:"originalFilename"`
	CreatedAt        time.Time `json:"createdAt"`
}
