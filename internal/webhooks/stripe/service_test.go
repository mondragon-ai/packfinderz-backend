package stripewebhook

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v84"
	"gorm.io/gorm"
)

func TestService_HandleCustomerSubscriptionEventCreatesRow(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: false}
	billingRepo := &stubBillingRepo{}
	service, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         &stubStoreRepo{store: store},
		StripeClient:      &stubStripeClient{},
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("setup service: %v", err)
	}

	subscription := &stripe.Subscription{
		ID:     "sub_test",
		Status: stripe.SubscriptionStatusActive,
		Metadata: map[string]string{
			"store_id":                 storeID.String(),
			"stripe_customer_id":       "cust",
			"stripe_payment_method_id": "pm",
		},
		Items: &stripe.SubscriptionItemList{
			Data: []*stripe.SubscriptionItem{{CurrentPeriodStart: 1, CurrentPeriodEnd: 2}},
		},
	}
	raw, _ := json.Marshal(subscription)
	event := &stripe.Event{
		Type: stripe.EventTypeCustomerSubscriptionCreated,
		Data: &stripe.EventData{Raw: raw},
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

func TestService_HandleCustomerSubscriptionEventCancelsStore(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		StripeSubscriptionID: "sub_cancel",
	}
	billingRepo := &stubBillingRepo{existing: existing}
	service, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         &stubStoreRepo{store: store},
		StripeClient:      &stubStripeClient{},
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("setup service: %v", err)
	}

	subscription := &stripe.Subscription{
		ID:     "sub_cancel",
		Status: stripe.SubscriptionStatusCanceled,
		Metadata: map[string]string{
			"store_id": storeID.String(),
		},
		Items: &stripe.SubscriptionItemList{
			Data: []*stripe.SubscriptionItem{{CurrentPeriodStart: 1, CurrentPeriodEnd: 2}},
		},
	}
	raw, _ := json.Marshal(subscription)
	event := &stripe.Event{
		Type: stripe.EventTypeCustomerSubscriptionUpdated,
		Data: &stripe.EventData{Raw: raw},
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

func TestService_HandleInvoiceEventFetchesStripe(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		StripeSubscriptionID: "sub_invoice",
	}
	billingRepo := &stubBillingRepo{existing: existing}
	stripeClient := &stubStripeClient{
		getResp: &stripe.Subscription{
			ID:     "sub_invoice",
			Status: stripe.SubscriptionStatusPastDue,
			Metadata: map[string]string{
				"store_id": storeID.String(),
			},
			Items: &stripe.SubscriptionItemList{
				Data: []*stripe.SubscriptionItem{{CurrentPeriodStart: 1, CurrentPeriodEnd: 2}},
			},
		},
	}
	service, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         &stubStoreRepo{store: store},
		StripeClient:      stripeClient,
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("setup service: %v", err)
	}

	event := &stripe.Event{
		Type: stripe.EventTypeInvoicePaymentFailed,
		Data: &stripe.EventData{
			Object: map[string]interface{}{"subscription": "sub_invoice"},
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
func (s *stubBillingRepo) CreateCharge(ctx context.Context, charge *models.Charge) error { return nil }
func (s *stubBillingRepo) ListChargesByStore(ctx context.Context, storeID uuid.UUID) ([]models.Charge, error) {
	return nil, nil
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
	getResp *stripe.Subscription
	getErr  error
}

func (s *stubStripeClient) Create(ctx context.Context, params *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	return nil, nil
}

func (s *stubStripeClient) Cancel(ctx context.Context, id string, params *stripe.SubscriptionCancelParams) (*stripe.Subscription, error) {
	return nil, nil
}

func (s *stubStripeClient) Get(ctx context.Context, id string, params *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.getResp, nil
}
