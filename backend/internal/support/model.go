package support

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrConversationNotFound       = errors.New("support conversation not found")
	ErrSessionNotFound            = errors.New("support session not found")
	ErrNoActiveSession            = errors.New("no active support session")
	ErrActiveSessionAlreadyExists = errors.New("active support session already exists")
	ErrUnauthorized               = errors.New("unauthorized support operation")
	ErrForbidden                  = errors.New("forbidden support operation")
	ErrInvalidSender              = errors.New("invalid support message sender")
	ErrEmptyMessage               = errors.New("support message cannot be empty")
	ErrInvalidPriority            = errors.New("invalid session priority")
	ErrCategoryNotFound           = errors.New("support category not found")
	ErrInvalidCategoryScope       = errors.New("category scope does not match conversation requester type or category is inactive")
	ErrInvalidAssignee            = errors.New("assigned user must be an active staff member")
	ErrInvalidContext             = errors.New("invalid or foreign context link")
	ErrAttachmentInvalid          = errors.New("invalid support attachment")
	ErrAttachmentTooLarge         = errors.New("support attachment exceeds max size limit")
	ErrAttachmentCountExceeded    = errors.New("support attachment count limit exceeded")
	ErrAttachmentNotFound         = errors.New("support attachment not found")
)

type RequesterType string

const (
	RequesterTypeCustomer RequesterType = "CUSTOMER"
	RequesterTypeSeller   RequesterType = "SELLER"
)

type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "ACTIVE"
	SessionStatusCompleted SessionStatus = "COMPLETED"
)

type SessionPriority string

const (
	SessionPriorityNormal SessionPriority = "NORMAL"
	SessionPriorityHigh   SessionPriority = "HIGH"
	SessionPriorityUrgent SessionPriority = "URGENT"
)

type SenderType string

const (
	SenderTypeCustomer SenderType = "CUSTOMER"
	SenderTypeSeller   SenderType = "SELLER"
	SenderTypeStaff    SenderType = "STAFF"
)

type ContextType string

const (
	ContextTypeOrder   ContextType = "ORDER"
	ContextTypeReturn  ContextType = "RETURN"
	ContextTypeProduct ContextType = "PRODUCT"
)

type RequesterScope string

const (
	RequesterScopeCustomer RequesterScope = "CUSTOMER"
	RequesterScopeSeller   RequesterScope = "SELLER"
	RequesterScopeBoth     RequesterScope = "BOTH"
)

type Category struct {
	ID             uuid.UUID      `json:"id"`
	Name           string         `json:"name"`
	RequesterScope RequesterScope `json:"requesterScope"`
	Active         bool           `json:"active"`
	SortOrder      int            `json:"sortOrder"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type Conversation struct {
	ID                uuid.UUID     `json:"id"`
	RequesterType     RequesterType `json:"requesterType"`
	RequesterUserID   *uuid.UUID    `json:"requesterUserId,omitempty"`
	RequesterSellerID *uuid.UUID    `json:"requesterSellerId,omitempty"`
	CreatedAt         time.Time     `json:"createdAt"`
	UpdatedAt         time.Time     `json:"updatedAt"`

	// Derived / computed properties
	ActiveSession      *Session   `json:"activeSession,omitempty"`
	UnreadCount        int        `json:"unreadCount"`
	RequesterName      string     `json:"requesterName,omitempty"`
	RequesterEmail     string     `json:"requesterEmail,omitempty"`
	RequesterStoreName string     `json:"requesterStoreName,omitempty"`
	LatestMessageText  string     `json:"latestMessageText,omitempty"`
	LatestMessageAt    *time.Time `json:"latestMessageAt,omitempty"`
}

type Session struct {
	ID             uuid.UUID       `json:"id"`
	ConversationID uuid.UUID       `json:"conversationId"`
	Status         SessionStatus   `json:"status"`
	Priority       SessionPriority `json:"priority,omitempty"`
	CategoryID     *uuid.UUID      `json:"categoryId,omitempty"`
	CategoryName   *string         `json:"categoryName,omitempty"`
	AssignedTo     *uuid.UUID      `json:"assignedTo,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	CompletedAt    *time.Time      `json:"completedAt,omitempty"`
}

