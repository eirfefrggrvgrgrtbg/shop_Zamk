package support

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Categories

func (r *Repository) ListCategories(ctx context.Context, scope RequesterScope) ([]Category, error) {
	query := `
		SELECT id, name, requester_scope, active, sort_order, created_at, updated_at
		FROM support_categories
		WHERE active = true AND (requester_scope = $1 OR requester_scope = 'BOTH' OR $1 = 'BOTH')
		ORDER BY sort_order ASC, name ASC
	`
	rows, err := r.db.Query(ctx, query, string(scope))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []Category
	for rows.Next() {
		var c Category
		var s string
		if err := rows.Scan(&c.ID, &c.Name, &s, &c.Active, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.RequesterScope = RequesterScope(s)
		cats = append(cats, c)
	}
	return cats, nil
}

func (r *Repository) GetCategory(ctx context.Context, id uuid.UUID) (*Category, error) {
	query := `
		SELECT id, name, requester_scope, active, sort_order, created_at, updated_at
		FROM support_categories
		WHERE id = $1
	`
	var c Category
	var s string
	err := r.db.QueryRow(ctx, query, id).Scan(&c.ID, &c.Name, &s, &c.Active, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCategoryNotFound
		}
		return nil, err
	}
	c.RequesterScope = RequesterScope(s)
	return &c, nil
}

// Conversations

