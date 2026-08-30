package returns

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

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) GetReturnMessages(ctx context.Context, returnID uuid.UUID) ([]ReturnMessageResponse, error) {
	_, _, err := s.repo.GetReturn(ctx, returnID)
	if err != nil {
		return nil, err
	}
	msgs, err := s.repo.GetReturnMessages(ctx, returnID)
	if err != nil {
		return nil, err
	}
	for i := range msgs {
		for j := range msgs[i].Attachments {
			storageKey := msgs[i].Attachments[j].URL
			msgs[i].Attachments[j].URL = s.storageProvider.BuildPublicURL(storageKey)
		}
	}
	return msgs, nil
}

func isTerminalReturnStatus(status string) bool {
	switch status {
	case "rejected", "refunded", "completed", "cancelled":
		return true
	default:
		return false
	}
}

func isActiveReturnStatus(status string) bool {
	switch status {
	case "requested", "needs_info", "approved", "receiving", "item_received":
		return true
	default:
		return false
	}
}

func (s *Service) SendAdminReturnMessage(ctx context.Context, returnID, adminID uuid.UUID, body string, needsResponse bool, attachmentIDs []uuid.UUID) error {
	if len(attachmentIDs) > 6 {
		return ErrReturnMessageTooManyAttachments
	}

	seenAtts := make(map[uuid.UUID]bool, len(attachmentIDs))
	for _, id := range attachmentIDs {
		if seenAtts[id] {
			return ErrReturnMessageAttachmentInvalid
		}
		seenAtts[id] = true
	}

	trimmed := strings.TrimSpace(body)
	if trimmed == "" && len(attachmentIDs) == 0 {
		return ErrReturnMessageRequired
	}

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		ret, err := s.repo.GetReturnTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		if needsResponse {
			if ret.Status != "requested" {
				return ErrReturnNotRequestableInfo
			}
		} else {
			if isTerminalReturnStatus(ret.Status) || !isActiveReturnStatus(ret.Status) {
				return ErrReturnTerminalState
			}
		}

		var orderedAtts []ReturnStagedMessageAttachment
		if len(attachmentIDs) > 0 {
			stagedAtts, err := s.repo.GetStagedMessageAttachmentsTx(ctx, tx, returnID, adminID, attachmentIDs)
			if err != nil {
				return err
			}
			if len(stagedAtts) != len(attachmentIDs) {
				return ErrReturnMessageAttachmentNotOwned
			}

			stagedMap := make(map[uuid.UUID]ReturnStagedMessageAttachment, len(stagedAtts))
			for _, att := range stagedAtts {
				stagedMap[att.ID] = att
			}

			orderedAtts = make([]ReturnStagedMessageAttachment, 0, len(attachmentIDs))
			for _, id := range attachmentIDs {
				att, ok := stagedMap[id]
				if !ok {
					return ErrReturnMessageAttachmentNotOwned
				}
				orderedAtts = append(orderedAtts, att)
			}
		}

		msgType := ReturnMessageTypeMessage
		if needsResponse {
			msgType = ReturnMessageTypeInfoRequest
		}

		msg := &ReturnMessage{
			ID:           uuid.New(),
			ReturnID:     returnID,
			SenderUserID: adminID,
			SenderRole:   ReturnMessageSenderRoleAdmin,
			MessageType:  msgType,
			Body:         trimmed,
			CreatedAt:    time.Now(),
		}

		if err := s.repo.CreateReturnMessageTx(ctx, tx, msg); err != nil {
			return err
		}

		if len(orderedAtts) > 0 {
			if err := s.repo.BindMessageAttachmentsTx(ctx, tx, msg.ID, orderedAtts); err != nil {
				return err
			}
		}

		if needsResponse {
			ret.Status = "needs_info"
			ret.UpdatedAt = time.Now()
			if err := s.repo.UpdateReturnTx(ctx, tx, ret); err != nil {
				return err
			}

			if s.notifs != nil {
				var orderNumber *string
				_ = tx.QueryRow(ctx, "SELECT o.order_number FROM orders o WHERE o.id = $1", ret.OrderID).Scan(&orderNumber)

				var bodyText string
				if orderNumber != nil && *orderNumber != "" {
					bodyText = fmt.Sprintf("По возврату %s требуется уточнение.", *orderNumber)
				} else {
					bodyText = "По вашей заявке на возврат требуется уточнение."
				}

				notif := notifications.Notification{
					RecipientKind:   notifications.RecipientKindCustomer,
					RecipientUserID: &ret.UserID,
					Type:            notifications.TypeReturnNeedsInfo,
					Title:           "Требуется уточнение по возврату",
					Body:            bodyText,
					EntityType:      "return",
					EntityID:        ret.ID,
				}
				if err := s.notifs.CreateNotificationTx(ctx, tx, notif); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func (s *Service) SendCustomerReturnMessage(ctx context.Context, returnID, customerID uuid.UUID, body string, attachmentIDs []uuid.UUID) error {
	if len(attachmentIDs) > 6 {
		return ErrReturnMessageTooManyAttachments
	}

	seenAtts := make(map[uuid.UUID]bool, len(attachmentIDs))
	for _, id := range attachmentIDs {
		if seenAtts[id] {
			return ErrReturnMessageAttachmentInvalid
		}
		seenAtts[id] = true
	}

	trimmed := strings.TrimSpace(body)
	if trimmed == "" && len(attachmentIDs) == 0 {
		return ErrReturnMessageRequired
	}

	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		ret, err := s.repo.GetReturnTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		if ret.UserID != customerID {
			return ErrReturnNotFound
		}

		if isTerminalReturnStatus(ret.Status) || !isActiveReturnStatus(ret.Status) {
			return ErrReturnTerminalState
		}

		var orderedAtts []ReturnStagedMessageAttachment
		if len(attachmentIDs) > 0 {
			stagedAtts, err := s.repo.GetStagedMessageAttachmentsTx(ctx, tx, returnID, customerID, attachmentIDs)
			if err != nil {
				return err
			}
			if len(stagedAtts) != len(attachmentIDs) {
				return ErrReturnMessageAttachmentNotOwned
			}

			stagedMap := make(map[uuid.UUID]ReturnStagedMessageAttachment, len(stagedAtts))
			for _, att := range stagedAtts {
				stagedMap[att.ID] = att
			}

			orderedAtts = make([]ReturnStagedMessageAttachment, 0, len(attachmentIDs))
			for _, id := range attachmentIDs {
				att, ok := stagedMap[id]
				if !ok {
					return ErrReturnMessageAttachmentNotOwned
				}
				orderedAtts = append(orderedAtts, att)
			}
		}

		msg := &ReturnMessage{
			ID:           uuid.New(),
			ReturnID:     returnID,
			SenderUserID: customerID,
			SenderRole:   ReturnMessageSenderRoleCustomer,
			MessageType:  ReturnMessageTypeMessage,
			Body:         trimmed,
			CreatedAt:    time.Now(),
		}

		if err := s.repo.CreateReturnMessageTx(ctx, tx, msg); err != nil {
			return err
		}

		if len(orderedAtts) > 0 {
			if err := s.repo.BindMessageAttachmentsTx(ctx, tx, msg.ID, orderedAtts); err != nil {
				return err
			}
		}

		if ret.Status == "needs_info" {
			ret.Status = "requested"
			ret.UpdatedAt = time.Now()
			if err := s.repo.UpdateReturnTx(ctx, tx, ret); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Service) uploadMessageAttachment(ctx context.Context, returnID, uploaderID uuid.UUID, file io.Reader, filename, contentType string, sizeBytes int64) (UploadReturnMessageAttachmentResponse, error) {
	if s.storageProvider == nil {
		return UploadReturnMessageAttachmentResponse{}, errors.New("storage provider not configured")
	}

	if sizeBytes <= 0 || sizeBytes > 10*1024*1024 {
		return UploadReturnMessageAttachmentResponse{}, ErrReturnMessageAttachmentTooLarge
	}

	ext := strings.ToLower(filepath.Ext(filename))
	validExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
	}
	if !validExts[ext] {
		return UploadReturnMessageAttachmentResponse{}, ErrReturnMessageAttachmentInvalid
	}

	validMimes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}
	if contentType != "" && !validMimes[contentType] {
		return UploadReturnMessageAttachmentResponse{}, ErrReturnMessageAttachmentInvalid
	}

	// Sniff the actual file content (first 512 bytes)
	headerBytes := make([]byte, 512)
	n, err := io.ReadFull(file, headerBytes)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return UploadReturnMessageAttachmentResponse{}, ErrReturnMessageAttachmentInvalid
	}
	headerBytes = headerBytes[:n]
	detectedType := http.DetectContentType(headerBytes)

	if !validMimes[detectedType] {
		return UploadReturnMessageAttachmentResponse{}, ErrReturnMessageAttachmentInvalid
	}

	fullReader := io.MultiReader(bytes.NewReader(headerBytes), file)

	id := uuid.New()
	storageKey := fmt.Sprintf("returns/%s/messages/%s%s", returnID.String(), id.String(), ext)

	if _, err := s.storageProvider.UploadImage(ctx, fullReader, sizeBytes, storageKey, detectedType); err != nil {
		return UploadReturnMessageAttachmentResponse{}, err
	}

	var originalName *string
	if filename != "" {
		originalName = &filename
	}

	att := &ReturnStagedMessageAttachment{
		ID:               id,
		ReturnID:         returnID,
		UploaderUserID:   uploaderID,
		StorageKey:       storageKey,
		ContentType:      detectedType,
		SizeBytes:        sizeBytes,
		OriginalFilename: originalName,
		CreatedAt:        time.Now(),
	}

	if err := s.repo.CreateStagedMessageAttachment(ctx, att); err != nil {
		_ = s.storageProvider.DeleteObject(ctx, storageKey)
		return UploadReturnMessageAttachmentResponse{}, err
	}

	return UploadReturnMessageAttachmentResponse{
		ID:  id,
		URL: s.storageProvider.BuildPublicURL(storageKey),
	}, nil
}

func (s *Service) UploadAdminReturnMessageAttachment(ctx context.Context, returnID, adminID uuid.UUID, file io.Reader, filename, contentType string, sizeBytes int64) (UploadReturnMessageAttachmentResponse, error) {
	ret, _, err := s.repo.GetReturn(ctx, returnID)
	if err != nil {
		return UploadReturnMessageAttachmentResponse{}, err
	}
	if isTerminalReturnStatus(ret.Status) || !isActiveReturnStatus(ret.Status) {
		return UploadReturnMessageAttachmentResponse{}, ErrReturnTerminalState
	}
	return s.uploadMessageAttachment(ctx, returnID, adminID, file, filename, contentType, sizeBytes)
}

func (s *Service) UploadCustomerReturnMessageAttachment(ctx context.Context, returnID, customerID uuid.UUID, file io.Reader, filename, contentType string, sizeBytes int64) (UploadReturnMessageAttachmentResponse, error) {
	ret, _, err := s.repo.GetReturn(ctx, returnID)
	if err != nil {
		return UploadReturnMessageAttachmentResponse{}, err
	}
	if ret.UserID != customerID {
		return UploadReturnMessageAttachmentResponse{}, ErrReturnNotFound
	}
	if isTerminalReturnStatus(ret.Status) || !isActiveReturnStatus(ret.Status) {
		return UploadReturnMessageAttachmentResponse{}, ErrReturnTerminalState
	}
	return s.uploadMessageAttachment(ctx, returnID, customerID, file, filename, contentType, sizeBytes)
}
