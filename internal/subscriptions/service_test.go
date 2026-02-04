package subscriptions

import (
	"context"
	"testing"

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
		SquareClient:      &stubSquareClient{},
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
	squareClient := &stubSquareClient{
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
	squareClient := &stubSquareClient{
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

// stubSquareClient satisfies SquareSubscriptionClient for tests.
type stubSquareClient struct {
	createResp   *SquareSubscription
	cancelResp   *SquareSubscription
	getResp      *SquareSubscription
	calledCreate bool
	calledCancel bool
}

func (s *stubSquareClient) Create(ctx context.Context, params *SquareSubscriptionParams) (*SquareSubscription, error) {
	s.calledCreate = true
	return s.createResp, nil
}

func (s *stubSquareClient) Cancel(ctx context.Context, id string, params *SquareSubscriptionCancelParams) (*SquareSubscription, error) {
	s.calledCancel = true
	return s.cancelResp, nil
}

func (s *stubSquareClient) Get(ctx context.Context, id string, params *SquareSubscriptionParams) (*SquareSubscription, error) {
	return s.getResp, nil
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
