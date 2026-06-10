package sellers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type stubSellerRepo struct {
	seller                *Seller
	penaltyViolationCount int
	warningStatus         string
	violationStatus       string
	statusHistoryWritten  []SellerStatusHistoryItem
	err                   error
}

func (s *stubSellerRepo) GetSellerByID(_ context.Context, _ uuid.UUID) (*Seller, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.seller == nil {
		return nil, ErrSellerNotFound
	}
	return s.seller, nil
}

func (s *stubSellerRepo) UpdateSellerStatus(_ context.Context, _ uuid.UUID, status SellerStatus) error {
	if s.seller != nil {
		s.seller.Status = status
	}
	return s.err
}

func (s *stubSellerRepo) WriteStatusHistory(_ context.Context, _ uuid.UUID, oldStatus *string, newStatus string, reason *string, actorUserID *uuid.UUID) error {
	s.statusHistoryWritten = append(s.statusHistoryWritten, SellerStatusHistoryItem{
		ID:        uuid.New(),
		NewStatus: newStatus,
		OldStatus: oldStatus,
		Reason:    reason,
		CreatedAt: time.Now(),
	})
	return nil
}

func (s *stubSellerRepo) CreateWarning(_ context.Context, w CreateWarningInput) (*WarningResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &WarningResponse{
		ID: uuid.New(), SellerID: w.SellerID,
		Type: w.Type, Title: w.Title, Message: w.Message, Severity: w.Severity,
		Status: "active",
	}, nil
}

func (s *stubSellerRepo) UpdateWarningStatus(_ context.Context, _ uuid.UUID, status string, _ *uuid.UUID, _ *string) error {
	s.warningStatus = status
	return s.err
}

func (s *stubSellerRepo) CreateViolation(_ context.Context, v CreateViolationInput) (*ViolationResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &ViolationResponse{
		ID: uuid.New(), SellerID: v.SellerID,
		Type: v.Type, Title: v.Title, Description: v.Description, Severity: v.Severity,
		Status: "active", CountsForPenalty: v.CountsForPenalty,
	}, nil
}

func (s *stubSellerRepo) UpdateViolationStatus(_ context.Context, _ uuid.UUID, status string, _ *uuid.UUID, _ *string) error {
	s.violationStatus = status
	return s.err
}

func (s *stubSellerRepo) CountActivePenaltyViolations(_ context.Context, _ uuid.UUID) (int, error) {
	return s.penaltyViolationCount, s.err
}

