package billing

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubRepo struct {
	listFn                   func(ctx context.Context, params ListChargesQuery) ([]models.Charge, *pagination.Cursor, error)
	listBillingPlansFn       func(ctx context.Context, params ListBillingPlansQuery) ([]models.BillingPlan, error)
	findDefaultBillingPlanFn func(ctx context.Context) (*models.BillingPlan, error)
}

func (s *stubRepo) WithTx(tx *gorm.DB) Repository { return s }
func (s *stubRepo) CreateSubscription(ctx context.Context, subscription *models.Subscription) error {
	return nil
}
func (s *stubRepo) UpdateSubscription(ctx context.Context, subscription *models.Subscription) error {
	return nil
}
func (s *stubRepo) ListSubscriptionsByStore(ctx context.Context, storeID uuid.UUID) ([]models.Subscription, error) {
	return nil, nil
}
func (s *stubRepo) FindSubscription(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	return nil, nil
}
func (s *stubRepo) FindSubscriptionBySquareID(ctx context.Context, squareSubscriptionID string) (*models.Subscription, error) {
	return nil, nil
}
func (s *stubRepo) CreateBillingPlan(ctx context.Context, plan *models.BillingPlan) error {
	return nil
}
func (s *stubRepo) UpdateBillingPlan(ctx context.Context, plan *models.BillingPlan) error {
	return nil
}
func (s *stubRepo) ListBillingPlans(ctx context.Context, params ListBillingPlansQuery) ([]models.BillingPlan, error) {
	if s.listBillingPlansFn != nil {
		return s.listBillingPlansFn(ctx, params)
	}
	return nil, nil
}
func (s *stubRepo) FindBillingPlanByID(ctx context.Context, id string) (*models.BillingPlan, error) {
	return nil, nil
}
func (s *stubRepo) FindBillingPlanBySquareID(ctx context.Context, squareBillingPlanID string) (*models.BillingPlan, error) {
	return nil, nil
}
func (s *stubRepo) FindDefaultBillingPlan(ctx context.Context) (*models.BillingPlan, error) {
	if s.findDefaultBillingPlanFn != nil {
		return s.findDefaultBillingPlanFn(ctx)
	}
	return nil, nil
}
func (s *stubRepo) CreatePaymentMethod(ctx context.Context, method *models.PaymentMethod) error {
	return nil
}
func (s *stubRepo) ListPaymentMethodsByStore(ctx context.Context, storeID uuid.UUID) ([]models.PaymentMethod, error) {
	return nil, nil
}
func (s *stubRepo) ClearDefaultPaymentMethod(ctx context.Context, storeID uuid.UUID) error {
	return nil
}
func (s *stubRepo) CreateCharge(ctx context.Context, charge *models.Charge) error {
	return nil
}
func (s *stubRepo) ListCharges(ctx context.Context, params ListChargesQuery) ([]models.Charge, *pagination.Cursor, error) {
	if s.listFn != nil {
		return s.listFn(ctx, params)
	}
	return nil, nil, nil
}
func (s *stubRepo) CreateUsageCharge(ctx context.Context, usage *models.UsageCharge) error {
	return nil
}
func (s *stubRepo) ListUsageChargesByStore(ctx context.Context, storeID uuid.UUID) ([]models.UsageCharge, error) {
	return nil, nil
}

func TestServiceListChargesRequiresStore(t *testing.T) {
	svc, _ := NewService(ServiceParams{Repo: &stubRepo{}})
	if _, err := svc.ListCharges(context.Background(), ListChargesParams{}); err == nil {
		t.Fatal("expected error when store id is missing")
	} else if pkgerrors.As(err).Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestServiceListChargesInvalidCursor(t *testing.T) {
	svc, _ := NewService(ServiceParams{Repo: &stubRepo{}})
	_, err := svc.ListCharges(context.Background(), ListChargesParams{
		StoreID: uuid.New(),
		Cursor:  "not-a-cursor",
	})
	if err == nil {
		t.Fatalf("expected error for invalid cursor")
	} else if pkgerrors.As(err).Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestServiceListChargesReturnsCursor(t *testing.T) {
	now := time.Now().UTC()
	next := pagination.Cursor{
		CreatedAt: now.Add(-time.Hour),
		ID:        uuid.New(),
	}

	captured := ListChargesQuery{}
	repo := &stubRepo{
		listFn: func(ctx context.Context, params ListChargesQuery) ([]models.Charge, *pagination.Cursor, error) {
			captured = params
			return []models.Charge{
				{
					ID:        uuid.New(),
					CreatedAt: now,
				},
			}, &next, nil
		},
	}

	svc, _ := NewService(ServiceParams{Repo: repo})
	typ := enums.ChargeTypeAdSpend
	status := enums.ChargeStatusSucceeded
	result, err := svc.ListCharges(context.Background(), ListChargesParams{
		StoreID: uuid.New(),
		Limit:   5,
		Cursor:  pagination.EncodeCursor(next),
		Type:    &typ,
		Status:  &status,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Limit != 5 {
		t.Fatalf("expected limit 5, got %d", captured.Limit)
	}
	if captured.Type == nil || *captured.Type != typ {
		t.Fatal("type filter not forwarded")
	}
	if captured.Status == nil || *captured.Status != status {
		t.Fatal("status filter not forwarded")
	}

	expectedCursor := pagination.EncodeCursor(next)
	if result.Cursor != expectedCursor {
		t.Fatalf("expected cursor %s, got %s", expectedCursor, result.Cursor)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 charge, got %d", len(result.Items))
	}
}

func TestServiceListBillingPlansForwardsFilters(t *testing.T) {
	status := enums.PlanStatusActive
	isDefault := true

	var captured ListBillingPlansQuery
	repo := &stubRepo{
		listBillingPlansFn: func(ctx context.Context, params ListBillingPlansQuery) ([]models.BillingPlan, error) {
			captured = params
			return []models.BillingPlan{
				{ID: "starter_v1"},
			}, nil
		},
	}

	svc, _ := NewService(ServiceParams{Repo: repo})
	plans, err := svc.ListBillingPlans(context.Background(), ListBillingPlansParams{
		Status:    &status,
		IsDefault: &isDefault,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Status == nil || *captured.Status != status {
		t.Fatal("status filter not forwarded")
	}
	if captured.IsDefault == nil || !*captured.IsDefault {
		t.Fatal("isDefault filter not forwarded")
	}

	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
}

func TestServiceFindDefaultBillingPlanReturnsRepoResult(t *testing.T) {
	expected := &models.BillingPlan{ID: "starter_v1"}
	repo := &stubRepo{
		findDefaultBillingPlanFn: func(ctx context.Context) (*models.BillingPlan, error) {
			return expected, nil
		},
	}

	svc, _ := NewService(ServiceParams{Repo: repo})
	result, err := svc.FindDefaultBillingPlan(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expected {
		t.Fatalf("expected plan %v, got %v", expected, result)
	}
}

func TestServiceFindBillingPlanByIDRequiresID(t *testing.T) {
	svc, _ := NewService(ServiceParams{Repo: &stubRepo{}})
	if _, err := svc.FindBillingPlanByID(context.Background(), ""); err == nil {
		t.Fatal("expected validation error for missing id")
	} else if pkgerrors.As(err).Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}
