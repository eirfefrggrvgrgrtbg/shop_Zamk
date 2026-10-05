package support

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
)

type Service struct {
	repo            *Repository
	db              *pgxpool.Pool
	storageProvider storage.Provider
}

func NewService(repo *Repository, db *pgxpool.Pool, storageProvider storage.Provider) *Service {
	return &Service{
		repo:            repo,
		db:              db,
		storageProvider: storageProvider,
	}
}

// Category methods

func (s *Service) ListCategories(ctx context.Context, scope RequesterScope) ([]Category, error) {
	if scope == "" {
		scope = RequesterScopeBoth
	}
	return s.repo.ListCategories(ctx, scope)
}

// Customer Methods

func (s *Service) GetCustomerConversation(ctx context.Context, customerUserID uuid.UUID) (*ConversationDetailResponse, error) {
	conv, err := s.repo.GetCustomerConversation(ctx, customerUserID)
	if err != nil {
		if errors.Is(err, ErrConversationNotFound) {
			// Auto create conversation row without active session
			tx, tErr := s.db.Begin(ctx)
			if tErr != nil {
				return nil, tErr
			}
			defer tx.Rollback(ctx)

			conv, err = s.repo.GetOrCreateCustomerConversationTx(ctx, tx, customerUserID)
			if err != nil {
				return nil, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	activeSess, _ := s.repo.GetActiveSession(ctx, conv.ID)
	if activeSess != nil {
		// Strictly omit operational metadata from customer
		activeSess.Priority = ""
		activeSess.CategoryID = nil
		activeSess.CategoryName = nil
		activeSess.AssignedTo = nil
	}
	conv.ActiveSession = activeSess

	unread, _ := s.repo.GetRequesterUnreadCount(ctx, customerUserID, conv.ID)
	conv.UnreadCount = unread

	msgs, err := s.repo.ListMessagesByConversation(ctx, conv.ID)
	if err != nil {
		return nil, err
	}

	return &ConversationDetailResponse{
		Conversation: *conv,
		Messages:     msgs,
		// InternalNotes strictly omitted (nil)
	}, nil
}

func (s *Service) SendCustomerMessage(ctx context.Context, customerUserID uuid.UUID, req SendMessageRequest) (*Message, error) {
	if strings.TrimSpace(req.TextContent) == "" {
		return nil, ErrEmptyMessage
	}
	if len(req.AttachmentIDs) > 5 {
		return nil, ErrAttachmentCountExceeded
	}

	// Validate context links
	for _, l := range req.ContextLinks {
		switch l.ContextType {
		case ContextTypeOrder:
			ok, err := s.repo.ValidateCustomerOrder(ctx, customerUserID, l.ContextID)
			if err != nil || !ok {
				return nil, ErrInvalidContext
			}
		case ContextTypeReturn:
			ok, err := s.repo.ValidateCustomerReturn(ctx, customerUserID, l.ContextID)
			if err != nil || !ok {
				return nil, ErrInvalidContext
			}
		case ContextTypeProduct:
			ok, err := s.repo.ValidateCustomerProduct(ctx, l.ContextID)
			if err != nil || !ok {
				return nil, ErrInvalidContext
			}
		default:
			return nil, ErrInvalidContext
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	conv, err := s.repo.GetOrCreateCustomerConversationTx(ctx, tx, customerUserID)
	if err != nil {
		return nil, err
	}

	// Explicitly lock conversation row to serialize against concurrent read marking
	var lockedID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM support_conversations WHERE id = $1 FOR UPDATE`, conv.ID).Scan(&lockedID); err != nil {
		return nil, err
	}

	sess, err := s.repo.GetActiveSessionTx(ctx, tx, conv.ID)
	if err != nil {
		if errors.Is(err, ErrNoActiveSession) {
			sess, err = s.repo.CreateSessionTx(ctx, tx, conv.ID, SessionPriorityNormal, nil, nil)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	var stagedAtts []StagedAttachment
	if len(req.AttachmentIDs) > 0 {
		stagedAtts, err = s.repo.GetStagedAttachmentsTx(ctx, tx, conv.ID, customerUserID, req.AttachmentIDs)
		if err != nil {
			return nil, err
		}
		if len(stagedAtts) != len(req.AttachmentIDs) {
			return nil, ErrAttachmentInvalid
		}
	}

	msg, err := s.repo.CreateMessageTx(ctx, tx, sess.ID, SenderTypeCustomer, customerUserID, req.TextContent)
	if err != nil {
		return nil, err
	}

	if len(stagedAtts) > 0 {
		bound, err := s.repo.BindAttachmentsTx(ctx, tx, msg.ID, stagedAtts)
		if err != nil {
			return nil, err
		}
		msg.Attachments = bound
	}

	if len(req.ContextLinks) > 0 {
		cls, err := s.repo.CreateContextLinksTx(ctx, tx, msg.ID, req.ContextLinks)
		if err != nil {
			return nil, err
		}
		msg.ContextLinks = cls
	}

	if err := s.repo.MarkRequesterReadTx(ctx, tx, customerUserID, conv.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *Service) UploadCustomerAttachment(ctx context.Context, customerUserID uuid.UUID, file io.Reader, filename, contentType string, sizeBytes int64) (*StagedAttachment, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Ensure conversation exists without creating any session
	conv, err := s.repo.GetOrCreateCustomerConversationTx(ctx, tx, customerUserID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.UploadStagedAttachment(ctx, conv.ID, customerUserID, file, filename, contentType, sizeBytes)
}

// Seller Methods

func (s *Service) GetSellerConversation(ctx context.Context, sellerID, memberUserID uuid.UUID) (*ConversationDetailResponse, error) {
	isMember, err := s.repo.ValidateSellerMember(ctx, sellerID, memberUserID)
	if err != nil || !isMember {
		return nil, ErrForbidden
	}

	conv, err := s.repo.GetSellerConversation(ctx, sellerID)
	if err != nil {
		if errors.Is(err, ErrConversationNotFound) {
			tx, tErr := s.db.Begin(ctx)
			if tErr != nil {
				return nil, tErr
			}
			defer tx.Rollback(ctx)

			conv, err = s.repo.GetOrCreateSellerConversationTx(ctx, tx, sellerID)
			if err != nil {
				return nil, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	activeSess, _ := s.repo.GetActiveSession(ctx, conv.ID)
	if activeSess != nil {
		// Strictly omit operational metadata from seller
		activeSess.Priority = ""
		activeSess.CategoryID = nil
		activeSess.CategoryName = nil
		activeSess.AssignedTo = nil
	}
	conv.ActiveSession = activeSess

	unread, _ := s.repo.GetRequesterUnreadCount(ctx, memberUserID, conv.ID)
	conv.UnreadCount = unread

	msgs, err := s.repo.ListMessagesByConversation(ctx, conv.ID)
	if err != nil {
		return nil, err
	}

	return &ConversationDetailResponse{
		Conversation: *conv,
		Messages:     msgs,
		// InternalNotes strictly omitted (nil)
	}, nil
}

func (s *Service) SendSellerMessage(ctx context.Context, sellerID, memberUserID uuid.UUID, req SendMessageRequest) (*Message, error) {
	isMember, err := s.repo.ValidateSellerMember(ctx, sellerID, memberUserID)
	if err != nil || !isMember {
		return nil, ErrForbidden
	}

	if strings.TrimSpace(req.TextContent) == "" {
		return nil, ErrEmptyMessage
	}
	if len(req.AttachmentIDs) > 5 {
		return nil, ErrAttachmentCountExceeded
	}

	// Validate context links
	for _, l := range req.ContextLinks {
		switch l.ContextType {
		case ContextTypeOrder:
			ok, err := s.repo.ValidateSellerOrder(ctx, sellerID, l.ContextID)
			if err != nil || !ok {
				return nil, ErrInvalidContext
			}
		case ContextTypeReturn:
			ok, err := s.repo.ValidateSellerReturn(ctx, sellerID, l.ContextID)
			if err != nil || !ok {
				return nil, ErrInvalidContext
			}
		case ContextTypeProduct:
			ok, err := s.repo.ValidateSellerProduct(ctx, sellerID, l.ContextID)
			if err != nil || !ok {
				return nil, ErrInvalidContext
			}
		default:
			return nil, ErrInvalidContext
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	conv, err := s.repo.GetOrCreateSellerConversationTx(ctx, tx, sellerID)
	if err != nil {
		return nil, err
	}

	// Explicitly lock conversation row to serialize against concurrent read marking
	var lockedID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM support_conversations WHERE id = $1 FOR UPDATE`, conv.ID).Scan(&lockedID); err != nil {
		return nil, err
	}

	sess, err := s.repo.GetActiveSessionTx(ctx, tx, conv.ID)
	if err != nil {
		if errors.Is(err, ErrNoActiveSession) {
			sess, err = s.repo.CreateSessionTx(ctx, tx, conv.ID, SessionPriorityNormal, nil, nil)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	var stagedAtts []StagedAttachment
	if len(req.AttachmentIDs) > 0 {
		stagedAtts, err = s.repo.GetStagedAttachmentsTx(ctx, tx, conv.ID, memberUserID, req.AttachmentIDs)
		if err != nil {
			return nil, err
		}
		if len(stagedAtts) != len(req.AttachmentIDs) {
			return nil, ErrAttachmentInvalid
		}
	}

	msg, err := s.repo.CreateMessageTx(ctx, tx, sess.ID, SenderTypeSeller, memberUserID, req.TextContent)
	if err != nil {
		return nil, err
	}

	if len(stagedAtts) > 0 {
		bound, err := s.repo.BindAttachmentsTx(ctx, tx, msg.ID, stagedAtts)
		if err != nil {
			return nil, err
		}
		msg.Attachments = bound
	}

	if len(req.ContextLinks) > 0 {
		cls, err := s.repo.CreateContextLinksTx(ctx, tx, msg.ID, req.ContextLinks)
		if err != nil {
			return nil, err
		}
		msg.ContextLinks = cls
	}

	if err := s.repo.MarkRequesterReadTx(ctx, tx, memberUserID, conv.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *Service) UploadSellerAttachment(ctx context.Context, sellerID, memberUserID uuid.UUID, file io.Reader, filename, contentType string, sizeBytes int64) (*StagedAttachment, error) {
	isMember, err := s.repo.ValidateSellerMember(ctx, sellerID, memberUserID)
	if err != nil || !isMember {
		return nil, ErrForbidden
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Ensure conversation exists without creating any session
	conv, err := s.repo.GetOrCreateSellerConversationTx(ctx, tx, sellerID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.UploadStagedAttachment(ctx, conv.ID, memberUserID, file, filename, contentType, sizeBytes)
}

// Staff / Admin Methods

func (s *Service) ListConversations(ctx context.Context, filter RequesterType, search string, staffUserID uuid.UUID) ([]Conversation, error) {
	return s.repo.ListConversations(ctx, filter, search, staffUserID)
}

func (s *Service) GetAdminConversation(ctx context.Context, conversationID, staffUserID uuid.UUID) (*ConversationDetailResponse, error) {
	conv, err := s.repo.GetConversationByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	activeSess, _ := s.repo.GetActiveSession(ctx, conv.ID)
	conv.ActiveSession = activeSess

	unread, _ := s.repo.GetStaffUnreadCount(ctx, staffUserID, conv.ID)
	conv.UnreadCount = unread

	msgs, err := s.repo.ListMessagesByConversation(ctx, conv.ID)
	if err != nil {
		return nil, err
	}

	notes, err := s.repo.ListInternalNotesByConversation(ctx, conv.ID)
	if err != nil {
		return nil, err
	}

	return &ConversationDetailResponse{
		Conversation:  *conv,
		Messages:      msgs,
		InternalNotes: notes,
	}, nil
}

func (s *Service) SendStaffReply(ctx context.Context, conversationID, staffUserID uuid.UUID, req StaffReplyRequest) (*Message, error) {
	if strings.TrimSpace(req.TextContent) == "" {
		return nil, ErrEmptyMessage
	}
	if len(req.AttachmentIDs) > 5 {
		return nil, ErrAttachmentCountExceeded
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Explicitly lock conversation row to serialize against concurrent read marking
	var lockedID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM support_conversations WHERE id = $1 FOR UPDATE`, conversationID).Scan(&lockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}

	sess, err := s.repo.GetActiveSessionTx(ctx, tx, conversationID)
	if err != nil {
		if errors.Is(err, ErrNoActiveSession) {
			sess, err = s.repo.CreateSessionTx(ctx, tx, conversationID, SessionPriorityNormal, nil, nil)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	var stagedAtts []StagedAttachment
	if len(req.AttachmentIDs) > 0 {
		stagedAtts, err = s.repo.GetStagedAttachmentsTx(ctx, tx, conversationID, staffUserID, req.AttachmentIDs)
		if err != nil {
			return nil, err
		}
		if len(stagedAtts) != len(req.AttachmentIDs) {
			return nil, ErrAttachmentInvalid
		}
	}

	msg, err := s.repo.CreateMessageTx(ctx, tx, sess.ID, SenderTypeStaff, staffUserID, req.TextContent)
	if err != nil {
		return nil, err
	}

	if len(stagedAtts) > 0 {
		bound, err := s.repo.BindAttachmentsTx(ctx, tx, msg.ID, stagedAtts)
		if err != nil {
			return nil, err
		}
		msg.Attachments = bound
	}

	if err := s.repo.MarkStaffReadTx(ctx, tx, staffUserID, conversationID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *Service) CreateInternalNote(ctx context.Context, conversationID, staffUserID uuid.UUID, req CreateInternalNoteRequest) (*InternalNote, error) {
	if strings.TrimSpace(req.TextContent) == "" {
		return nil, ErrEmptyMessage
	}

	isStaff, err := s.repo.ValidateStaffUser(ctx, staffUserID)
	if err != nil || !isStaff {
		return nil, ErrForbidden
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	sess, err := s.repo.GetActiveSessionTx(ctx, tx, conversationID)
	if err != nil {
		if errors.Is(err, ErrNoActiveSession) {
			sess, err = s.repo.CreateSessionTx(ctx, tx, conversationID, SessionPriorityNormal, nil, nil)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	note, err := s.repo.CreateInternalNoteTx(ctx, tx, sess.ID, staffUserID, req.TextContent)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return note, nil
}

func (s *Service) CompleteSession(ctx context.Context, conversationID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	sess, err := s.repo.GetActiveSessionTx(ctx, tx, conversationID)
	if err != nil {
		return err
	}

	if err := s.repo.CompleteSessionTx(ctx, tx, sess.ID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) ReopenSession(ctx context.Context, sessionID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.ReopenSessionTx(ctx, tx, sessionID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) UpdateSession(ctx context.Context, conversationID uuid.UUID, req UpdateSessionRequest) error {
	if req.Priority != nil {
		switch *req.Priority {
		case SessionPriorityNormal, SessionPriorityHigh, SessionPriorityUrgent:
		default:
			return ErrInvalidPriority
		}
	}

	conv, err := s.repo.GetConversationByID(ctx, conversationID)
	if err != nil {
		return err
	}

	// Validate category assignment if provided
	if req.CategoryID != nil && *req.CategoryID != uuid.Nil {
		cat, err := s.repo.GetCategory(ctx, *req.CategoryID)
		if err != nil {
			return ErrCategoryNotFound
		}
		if !cat.Active {
			return ErrInvalidCategoryScope
		}
		if conv.RequesterType == RequesterTypeCustomer {
			if cat.RequesterScope != RequesterScopeCustomer && cat.RequesterScope != RequesterScopeBoth {
				return ErrInvalidCategoryScope
			}
		} else if conv.RequesterType == RequesterTypeSeller {
			if cat.RequesterScope != RequesterScopeSeller && cat.RequesterScope != RequesterScopeBoth {
				return ErrInvalidCategoryScope
			}
		}
	}

	// Validate staff assignment if provided
	if req.AssignedTo != nil && *req.AssignedTo != uuid.Nil {
		isActiveStaff, err := s.repo.ValidateActiveStaff(ctx, *req.AssignedTo)
		if err != nil || !isActiveStaff {
			return ErrInvalidAssignee
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	sess, err := s.repo.GetActiveSessionTx(ctx, tx, conversationID)
	if err != nil {
		return err
	}

	clearCat := req.ClearCategory || (req.CategoryID != nil && *req.CategoryID == uuid.Nil)
	clearAssign := req.ClearAssignee || (req.AssignedTo != nil && *req.AssignedTo == uuid.Nil)

	var targetCat *uuid.UUID
	if req.CategoryID != nil && *req.CategoryID != uuid.Nil {
		targetCat = req.CategoryID
	}

	var targetAssign *uuid.UUID
	if req.AssignedTo != nil && *req.AssignedTo != uuid.Nil {
		targetAssign = req.AssignedTo
	}

	if err := s.repo.UpdateSessionTx(ctx, tx, sess.ID, req.Priority, targetCat, clearCat, targetAssign, clearAssign); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) MarkStaffRead(ctx context.Context, staffUserID, conversationID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var lockedID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM support_conversations WHERE id = $1 FOR UPDATE`, conversationID).Scan(&lockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConversationNotFound
		}
		return err
	}

	if err := s.repo.MarkStaffReadTx(ctx, tx, staffUserID, conversationID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) MarkRequesterRead(ctx context.Context, userID, conversationID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var lockedID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM support_conversations WHERE id = $1 FOR UPDATE`, conversationID).Scan(&lockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConversationNotFound
		}
		return err
	}

	if err := s.repo.MarkRequesterReadTx(ctx, tx, userID, conversationID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Attachment Operations

func (s *Service) UploadStagedAttachment(ctx context.Context, conversationID, uploaderUserID uuid.UUID, file io.Reader, filename, contentType string, sizeBytes int64) (*StagedAttachment, error) {
	if sizeBytes <= 0 || sizeBytes > 10*1024*1024 {
		return nil, ErrAttachmentTooLarge
	}

	ext := strings.ToLower(filepath.Ext(filename))
	validExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
		".pdf":  true,
	}
	if !validExts[ext] {
		return nil, ErrAttachmentInvalid
	}

	validMimes := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"image/webp":      true,
		"application/pdf": true,
	}

	headerBytes := make([]byte, 512)
	n, err := io.ReadFull(file, headerBytes)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, ErrAttachmentInvalid
	}
	headerBytes = headerBytes[:n]
	detectedType := http.DetectContentType(headerBytes)

	if strings.HasPrefix(string(headerBytes), "%PDF") {
		detectedType = "application/pdf"
	}

	if !validMimes[detectedType] {
		return nil, ErrAttachmentInvalid
	}

	fullReader := io.MultiReader(bytes.NewReader(headerBytes), file)

	id := uuid.New()
	storageKey := fmt.Sprintf("support/%s/attachments/%s%s", conversationID.String(), id.String(), ext)

	if s.storageProvider != nil {
		if _, err := s.storageProvider.UploadImage(ctx, fullReader, sizeBytes, storageKey, detectedType); err != nil {
			return nil, err
		}
	}

	var originalName *string
	if filename != "" {
		originalName = &filename
	}

	att := &StagedAttachment{
		ID:               id,
		ConversationID:   conversationID,
		UploaderUserID:   uploaderUserID,
		StorageKey:       storageKey,
		ContentType:      detectedType,
		SizeBytes:        sizeBytes,
		OriginalFilename: originalName,
		CreatedAt:        time.Now(),
	}

	if err := s.repo.CreateStagedAttachment(ctx, att); err != nil {
		if s.storageProvider != nil {
			_ = s.storageProvider.DeleteObject(ctx, storageKey)
		}
		return nil, err
	}

	return att, nil
}

func (s *Service) DownloadAttachment(ctx context.Context, attachmentID, requesterUserID uuid.UUID, isStaff bool, sellerID *uuid.UUID) ([]byte, string, error) {
	att, convID, err := s.repo.GetAttachmentByID(ctx, attachmentID)
	if err != nil {
		return nil, "", err
	}

	// Verify authorization
	if !isStaff {
		conv, err := s.repo.GetConversationByID(ctx, convID)
		if err != nil {
			return nil, "", ErrForbidden
		}

		if conv.RequesterType == RequesterTypeCustomer {
			if conv.RequesterUserID == nil || *conv.RequesterUserID != requesterUserID {
				return nil, "", ErrForbidden
			}
		} else if conv.RequesterType == RequesterTypeSeller {
			if sellerID == nil || conv.RequesterSellerID == nil || *conv.RequesterSellerID != *sellerID {
				return nil, "", ErrForbidden
			}
			isMember, err := s.repo.ValidateSellerMember(ctx, *sellerID, requesterUserID)
			if err != nil || !isMember {
				return nil, "", ErrForbidden
			}
		} else {
			return nil, "", ErrForbidden
		}
	}

	if s.storageProvider == nil {
		return []byte("dummy file content"), att.ContentType, nil
	}

	data, err := s.storageProvider.DownloadObject(ctx, att.StorageKey)
	if err != nil {
		return nil, "", err
	}
	return data, att.ContentType, nil
}
