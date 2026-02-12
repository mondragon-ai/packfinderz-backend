package paymentmethods

import (
	"context"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	squarepkg "github.com/angelmondragon/packfinderz-backend/pkg/square"
	"github.com/google/uuid"
	sq "github.com/square/square-go-sdk"
	"gorm.io/gorm"
)

func TestServiceStoreCardDefaultsFirstCard(t *testing.T) {
	storeID := uuid.New()
	cardClient := &stubCardClient{card: stubCard("card-1")}
	billingRepo := &stubBillingRepo{}
	storeRepo := &stubStoreRepo{customerID: ptrString("cust-1")}
	service, err := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreLoader:       storeRepo,
		SquareClient:      cardClient,
		TransactionRunner: &stubTxRunner{},
	})
	if err != nil {
		t.Fatalf("setup error: %v", err)
	}

	method, err := service.StoreCard(context.Background(), storeID, StoreCardInput{
		SourceID:       "src",
		IdempotencyKey: "idem",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !method.IsDefault {
		t.Fatal("expected first card to become default")
	}
	if len(billingRepo.created) != 1 {
		t.Fatalf("expected payment method persisted")
	}
	if billingRepo.cleared {
		t.Fatalf("expected no default-clearing when repo empty")
	}
}

func TestServiceStoreCardHonorsExistingDefault(t *testing.T) {
	storeID := uuid.New()
	billingRepo := &stubBillingRepo{
		paymentMethods: []models.PaymentMethod{
			{IsDefault: true},
		},
	}
	cardClient := &stubCardClient{card: stubCard("card-2")}
	storeRepo := &stubStoreRepo{customerID: ptrString("cust-1")}
	service, _ := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreLoader:       storeRepo,
		SquareClient:      cardClient,
		TransactionRunner: &stubTxRunner{},
	})

	method, err := service.StoreCard(context.Background(), storeID, StoreCardInput{
		SourceID:       "src",
		IdempotencyKey: "idem",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method.IsDefault {
		t.Fatal("expected new card to not become default when one already exists")
	}
	if len(billingRepo.created) != 1 {
		t.Fatalf("expected payment method persisted")
	}
	if billingRepo.cleared {
		t.Fatal("expected no default clearance when not requested")
	}
}

func TestServiceStoreCardClearsDefaultWhenRequested(t *testing.T) {
	storeID := uuid.New()
	billingRepo := &stubBillingRepo{
		paymentMethods: []models.PaymentMethod{
			{IsDefault: true},
		},
	}
	cardClient := &stubCardClient{card: stubCard("card-3")}
	storeRepo := &stubStoreRepo{customerID: ptrString("cust-1")}
	service, _ := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreLoader:       storeRepo,
		SquareClient:      cardClient,
		TransactionRunner: &stubTxRunner{},
	})

	method, err := service.StoreCard(context.Background(), storeID, StoreCardInput{
		SourceID:       "src",
		IdempotencyKey: "idem",
		IsDefault:      true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !method.IsDefault {
		t.Fatal("expected new card to be default when requested")
	}
	if !billingRepo.cleared {
		t.Fatal("expected existing defaults cleared")
	}
}

func TestServiceStoreCardRejectsMissingCustomer(t *testing.T) {
	storeID := uuid.New()
	billingRepo := &stubBillingRepo{}
	storeRepo := &stubStoreRepo{}
	service, _ := NewService(ServiceParams{
		BillingRepo:       billingRepo,
		StoreLoader:       storeRepo,
		SquareClient:      &stubCardClient{},
		TransactionRunner: &stubTxRunner{},
	})

	_, err := service.StoreCard(context.Background(), storeID, StoreCardInput{
		SourceID:       "src",
		IdempotencyKey: "idem",
	})
	if err == nil {
		t.Fatal("expected error when customer id missing")
	}
	if pkgerrors.As(err).Code() != pkgerrors.CodeStateConflict {
		t.Fatalf("expected state conflict, got %v", err)
	}
}

type stubBillingRepo struct {
	paymentMethods []models.PaymentMethod
	created        []*models.PaymentMethod
	cleared        bool
}

func (s *stubBillingRepo) WithTx(tx *gorm.DB) billing.Repository {
	return s
}

func (s *stubBillingRepo) CreateSubscription(ctx context.Context, subscription *models.Subscription) error {
	return nil
}
func (s *stubBillingRepo) UpdateSubscription(ctx context.Context, subscription *models.Subscription) error {
	return nil
}
func (s *stubBillingRepo) ListSubscriptionsByStore(ctx context.Context, storeID uuid.UUID) ([]models.Subscription, error) {
	return nil, nil
}
func (s *stubBillingRepo) FindSubscription(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	return nil, nil
}
func (s *stubBillingRepo) FindSubscriptionBySquareID(ctx context.Context, squareSubscriptionID string) (*models.Subscription, error) {
	return nil, nil
}
func (s *stubBillingRepo) CreatePaymentMethod(ctx context.Context, method *models.PaymentMethod) error {
	s.created = append(s.created, method)
	return nil
}
func (s *stubBillingRepo) ListPaymentMethodsByStore(ctx context.Context, storeID uuid.UUID) ([]models.PaymentMethod, error) {
	return s.paymentMethods, nil
}
func (s *stubBillingRepo) ClearDefaultPaymentMethod(ctx context.Context, storeID uuid.UUID) error {
	s.cleared = true
	for i := range s.paymentMethods {
		s.paymentMethods[i].IsDefault = false
	}
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
	customerID *string
}

func (s *stubStoreRepo) SquareCustomerID(ctx context.Context, storeID uuid.UUID) (*string, error) {
	return s.customerID, nil
}

type stubCardClient struct {
	card *sq.Card
	err  error
}

func (s *stubCardClient) CreateCard(ctx context.Context, params squarepkg.CardCreateParams) (*sq.Card, error) {
	return s.card, s.err
}

type stubTxRunner struct{}

func (s *stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

func stubCard(id string) *sq.Card {
	card := &sq.Card{}
	card.ID = &id
	brand := sq.CardBrandVisa
	card.CardBrand = &brand
	last4 := "4242"
	card.Last4 = &last4
	expMonth := int64(12)
	card.ExpMonth = &expMonth
	expYear := int64(2050)
	card.ExpYear = &expYear
	return card
}

func ptrString(value string) *string {
	return &value
}
