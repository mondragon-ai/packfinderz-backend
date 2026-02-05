package subscriptions

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestServiceCreateReturnsExisting(t *testing.T) {
	storeID := uuid.New()
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		SquareSubscriptionID: "sub-existing",
	}
	billingRepo := &stubBillingRepo{
		existing: existing,
	}
	store := &models.Store{SubscriptionActive: true}
	svc, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         &stubStoreRepo{store: store},
		SquareClient:      &stubSquareSubscriptionClient{},
		DefaultPriceID:    "price-default",
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, created, err := svc.Create(context.Background(), storeID, CreateSubscriptionInput{
		SquareCustomerID:      "cust-1",
		SquarePaymentMethodID: "pm-1",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if created {
		t.Fatalf("expected existing subscription, got create")
	}
	if sub == nil || sub.SquareSubscriptionID != "sub-existing" {
		t.Fatalf("expected existing subscription returned")
	}
	if len(billingRepo.created) != 0 {
		t.Fatalf("subscription should not be created")
	}
}

func TestServiceCreatesNewSubscription(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: false}
	billingRepo := &stubBillingRepo{}
	squareClient := &stubSquareSubscriptionClient{
		createResp: &SquareSubscription{
			ID:     "sub-new",
			Status: "ACTIVE",
			Metadata: map[string]string{
				"store_id":                 storeID.String(),
				"square_customer_id":       "cust",
				"square_payment_method_id": "pm",
			},
			Items: &SquareSubscriptionItemList{
				Data: []*SquareSubscriptionItem{
					{CurrentPeriodStart: 1, CurrentPeriodEnd: 2},
				},
			},
		},
	}
	svc, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         &stubStoreRepo{store: store},
		SquareClient:      squareClient,
		DefaultPriceID:    "price-default",
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, created, err := svc.Create(context.Background(), storeID, CreateSubscriptionInput{
		SquareCustomerID:      "cust-1",
		SquarePaymentMethodID: "pm-1",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !created {
		t.Fatalf("expected creation")
	}
	if sub == nil || sub.SquareSubscriptionID != "sub-new" {
		t.Fatalf("unexpected subscription returned")
	}
	if len(billingRepo.created) != 1 {
		t.Fatalf("expected subscription row created")
	}
	if !store.SubscriptionActive {
		t.Fatalf("expected store flagged active")
	}
	if !squareClient.calledCreate {
		t.Fatalf("square client create not invoked")
	}
}

func TestServiceCancelsSubscription(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		SquareSubscriptionID: "sub-cancel",
	}
	billingRepo := &stubBillingRepo{existing: existing}
	squareClient := &stubSquareSubscriptionClient{
		cancelResp: &SquareSubscription{
			ID:     "sub-cancel",
			Status: "CANCELED",
			Metadata: map[string]string{
				"store_id": storeID.String(),
			},
		},
	}
	svc, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         &stubStoreRepo{store: store},
		SquareClient:      squareClient,
		DefaultPriceID:    "price-default",
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := svc.Cancel(context.Background(), storeID); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if len(billingRepo.updated) == 0 {
		t.Fatalf("expected subscription update")
	}
	if billingRepo.updated[0].Status != enums.SubscriptionStatusCanceled {
		t.Fatalf("expected canceled status, got %s", billingRepo.updated[0].Status)
	}
	if squareClient.calledCancel == false {
		t.Fatalf("square cancel not invoked")
	}
}

func TestServicePausesSubscription(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		SquareSubscriptionID: "sub-pause",
	}
	billingRepo := &stubBillingRepo{existing: existing}
	squareClient := &stubSquareSubscriptionClient{
		pauseResp: &SquareSubscription{
			ID:     "sub-pause",
			Status: "PAUSED",
			Metadata: map[string]string{
				"store_id": storeID.String(),
			},
		},
	}
	svc, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         &stubStoreRepo{store: store},
		SquareClient:      squareClient,
		DefaultPriceID:    "price-default",
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := svc.Pause(context.Background(), storeID); err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	if len(billingRepo.updated) == 0 {
		t.Fatalf("expected subscription update")
	}
	if billingRepo.updated[0].Status != enums.SubscriptionStatusPaused {
		t.Fatalf("expected paused status, got %s", billingRepo.updated[0].Status)
	}
	if squareClient.calledPause == false {
		t.Fatalf("square pause not invoked")
	}
	if store.SubscriptionActive {
		t.Fatalf("expected store flag false after pause")
	}
	if billingRepo.updated[0].PausedAt == nil {
		t.Fatalf("expected paused timestamp set")
	}
}