func (r *Repository) GetCustomerConversation(ctx context.Context, customerUserID uuid.UUID) (*Conversation, error) {
	query := `
		SELECT id, requester_type, requester_user_id, requester_seller_id, created_at, updated_at
		FROM support_conversations
		WHERE requester_type = 'CUSTOMER' AND requester_user_id = $1
	`
	var c Conversation
	var reqType string
	err := r.db.QueryRow(ctx, query, customerUserID).Scan(&c.ID, &reqType, &c.RequesterUserID, &c.RequesterSellerID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	c.RequesterType = RequesterType(reqType)
	return &c, nil
}

func (r *Repository) GetSellerConversation(ctx context.Context, sellerID uuid.UUID) (*Conversation, error) {
	query := `
		SELECT id, requester_type, requester_user_id, requester_seller_id, created_at, updated_at
		FROM support_conversations
		WHERE requester_type = 'SELLER' AND requester_seller_id = $1
	`
	var c Conversation
	var reqType string
	err := r.db.QueryRow(ctx, query, sellerID).Scan(&c.ID, &reqType, &c.RequesterUserID, &c.RequesterSellerID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	c.RequesterType = RequesterType(reqType)
	return &c, nil
}

func (r *Repository) GetConversationByID(ctx context.Context, id uuid.UUID) (*Conversation, error) {
	query := `
		SELECT c.id, c.requester_type, c.requester_user_id, c.requester_seller_id, c.created_at, c.updated_at,
		       COALESCE(u.name, ''), COALESCE(u.email, ''), COALESCE(sel.brand_name, '')
		FROM support_conversations c
		LEFT JOIN users u ON u.id = c.requester_user_id
		LEFT JOIN sellers sel ON sel.id = c.requester_seller_id
		WHERE c.id = $1
	`
	var c Conversation
	var reqType string
	err := r.db.QueryRow(ctx, query, id).Scan(&c.ID, &reqType, &c.RequesterUserID, &c.RequesterSellerID, &c.CreatedAt, &c.UpdatedAt, &c.RequesterName, &c.RequesterEmail, &c.RequesterStoreName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	c.RequesterType = RequesterType(reqType)
	return &c, nil
}

func (r *Repository) GetOrCreateCustomerConversationTx(ctx context.Context, tx pgx.Tx, customerUserID uuid.UUID) (*Conversation, error) {
	query := `
		INSERT INTO support_conversations (requester_type, requester_user_id)
		VALUES ('CUSTOMER', $1)
		ON CONFLICT (requester_user_id) WHERE requester_type = 'CUSTOMER'
		DO UPDATE SET updated_at = now()
		RETURNING id, requester_type, requester_user_id, requester_seller_id, created_at, updated_at
	`
	var c Conversation
	var reqType string
	err := tx.QueryRow(ctx, query, customerUserID).Scan(&c.ID, &reqType, &c.RequesterUserID, &c.RequesterSellerID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.RequesterType = RequesterType(reqType)
	return &c, nil
}

func (r *Repository) GetOrCreateSellerConversationTx(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID) (*Conversation, error) {
	query := `
		INSERT INTO support_conversations (requester_type, requester_seller_id)
		VALUES ('SELLER', $1)
		ON CONFLICT (requester_seller_id) WHERE requester_type = 'SELLER'
		DO UPDATE SET updated_at = now()
		RETURNING id, requester_type, requester_user_id, requester_seller_id, created_at, updated_at
	`
	var c Conversation
	var reqType string
	err := tx.QueryRow(ctx, query, sellerID).Scan(&c.ID, &reqType, &c.RequesterUserID, &c.RequesterSellerID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.RequesterType = RequesterType(reqType)
	return &c, nil
}

func (r *Repository) ListConversations(ctx context.Context, filter RequesterType, search string, staffUserID uuid.UUID) ([]Conversation, error) {
	// Only list conversations that have an ACTIVE session (active support inbox).
	// Abandoned uploads with 0 sessions do not appear.
	query := `
		SELECT c.id, c.requester_type, c.requester_user_id, c.requester_seller_id, c.created_at, c.updated_at,
		       s.id, s.status, s.priority, s.category_id, s.assigned_to, s.created_at, s.updated_at,
		       (
		           SELECT COUNT(*)
		           FROM support_messages m
		           JOIN support_sessions ms ON ms.id = m.session_id
		           LEFT JOIN support_staff_reads r ON r.conversation_id = c.id AND r.staff_user_id = $2
		           WHERE ms.conversation_id = c.id
		             AND m.sender_type != 'STAFF'
		             AND m.created_at > COALESCE(r.last_read_at, '1970-01-01'::timestamptz)
		       ) AS unread_count,
		       COALESCE(u.name, '') AS requester_name,
		       COALESCE(u.email, '') AS requester_email,
		       COALESCE(sel.brand_name, '') AS requester_store_name,
		       COALESCE(lm.text_content, '') AS latest_msg_text,
		       lm.created_at AS latest_msg_at
		FROM support_conversations c
		JOIN support_sessions s ON s.conversation_id = c.id AND s.status = 'ACTIVE'
		LEFT JOIN users u ON u.id = c.requester_user_id
		LEFT JOIN sellers sel ON sel.id = c.requester_seller_id
		LEFT JOIN LATERAL (
		    SELECT sm.text_content, sm.created_at
		    FROM support_messages sm
		    JOIN support_sessions ss ON ss.id = sm.session_id
		    WHERE ss.conversation_id = c.id
		    ORDER BY sm.created_at DESC
		    LIMIT 1
		) lm ON true
		WHERE ($1 = '' OR c.requester_type = $1)
		  AND ($3 = '' OR
		       u.name ILIKE '%' || $3 || '%' OR
		       u.email ILIKE '%' || $3 || '%' OR
		       sel.brand_name ILIKE '%' || $3 || '%' OR
		       EXISTS (
		           SELECT 1
		           FROM support_messages sm2
		           JOIN support_sessions ss2 ON ss2.id = sm2.session_id
		           WHERE ss2.conversation_id = c.id AND sm2.text_content ILIKE '%' || $3 || '%'
		       )
		  )
		ORDER BY c.updated_at DESC
	`
	rows, err := r.db.Query(ctx, query, string(filter), staffUserID, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var c Conversation
		var reqType string
		var sID *uuid.UUID
		var sStatus, sPriority *string
		var sCatID, sAssigned *uuid.UUID
		var sCreated, sUpdated *time.Time
		var unread int
		var reqName, reqEmail, reqStore, latestText string
		var latestAt *time.Time

		if err := rows.Scan(
			&c.ID, &reqType, &c.RequesterUserID, &c.RequesterSellerID, &c.CreatedAt, &c.UpdatedAt,
			&sID, &sStatus, &sPriority, &sCatID, &sAssigned, &sCreated, &sUpdated,
			&unread,
			&reqName, &reqEmail, &reqStore, &latestText, &latestAt,
		); err != nil {
			return nil, err
		}
		c.RequesterType = RequesterType(reqType)
		c.UnreadCount = unread
		c.RequesterName = reqName
		c.RequesterEmail = reqEmail
		c.RequesterStoreName = reqStore
		c.LatestMessageText = latestText
		c.LatestMessageAt = latestAt

		if sID != nil {
			c.ActiveSession = &Session{
				ID:             *sID,
				ConversationID: c.ID,
				Status:         SessionStatus(*sStatus),
				Priority:       SessionPriority(*sPriority),
				CategoryID:     sCatID,
				AssignedTo:     sAssigned,
				CreatedAt:      *sCreated,
				UpdatedAt:      *sUpdated,
			}
		}
		convs = append(convs, c)
	}
	return convs, nil
}

// Sessions

func (r *Repository) GetActiveSession(ctx context.Context, conversationID uuid.UUID) (*Session, error) {
	query := `
		SELECT s.id, s.conversation_id, s.status, s.priority, s.category_id, cat.name, s.assigned_to, s.created_at, s.updated_at, s.completed_at
		FROM support_sessions s
		LEFT JOIN support_categories cat ON cat.id = s.category_id
		WHERE s.conversation_id = $1 AND s.status = 'ACTIVE'
	`
	var sess Session
	var status, priority string
	var catName *string
	err := r.db.QueryRow(ctx, query, conversationID).Scan(
		&sess.ID, &sess.ConversationID, &status, &priority, &sess.CategoryID, &catName, &sess.AssignedTo, &sess.CreatedAt, &sess.UpdatedAt, &sess.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoActiveSession
		}
		return nil, err
	}
	sess.Status = SessionStatus(status)
	sess.Priority = SessionPriority(priority)
	sess.CategoryName = catName
	return &sess, nil
}

func (r *Repository) GetActiveSessionTx(ctx context.Context, tx pgx.Tx, conversationID uuid.UUID) (*Session, error) {
	query := `
		SELECT s.id, s.conversation_id, s.status, s.priority, s.category_id, cat.name, s.assigned_to, s.created_at, s.updated_at, s.completed_at
		FROM support_sessions s
		LEFT JOIN support_categories cat ON cat.id = s.category_id
		WHERE s.conversation_id = $1 AND s.status = 'ACTIVE'
		FOR UPDATE OF s
	`
	var sess Session
	var status, priority string
	var catName *string
	err := tx.QueryRow(ctx, query, conversationID).Scan(
		&sess.ID, &sess.ConversationID, &status, &priority, &sess.CategoryID, &catName, &sess.AssignedTo, &sess.CreatedAt, &sess.UpdatedAt, &sess.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoActiveSession
		}
		return nil, err
	}
	sess.Status = SessionStatus(status)
	sess.Priority = SessionPriority(priority)
	sess.CategoryName = catName
	return &sess, nil
}

func (r *Repository) GetSessionByID(ctx context.Context, sessionID uuid.UUID) (*Session, error) {
	query := `
		SELECT s.id, s.conversation_id, s.status, s.priority, s.category_id, cat.name, s.assigned_to, s.created_at, s.updated_at, s.completed_at
		FROM support_sessions s
		LEFT JOIN support_categories cat ON cat.id = s.category_id
		WHERE s.id = $1
	`
	var sess Session
	var status, priority string
	var catName *string
	err := r.db.QueryRow(ctx, query, sessionID).Scan(
		&sess.ID, &sess.ConversationID, &status, &priority, &sess.CategoryID, &catName, &sess.AssignedTo, &sess.CreatedAt, &sess.UpdatedAt, &sess.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	sess.Status = SessionStatus(status)
	sess.Priority = SessionPriority(priority)
	sess.CategoryName = catName
	return &sess, nil
}

func (r *Repository) CreateSessionTx(ctx context.Context, tx pgx.Tx, conversationID uuid.UUID, priority SessionPriority, categoryID, assignedTo *uuid.UUID) (*Session, error) {
	if priority == "" {
		priority = SessionPriorityNormal
	}
	query := `
		INSERT INTO support_sessions (conversation_id, status, priority, category_id, assigned_to)
		VALUES ($1, 'ACTIVE', $2, $3, $4)
		RETURNING id, conversation_id, status, priority, category_id, assigned_to, created_at, updated_at
	`
	var sess Session
	var status, prio string
	err := tx.QueryRow(ctx, query, conversationID, string(priority), categoryID, assignedTo).Scan(
		&sess.ID, &sess.ConversationID, &status, &prio, &sess.CategoryID, &sess.AssignedTo, &sess.CreatedAt, &sess.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	sess.Status = SessionStatus(status)
	sess.Priority = SessionPriority(prio)
	return &sess, nil
}

func (r *Repository) CompleteSessionTx(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID) error {
	query := `
		UPDATE support_sessions
		SET status = 'COMPLETED', completed_at = now(), updated_at = now()
		WHERE id = $1 AND status = 'ACTIVE'
	`
	res, err := tx.Exec(ctx, query, sessionID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (r *Repository) ReopenSessionTx(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID) error {
	// First check if another active session exists for this conversation
	var activeCount int
	checkQuery := `
		SELECT COUNT(*)
		FROM support_sessions
		WHERE conversation_id = (SELECT conversation_id FROM support_sessions WHERE id = $1)
		  AND status = 'ACTIVE'
	`
	if err := tx.QueryRow(ctx, checkQuery, sessionID).Scan(&activeCount); err != nil {
		return err
	}
	if activeCount > 0 {
		return ErrActiveSessionAlreadyExists
	}

	query := `
		UPDATE support_sessions
		SET status = 'ACTIVE', completed_at = NULL, updated_at = now()
		WHERE id = $1 AND status = 'COMPLETED'
	`
	res, err := tx.Exec(ctx, query, sessionID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (r *Repository) UpdateSessionTx(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID, priority *SessionPriority, categoryID *uuid.UUID, clearCategory bool, assignedTo *uuid.UUID, clearAssignee bool) error {
	query := `
		UPDATE support_sessions
		SET priority = COALESCE($2, priority),
		    category_id = CASE
		        WHEN $3::boolean THEN NULL
		        WHEN $4::uuid IS NOT NULL THEN $4
		        ELSE category_id
		    END,
		    assigned_to = CASE
		        WHEN $5::boolean THEN NULL
		        WHEN $6::uuid IS NOT NULL THEN $6
		        ELSE assigned_to
		    END,
		    updated_at = now()
		WHERE id = $1
	`
	var p *string
	if priority != nil {
		s := string(*priority)
		p = &s
	}
	res, err := tx.Exec(ctx, query, sessionID, p, clearCategory, categoryID, clearAssignee, assignedTo)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// Messages

func (r *Repository) CreateMessageTx(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID, senderType SenderType, senderUserID uuid.UUID, textContent string) (*Message, error) {
	query := `
		INSERT INTO support_messages (session_id, sender_type, sender_user_id, text_content, created_at)
		VALUES ($1, $2, $3, $4, clock_timestamp())
		RETURNING id, session_id, sender_type, sender_user_id, text_content, created_at
	`
	var m Message
	var sType string
	err := tx.QueryRow(ctx, query, sessionID, string(senderType), senderUserID, textContent).Scan(
		&m.ID, &m.SessionID, &sType, &m.SenderUserID, &m.TextContent, &m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	m.SenderType = SenderType(sType)
	m.Attachments = make([]Attachment, 0)
	m.ContextLinks = make([]ContextLink, 0)
	return &m, nil
}

func (r *Repository) ListMessagesByConversation(ctx context.Context, conversationID uuid.UUID) ([]Message, error) {
	query := `
		SELECT m.id, m.session_id, m.sender_type, m.sender_user_id, m.text_content, m.created_at
		FROM support_messages m
		JOIN support_sessions s ON s.id = m.session_id
		WHERE s.conversation_id = $1
		ORDER BY m.created_at ASC
	`
	rows, err := r.db.Query(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	var msgIDs []uuid.UUID
	for rows.Next() {
		var m Message
		var sType string
		if err := rows.Scan(&m.ID, &m.SessionID, &sType, &m.SenderUserID, &m.TextContent, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.SenderType = SenderType(sType)
		m.Attachments = make([]Attachment, 0)
		m.ContextLinks = make([]ContextLink, 0)
		msgs = append(msgs, m)
		msgIDs = append(msgIDs, m.ID)
	}

	if len(msgIDs) == 0 {
		return msgs, nil
	}

	// Fetch attachments
	attQuery := `
		SELECT id, message_id, storage_key, content_type, size_bytes, original_filename, sort_order, created_at
		FROM support_attachments
		WHERE message_id = ANY($1)
		ORDER BY message_id, sort_order ASC
	`
	attRows, err := r.db.Query(ctx, attQuery, msgIDs)
	if err != nil {
		return nil, err
	}
	defer attRows.Close()

	attMap := make(map[uuid.UUID][]Attachment)
	for attRows.Next() {
		var a Attachment
		if err := attRows.Scan(&a.ID, &a.MessageID, &a.StorageKey, &a.ContentType, &a.SizeBytes, &a.OriginalFilename, &a.SortOrder, &a.CreatedAt); err != nil {
			return nil, err
		}
		attMap[a.MessageID] = append(attMap[a.MessageID], a)
	}

	// Fetch context links
	ctxQuery := `
		SELECT cl.id, cl.message_id, cl.context_type, cl.context_id, cl.created_at,
		       CASE cl.context_type
		           WHEN 'ORDER' THEN COALESCE(o.order_number, SUBSTRING(o.id::text, 1, 8))
		           WHEN 'PRODUCT' THEN COALESCE(p.title, '')
		           WHEN 'RETURN' THEN COALESCE(ro.order_number, SUBSTRING(ret.id::text, 1, 8))
		           ELSE ''
		       END AS label
		FROM support_context_links cl
		LEFT JOIN orders o ON cl.context_type = 'ORDER' AND o.id = cl.context_id
		LEFT JOIN products p ON cl.context_type = 'PRODUCT' AND p.id = cl.context_id
		LEFT JOIN returns ret ON cl.context_type = 'RETURN' AND ret.id = cl.context_id
		LEFT JOIN orders ro ON ret.order_id = ro.id
		WHERE cl.message_id = ANY($1)
		ORDER BY cl.message_id, cl.created_at ASC
	`
	ctxRows, err := r.db.Query(ctx, ctxQuery, msgIDs)
	if err != nil {
		return nil, err
	}
	defer ctxRows.Close()

	ctxMap := make(map[uuid.UUID][]ContextLink)
	for ctxRows.Next() {
		var cl ContextLink
		var cType string
		if err := ctxRows.Scan(&cl.ID, &cl.MessageID, &cType, &cl.ContextID, &cl.CreatedAt, &cl.Label); err != nil {
			return nil, err
		}
		cl.ContextType = ContextType(cType)
		ctxMap[cl.MessageID] = append(ctxMap[cl.MessageID], cl)
	}

	for i := range msgs {
		if atts, ok := attMap[msgs[i].ID]; ok {
			msgs[i].Attachments = atts
		}
		if cls, ok := ctxMap[msgs[i].ID]; ok {
			msgs[i].ContextLinks = cls
		}
	}

	return msgs, nil
}

// Internal Notes

func (r *Repository) CreateInternalNoteTx(ctx context.Context, tx pgx.Tx, sessionID, authorUserID uuid.UUID, textContent string) (*InternalNote, error) {
	query := `
		INSERT INTO support_internal_notes (session_id, author_user_id, text_content)
		VALUES ($1, $2, $3)
		RETURNING id, session_id, author_user_id, text_content, created_at
	`
	var n InternalNote
	err := tx.QueryRow(ctx, query, sessionID, authorUserID, textContent).Scan(
		&n.ID, &n.SessionID, &n.AuthorUserID, &n.TextContent, &n.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *Repository) ListInternalNotesByConversation(ctx context.Context, conversationID uuid.UUID) ([]InternalNote, error) {
	query := `
		SELECT n.id, n.session_id, n.author_user_id, COALESCE(u.name, 'Сотрудник'), n.text_content, n.created_at
		FROM support_internal_notes n
		JOIN support_sessions s ON s.id = n.session_id
		LEFT JOIN users u ON u.id = n.author_user_id
		WHERE s.conversation_id = $1
		ORDER BY n.created_at ASC
	`
	rows, err := r.db.Query(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []InternalNote
	for rows.Next() {
		var n InternalNote
		if err := rows.Scan(&n.ID, &n.SessionID, &n.AuthorUserID, &n.AuthorName, &n.TextContent, &n.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, nil
}

// Context Links

func (r *Repository) CreateContextLinksTx(ctx context.Context, tx pgx.Tx, messageID uuid.UUID, links []CreateContextLinkReq) ([]ContextLink, error) {
	if len(links) == 0 {
		return []ContextLink{}, nil
	}
	query := `
		INSERT INTO support_context_links (message_id, context_type, context_id)
		VALUES ($1, $2, $3)
		RETURNING id, message_id, context_type, context_id, created_at
	`
	var res []ContextLink
	for _, l := range links {
		var cl ContextLink
		var cType string
		if err := tx.QueryRow(ctx, query, messageID, string(l.ContextType), l.ContextID).Scan(&cl.ID, &cl.MessageID, &cType, &cl.ContextID, &cl.CreatedAt); err != nil {
			return nil, err
		}
		cl.ContextType = ContextType(cType)
		res = append(res, cl)
	}
	return res, nil
}

// Attachments

func (r *Repository) CreateStagedAttachment(ctx context.Context, att *StagedAttachment) error {
	query := `
		INSERT INTO support_staged_attachments (id, conversation_id, uploader_user_id, storage_key, content_type, size_bytes, original_filename, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query, att.ID, att.ConversationID, att.UploaderUserID, att.StorageKey, att.ContentType, att.SizeBytes, att.OriginalFilename, att.CreatedAt)
	return err
}

func (r *Repository) GetStagedAttachmentsTx(ctx context.Context, tx pgx.Tx, conversationID, uploaderID uuid.UUID, ids []uuid.UUID) ([]StagedAttachment, error) {
	if len(ids) == 0 {
		return []StagedAttachment{}, nil
	}
	query := `
		SELECT id, conversation_id, uploader_user_id, storage_key, content_type, size_bytes, original_filename, created_at
		FROM support_staged_attachments
		WHERE conversation_id = $1 AND uploader_user_id = $2 AND id = ANY($3)
		FOR UPDATE
	`
	rows, err := tx.Query(ctx, query, conversationID, uploaderID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var atts []StagedAttachment
	for rows.Next() {
		var a StagedAttachment
		if err := rows.Scan(&a.ID, &a.ConversationID, &a.UploaderUserID, &a.StorageKey, &a.ContentType, &a.SizeBytes, &a.OriginalFilename, &a.CreatedAt); err != nil {
			return nil, err
		}
		atts = append(atts, a)
	}
	return atts, nil
}

func (r *Repository) BindAttachmentsTx(ctx context.Context, tx pgx.Tx, messageID uuid.UUID, stagedAtts []StagedAttachment) ([]Attachment, error) {
	if len(stagedAtts) == 0 {
		return []Attachment{}, nil
	}

	insertQuery := `
		INSERT INTO support_attachments (id, message_id, storage_key, content_type, size_bytes, original_filename, sort_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		RETURNING id, message_id, storage_key, content_type, size_bytes, original_filename, sort_order, created_at
	`
	deleteQuery := `DELETE FROM support_staged_attachments WHERE id = $1`

	var bound []Attachment
	for i, att := range stagedAtts {
		var b Attachment
		if err := tx.QueryRow(ctx, insertQuery, att.ID, messageID, att.StorageKey, att.ContentType, att.SizeBytes, att.OriginalFilename, i).Scan(
			&b.ID, &b.MessageID, &b.StorageKey, &b.ContentType, &b.SizeBytes, &b.OriginalFilename, &b.SortOrder, &b.CreatedAt,
		); err != nil {
			return nil, err
		}
		bound = append(bound, b)

		if _, err := tx.Exec(ctx, deleteQuery, att.ID); err != nil {
			return nil, err
		}
	}
	return bound, nil
}

func (r *Repository) GetAttachmentByID(ctx context.Context, id uuid.UUID) (*Attachment, uuid.UUID, error) {
	query := `
		SELECT a.id, a.message_id, a.storage_key, a.content_type, a.size_bytes, a.original_filename, a.sort_order, a.created_at, s.conversation_id
		FROM support_attachments a
		JOIN support_messages m ON m.id = a.message_id
		JOIN support_sessions s ON s.id = m.session_id
		WHERE a.id = $1
	`
	var a Attachment
	var convID uuid.UUID
	err := r.db.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.MessageID, &a.StorageKey, &a.ContentType, &a.SizeBytes, &a.OriginalFilename, &a.SortOrder, &a.CreatedAt, &convID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, uuid.Nil, ErrAttachmentNotFound
		}
		return nil, uuid.Nil, err
	}
	return &a, convID, nil
}

// Read states

func (r *Repository) MarkStaffReadTx(ctx context.Context, tx pgx.Tx, staffUserID, conversationID uuid.UUID) error {
	query := `
		INSERT INTO support_staff_reads (staff_user_id, conversation_id, last_read_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (staff_user_id, conversation_id)
		DO UPDATE SET last_read_at = clock_timestamp()
	`
	_, err := tx.Exec(ctx, query, staffUserID, conversationID)
	return err
}

func (r *Repository) MarkRequesterReadTx(ctx context.Context, tx pgx.Tx, userID, conversationID uuid.UUID) error {
	query := `
		INSERT INTO support_requester_reads (user_id, conversation_id, last_read_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (user_id, conversation_id)
		DO UPDATE SET last_read_at = clock_timestamp()
	`
	_, err := tx.Exec(ctx, query, userID, conversationID)
	return err
}

func (r *Repository) GetStaffUnreadCount(ctx context.Context, staffUserID, conversationID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM support_messages m
		JOIN support_sessions s ON s.id = m.session_id
		LEFT JOIN support_staff_reads r ON r.conversation_id = s.conversation_id AND r.staff_user_id = $1
		WHERE s.conversation_id = $2
		  AND m.sender_type != 'STAFF'
		  AND m.created_at > COALESCE(r.last_read_at, '1970-01-01'::timestamptz)
	`
	var count int
	err := r.db.QueryRow(ctx, query, staffUserID, conversationID).Scan(&count)
	return count, err
}

func (r *Repository) GetRequesterUnreadCount(ctx context.Context, userID, conversationID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM support_messages m
		JOIN support_sessions s ON s.id = m.session_id
		LEFT JOIN support_requester_reads r ON r.conversation_id = s.conversation_id AND r.user_id = $1
		WHERE s.conversation_id = $2
		  AND m.sender_type = 'STAFF'
		  AND m.created_at > COALESCE(r.last_read_at, '1970-01-01'::timestamptz)
	`
	var count int
	err := r.db.QueryRow(ctx, query, userID, conversationID).Scan(&count)
	return count, err
}

// Ownership / Context Validation Queries

func (r *Repository) ValidateCustomerOrder(ctx context.Context, customerID, orderID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM orders WHERE id = $1 AND user_id = $2)`, orderID, customerID).Scan(&exists)
	return exists, err
}

func (r *Repository) ValidateCustomerReturn(ctx context.Context, customerID, returnID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM returns WHERE id = $1 AND user_id = $2)`, returnID, customerID).Scan(&exists)
	return exists, err
}

func (r *Repository) ValidateCustomerProduct(ctx context.Context, productID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products WHERE id = $1 AND status = 'approved')`, productID).Scan(&exists)
	return exists, err
}

func (r *Repository) ValidateSellerOrder(ctx context.Context, sellerID, orderID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM order_items WHERE order_id = $1 AND seller_id = $2)`, orderID, sellerID).Scan(&exists)
	return exists, err
}

func (r *Repository) ValidateSellerReturn(ctx context.Context, sellerID, returnID uuid.UUID) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM return_items ri
			JOIN order_items oi ON oi.id = ri.order_item_id
			WHERE ri.return_id = $1 AND oi.seller_id = $2
		)
	`
	err := r.db.QueryRow(ctx, query, returnID, sellerID).Scan(&exists)
	return exists, err
}

func (r *Repository) ValidateSellerProduct(ctx context.Context, sellerID, productID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products WHERE id = $1 AND seller_id = $2)`, productID, sellerID).Scan(&exists)
	return exists, err
}

func (r *Repository) ValidateSellerMember(ctx context.Context, sellerID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM seller_users WHERE seller_id = $1 AND user_id = $2)`, sellerID, userID).Scan(&exists)
	return exists, err
}

func (r *Repository) ValidateStaffUser(ctx context.Context, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM staff_members WHERE user_id = $1 AND status = 'active')`, userID).Scan(&exists)
	return exists, err
}

func (r *Repository) ValidateActiveStaff(ctx context.Context, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM staff_members WHERE user_id = $1 AND status = 'active')`, userID).Scan(&exists)
	return exists, err
}
