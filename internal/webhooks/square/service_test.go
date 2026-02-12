package squarewebhook

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/internal/subscriptions"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestService_HandleEvent_InvoiceUpdatesStoreStatus(t *testing.T) {
	storeID := uuid.New()
	subscription := &models.Subscription{
		StoreID:              storeID,
		SquareSubscriptionID: "sub-123",
		Status:               enums.SubscriptionStatusCanceled,
	}
	store := &models.Store{
		ID:                 storeID,
		SubscriptionActive: false,
	}

	billingRepo := &stubBillingRepo{sub: subscription}
	storeRepo := &stubStoreRepo{store: store}
	squareClient := &stubSquareClient{
		sub: &subscriptions.SquareSubscription{
			ID:     subscription.SquareSubscriptionID,
			Status: "ACTIVE",
		},
	}
	txRunner := &stubTxRunner{}

	svc, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreRepo:         storeRepo,
		SquareClient:      squareClient,
		TransactionRunner: txRunner,
	})
	if err != nil {
		t.Fatalf("service init: %v", err)
	}

	event := &SquareWebhookEvent{
		EventID: "evt-1",
		Type:    "invoice.paid",
		Data: SquareWebhookData{
			Object: SquareWebhookObject{
				Invoice: &SquareWebhookInvoice{
					ID:             "inv-1",
					SubscriptionID: subscription.SquareSubscriptionID,
				},
			},
		},
	}

	if err := svc.HandleEvent(context.Background(), event); err != nil {
		t.Fatalf("handle event: %v", err)
	}

	if len(squareClient.lastGet) != 1 || squareClient.lastGet[0] != subscription.SquareSubscriptionID {
		t.Fatalf("expected square client to load subscription, got %v", squareClient.lastGet)
	}
	if subscription.Status != enums.SubscriptionStatusActive {
		t.Fatalf("expected subscription active, got %s", subscription.Status)
	}
	if !store.SubscriptionActive {
		t.Fatalf("expected store subscription flag true")
	}
	if len(storeRepo.updated) != 1 {
		t.Fatalf("expected store update, got %d", len(storeRepo.updated))
	}
}

func TestSubscriptionIDFromEvent(t *testing.T) {
	tests := []struct {
		name   string
		object SquareWebhookObject
		want   string
	}{
		{name: "subscription object takes precedence", object: SquareWebhookObject{Subscription: &subscriptions.SquareSubscription{ID: "from-sub"}}, want: "from-sub"},
		{name: "subscription id field used", object: SquareWebhookObject{SubscriptionID: "from-field"}, want: "from-field"},
		{name: "invoice object used", object: SquareWebhookObject{Invoice: &SquareWebhookInvoice{SubscriptionID: "from-invoice"}}, want: "from-invoice"},
		{name: "fallback to object id", object: SquareWebhookObject{ID: "fallback"}, want: "fallback"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := subscriptionIDFromEvent(&SquareWebhookEvent{
				Data: SquareWebhookData{Object: tc.object},
			})
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

type stubBillingRepo struct {
	sub     *models.Subscription
	updated []*models.Subscription
}

func (s *stubBillingRepo) WithTx(tx *gorm.DB) billing.Repository {
	return s
}

func (s *stubBillingRepo) CreateSubscription(ctx context.Context, subscription *models.Subscription) error {
	s.sub = subscription
	return nil
}

func (s *stubBillingRepo) UpdateSubscription(ctx context.Context, subscription *models.Subscription) error {
	s.updated = append(s.updated, subscription)
	s.sub = subscription
	return nil
}

func (s *stubBillingRepo) ListSubscriptionsByStore(ctx context.Context, storeID uuid.UUID) ([]models.Subscription, error) {
	return nil, nil
}

func (s *stubBillingRepo) FindSubscription(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	return nil, nil
}

func (s *stubBillingRepo) FindSubscriptionBySquareID(ctx context.Context, squareSubscriptionID string) (*models.Subscription, error) {
	if s.sub != nil && s.sub.SquareSubscriptionID == squareSubscriptionID {
		return s.sub, nil
	}
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

func (s *stubBillingRepo) ListSubscriptionsForReconciliation(ctx context.Context, limit int, lookback time.Duration) ([]models.Subscription, error) {
	return nil, nil
}

type stubStoreRepo struct {
	store   *models.Store
	updated []*models.Store
}

func (s *stubStoreRepo) FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.Store, error) {
	if s.store != nil && s.store.ID == id {
		return s.store, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *stubStoreRepo) UpdateWithTx(tx *gorm.DB, store *models.Store) error {
	s.updated = append(s.updated, store)
	s.store = store
	return nil
}

type stubTxRunner struct{}

func (s *stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type stubSquareClient struct {
	lastGet []string
	sub     *subscriptions.SquareSubscription
}

func (s *stubSquareClient) Create(ctx context.Context, params *subscriptions.SquareSubscriptionParams) (*subscriptions.SquareSubscription, error) {
	return nil, nil
}

func (s *stubSquareClient) Cancel(ctx context.Context, id string, params *subscriptions.SquareSubscriptionCancelParams) (*subscriptions.SquareSubscription, error) {
	return nil, nil
}

func (s *stubSquareClient) Get(ctx context.Context, id string, params *subscriptions.SquareSubscriptionParams) (*subscriptions.SquareSubscription, error) {
	s.lastGet = append(s.lastGet, id)
	return s.sub, nil
}

func (s *stubSquareClient) Pause(ctx context.Context, id string, params *subscriptions.SquareSubscriptionPauseParams) (*subscriptions.SquareSubscription, error) {
	return nil, nil
}

func (s *stubSquareClient) Resume(ctx context.Context, id string, params *subscriptions.SquareSubscriptionResumeParams) (*subscriptions.SquareSubscription, error) {
	return nil, nil
}

func (s *stubSquareClient) DeleteAction(ctx context.Context, id, actionID string) (*subscriptions.SquareSubscription, error) {
	return nil, nil
}
