package sellers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

// WithTx returns a new Repository bound to the provided transaction
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{db: tx}
}

func (r *Repository) CreateSeller(ctx context.Context, s *Seller) error {
	query := `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, logo_url, logo_object_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(ctx, query,
		s.ID, s.BrandName, s.Slug, s.Description, s.ContactEmail, s.ContactPhone, s.Status, s.LogoURL, s.LogoObjectKey, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create seller: %w", err)
	}
	return nil
}

func (r *Repository) CreateSellerUser(ctx context.Context, su *SellerUser) error {
	query := `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query,
		su.ID, su.SellerID, su.UserID, su.Role, su.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create seller user: %w", err)
	}
	return nil
}

func (r *Repository) GetSellerByID(ctx context.Context, id uuid.UUID) (*Seller, error) {
	query := `
		SELECT id, brand_name, slug, description, contact_email, contact_phone, status, logo_url, logo_object_key, average_rating, reviews_count, created_at, updated_at
		FROM sellers
		WHERE id = $1
	`
	var s Seller
	err := r.db.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.BrandName, &s.Slug, &s.Description, &s.ContactEmail, &s.ContactPhone, &s.Status, &s.LogoURL, &s.LogoObjectKey, &s.AverageRating, &s.ReviewsCount, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSellerNotFound
		}
		return nil, fmt.Errorf("failed to get seller by id: %w", err)
	}
	return &s, nil
}

func (r *Repository) GetSellerBySlug(ctx context.Context, slug string) (*Seller, error) {
	query := `
		SELECT id, brand_name, slug, description, contact_email, contact_phone, status, logo_url, logo_object_key, average_rating, reviews_count, created_at, updated_at
		FROM sellers
		WHERE slug = $1
	`
	var s Seller
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&s.ID, &s.BrandName, &s.Slug, &s.Description, &s.ContactEmail, &s.ContactPhone, &s.Status, &s.LogoURL, &s.LogoObjectKey, &s.AverageRating, &s.ReviewsCount, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSellerNotFound
		}
		return nil, fmt.Errorf("failed to get seller by slug: %w", err)
	}
	return &s, nil
}

func (r *Repository) GetSellerByUserID(ctx context.Context, userID uuid.UUID) (*Seller, *SellerUser, error) {
	query := `
		SELECT s.id, s.brand_name, s.slug, s.description, s.contact_email, s.contact_phone, s.status, s.logo_url, s.logo_object_key, s.average_rating, s.reviews_count, s.created_at, s.updated_at,
		       su.id, su.seller_id, su.user_id, su.role, su.created_at
		FROM sellers s
		JOIN seller_users su ON s.id = su.seller_id
		WHERE su.user_id = $1
	`
	var s Seller
	var su SellerUser
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&s.ID, &s.BrandName, &s.Slug, &s.Description, &s.ContactEmail, &s.ContactPhone, &s.Status, &s.LogoURL, &s.LogoObjectKey, &s.AverageRating, &s.ReviewsCount, &s.CreatedAt, &s.UpdatedAt,
		&su.ID, &su.SellerID, &su.UserID, &su.Role, &su.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrSellerUserNotFound
		}
		return nil, nil, fmt.Errorf("failed to get seller by user id: %w", err)
	}
	return &s, &su, nil
}

func (r *Repository) UpdateSellerStatus(ctx context.Context, id uuid.UUID, status SellerStatus) error {
	query := `
		UPDATE sellers
		SET status = $1, updated_at = now()
		WHERE id = $2
	`
	res, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update seller status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrSellerNotFound
	}
	return nil
}