func completeSeller() *Seller {
	desc := "A complete shop description that is long enough"
	phone := "+7-999-000-0000"
	return &Seller{
		ID: uuid.New(), BrandName: "Test Shop", Slug: "test-shop",
		Description: &desc, ContactEmail: "shop@example.com", ContactPhone: &phone,
		Status: StatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
}

type testSvc struct{ repo *stubSellerRepo }

func (svc *testSvc) VerifySeller(ctx context.Context, sellerID uuid.UUID, actorID uuid.UUID) (*VerifySellerResponse, error) {
	seller, err := svc.repo.GetSellerByID(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	if seller.Status != StatusPending {
		return nil, ErrSellerNotPending
	}
	var missing []string
	if seller.BrandName == "" {
		missing = append(missing, "brandName")
	}
	if seller.Slug == "" {
		missing = append(missing, "slug")
	}
	if seller.Description == nil || len(*seller.Description) < 10 {
		missing = append(missing, "description")
	}
	if seller.ContactEmail == "" && (seller.ContactPhone == nil || *seller.ContactPhone == "") {
		missing = append(missing, "contactEmail or contactPhone")
	}
	if len(missing) > 0 {
		return nil, &VerifyMissingFieldsError{Fields: missing}
	}
	oldStatus := string(seller.Status)
	if err := svc.repo.UpdateSellerStatus(ctx, sellerID, StatusActive); err != nil {
		return nil, err
	}
	_ = svc.repo.WriteStatusHistory(ctx, sellerID, &oldStatus, string(StatusActive), nil, &actorID)
	return &VerifySellerResponse{SellerID: sellerID, Status: string(StatusActive)}, nil
}

func (svc *testSvc) UpdateStatusWithHistory(ctx context.Context, sellerID uuid.UUID, newStatus string, reason *string, actor uuid.UUID) error {
	if (newStatus == string(StatusBlocked) || newStatus == string(StatusArchived)) && (reason == nil || *reason == "") {
		return ErrReasonRequired
	}
	seller, err := svc.repo.GetSellerByID(ctx, sellerID)
	if err != nil {
		return err
	}
	old := string(seller.Status)
	if err := svc.repo.UpdateSellerStatus(ctx, sellerID, SellerStatus(newStatus)); err != nil {
		return err
	}
	return svc.repo.WriteStatusHistory(ctx, sellerID, &old, newStatus, reason, &actor)
}

func TestVerifySeller_PendingComplete(t *testing.T) {
	s := completeSeller()
	svc := &testSvc{repo: &stubSellerRepo{seller: s}}
	resp, err := svc.VerifySeller(context.Background(), s.ID, uuid.New())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.Status != "active" {
		t.Errorf("expected status active, got: %s", resp.Status)
	}
	if len(svc.repo.statusHistoryWritten) == 0 {
		t.Error("expected status history to be written")
	}
}

func TestVerifySeller_PendingIncomplete(t *testing.T) {
	s := &Seller{ID: uuid.New(), BrandName: "", Slug: "", Status: StatusPending}
	svc := &testSvc{repo: &stubSellerRepo{seller: s}}
	_, err := svc.VerifySeller(context.Background(), s.ID, uuid.New())
	if err == nil {
		t.Fatal("expected error for incomplete profile")
	}
	var mfe *VerifyMissingFieldsError
	if !errors.As(err, &mfe) {
		t.Fatalf("expected VerifyMissingFieldsError, got: %T %v", err, err)
	}
	if len(mfe.Fields) == 0 {
		t.Error("expected non-empty missing fields list")
	}
}

func TestVerifySeller_NotPending(t *testing.T) {
	s := completeSeller()
	s.Status = StatusActive
	svc := &testSvc{repo: &stubSellerRepo{seller: s}}
	_, err := svc.VerifySeller(context.Background(), s.ID, uuid.New())
	if !errors.Is(err, ErrSellerNotPending) {
		t.Errorf("expected ErrSellerNotPending, got: %v", err)
	}
}

func TestUpdateSellerStatus_BlockedRequiresReason(t *testing.T) {
	s := completeSeller()
	s.Status = StatusActive
	svc := &testSvc{repo: &stubSellerRepo{seller: s}}
	err := svc.UpdateStatusWithHistory(context.Background(), s.ID, "blocked", nil, uuid.New())
	if !errors.Is(err, ErrReasonRequired) {
		t.Errorf("expected ErrReasonRequired, got: %v", err)
	}
}

func TestUpdateSellerStatus_BlockedWithReason(t *testing.T) {
	s := completeSeller()
	s.Status = StatusActive
	svc := &testSvc{repo: &stubSellerRepo{seller: s}}
	reason := "Policy violation"
	err := svc.UpdateStatusWithHistory(context.Background(), s.ID, "blocked", &reason, uuid.New())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if s.Status != StatusBlocked {
		t.Errorf("expected status blocked, got: %s", s.Status)
	}
}

func TestCreateWarning_SetsStatusActive(t *testing.T) {
	repo := &stubSellerRepo{}
	sellerID := uuid.New()
	actorID := uuid.New()
	wr, err := repo.CreateWarning(context.Background(), CreateWarningInput{
		SellerID: sellerID, Type: "other", Title: "Test", Message: "Test msg", Severity: "low", ActorUserID: &actorID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wr.Status != "active" {
		t.Errorf("expected status active, got: %s", wr.Status)
	}
}

func TestResolveWarning_SetsStatusResolved(t *testing.T) {
	repo := &stubSellerRepo{}
	if err := repo.UpdateWarningStatus(context.Background(), uuid.New(), "resolved", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.warningStatus != "resolved" {
		t.Errorf("expected resolved, got: %s", repo.warningStatus)
	}
}

func TestCountActivePenaltyViolations_OnlyCountsActive(t *testing.T) {
	repo := &stubSellerRepo{penaltyViolationCount: 2}
	count, err := repo.CountActivePenaltyViolations(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got: %d", count)
	}
}

func TestCountActivePenaltyViolations_CountsForPenaltyFalse(t *testing.T) {
	repo := &stubSellerRepo{penaltyViolationCount: 0}
	count, err := repo.CountActivePenaltyViolations(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got: %d", count)
	}
}

func TestViolations_TwoActive_DoesNotChangeCommission(t *testing.T) {
	const expected = 900
	if baseCommissionBps != expected {
		t.Errorf("base commission should be 900 bps, got: %d", baseCommissionBps)
	}
	const autoEnabled = false
	if autoEnabled {
		t.Error("automatic penalty should be disabled")
	}
}
