package subscriptions

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	sqcore "github.com/square/square-go-sdk/core"
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
	startTS := time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC).Unix()
	endTS := time.Date(2027, 2, 11, 0, 0, 0, 0, time.UTC).Unix()
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
		getResp: &SquareSubscription{
			ID:                 "sub-new",
			Status:             "ACTIVE",
			StartDate:          startTS,
			ChargedThroughDate: endTS,
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
	if !squareClient.calledGet {
		t.Fatalf("expected square get after create")
	}
	if sub.CurrentPeriodEnd.IsZero() {
		t.Fatalf("expected current period end populated")
	}
	if len(billingRepo.created) == 1 && billingRepo.created[0].CurrentPeriodEnd.IsZero() {
		t.Fatalf("expected stored subscription current period end populated")
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

func TestServiceCancelIgnoresPendingCancelError(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		SquareSubscriptionID: "sub-pending-cancel",
		PriceID:              ptrString("price-1"),
	}
	billingRepo := &stubBillingRepo{existing: existing}
	noticeTs := time.Date(2027, 2, 11, 0, 0, 0, 0, time.UTC).Unix()
	payload := `{"errors":[{"detail":"already has a pending cancel date of '2027-02-11'."}]}`
	squareClient := &stubSquareSubscriptionClient{
		getResp: &SquareSubscription{
			ID:                 "sub-pending-cancel",
			Status:             "ACTIVE",
			ChargedThroughDate: noticeTs,
			Metadata: map[string]string{
				"store_id": storeID.String(),
			},
		},
		cancelErr: pkgerrors.Wrap(pkgerrors.CodeStateConflict, sqcore.NewAPIError(http.StatusBadRequest, errors.New(payload)), "square cancel subscription failed"),
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
		t.Fatalf("expected subscription update after sync")
	}
	if !squareClient.calledCancel {
		t.Fatalf("square cancel invoked")
	}
	if !squareClient.calledGet {
		t.Fatalf("expected square get for sync")
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
	liveEnd := time.Date(2027, 2, 11, 0, 0, 0, 0, time.UTC).Unix()
	squareClient := &stubSquareSubscriptionClient{
		getResp: &SquareSubscription{
			ID:                 "sub-pause",
			Status:             "ACTIVE",
			ChargedThroughDate: liveEnd,
			Metadata: map[string]string{
				"store_id": storeID.String(),
			},
		},
		pauseResp: &SquareSubscription{
			ID:     "sub-pause",
			Status: "PAUSED",
			Metadata: map[string]string{
				"store_id": storeID.String(),
			},
			Actions: []*SquareSubscriptionAction{
				{
					Type:          "PAUSE",
					EffectiveDate: liveEnd,
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
	if !squareClient.calledGet {
		t.Fatalf("square get not invoked before pause")
	}
	if squareClient.lastGetParams == nil || !squareClient.lastGetParams.IncludeActions {
		t.Fatalf("expected square get include actions")
	}
	if squareClient.lastPauseParams == nil {
		t.Fatalf("pause params missing")
	}
	expectedDate := time.Unix(liveEnd, 0).UTC().Format("2006-01-02")
	if squareClient.lastPauseParams.PauseEffectiveDate != expectedDate {
		t.Fatalf("expected pause date %s, got %s", expectedDate, squareClient.lastPauseParams.PauseEffectiveDate)
	}
	if store.SubscriptionActive {
		t.Fatalf("expected store flag false after pause")
	}
	if billingRepo.updated[0].PausedAt == nil {
		t.Fatalf("expected paused timestamp set")
	}
	if billingRepo.updated[0].PauseEffectiveAt == nil || billingRepo.updated[0].PauseEffectiveAt.Unix() != liveEnd {
		t.Fatalf("expected pause effective time from Square, got %v", billingRepo.updated[0].PauseEffectiveAt)
	}
}

func TestServicePauseIgnoresPendingPauseError(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		SquareSubscriptionID: "sub-pause",
	}
	billingRepo := &stubBillingRepo{existing: existing}
	noticeTs := time.Date(2027, 2, 11, 0, 0, 0, 0, time.UTC).Unix()
	payload := `{"errors":[{"detail":"The provided subscription 'sub-pause' already has a pending pause date of '2027-02-11'."}]}`
	squareClient := &stubSquareSubscriptionClient{
		getResp: &SquareSubscription{
			ID:                 "sub-pause",
			Status:             "ACTIVE",
			ChargedThroughDate: noticeTs,
			Metadata: map[string]string{
				"store_id": storeID.String(),
			},
			Actions: []*SquareSubscriptionAction{
				{Type: "PAUSE", EffectiveDate: noticeTs},
			},
		},
		pauseErr: pkgerrors.Wrap(pkgerrors.CodeStateConflict, sqcore.NewAPIError(http.StatusBadRequest, errors.New(payload)), "square pause subscription failed"),
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
		t.Fatalf("expected subscription update after sync error")
	}
	if billingRepo.updated[0].PauseEffectiveAt == nil || billingRepo.updated[0].PauseEffectiveAt.Unix() != noticeTs {
		t.Fatalf("expected pause effective time from Square, got %v", billingRepo.updated[0].PauseEffectiveAt)
	}
	if !squareClient.calledGet {
		t.Fatalf("expected square get called to refresh state")
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
		getResp: &SquareSubscription{
			ID:     "sub-resume",
			Status: "PAUSED",
			Metadata: map[string]string{
				"store_id": storeID.String(),
			},
		},
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

func TestServiceResumeCancelsPendingPauseAction(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		SquareSubscriptionID: "sub-pending",
	}
	billingRepo := &stubBillingRepo{existing: existing}
	pauseTs := time.Date(2027, 2, 11, 0, 0, 0, 0, time.UTC).Unix()
	actionID := "pause-action"
	squareClient := &stubSquareSubscriptionClient{
		getResp: &SquareSubscription{
			ID:                 "sub-pending",
			Status:             "ACTIVE",
			ChargedThroughDate: pauseTs,
			Metadata: map[string]string{
				"store_id": storeID.String(),
			},
			Actions: []*SquareSubscriptionAction{
				{ID: actionID, Type: "PAUSE", EffectiveDate: pauseTs},
			},
		},
		deleteResp: &SquareSubscription{
			ID:                 "sub-pending",
			Status:             "ACTIVE",
			ChargedThroughDate: pauseTs,
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
	if !squareClient.calledDelete {
		t.Fatalf("delete action not invoked")
	}
	if squareClient.lastDeleteActionID != actionID {
		t.Fatalf("unexpected action deleted: %s", squareClient.lastDeleteActionID)
	}
	if !squareClient.calledGet {
		t.Fatalf("square get should run")
	}
	if len(billingRepo.updated) == 0 {
		t.Fatalf("expected sync to persist subscription")
	}
}

func TestServiceGetActiveReconcilesSquare(t *testing.T) {
	storeID := uuid.New()
	store := &models.Store{SubscriptionActive: true}
	existing := &models.Subscription{
		ID:                   uuid.New(),
		StoreID:              storeID,
		Status:               enums.SubscriptionStatusActive,
		SquareSubscriptionID: "sub-sync",
	}
	billingRepo := &stubBillingRepo{existing: existing}
	startTS := time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC).Unix()
	endTS := time.Date(2027, 2, 11, 0, 0, 0, 0, time.UTC).Unix()
	squareClient := &stubSquareSubscriptionClient{
		getResp: &SquareSubscription{
			ID:                 "sub-sync",
			Status:             "ACTIVE",
			StartDate:          startTS,
			ChargedThroughDate: endTS,
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

	sub, err := svc.GetActive(context.Background(), storeID)
	if err != nil {
		t.Fatalf("get active failed: %v", err)
	}
	if sub == nil {
		t.Fatalf("expected subscription returned")
	}
	if sub.CurrentPeriodEnd.IsZero() {
		t.Fatalf("expected current period end set")
	}
	if !squareClient.calledGet {
		t.Fatalf("expected square get invoked")
	}
	if len(billingRepo.updated) == 0 {
		t.Fatalf("expected subscription update")
	}
	if billingRepo.updated[0].CurrentPeriodEnd.IsZero() {
		t.Fatalf("expected updated period end")
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

func (s *stubStoreRepo) UpdateSubscriptionActiveWithTx(tx *gorm.DB, storeID uuid.UUID, active bool) error {
	if s.store == nil {
		return gorm.ErrRecordNotFound
	}
	s.store.SubscriptionActive = active
	return nil
}

type stubTxRunner struct{}

func (s *stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func ptrString(s string) *string {
	return &s
}