func (r *Repository) ListSellers(ctx context.Context, filter SellersFilter) ([]AdminListSeller, int, error) {
	cte := `
		WITH metrics AS (
			SELECT 
				s.id as seller_id,
				COALESCE(w.active_warnings, 0) as active_warnings,
				COALESCE(v.active_violations, 0) as active_violations,
				COALESCE(sales.gross_sales, 0) as gross_sales_30d,
				COALESCE(sales.orders_count, 0) as orders_count_30d,
				COALESCE(sales.cancelled_count, 0) as cancelled_count_30d,
				CASE WHEN COALESCE(sales.orders_count, 0) = 0 THEN 0 ELSE (COALESCE(sales.cancelled_count, 0) * 100 / COALESCE(sales.orders_count, 0)) END as cancel_rate_30d,
				0 as performance_score, -- will be calculated in service
				'no_data' as performance -- will be calculated in service
			FROM sellers s
			LEFT JOIN (
				SELECT seller_id, COUNT(*) as active_warnings 
				FROM seller_warnings 
				WHERE status = 'active' 
				GROUP BY seller_id
			) w ON w.seller_id = s.id
			LEFT JOIN (
				SELECT seller_id, COUNT(*) as active_violations
				FROM seller_violations
				WHERE status = 'active'
				GROUP BY seller_id
			) v ON v.seller_id = s.id
			LEFT JOIN (
				SELECT 
					seller_id,
					COALESCE(SUM(subtotal_cents), 0) AS gross_sales,
					COUNT(DISTINCT order_id) AS orders_count,
					COALESCE(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0) AS cancelled_count
				FROM order_fulfillments
				WHERE created_at >= NOW() - INTERVAL '30 days'
				GROUP BY seller_id
			) sales ON sales.seller_id = s.id
		)
	`
	baseQuery := `
		FROM sellers s
		LEFT JOIN seller_users su ON su.seller_id = s.id AND su.role = 'owner'
		LEFT JOIN users u ON u.id = su.user_id
		LEFT JOIN metrics m ON m.seller_id = s.id
		WHERE 1=1
	`
	var args []interface{}
	argID := 1

	if len(filter.Status) > 0 {
		placeholders := ""
		for i, st := range filter.Status {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += fmt.Sprintf("$%d", argID)
			args = append(args, st)
			argID++
		}
		baseQuery += fmt.Sprintf(" AND s.status IN (%s)", placeholders)
	}

	if filter.Query != "" {
		baseQuery += fmt.Sprintf(" AND (s.brand_name ILIKE $%d OR s.contact_email ILIKE $%d OR s.slug ILIKE $%d OR u.name ILIKE $%d OR u.email ILIKE $%d)", argID, argID, argID, argID, argID)
		args = append(args, "%"+filter.Query+"%")
		argID++
	}

	if filter.Store == "created" {
		baseQuery += " AND s.brand_name IS NOT NULL AND s.brand_name != ''"
	} else if filter.Store == "not_created" {
		baseQuery += " AND (s.brand_name IS NULL OR s.brand_name = '')"
	}

	// Rating filters
	if filter.RatingMin != nil {
		baseQuery += fmt.Sprintf(" AND s.average_rating >= $%d", argID)
		args = append(args, *filter.RatingMin)
		argID++
	}
	if filter.RatingMax != nil {
		baseQuery += fmt.Sprintf(" AND s.average_rating <= $%d", argID)
		args = append(args, *filter.RatingMax)
		argID++
	}
	if filter.HasReviews != nil {
		if *filter.HasReviews {
			baseQuery += " AND s.reviews_count > 0"
		} else {
			baseQuery += " AND s.reviews_count = 0"
		}
	}

	// Warning/Violation filters
	if filter.HasWarnings != nil {
		if *filter.HasWarnings {
			baseQuery += " AND m.active_warnings > 0"
		} else {
			baseQuery += " AND m.active_warnings = 0"
		}
	}
	if filter.HasViolations != nil {
		if *filter.HasViolations {
			baseQuery += " AND m.active_violations > 0"
		} else {
			baseQuery += " AND m.active_violations = 0"
		}
	}

	// Sales filters
	if filter.SalesGrossMin != nil {
		baseQuery += fmt.Sprintf(" AND m.gross_sales_30d >= $%d", argID)
		args = append(args, *filter.SalesGrossMin)
		argID++
	}
	if filter.SalesGrossMax != nil {
		baseQuery += fmt.Sprintf(" AND m.gross_sales_30d <= $%d", argID)
		args = append(args, *filter.SalesGrossMax)
		argID++
	}
	if filter.OrdersCountMin != nil {
		baseQuery += fmt.Sprintf(" AND m.orders_count_30d >= $%d", argID)
		args = append(args, *filter.OrdersCountMin)
		argID++
	}
	if filter.OrdersCountMax != nil {
		baseQuery += fmt.Sprintf(" AND m.orders_count_30d <= $%d", argID)
		args = append(args, *filter.OrdersCountMax)
		argID++
	}

	// We cannot filter easily by performance here if it's computed purely in Go service.
	// We will compute PerformanceScore dynamically in service, but if they filter by it, we could have a mismatch unless we calculate it in SQL.
	// For now, if we must, we fetch more records or implement score in SQL. 
	// The plan specifies "Все веса и пороги хранить централизованно на Backend. Не размазывать их по SQL".
	// Therefore, Performance filtering must be applied in Go code AFTER fetching, or we do a best-effort in SQL.
	// Since DB pagination is required, we must do it in SQL if performance filtering is strictly required for pagination.
	// Let's assume we fetch first then filter? No, limit/offset requires DB.
	// Given instructions: "Все веса и пороги хранить централизованно на Backend."
	// We'll calculate it in the service. The filter for Performance category/min/max will be applied in memory after fetching from DB, but that breaks DB pagination.
	// Alternative: we return a list, then filter, but that's bad.
	// Let's implement it in Go by fetching all matching basic criteria, scoring them, sorting, and slicing for pagination if performance filter is present.
	// But `total` needs to be accurate. We will fetch all without limit if performance filter is present!
	// Wait, for MVP, we just fetch all if Performance filter is used, or we just build it into SQL.
	// The prompt: "Все веса и пороги хранить централизованно на Backend. Не размазывать их по SQL".
	// So we must fetch all matching records, score them in Go, then apply Limit/Offset.
	
	// Count is not done here if we post-filter. Let's do a flag.
	postFilterPerformance := filter.PerformanceMin != nil || filter.PerformanceMax != nil || filter.PerformanceCategory != "" || filter.Sort == "performance_score" || filter.Sort == "performance"
	
	var total int
	if !postFilterPerformance {
		err := r.db.QueryRow(ctx, cte + " SELECT COUNT(s.id) " + baseQuery, args...).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to count sellers: %w", err)
		}
	}

	dir := "DESC"
	if filter.Direction == "asc" {
		dir = "ASC"
	}

	orderClause := "ORDER BY s.created_at " + dir
	if !postFilterPerformance {
		switch filter.Sort {
		case "created_at", "last_active":
			orderClause = "ORDER BY s.created_at " + dir
		case "owner_name", "owner":
			orderClause = "ORDER BY u.name " + dir
		case "brand_name", "brand":
			orderClause = "ORDER BY s.brand_name " + dir
		case "status", "onboarding_stage":
			orderClause = "ORDER BY s.status " + dir + ", s.created_at DESC"
		case "rating":
			orderClause = "ORDER BY s.average_rating " + dir + ", s.reviews_count " + dir
		case "gross_sales", "turnover":
			orderClause = "ORDER BY m.gross_sales_30d " + dir
		case "orders_count", "orders":
			orderClause = "ORDER BY m.orders_count_30d " + dir
		case "warnings_active", "problems":
			orderClause = "ORDER BY m.active_warnings " + dir + ", m.active_violations " + dir
		}
	}

	limitOffset := ""
	if !postFilterPerformance && filter.Limit > 0 {
		limitOffset = fmt.Sprintf(" LIMIT %d OFFSET %d", filter.Limit, filter.Offset)
	}

	query := cte + `
		SELECT 
			s.id, s.brand_name, s.slug, s.description, s.contact_email, s.contact_phone, s.status, s.logo_url, s.logo_object_key, s.average_rating, s.reviews_count, s.created_at, s.updated_at,
			COALESCE(u.name, '') as owner_name, COALESCE(u.email, '') as owner_email,
			m.active_warnings, m.active_violations, m.performance, m.gross_sales_30d, m.orders_count_30d, m.cancel_rate_30d
	` + baseQuery + " " + orderClause + limitOffset

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query sellers: %w", err)
	}
	defer rows.Close()

	var sellers []AdminListSeller
	for rows.Next() {
		var s AdminListSeller
		err := rows.Scan(
			&s.ID, &s.BrandName, &s.Slug, &s.Description, &s.ContactEmail, &s.ContactPhone, &s.Status, &s.LogoURL, &s.LogoObjectKey, &s.AverageRating, &s.ReviewsCount, &s.CreatedAt, &s.UpdatedAt,
			&s.OwnerName, &s.OwnerEmail,
			&s.WarningsActive, &s.Violations, &s.PerformanceCategory, &s.GrossSales30d, &s.OrdersCount30d, &s.CancelRate30d,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan seller row: %w", err)
		}
		sellers = append(sellers, s)
	}

	// Post-processing for Performance Filtering and Pagination in memory if needed
	// We return `total=0` if postFilterPerformance is true to indicate service should compute it.
	if postFilterPerformance {
		total = 0
	}

	return sellers, total, nil
}
func (r *Repository) GetAdminSellersStatusCounts(ctx context.Context, filter SellersFilter) (map[string]int, error) {
	query := `
		SELECT s.status, COUNT(s.id)
		FROM sellers s
		LEFT JOIN seller_users su ON su.seller_id = s.id AND su.role = 'owner'
		LEFT JOIN users u ON u.id = su.user_id
		LEFT JOIN (
			SELECT seller_id, COUNT(*) as active_warnings 
			FROM seller_warnings 
			WHERE status = 'active' 
			GROUP BY seller_id
		) w ON w.seller_id = s.id
		WHERE 1=1
	`
	var args []interface{}
	argID := 1
	
	if filter.Query != "" {
		query += fmt.Sprintf(" AND (s.brand_name ILIKE $%d OR s.contact_email ILIKE $%d OR s.slug ILIKE $%d OR u.name ILIKE $%d OR u.email ILIKE $%d)", argID, argID, argID, argID, argID)
		args = append(args, "%"+filter.Query+"%")
		argID++
	}

	if filter.Store == "created" {
		query += " AND s.brand_name IS NOT NULL AND s.brand_name != ''"
	} else if filter.Store == "not_created" {
		query += " AND (s.brand_name IS NULL OR s.brand_name = '')"
	}

	if filter.Problems == "with_warnings" {
		query += " AND COALESCE(w.active_warnings, 0) > 0"
	} else if filter.Problems == "without_warnings" {
		query += " AND COALESCE(w.active_warnings, 0) = 0"
	}

	query += " GROUP BY s.status"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to count status: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan status count: %w", err)
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

func (r *Repository) UpdateSellerProfile(ctx context.Context, sellerID uuid.UUID, req *UpdateSellerProfileRequest) error {
	query := `
		UPDATE sellers
		SET
			brand_name    = COALESCE($1, brand_name),
			description   = COALESCE($2, description),
			contact_email = COALESCE($3, contact_email),
			contact_phone = COALESCE($4, contact_phone),
			slug          = COALESCE($5, slug),
			updated_at    = now()
		WHERE id = $6
	`
	res, err := r.db.Exec(ctx, query,
		req.BrandName, req.Description, req.ContactEmail, req.ContactPhone, req.Slug,
		sellerID,
	)
	if err != nil {
		return fmt.Errorf("failed to update seller profile: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrSellerNotFound
	}
	return nil
}

func (r *Repository) UpdateSellerLogo(ctx context.Context, sellerID uuid.UUID, logoURL string, logoObjectKey string) error {
	query := `
		UPDATE sellers
		SET logo_url = $1, logo_object_key = $2, updated_at = now()
		WHERE id = $3
	`
	res, err := r.db.Exec(ctx, query, logoURL, logoObjectKey, sellerID)
	if err != nil {
		return fmt.Errorf("failed to update seller logo: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrSellerNotFound
	}
	return nil
}

// CountSellers returns total number of sellers.
func (r *Repository) CountSellers(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM sellers`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sellers: %w", err)
	}
	return count, nil
}

// GetSellerDetailByID returns a full seller aggregate with owner info and counts.
func (r *Repository) GetSellerDetailByID(ctx context.Context, sellerID uuid.UUID) (*SellerDetail, error) {
	query := `
		SELECT
			s.id, s.brand_name, s.slug, s.description, s.contact_email, s.contact_phone, s.logo_url, s.status, s.average_rating, s.reviews_count, s.created_at, s.updated_at,
			u.id, u.name, u.email, u.status,
			(SELECT COUNT(*) FROM seller_warnings sw WHERE sw.seller_id = s.id AND sw.status = 'active')  AS warnings_active,
			(SELECT COUNT(*) FROM seller_violations sv WHERE sv.seller_id = s.id AND sv.status = 'active') AS violations_active,
			(SELECT COUNT(*) FROM seller_violations sv WHERE sv.seller_id = s.id AND sv.status = 'active' AND sv.counts_for_penalty = TRUE) AS active_penalty_violations
		FROM sellers s
		JOIN seller_users su ON su.seller_id = s.id
		JOIN users u ON u.id = su.user_id
		WHERE s.id = $1
		LIMIT 1
	`
	var d SellerDetail
	err := r.db.QueryRow(ctx, query, sellerID).Scan(
		&d.ID, &d.BrandName, &d.Slug, &d.Description, &d.ContactEmail, &d.ContactPhone, &d.LogoURL, &d.Status, &d.AverageRating, &d.ReviewsCount, &d.CreatedAt, &d.UpdatedAt,
		&d.OwnerID, &d.OwnerName, &d.OwnerEmail, &d.OwnerStatus,
		&d.WarningsActive, &d.ViolationsActive, &d.ActivePenaltyViolations,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSellerNotFound
		}
		return nil, fmt.Errorf("failed to get seller detail: %w", err)
	}
	return &d, nil
}

// WriteStatusHistory inserts a seller_status_history row.
func (r *Repository) WriteStatusHistory(ctx context.Context, sellerID uuid.UUID, oldStatus *string, newStatus string, reason *string, actorUserID *uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO seller_status_history (seller_id, old_status, new_status, reason, actor_user_id)
		VALUES ($1, $2, $3, $4, $5)
	`, sellerID, oldStatus, newStatus, reason, actorUserID)
	if err != nil {
		return fmt.Errorf("failed to write status history: %w", err)
	}
	return nil
}

// GetStatusHistory returns status history ordered by created_at desc.
func (r *Repository) GetStatusHistory(ctx context.Context, sellerID uuid.UUID) ([]SellerStatusHistoryItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, old_status, new_status, reason, actor_user_id, created_at
		FROM seller_status_history
		WHERE seller_id = $1
		ORDER BY created_at DESC
	`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get status history: %w", err)
	}
	defer rows.Close()

	var items []SellerStatusHistoryItem
	for rows.Next() {
		var item SellerStatusHistoryItem
		if err := rows.Scan(&item.ID, &item.OldStatus, &item.NewStatus, &item.Reason, &item.ActorUserID, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan status history: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateWarning inserts a seller_warnings row.
func (r *Repository) CreateWarning(ctx context.Context, w CreateWarningInput) (*WarningResponse, error) {
	var wr WarningResponse
	err := r.db.QueryRow(ctx, `
		INSERT INTO seller_warnings (seller_id, type, title, message, severity, status, actor_user_id)
		VALUES ($1, $2, $3, $4, $5, 'active', $6)
		RETURNING id, seller_id, type, title, message, severity, status, actor_user_id, created_at, resolved_at, resolved_by, resolution_note
	`, w.SellerID, w.Type, w.Title, w.Message, w.Severity, w.ActorUserID).Scan(
		&wr.ID, &wr.SellerID, &wr.Type, &wr.Title, &wr.Message, &wr.Severity, &wr.Status,
		&wr.ActorUserID, &wr.CreatedAt, &wr.ResolvedAt, &wr.ResolvedBy, &wr.ResolutionNote,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create warning: %w", err)
	}
	return &wr, nil
}

// ListWarnings returns all warnings for a seller ordered by created_at desc.
func (r *Repository) ListWarnings(ctx context.Context, sellerID uuid.UUID) ([]WarningResponse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, seller_id, type, title, message, severity, status, actor_user_id, created_at, resolved_at, resolved_by, resolution_note
		FROM seller_warnings
		WHERE seller_id = $1
		ORDER BY created_at DESC
	`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list warnings: %w", err)
	}
	defer rows.Close()

	var items []WarningResponse
	for rows.Next() {
		var wr WarningResponse
		if err := rows.Scan(
			&wr.ID, &wr.SellerID, &wr.Type, &wr.Title, &wr.Message, &wr.Severity, &wr.Status,
			&wr.ActorUserID, &wr.CreatedAt, &wr.ResolvedAt, &wr.ResolvedBy, &wr.ResolutionNote,
		); err != nil {
			return nil, fmt.Errorf("failed to scan warning: %w", err)
		}
		items = append(items, wr)
	}
	return items, rows.Err()
}

// UpdateWarningStatus updates warning status and resolved fields.
func (r *Repository) UpdateWarningStatus(ctx context.Context, warningID uuid.UUID, status string, resolvedBy *uuid.UUID, note *string) error {
	res, err := r.db.Exec(ctx, `
		UPDATE seller_warnings
		SET status = $1, resolved_at = CASE WHEN $1 IN ('resolved','cancelled') THEN NOW() ELSE resolved_at END,
		    resolved_by = $2, resolution_note = COALESCE($3, resolution_note)
		WHERE id = $4
	`, status, resolvedBy, note, warningID)
	if err != nil {
		return fmt.Errorf("failed to update warning status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrWarningNotFound
	}
	return nil
}

// CreateViolation inserts a seller_violations row.
func (r *Repository) CreateViolation(ctx context.Context, v CreateViolationInput) (*ViolationResponse, error) {
	var vr ViolationResponse
	err := r.db.QueryRow(ctx, `
		INSERT INTO seller_violations (seller_id, type, title, description, severity, status, counts_for_penalty, actor_user_id)
		VALUES ($1, $2, $3, $4, $5, 'active', $6, $7)
		RETURNING id, seller_id, type, title, description, severity, status, counts_for_penalty, actor_user_id, created_at, resolved_at, resolved_by, resolution_note
	`, v.SellerID, v.Type, v.Title, v.Description, v.Severity, v.CountsForPenalty, v.ActorUserID).Scan(
		&vr.ID, &vr.SellerID, &vr.Type, &vr.Title, &vr.Description, &vr.Severity, &vr.Status,
		&vr.CountsForPenalty, &vr.ActorUserID, &vr.CreatedAt, &vr.ResolvedAt, &vr.ResolvedBy, &vr.ResolutionNote,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create violation: %w", err)
	}
	return &vr, nil
}

// ListViolations returns all violations for a seller ordered by created_at desc.
func (r *Repository) ListViolations(ctx context.Context, sellerID uuid.UUID) ([]ViolationResponse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, seller_id, type, title, description, severity, status, counts_for_penalty, actor_user_id, created_at, resolved_at, resolved_by, resolution_note
		FROM seller_violations
		WHERE seller_id = $1
		ORDER BY created_at DESC
	`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list violations: %w", err)
	}
	defer rows.Close()

	var items []ViolationResponse
	for rows.Next() {
		var vr ViolationResponse
		if err := rows.Scan(
			&vr.ID, &vr.SellerID, &vr.Type, &vr.Title, &vr.Description, &vr.Severity, &vr.Status,
			&vr.CountsForPenalty, &vr.ActorUserID, &vr.CreatedAt, &vr.ResolvedAt, &vr.ResolvedBy, &vr.ResolutionNote,
		); err != nil {
			return nil, fmt.Errorf("failed to scan violation: %w", err)
		}
		items = append(items, vr)
	}
	return items, rows.Err()
}

// UpdateViolationStatus updates violation status and resolved fields.
func (r *Repository) UpdateViolationStatus(ctx context.Context, violationID uuid.UUID, status string, resolvedBy *uuid.UUID, note *string) error {
	res, err := r.db.Exec(ctx, `
		UPDATE seller_violations
		SET status = $1, resolved_at = CASE WHEN $1 IN ('resolved','cancelled') THEN NOW() ELSE resolved_at END,
		    resolved_by = $2, resolution_note = COALESCE($3, resolution_note)
		WHERE id = $4
	`, status, resolvedBy, note, violationID)
	if err != nil {
		return fmt.Errorf("failed to update violation status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrViolationNotFound
	}
	return nil
}

// CountActivePenaltyViolations counts active violations where counts_for_penalty=true.
func (r *Repository) CountActivePenaltyViolations(ctx context.Context, sellerID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM seller_violations
		WHERE seller_id = $1 AND status = 'active' AND counts_for_penalty = TRUE
	`, sellerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count penalty violations: %w", err)
	}
	return count, nil
}

// GetSellerOverview aggregates business metrics for a seller.
func (r *Repository) GetSellerOverview(ctx context.Context, sellerID uuid.UUID, period string) (*SellerOverviewResponse, error) {
	resp := &SellerOverviewResponse{
		Period: period,
	}

	var intervalClause string
	switch period {
	case "7d":
		intervalClause = "AND created_at >= NOW() - INTERVAL '7 days'"
	case "all":
		intervalClause = ""
	default:
		resp.Period = "30d"
		intervalClause = "AND created_at >= NOW() - INTERVAL '30 days'"
	}

	// 1. Sales & Orders
	salesQuery := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(subtotal_cents), 0) AS gross_sales,
			COUNT(DISTINCT order_id) AS orders_count,
			COALESCE(SUM(CASE WHEN status = 'delivered' THEN 1 ELSE 0 END), 0) AS delivered_count,
			COALESCE(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0) AS cancelled_count,
			COALESCE(SUM(CASE WHEN status = 'returned' THEN 1 ELSE 0 END), 0) AS returned_count
		FROM order_fulfillments
		WHERE seller_id = $1 %s
	`, intervalClause)
	var grossSales int64
	var ordersCount, deliveredCount, cancelledCount, returnedCount int
	if err := r.db.QueryRow(ctx, salesQuery, sellerID).Scan(&grossSales, &ordersCount, &deliveredCount, &cancelledCount, &returnedCount); err != nil {
		// Log or fallback
	}
	resp.Sales.GrossSalesCents = grossSales
	resp.Sales.OrdersCount = ordersCount
	resp.Sales.DeliveredOrders = deliveredCount
	resp.Sales.CancelledOrders = cancelledCount
	resp.Sales.ReturnedOrders = returnedCount
	
	if ordersCount > 0 {
		resp.Sales.AverageOrderValueCents = grossSales / int64(ordersCount)
		resp.Sales.SellerCausedCancellationRate = (cancelledCount * 100) / ordersCount
		resp.Sales.ReturnRateBySellerReason = (returnedCount * 100) / ordersCount
	}

	// Items sold
	itemsQuery := fmt.Sprintf(`
		SELECT COALESCE(SUM(quantity), 0)
		FROM order_items
		WHERE seller_id = $1 %s
	`, intervalClause)
	_ = r.db.QueryRow(ctx, itemsQuery, sellerID).Scan(&resp.Sales.ItemsSold)

	// 2. Catalog
	_ = r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status = 'published' THEN 1 ELSE 0 END), 0) AS published,
			COALESCE(SUM(CASE WHEN status IN ('pending', 'pending_review') THEN 1 ELSE 0 END), 0) AS moderation,
			COALESCE(SUM(CASE WHEN status = 'rejected' THEN 1 ELSE 0 END), 0) AS rejected,
			COALESCE(SUM(CASE WHEN status = 'draft' THEN 1 ELSE 0 END), 0) AS draft
		FROM products
		WHERE seller_id = $1
	`, sellerID).Scan(
		&resp.Catalog.ProductsTotal,
		&resp.Catalog.ProductsPublished,
		&resp.Catalog.ProductsModeration,
		&resp.Catalog.ProductsRejected,
		&resp.Catalog.ProductsDraft,
	)

	// Inventory stock & Variants
	_ = r.db.QueryRow(ctx, `
		SELECT
			COUNT(id) AS total_variants,
			COALESCE(SUM(CASE WHEN quantity = 0 THEN 1 ELSE 0 END), 0) AS out_of_stock,
			COALESCE(SUM(CASE WHEN quantity > 0 AND quantity <= 5 THEN 1 ELSE 0 END), 0) AS low_stock
		FROM inventory_items
		WHERE seller_id = $1
	`, sellerID).Scan(&resp.Catalog.VariantsTotal, &resp.Catalog.ProductsOutOfStock, &resp.Catalog.ProductsLowStock)

	// 3. Fulfillment
	_ = r.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'new' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'packed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'shipped' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'delivered' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status IN ('problem', 'returned') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status IN ('new', 'processing', 'packed') AND created_at < NOW() - INTERVAL '2 days' THEN 1 ELSE 0 END), 0)
		FROM order_fulfillments
		WHERE seller_id = $1
	`, sellerID).Scan(
		&resp.Fulfillment.FulfillmentsNew,
		&resp.Fulfillment.FulfillmentsProcessing,
		&resp.Fulfillment.FulfillmentsPacked,
		&resp.Fulfillment.FulfillmentsShipped,
		&resp.Fulfillment.FulfillmentsDelivered,
		&resp.Fulfillment.FulfillmentsProblematic,
		&resp.Fulfillment.OverdueFulfillments,
	)
	
	// Average Assembly Time
	_ = r.db.QueryRow(ctx, `
		SELECT COALESCE(EXTRACT(EPOCH FROM AVG(updated_at - created_at))/3600, 0)
		FROM order_fulfillments
		WHERE seller_id = $1 AND status IN ('packed', 'shipped', 'delivered')
	`, sellerID).Scan(&resp.Fulfillment.AverageAssemblyTimeHours)

	// 4. Finance
	resp.Finance.PaidByCustomersCents = grossSales
	_ = r.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'pending' THEN amount_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'paid' THEN amount_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'frozen' THEN amount_cents ELSE 0 END), 0)
		FROM payouts
		WHERE seller_id = $1
	`, sellerID).Scan(&resp.Finance.PendingPayoutCents, &resp.Finance.PaidPayoutCents, &resp.Finance.FrozenCents)
	
	resp.Sales.PendingPayoutCents = resp.Finance.PendingPayoutCents
	
	_ = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM order_refunds
		WHERE order_id IN (SELECT order_id FROM order_fulfillments WHERE seller_id = $1)
		AND status = 'completed'
	`, sellerID).Scan(&resp.Finance.RefundsCents)
	
	resp.Finance.CommissionConfigured = false
	resp.Finance.PlatformCommissionCents = 0 // Stub

	// 5. Quality
	_ = r.db.QueryRow(ctx, `
		SELECT COALESCE(average_rating, 0), reviews_count FROM sellers WHERE id = $1
	`, sellerID).Scan(&resp.Quality.Rating, &resp.Quality.ReviewsCount)

	_ = r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status IN ('resolved', 'cancelled') THEN 1 ELSE 0 END), 0)
		FROM seller_warnings WHERE seller_id = $1
	`, sellerID).Scan(&resp.Quality.WarningsActive, &resp.Quality.WarningsClosed)

	_ = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM seller_violations WHERE seller_id = $1 AND status = 'active'
	`, sellerID).Scan(&resp.Quality.ViolationsActive)

	resp.Quality.RejectedProducts = resp.Catalog.ProductsRejected
	resp.Quality.OpenReturns = resp.Sales.ReturnedOrders // approximation

	// 6. Activity
	var lastLogin, lastUpdate, lastOrder *time.Time
	_ = r.db.QueryRow(ctx, `SELECT MAX(last_login_at) FROM users WHERE seller_id = $1`, sellerID).Scan(&lastLogin)
	_ = r.db.QueryRow(ctx, `SELECT MAX(updated_at) FROM products WHERE seller_id = $1`, sellerID).Scan(&lastUpdate)
	_ = r.db.QueryRow(ctx, `SELECT MAX(created_at) FROM order_fulfillments WHERE seller_id = $1`, sellerID).Scan(&lastOrder)
	
	if lastLogin != nil {
		s := lastLogin.Format(time.RFC3339)
		resp.Activity.LastLoginAt = &s
	}
	if lastUpdate != nil {
		s := lastUpdate.Format(time.RFC3339)
		resp.Activity.LastProductUpdatedAt = &s
	}
	if lastOrder != nil {
		s := lastOrder.Format(time.RFC3339)
		resp.Activity.LastOrderProcessedAt = &s
	}

	// 7. Profile
	d, err := r.GetSellerDetailByID(ctx, sellerID)
	if err == nil && d != nil {
		resp.Profile.StoreCreated = d.BrandName != nil && *d.BrandName != ""
		resp.Profile.StoreStatus = string(d.Status)
		resp.Profile.OwnerAccessStatus = d.OwnerStatus
		resp.Profile.ProfileCompleteness = 100
		if !resp.Profile.StoreCreated {
			resp.Profile.ProfileCompleteness = 50
			resp.Profile.MissingFields = append(resp.Profile.MissingFields, "brandName")
		}
	}
	
	// 8. Performance rules
	var reasons []PerformanceReason
	
	if ordersCount < 10 || !resp.Profile.StoreCreated {
		resp.Performance.PerformanceCategory = "no_data"
	} else {
		// Calculate rules
		if resp.Quality.Rating < 4.0 || resp.Sales.SellerCausedCancellationRate > 15 || resp.Sales.ReturnRateBySellerReason > 15 || resp.Quality.ViolationsActive > 0 {
			resp.Performance.PerformanceCategory = "low"
			if resp.Quality.Rating < 4.0 {
				reasons = append(reasons, PerformanceReason{Code: "low_rating", Label: "Низкий рейтинг", Value: int(resp.Quality.Rating * 10), Unit: "/50"})
			}
			if resp.Sales.SellerCausedCancellationRate > 15 {
				reasons = append(reasons, PerformanceReason{Code: "high_cancellation_rate", Label: "Высокий процент отмен", Value: resp.Sales.SellerCausedCancellationRate, Unit: "%"})
			}
			if resp.Sales.ReturnRateBySellerReason > 15 {
				reasons = append(reasons, PerformanceReason{Code: "high_return_rate", Label: "Высокий процент возвратов", Value: resp.Sales.ReturnRateBySellerReason, Unit: "%"})
			}
			if resp.Quality.ViolationsActive > 0 {
				reasons = append(reasons, PerformanceReason{Code: "active_violations", Label: "Активные нарушения", Value: resp.Quality.ViolationsActive})
			}
		} else if resp.Quality.Rating < 4.5 || resp.Sales.SellerCausedCancellationRate > 5 || resp.Fulfillment.OverdueFulfillments > 0 || resp.Quality.WarningsActive > 0 {
			resp.Performance.PerformanceCategory = "attention"
			if resp.Quality.Rating < 4.5 {
				reasons = append(reasons, PerformanceReason{Code: "needs_rating_improvement", Label: "Рейтинг ниже среднего", Value: int(resp.Quality.Rating * 10), Unit: "/50"})
			}
			if resp.Sales.SellerCausedCancellationRate > 5 {
				reasons = append(reasons, PerformanceReason{Code: "elevated_cancellations", Label: "Повышенный процент отмен", Value: resp.Sales.SellerCausedCancellationRate, Unit: "%"})
			}
			if resp.Fulfillment.OverdueFulfillments > 0 {
				reasons = append(reasons, PerformanceReason{Code: "overdue_fulfillments", Label: "Просроченные сборки", Value: resp.Fulfillment.OverdueFulfillments})
			}
			if resp.Quality.WarningsActive > 0 {
				reasons = append(reasons, PerformanceReason{Code: "active_warnings", Label: "Активные предупреждения", Value: resp.Quality.WarningsActive})
			}
		} else if resp.Quality.Rating >= 4.8 && resp.Sales.SellerCausedCancellationRate < 2 && resp.Sales.ReturnRateBySellerReason < 2 {
			resp.Performance.PerformanceCategory = "high"
		} else {
			resp.Performance.PerformanceCategory = "stable"
		}
	}
	
	if reasons == nil {
		reasons = []PerformanceReason{}
	}
	resp.Performance.PerformanceReasons = reasons

	return resp, nil
}
