package sellers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"regexp"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo     *Repository
	userRepo *users.Repository
	dbClient *postgres.Client
	notifs   *notifications.Service
}

func NewService(repo *Repository, userRepo *users.Repository, dbClient *postgres.Client, notifs *notifications.Service) *Service {
	return &Service{
		repo:     repo,
		userRepo: userRepo,
		dbClient: dbClient,
		notifs:   notifs,
	}
}

func (s *Service) CreateSellerByAdmin(ctx context.Context, req *CreateSellerRequest) (*CreateSellerResponse, error) {
	// 1. Check if user exists
	existingUser, err := s.userRepo.GetUserByEmail(ctx, req.OwnerEmail)
	if err != nil && !errors.Is(err, users.ErrNotFound) {
		return nil, err
	}

	var user *users.User
	var temporaryPassword string
	now := time.Now()
	status := "created_new"
	var generatePassword bool

	if existingUser != nil {
		// User exists. Let's check if they are already a seller
		seller, sellerUser, err := s.repo.GetSellerByUserID(ctx, existingUser.ID)
		if err == nil && sellerUser != nil && seller != nil {
			// Already a seller
			return &CreateSellerResponse{
				Status: "existing_seller",
				Seller: *seller,
				OwnerUser: *existingUser,
			}, ErrSellerAlreadyExists
		}
		if err != nil && !errors.Is(err, ErrSellerNotFound) && !errors.Is(err, ErrSellerUserNotFound) {
			return nil, err
		}

		// They are a user but not a seller.
		if !req.GrantExistingUser {
			return nil, ErrUserExistsPrompt
		}

		user = existingUser
		status = "granted_existing"
		generatePassword = false
	} else {
		// New user
		generatePassword = true
		for {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			temporaryPassword = "Z!1a" + hex.EncodeToString(b)
			if err := auth.ValidatePassword(temporaryPassword, req.OwnerName, req.OwnerEmail); err == nil {
				break
			}
		}

		hashedPassword, err := auth.HashPassword(temporaryPassword)
		if err != nil {
			return nil, err
		}

		user = &users.User{
			ID:                 uuid.New(),
			Name:               req.OwnerName,
			Email:              req.OwnerEmail,
			PasswordHash:       hashedPassword,
			Role:               users.RoleSeller,
			Status:             users.StatusActive,
			MustChangePassword: true,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
	}

	seller := &Seller{
		ID:           uuid.New(),
		BrandName:    nil, // Start with nulls for clean onboarding
		Slug:         nil,
		Description:  nil,
		ContactEmail: nil,
		ContactPhone: nil,
		Status:       StatusPendingSetup,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	sellerUser := &SellerUser{
		ID:        uuid.New(),
		SellerID:  seller.ID,
		UserID:    user.ID,
		Role:      RoleOwner,
		CreatedAt: now,
	}

	err = s.dbClient.RunInTx(ctx, func(tx pgx.Tx) error {
		txUserRepo := s.userRepo.WithTx(tx)
		if generatePassword {
			if errTx := txUserRepo.CreateUser(ctx, user); errTx != nil {
				return errTx
			}
		} else {
			if user.Role == users.RoleCustomer {
				if errTx := txUserRepo.UpdateRole(ctx, user.ID, users.RoleSeller); errTx != nil {
					return errTx
				}
				user.Role = users.RoleSeller
			}
		}

		txSellerRepo := s.repo.WithTx(tx)
		if errTx := txSellerRepo.CreateSeller(ctx, seller); errTx != nil {
			return errTx
		}

		if errTx := txSellerRepo.CreateSellerUser(ctx, sellerUser); errTx != nil {
			return errTx
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if s.notifs != nil && generatePassword {
		_ = s.notifs.SendSellerInvitationEmail(req.OwnerEmail, temporaryPassword)
	}

	return &CreateSellerResponse{
		Status:                    status,
		Seller:                    *seller,
		OwnerUser:                 *user,
		TemporaryPassword:         temporaryPassword,
		TemporaryPasswordReturned: generatePassword,
	}, nil
}

// generateSecurePassword is handled inline.

func CalculatePerformance(s *AdminListSeller) {
	if s.Status == "pending" || s.Status == "pending_setup" || s.BrandName == nil || *s.BrandName == "" || s.OrdersCount30d < 10 {
		s.PerformanceCategory = "no_data"
		s.PerformanceScore = nil
		s.PerformanceReasons = []string{"Недостаточно данных (менее 10 заказов за 30 дней)"}
		return
	}

	score := 100
	reasons := []string{}
	components := []PerformanceComponent{}

	addComp := func(code, label, unit string, raw *float64, sc *int, w float64, exp string) {
		components = append(components, PerformanceComponent{
			Code:        code,
			Label:       label,
			RawValue:    raw,
			Unit:        unit,
			Score:       sc,
			Weight:      w,
			Explanation: exp,
		})
	}

	// 1. sellerCausedCancellationRate
	cancRate := float64(s.CancelRate30d)
	cancScore := 100 - (s.CancelRate30d * 2)
	if cancScore < 0 {
		cancScore = 0
	}
	score -= (100 - cancScore)
	if cancRate > 0 {
		reasons = append(reasons, fmt.Sprintf("Высокий процент отмен (%.0f%%)", cancRate))
	}
	addComp("sellerCausedCancellationRate", "Отмены по вине продавца", "%", &cancRate, &cancScore, 0.3, "Снижает общий рейтинг. Цель: 0%")

	// 2. rating
	if s.ReviewsCount > 5 {
		r := s.AverageRating
		rs := 100
		if r < 4.0 {
			rs = 70
			score -= 30
			reasons = append(reasons, "Низкий рейтинг покупателей")
		} else if r < 4.5 {
			rs = 90
			score -= 10
			reasons = append(reasons, "Рейтинг ниже 4.5")
		}
		addComp("rating", "Рейтинг товаров", "⭐", &r, &rs, 0.2, "На основе отзывов покупателей")
	} else {
		// Not penalized for missing reviews
		addComp("rating", "Рейтинг товаров", "⭐", nil, nil, 0.0, "Недостаточно отзывов")
	}

	// 3. warnings
	warnCount := float64(s.WarningsActive)
	warnScore := 100 - (s.WarningsActive * 10)
	if warnScore < 0 {
		warnScore = 0
	}
	if s.WarningsActive > 0 {
		score -= s.WarningsActive * 10
		reasons = append(reasons, fmt.Sprintf("Есть активные предупреждения (%d)", s.WarningsActive))
	}
	addComp("warnings", "Предупреждения", "шт", &warnCount, &warnScore, 0.1, "Влияет на общий рейтинг")

	// 4. violations
	violCount := float64(s.Violations)
	violScore := 100 - (s.Violations * 20)
	if violScore < 0 {
		violScore = 0
	}
	if s.Violations > 0 {
		score -= s.Violations * 20
		reasons = append(reasons, fmt.Sprintf("Есть активные нарушения (%d)", s.Violations))
	}
	addComp("violations", "Нарушения", "шт", &violCount, &violScore, 0.2, "Серьезно снижает рейтинг")

	// Placeholder metrics that are technically unavailable right now
	addComp("assemblyOnTimeRate", "Сборка вовремя", "%", nil, nil, 0.1, "Данные временно недоступны")
	addComp("overdueFulfillments", "Просроченные отгрузки", "шт", nil, nil, 0.05, "Данные временно недоступны")
	addComp("averageAssemblyTimeHours", "Среднее время сборки", "ч", nil, nil, 0.05, "Данные временно недоступны")
	addComp("handoverOnTimeRate", "Передача в доставку вовремя", "%", nil, nil, 0.0, "Данные временно недоступны")

	if score < 0 {
		score = 0
	}

	s.PerformanceScore = &score

	if score >= 90 && len(reasons) == 0 {
		s.PerformanceCategory = "high"
	} else if score >= 70 {
		s.PerformanceCategory = "stable"
	} else if score >= 50 {
		s.PerformanceCategory = "attention"
	} else {
		s.PerformanceCategory = "low"
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "Отличные показатели работы")
	}

	s.PerformanceReasons = reasons
	s.PerformanceComponents = components
}

func (s *Service) ListSellers(ctx context.Context, filter SellersFilter) (*ListSellersResponse, error) {
	if filter.Limit <= 0 {
		filter.Limit = 25
	}

	items, total, err := s.repo.ListSellers(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Apply performance calculation
	for i := range items {
		CalculatePerformance(&items[i])
	}

	// Post-filtering for performance if necessary
	postFilterPerformance := filter.PerformanceMin != nil || filter.PerformanceMax != nil || filter.PerformanceCategory != "" || filter.Sort == "performance_score" || filter.Sort == "performance"
	
	if postFilterPerformance {
		var filtered []AdminListSeller
		for _, item := range items {
			match := true
			if filter.PerformanceCategory != "" && item.PerformanceCategory != filter.PerformanceCategory {
				match = false
			}
			if filter.PerformanceMin != nil && (item.PerformanceScore == nil || *item.PerformanceScore < *filter.PerformanceMin) {
				match = false
			}
			if filter.PerformanceMax != nil && (item.PerformanceScore == nil || *item.PerformanceScore > *filter.PerformanceMax) {
				match = false
			}
			if match {
				filtered = append(filtered, item)
			}
		}

		// Sort
		if filter.Sort == "performance_score" || filter.Sort == "performance" {
			sort.Slice(filtered, func(i, j int) bool {
				s1 := 0
				if filtered[i].PerformanceScore != nil {
					s1 = *filtered[i].PerformanceScore
				}
				s2 := 0
				if filtered[j].PerformanceScore != nil {
					s2 = *filtered[j].PerformanceScore
				}
				if filter.Direction == "asc" {
					return s1 < s2
				}
				return s1 > s2
			})
		}
		
		total = len(filtered)
		
		// Apply Limit / Offset
		start := filter.Offset
		end := filter.Offset + filter.Limit
		if start > total {
			start = total
		}
		if end > total {
			end = total
		}
		items = filtered[start:end]
	}

	counts, err := s.repo.GetAdminSellersStatusCounts(ctx, filter)
	if err != nil {
		return nil, err
	}

	allCount := 0
	for _, c := range counts {
		allCount += c
	}
	
	statusCounts := map[string]int{
		"all":            allCount,
		"pending_setup":  counts["pending_setup"],
		"pending_review": counts["pending_review"],
		"active":         counts["active"],
		"blocked":        counts["blocked"],
		"archived":       counts["archived"],
	}

	page := (filter.Offset / filter.Limit) + 1
	totalPages := (total + filter.Limit - 1) / filter.Limit

	return &ListSellersResponse{
		Items:        items,
		TotalCount:   total,
		Page:         page,
		Limit:        filter.Limit,
		TotalPages:   totalPages,
		StatusCounts: statusCounts,
	}, nil
}

func (s *Service) UpdateSellerStatus(ctx context.Context, id uuid.UUID, req *UpdateSellerStatusRequest) error {
	switch req.Status {
	case StatusPendingSetup, StatusPending, StatusActive, StatusBlocked, StatusArchived:
		// Valid
	default:
		return errors.New("invalid status")
	}

	return s.repo.UpdateSellerStatus(ctx, id, req.Status)
}

func (s *Service) UpdateSellerProfile(ctx context.Context, currentUserID uuid.UUID, req *UpdateSellerProfileRequest) (*SellerMeResponse, error) {
	seller, _, err := s.repo.GetSellerByUserID(ctx, currentUserID)
	if err != nil {
		return nil, err
	}

	if req.Slug != nil && *req.Slug != "" {
		existing, slugErr := s.repo.GetSellerBySlug(ctx, *req.Slug)
		if slugErr == nil && existing != nil && existing.ID != seller.ID {
			return nil, ErrDuplicateSlug
		}
	}

	if err := s.repo.UpdateSellerProfile(ctx, seller.ID, req); err != nil {
		return nil, err
	}

	return s.GetSellerMe(ctx, currentUserID)
}

func (s *Service) CompleteOnboarding(ctx context.Context, currentUserID uuid.UUID) error {
	seller, _, err := s.repo.GetSellerByUserID(ctx, currentUserID)
	if err != nil {
		return err
	}

	if seller.Status != StatusPendingSetup {
		return errors.New("seller is not in pending_setup status")
	}

	err = s.repo.UpdateSellerStatus(ctx, seller.ID, StatusPending)
	if err == nil && s.notifs != nil {
		brandName := "Магазин"
		if seller.BrandName != nil {
			brandName = *seller.BrandName
		}
		_ = s.dbClient.RunInTx(ctx, func(tx pgx.Tx) error {
			return s.notifs.CreateStaffNotificationTx(ctx, tx, notifications.Notification{
				Type:       notifications.TypeSellerOnboardingCompleted,
				Title:      "Продавец завершил настройку",
				Body:       "Продавец " + brandName + " ожидает верификации.",
				EntityType: "seller",
				EntityID:   seller.ID,
			})
		})
	}
	return err
}

func (s *Service) GetSellerMe(ctx context.Context, currentUserID uuid.UUID) (*SellerMeResponse, error) {
	seller, sellerUser, err := s.repo.GetSellerByUserID(ctx, currentUserID)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetUserByID(ctx, currentUserID)
	if err != nil {
		return nil, err
	}

	return &SellerMeResponse{
		Seller:     *seller,
		SellerUser: *sellerUser,
		User:       *user,
	}, nil
}

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}

// ---- Phase E: Seller Management Service Methods ----

const (
	baseCommissionBps    = 900  // 9%
	penaltyCommissionBps = 1800 // 18%
)

// GetSellerDetail returns full seller detail for admin view.
func (s *Service) GetSellerDetail(ctx context.Context, sellerID uuid.UUID) (*SellerDetailResponse, error) {
	d, err := s.repo.GetSellerDetailByID(ctx, sellerID)
	if err != nil {
		return nil, err
	}

	resp := &SellerDetailResponse{
		ID:        d.ID,
		BrandName: d.BrandName,
		Status:    string(d.Status),
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
	if d.Slug != nil && *d.Slug != "" {
		resp.Slug = d.Slug
	}
	resp.Description = d.Description
	resp.LogoURL = d.LogoURL
	if d.ContactEmail != nil && *d.ContactEmail != "" {
		resp.ContactEmail = d.ContactEmail
	}
	resp.ContactPhone = d.ContactPhone

	resp.Owner.ID = d.OwnerID
	resp.Owner.Name = d.OwnerName
	resp.Owner.Email = d.OwnerEmail
	resp.Owner.Status = d.OwnerStatus

	resp.Counts.WarningsActive = d.WarningsActive
	resp.Counts.ViolationsActive = d.ViolationsActive
	resp.Counts.ActivePenaltyViolations = d.ActivePenaltyViolations

	resp.CommissionPolicy.BaseCommissionBps = baseCommissionBps
	resp.CommissionPolicy.PenaltyCommissionBps = penaltyCommissionBps
	resp.CommissionPolicy.PenaltyRule = "2 active penalty violations triggers 18% for 1 month"
	if d.ActivePenaltyViolations >= 2 {
		resp.CommissionPolicy.CurrentAppliedCommissionBps = penaltyCommissionBps
		resp.CommissionPolicy.AutomaticPenaltyEnabled = true
	} else {
		resp.CommissionPolicy.CurrentAppliedCommissionBps = baseCommissionBps
		resp.CommissionPolicy.AutomaticPenaltyEnabled = false
	}

	return resp, nil
}

func (s *Service) GetSellerOverview(ctx context.Context, sellerID uuid.UUID, period string) (*SellerOverviewResponse, error) {
	return s.repo.GetSellerOverview(ctx, sellerID, period)
}

// UpdateSellerStatusWithHistory changes seller status and writes status history.
func (s *Service) UpdateSellerStatusWithHistory(ctx context.Context, sellerID uuid.UUID, newStatus string, reason *string, actorUserID uuid.UUID) error {
	switch SellerStatus(newStatus) {
	case StatusPendingSetup, StatusPending, StatusActive, StatusBlocked, StatusArchived:
		// valid
	default:
		return errors.New("invalid status")
	}

	if (newStatus == string(StatusBlocked) || newStatus == string(StatusArchived)) && (reason == nil || *reason == "") {
		return ErrReasonRequired
	}

	seller, err := s.repo.GetSellerByID(ctx, sellerID)
	if err != nil {
		return err
	}

	if seller.IsPlatform && (newStatus == string(StatusBlocked) || newStatus == string(StatusArchived)) {
		return errors.New("Системного продавца нельзя заблокировать или удалить")
	}

	oldStatus := string(seller.Status)

	if err := s.repo.UpdateSellerStatus(ctx, sellerID, SellerStatus(newStatus)); err != nil {
		return err
	}

	actor := actorUserID
	return s.repo.WriteStatusHistory(ctx, sellerID, &oldStatus, newStatus, reason, &actor)
}

// VerifySeller verifies a pending seller.
func (s *Service) VerifySeller(ctx context.Context, sellerID uuid.UUID, actorUserID uuid.UUID) (*VerifySellerResponse, error) {
	seller, err := s.repo.GetSellerByID(ctx, sellerID)
	if err != nil {
		return nil, err
	}

	if seller.Status != StatusPending {
		return nil, ErrSellerNotPending
	}

	var missing []string
	if seller.BrandName == nil || *seller.BrandName == "" {
		missing = append(missing, "brandName")
	}
	if seller.Slug == nil || *seller.Slug == "" {
		missing = append(missing, "slug")
	}
	if seller.Description == nil || len(*seller.Description) < 10 {
		missing = append(missing, "description")
	}
	if (seller.ContactEmail == nil || *seller.ContactEmail == "") && (seller.ContactPhone == nil || *seller.ContactPhone == "") {
		missing = append(missing, "contactEmail or contactPhone")
	}

	if len(missing) > 0 {
		return nil, &VerifyMissingFieldsError{Fields: missing}
	}

	oldStatus := string(seller.Status)
	if err := s.repo.UpdateSellerStatus(ctx, sellerID, StatusActive); err != nil {
		return nil, err
	}

	actor := actorUserID
	_ = s.repo.WriteStatusHistory(ctx, sellerID, &oldStatus, string(StatusActive), nil, &actor)

	return &VerifySellerResponse{
		SellerID: sellerID,
		Status:   string(StatusActive),
	}, nil
}

// GetStatusHistory returns seller status timeline.
func (s *Service) GetStatusHistory(ctx context.Context, sellerID uuid.UUID) ([]SellerStatusHistoryItem, error) {
	return s.repo.GetStatusHistory(ctx, sellerID)
}

// CreateWarning creates a seller warning.
func (s *Service) CreateWarning(ctx context.Context, sellerID uuid.UUID, req CreateWarningRequest, actorUserID uuid.UUID) (*WarningResponse, error) {
	actor := actorUserID
	return s.repo.CreateWarning(ctx, CreateWarningInput{
		SellerID:    sellerID,
		Type:        req.Type,
		Title:       req.Title,
		Message:     req.Message,
		Severity:    req.Severity,
		ActorUserID: &actor,
	})
}

// ListWarnings lists warnings for a seller.
func (s *Service) ListWarnings(ctx context.Context, sellerID uuid.UUID) ([]WarningResponse, error) {
	return s.repo.ListWarnings(ctx, sellerID)
}

// ResolveWarning resolves a warning.
func (s *Service) ResolveWarning(ctx context.Context, sellerID uuid.UUID, warningID uuid.UUID, req ResolveWarningRequest, actorUserID uuid.UUID) error {
	actor := actorUserID
	return s.repo.UpdateWarningStatus(ctx, warningID, "resolved", &actor, req.ResolutionNote)
}

// CancelWarning cancels a warning.
func (s *Service) CancelWarning(ctx context.Context, sellerID uuid.UUID, warningID uuid.UUID, actorUserID uuid.UUID) error {
	actor := actorUserID
	return s.repo.UpdateWarningStatus(ctx, warningID, "cancelled", &actor, nil)
}

// CreateViolation creates a seller violation.
func (s *Service) CreateViolation(ctx context.Context, sellerID uuid.UUID, req CreateViolationRequest, actorUserID uuid.UUID) (*ViolationResponse, error) {
	actor := actorUserID
	return s.repo.CreateViolation(ctx, CreateViolationInput{
		SellerID:         sellerID,
		Type:             req.Type,
		Title:            req.Title,
		Description:      req.Description,
		Severity:         req.Severity,
		CountsForPenalty: req.CountsForPenalty,
		ActorUserID:      &actor,
	})
}

// ListViolations lists violations for a seller.
func (s *Service) ListViolations(ctx context.Context, sellerID uuid.UUID) ([]ViolationResponse, error) {
	return s.repo.ListViolations(ctx, sellerID)
}

// ResolveViolation resolves a violation.
func (s *Service) ResolveViolation(ctx context.Context, sellerID uuid.UUID, violationID uuid.UUID, req ResolveViolationRequest, actorUserID uuid.UUID) error {
	actor := actorUserID
	return s.repo.UpdateViolationStatus(ctx, violationID, "resolved", &actor, req.ResolutionNote)
}

// CancelViolation cancels a violation.
func (s *Service) CancelViolation(ctx context.Context, sellerID uuid.UUID, violationID uuid.UUID, actorUserID uuid.UUID) error {
	actor := actorUserID
	return s.repo.UpdateViolationStatus(ctx, violationID, "cancelled", &actor, nil)
}

// ResetOwnerPassword generates a new temporary password for the seller owner
func (s *Service) ResetOwnerPassword(ctx context.Context, sellerID uuid.UUID) (string, error) {
	seller, err := s.repo.GetSellerByID(ctx, sellerID)
	if err != nil {
		return "", err
	}

	d, err := s.repo.GetSellerDetailByID(ctx, seller.ID)
	if err != nil {
		return "", err
	}

	tempPassword := uuid.New().String()[:12]
	hashedPassword, err := auth.HashPassword(tempPassword)
	if err != nil {
		return "", err
	}

	user, err := s.userRepo.GetUserByID(ctx, d.OwnerID)
	if err != nil {
		return "", err
	}

	err = s.userRepo.UpdatePasswordAndMustChange(ctx, user.ID, hashedPassword, true)
	if err != nil {
		return "", err
	}

	return tempPassword, nil
}