type Message struct {
	ID           uuid.UUID     `json:"id"`
	SessionID    uuid.UUID     `json:"sessionId"`
	SenderType   SenderType    `json:"senderType"`
	SenderUserID uuid.UUID     `json:"senderUserId"`
	TextContent  string        `json:"textContent"`
	CreatedAt    time.Time     `json:"createdAt"`
	Attachments  []Attachment  `json:"attachments"`
	ContextLinks []ContextLink `json:"contextLinks"`
}

type InternalNote struct {
	ID           uuid.UUID `json:"id"`
	SessionID    uuid.UUID `json:"sessionId"`
	AuthorUserID uuid.UUID `json:"authorUserId"`
	AuthorName   string    `json:"authorName,omitempty"`
	TextContent  string    `json:"textContent"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ContextLink struct {
	ID          uuid.UUID   `json:"id"`
	MessageID   uuid.UUID   `json:"messageId"`
	ContextType ContextType `json:"contextType"`
	ContextID   uuid.UUID   `json:"contextId"`
	CreatedAt   time.Time   `json:"createdAt"`
	Label       string      `json:"label,omitempty"`
}

type StagedAttachment struct {
	ID               uuid.UUID `json:"id"`
	ConversationID   uuid.UUID `json:"conversationId"`
	UploaderUserID   uuid.UUID `json:"uploaderUserId"`
	StorageKey       string    `json:"storageKey"`
	ContentType      string    `json:"contentType"`
	SizeBytes        int64     `json:"sizeBytes"`
	OriginalFilename *string   `json:"originalFilename,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Attachment struct {
	ID               uuid.UUID `json:"id"`
	MessageID        uuid.UUID `json:"messageId"`
	StorageKey       string    `json:"storageKey"`
	ContentType      string    `json:"contentType"`
	SizeBytes        int64     `json:"sizeBytes"`
	OriginalFilename *string   `json:"originalFilename,omitempty"`
	SortOrder        int       `json:"sortOrder"`
	CreatedAt        time.Time `json:"createdAt"`
}

type StaffRead struct {
	StaffUserID    uuid.UUID `json:"staffUserId"`
	ConversationID uuid.UUID `json:"conversationId"`
	LastReadAt     time.Time `json:"lastReadAt"`
}

type RequesterRead struct {
	UserID         uuid.UUID `json:"userId"`
	ConversationID uuid.UUID `json:"conversationId"`
	LastReadAt     time.Time `json:"lastReadAt"`
}

// Request and response DTOs

type SendMessageRequest struct {
	TextContent   string                 `json:"textContent"`
	AttachmentIDs []uuid.UUID            `json:"attachmentIds,omitempty"`
	ContextLinks  []CreateContextLinkReq `json:"contextLinks,omitempty"`
}

type CreateContextLinkReq struct {
	ContextType ContextType `json:"contextType"`
	ContextID   uuid.UUID   `json:"contextId"`
}

type StaffReplyRequest struct {
	TextContent   string      `json:"textContent"`
	AttachmentIDs []uuid.UUID `json:"attachmentIds,omitempty"`
}

type CreateInternalNoteRequest struct {
	TextContent string `json:"textContent"`
}

type UpdateSessionRequest struct {
	Priority      *SessionPriority `json:"priority,omitempty"`
	CategoryID    *uuid.UUID       `json:"categoryId,omitempty"`
	ClearCategory bool             `json:"clearCategory,omitempty"`
	AssignedTo    *uuid.UUID       `json:"assignedTo,omitempty"`
	ClearAssignee bool             `json:"clearAssignee,omitempty"`
}

type ConversationDetailResponse struct {
	Conversation  Conversation   `json:"conversation"`
	Messages      []Message      `json:"messages"`
	InternalNotes []InternalNote `json:"internalNotes,omitempty"` // populated ONLY for staff
}
