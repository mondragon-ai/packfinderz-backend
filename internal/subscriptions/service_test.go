package subscriptions

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v84"
	"gorm.io/gorm"
)

func TestServiceCreateReturnsExisting(t *testing.T) {
	billing := &stubBillingRepo{
		existing: &models.Subscription{
			ID:                   uuid.New(),
			StoreID:              uuid.New(),
			Status:               enums.SubscriptionStatusActive,
			StripeSubscriptionID: "sub-existing",
		},
	}
	store := &models.Store{SubscriptionActive: true}

	svc, err := NewService(ServiceParams{
		BillingRepo:       billing,
		StoreRepo:         &stubStoreRepo{store: store},
		StripeClient:      &stubStripeClient{},
		DefaultPriceID:    "price-default",
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, created, err := svc.Create(context.Background(), billing.existing.StoreID, CreateSubscriptionInput{
		StripeCustomerID:      "cust-1",
		StripePaymentMethodID: "pm-1",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if created {
		t.Fatalf("expected existing subscription, got created flag")
	}
	if sub == nil || sub.StripeSubscriptionID != "sub-existing" {
		t.Fatalf("expected existing subscription returned")
	}
	if len(billing.created) != 0 {
		t.Fatalf("stripe creation should not run when active subscription exists")
	}
	if billing.calledCreate {
		t.Fatalf("stripe client should not be invoked")
	}
}

func TestServiceCreatePersistsNewSubscription(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: false}
	stripeSub := &stripe.Subscription{
		ID:                "sub-new",
		Status:            stripe.SubscriptionStatusActive,
		CancelAtPeriodEnd: false,
		CanceledAt:        0,
		Metadata:          map[string]string{"source": "test"},
		Items: &stripe.SubscriptionItemList{
			Data: []*stripe.SubscriptionItem{
				{
					CurrentPeriodStart: 1689996400,
					CurrentPeriodEnd:   1690000000,
				},
			},
		},
	}

	stripeClient := &stubStripeClient{createResp: stripeSub}
	billing := &stubBillingRepo{}

	svc, err := NewService(ServiceParams{
		BillingRepo:       billing,
		StoreRepo:         &stubStoreRepo{store: store},
		StripeClient:      stripeClient,
		DefaultPriceID:    "price-default",
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, created, err := svc.Create(context.Background(), storeID, CreateSubscriptionInput{
		StripeCustomerID:      "cust-1",
		StripePaymentMethodID: "pm-1",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !created {
		t.Fatalf("expected creation flag")
	}
	if sub == nil || sub.StripeSubscriptionID != "sub-new" {
		t.Fatalf("unexpected subscription returned")
	}
	if len(billing.created) != 1 {
		t.Fatalf("expected subscription row to be created")
	}
	if !store.SubscriptionActive {
		t.Fatalf("expected store to be marked active")
	}
	if !stripeClient.calledCreate {
		t.Fatalf("stripe client create not invoked")
	}
}

func TestServiceCancelWithoutSubscriptionUpdatesStore(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	billing := &stubBillingRepo{}

	svc, err := NewService(ServiceParams{
		BillingRepo:       billing,
		StoreRepo:         &stubStoreRepo{store: store},
		StripeClient:      &stubStripeClient{},
		DefaultPriceID:    "price-default",
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.Cancel(context.Background(), storeID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.SubscriptionActive {
		t.Fatalf("expected store flag to be cleared")
	}
}

func TestServiceCancelActiveSubscription(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	activeSub := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		StripeSubscriptionID: "sub-active",
		PriceID:              ptr("price-default"),
	}
	billing := &stubBillingRepo{existing: activeSub}

	cancelResp := &stripe.Subscription{
		ID:       "sub-active",
		Status:   stripe.SubscriptionStatusCanceled,
		Metadata: map[string]string{"cancelled": "true"},
		Items: &stripe.SubscriptionItemList{
			Data: []*stripe.SubscriptionItem{
				{
					CurrentPeriodStart: 1689996400,
					CurrentPeriodEnd:   1690000000,
				},
			},
		},
	}
	stripeClient := &stubStripeClient{cancelResp: cancelResp}

	svc, err := NewService(ServiceParams{
		BillingRepo:       billing,
		StoreRepo:         &stubStoreRepo{store: store},
		StripeClient:      stripeClient,
		DefaultPriceID:    "price-default",
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.Cancel(context.Background(), storeID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(billing.updated) == 0 {
		t.Fatalf("expected subscription update")
	}
	updated := billing.updated[0]
	if updated.Status != enums.SubscriptionStatusCanceled {
		t.Fatalf("expected status canceled, got %s", updated.Status)
	}
	if store.SubscriptionActive {
		t.Fatalf("expected store flag to be cleared")
	}
	if !stripeClient.calledCancel {
		t.Fatalf("expected stripe cancel call")
	}
}

func ptr(value string) *string {
	return &value
}

type stubBillingRepo struct {
	existing     *models.Subscription
	created      []*models.Subscription
	updated      []*models.Subscription
	calledCreate bool
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
	if s.existing != nil && s.existing.StoreID == storeID {
		return s.existing, nil
	}
	return nil, nil
}

func (s *stubBillingRepo) FindSubscriptionByStripeID(ctx context.Context, stripeSubscriptionID string) (*models.Subscription, error) {
	if s.existing != nil && s.existing.StripeSubscriptionID == stripeSubscriptionID {
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

type stubStripeClient struct {
	createResp   *stripe.Subscription
	cancelResp   *stripe.Subscription
	getResp      *stripe.Subscription
	createErr    error
	cancelErr    error
	getErr       error
	calledCreate bool
	calledCancel bool
}

func (s *stubStripeClient) Create(ctx context.Context, params *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	s.calledCreate = true
	return s.createResp, s.createErr
}

func (s *stubStripeClient) Cancel(ctx context.Context, id string, params *stripe.SubscriptionCancelParams) (*stripe.Subscription, error) {
	s.calledCancel = true
	return s.cancelResp, s.cancelErr
}

func (s *stubStripeClient) Get(ctx context.Context, id string, params *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	return s.getResp, s.getErr
}