func TestServiceResumesSubscription(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: false}
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusPaused,
		SquareSubscriptionID: "sub-resume",
		PausedAt:             ptrTime(time.Now()),
	}
	billingRepo := &stubBillingRepo{existing: existing}
	squareClient := &stubSquareSubscriptionClient{
		resumeResp: &SquareSubscription{
			ID:     "sub-resume",
			Status: "ACTIVE",
			Metadata: map[string]string{
				"store_id": storeID.String(),
			},
		},
	}
	svc, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         &stubStoreRepo{store: store},
		SquareClient:      squareClient,
		DefaultPriceID:    "price-default",
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := svc.Resume(context.Background(), storeID); err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if len(billingRepo.updated) == 0 {
		t.Fatalf("expected subscription update")
	}
	if billingRepo.updated[0].Status != enums.SubscriptionStatusActive {
		t.Fatalf("expected active status, got %s", billingRepo.updated[0].Status)
	}
	if squareClient.calledResume == false {
		t.Fatalf("square resume not invoked")
	}
	if !store.SubscriptionActive {
		t.Fatalf("expected store flag true after resume")
	}
	if billingRepo.updated[0].PausedAt != nil {
		t.Fatalf("expected paused timestamp cleared")
	}
}

type stubBillingRepo struct {
	existing *models.Subscription
	created  []*models.Subscription
	updated  []*models.Subscription
}

func (s *stubBillingRepo) WithTx(tx *gorm.DB) billing.Repository {
	return s
}

func (s *stubBillingRepo) CreateSubscription(ctx context.Context, subscription *models.Subscription) error {
	s.created = append(s.created, subscription)
	return nil
}

func (s *stubBillingRepo) UpdateSubscription(ctx context.Context, subscription *models.Subscription) error {
	s.updated = append(s.updated, subscription)
	return nil
}

func (s *stubBillingRepo) ListSubscriptionsByStore(ctx context.Context, storeID uuid.UUID) ([]models.Subscription, error) {
	return nil, nil
}

func (s *stubBillingRepo) FindSubscription(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	return s.existing, nil
}

func (s *stubBillingRepo) FindSubscriptionBySquareID(ctx context.Context, squareSubscriptionID string) (*models.Subscription, error) {
	if s.existing != nil && s.existing.SquareSubscriptionID == squareSubscriptionID {
		return s.existing, nil
	}
	return nil, nil
}

func (s *stubBillingRepo) CreatePaymentMethod(ctx context.Context, method *models.PaymentMethod) error {
	return nil
}

func (s *stubBillingRepo) ListPaymentMethodsByStore(ctx context.Context, storeID uuid.UUID) ([]models.PaymentMethod, error) {
	return nil, nil
}
func (s *stubBillingRepo) ClearDefaultPaymentMethod(ctx context.Context, storeID uuid.UUID) error {
	return nil
}

func (s *stubBillingRepo) CreateCharge(ctx context.Context, charge *models.Charge) error {
	return nil
}

func (s *stubBillingRepo) ListCharges(ctx context.Context, params billing.ListChargesQuery) ([]models.Charge, *pagination.Cursor, error) {
	return nil, nil, nil
}

func (s *stubBillingRepo) CreateUsageCharge(ctx context.Context, usage *models.UsageCharge) error {
	return nil
}

func (s *stubBillingRepo) ListUsageChargesByStore(ctx context.Context, storeID uuid.UUID) ([]models.UsageCharge, error) {
	return nil, nil
}

func (s *stubBillingRepo) CreateBillingPlan(ctx context.Context, plan *models.BillingPlan) error {
	return nil
}

func (s *stubBillingRepo) UpdateBillingPlan(ctx context.Context, plan *models.BillingPlan) error {
	return nil
}

func (s *stubBillingRepo) ListBillingPlans(ctx context.Context, params billing.ListBillingPlansQuery) ([]models.BillingPlan, error) {
	return nil, nil
}

func (s *stubBillingRepo) FindBillingPlanByID(ctx context.Context, id string) (*models.BillingPlan, error) {
	return nil, nil
}

func (s *stubBillingRepo) FindBillingPlanBySquareID(ctx context.Context, squareBillingPlanID string) (*models.BillingPlan, error) {
	return nil, nil
}

func (s *stubBillingRepo) FindDefaultBillingPlan(ctx context.Context) (*models.BillingPlan, error) {
	return nil, nil
}

type stubStoreRepo struct {
	store   *models.Store
	updates []*models.Store
}

func (s *stubStoreRepo) FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.Store, error) {
	if s.store == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.store, nil
}

func (s *stubStoreRepo) UpdateWithTx(tx *gorm.DB, store *models.Store) error {
	s.updates = append(s.updates, store)
	return nil
}

type stubTxRunner struct{}

func (s *stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
