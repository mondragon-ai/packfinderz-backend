package squarewebhook

import (
	"context"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/internal/subscriptions"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestService_HandleSubscriptionEventCreatesRow(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: false}
	billingRepo := &stubBillingRepo{}
	service, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         &stubStoreRepo{store: store},
		SquareClient:      &stubSquareClient{},
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("setup service: %v", err)
	}

	subscription := &subscriptions.SquareSubscription{
		ID:     "sub_test",
		Status: "ACTIVE",
		Metadata: map[string]string{
			"store_id":                 storeID.String(),
			"square_customer_id":       "cust",
			"square_payment_method_id": "pm",
		},
		Items: &subscriptions.SquareSubscriptionItemList{
			Data: []*subscriptions.SquareSubscriptionItem{{CurrentPeriodStart: 1, CurrentPeriodEnd: 2}},
		},
	}
	event := &SquareWebhookEvent{
		Type: "subscription.created",
		Data: SquareWebhookData{
			Object: SquareWebhookObject{
				Subscription: subscription,
			},
		},
	}

	if err := service.HandleEvent(context.Background(), event); err != nil {
		t.Fatalf("handle event: %v", err)
	}
	if len(billingRepo.created) != 1 {
		t.Fatalf("expected subscription created")
	}
	if !store.SubscriptionActive {
		t.Fatalf("expected store activated")
	}
}

func TestService_HandleSubscriptionEventCancelsStore(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		SquareSubscriptionID: "sub_cancel",
	}
	billingRepo := &stubBillingRepo{existing: existing}
	service, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         &stubStoreRepo{store: store},
		SquareClient:      &stubSquareClient{},
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("setup service: %v", err)
	}

	subscription := &subscriptions.SquareSubscription{
		ID:     "sub_cancel",
		Status: "CANCELED",
		Metadata: map[string]string{
			"store_id": storeID.String(),
		},
		Items: &subscriptions.SquareSubscriptionItemList{
			Data: []*subscriptions.SquareSubscriptionItem{{CurrentPeriodStart: 1, CurrentPeriodEnd: 2}},
		},
	}
	event := &SquareWebhookEvent{
		Type: "subscription.updated",
		Data: SquareWebhookData{
			Object: SquareWebhookObject{
				Subscription: subscription,
			},
		},
	}

	if err := service.HandleEvent(context.Background(), event); err != nil {
		t.Fatalf("handle event: %v", err)
	}
	if len(billingRepo.updated) == 0 {
		t.Fatalf("expected update recorded")
	}
	if store.SubscriptionActive {
		t.Fatalf("expected store deactivated")
	}
}

func TestService_HandleInvoiceEventFetchesSquare(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		SquareSubscriptionID: "sub_invoice",
	}
	squareClient := &stubSquareClient{
		getResp: &subscriptions.SquareSubscription{
			ID:     "sub_invoice",
			Status: "PAST_DUE",
			Metadata: map[string]string{
				"store_id": storeID.String(),
			},
			Items: &subscriptions.SquareSubscriptionItemList{
				Data: []*subscriptions.SquareSubscriptionItem{{CurrentPeriodStart: 1, CurrentPeriodEnd: 2}},
			},
		},
	}
	billingRepo := &stubBillingRepo{existing: existing}
	service, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         &stubStoreRepo{store: store},
		SquareClient:      squareClient,
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("setup service: %v", err)
	}

	event := &SquareWebhookEvent{
		Type: "invoice.payment_failed",
		Data: SquareWebhookData{
			Object: SquareWebhookObject{
				ID: "sub_invoice",
			},
		},
	}
	if err := service.HandleEvent(context.Background(), event); err != nil {
		t.Fatalf("handle event: %v", err)
	}
	if len(billingRepo.updated) == 0 {
		t.Fatalf("expected subscription update")
	}
	if billingRepo.updated[0].Status != enums.SubscriptionStatusPastDue {
		t.Fatalf("expected status past_due, got %s", billingRepo.updated[0].Status)
	}
}

type stubSquareClient struct {
	getResp *subscriptions.SquareSubscription
}

func (s *stubSquareClient) Create(ctx context.Context, params *subscriptions.SquareSubscriptionParams) (*subscriptions.SquareSubscription, error) {
	return nil, nil
}

func (s *stubSquareClient) Cancel(ctx context.Context, id string, params *subscriptions.SquareSubscriptionCancelParams) (*subscriptions.SquareSubscription, error) {
	return nil, nil
}

func (s *stubSquareClient) Get(ctx context.Context, id string, params *subscriptions.SquareSubscriptionParams) (*subscriptions.SquareSubscription, error) {
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
