package subscriptions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	sq "github.com/square/square-go-sdk"
	"gorm.io/gorm"
)

type storeRepository interface {
	FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.Store, error)
	UpdateWithTx(tx *gorm.DB, store *models.Store) error
	UpdateSubscriptionActiveWithTx(tx *gorm.DB, storeID uuid.UUID, active bool) error
}

type txRunner interface {
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

// Service defines the subscription lifecycle surface.
type Service interface {
	Create(ctx context.Context, storeID uuid.UUID, input CreateSubscriptionInput) (*models.Subscription, bool, error)
	Cancel(ctx context.Context, storeID uuid.UUID) error
	GetActive(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error)
	Pause(ctx context.Context, storeID uuid.UUID) error
	Resume(ctx context.Context, storeID uuid.UUID) error
}

// ServiceParams groups dependencies for the subscription service.
type ServiceParams struct {
	BillingRepo       billing.Repository
	StoreRepo         storeRepository
	SquareClient      SquareSubscriptionClient
	DefaultPriceID    string
	TransactionRunner txRunner
}

// CreateSubscriptionInput captures the data required to start a subscription.
type CreateSubscriptionInput struct {
	SquareCustomerID      string
	SquarePaymentMethodID string
	PriceID               string
}

type service struct {
	billingRepo billing.Repository
	storeRepo   storeRepository
	square      SquareSubscriptionClient
	priceID     string
	txRunner    txRunner
}

// NewService builds a subscription service with the required dependencies.
func NewService(params ServiceParams) (Service, error) {
	if params.BillingRepo == nil {
		return nil, fmt.Errorf("billing repo required")
	}
	if params.StoreRepo == nil {
		return nil, fmt.Errorf("store repo required")
	}
	if params.SquareClient == nil {
		return nil, fmt.Errorf("square client required")
	}
	if params.TransactionRunner == nil {
		return nil, fmt.Errorf("transaction runner required")
	}
	if strings.TrimSpace(params.DefaultPriceID) == "" {
		return nil, fmt.Errorf("default price id required")
	}
	return &service{
		billingRepo: params.BillingRepo,
		storeRepo:   params.StoreRepo,
		square:      params.SquareClient,
		priceID:     strings.TrimSpace(params.DefaultPriceID),
		txRunner:    params.TransactionRunner,
	}, nil
}

// Create either returns the existing active subscription or creates a new one.
func (s *service) Create(ctx context.Context, storeID uuid.UUID, input CreateSubscriptionInput) (*models.Subscription, bool, error) {
	fmt.Printf("[subscriptions.Create] start storeID=%s\n", storeID)

	if storeID == uuid.Nil {
		fmt.Printf("[subscriptions.Create] FAIL storeID is nil\n")
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}

	customerID := strings.TrimSpace(input.SquareCustomerID)
	fmt.Printf("[subscriptions.Create] customerID='%s'\n", customerID)
	if customerID == "" {
		fmt.Printf("[subscriptions.Create] FAIL missing customerID\n")
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "square_customer_id is required")
	}

	paymentMethodID := strings.TrimSpace(input.SquarePaymentMethodID)
	fmt.Printf("[subscriptions.Create] paymentMethodID='%s'\n", paymentMethodID)
	if paymentMethodID == "" {
		fmt.Printf("[subscriptions.Create] FAIL missing paymentMethodID\n")
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "square_payment_method_id is required")
	}

	priceID := strings.TrimSpace(input.PriceID)
	if priceID == "" {
		priceID = s.priceID
	}
	fmt.Printf("[subscriptions.Create] priceID='%s' (input=%t default=%t)\n", priceID, strings.TrimSpace(input.PriceID) != "", s.priceID != "")
	if priceID == "" {
		fmt.Printf("[subscriptions.Create] FAIL missing priceID\n")
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "price_id is required")
	}

	fmt.Printf("[subscriptions.Create] findActive (pre)\n")
	if existing, err := s.findActive(ctx, storeID); err != nil {
		fmt.Printf("[subscriptions.Create] FAIL findActive err=%T %v\n", err, err)
		return nil, false, err
	} else if existing != nil {
		fmt.Printf("[subscriptions.Create] existing active subscription found id=%s status=%s\n", existing.ID, existing.Status)
		return existing, false, nil
	}

	params := &SquareSubscriptionParams{
		CustomerID:      customerID,
		PriceID:         priceID,
		PaymentMethodID: paymentMethodID,
		Metadata: map[string]string{
			"store_id": storeID.String(),
		},
	}

	fmt.Printf("[subscriptions.Create] square params=%+v\n", params)
	fmt.Printf("[subscriptions.Create] square.Create about to call Square\n")
	squareSub, err := s.square.Create(ctx, params)
	if err != nil {
		fmt.Printf("[subscriptions.Create] FAIL square.Create err=%T %v\n", err, err)
		// If Square SDK returns a structured error, this prints more detail:
		debugSquareErr(err)
		return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create square subscription")
	}
	fmt.Printf("[subscriptions.Create] square.Create OK squareSubID=%s status=%s\n", squareSub.ID, squareSub.Status)
	fmt.Printf("[subscriptions.Create] square response=%+v\n", squareSub)

	var (
		createdSub    *models.Subscription
		existingAfter *models.Subscription
		skipped       bool
	)

	fmt.Printf("[subscriptions.Create] begin tx persist\n")
	err = s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		fmt.Printf("[subscriptions.Create.tx] start\n")
		txRepo := s.billingRepo.WithTx(tx)

		fmt.Printf("[subscriptions.Create.tx] findActiveWithTx\n")
		active, err := s.findActiveWithTx(ctx, txRepo, storeID)
		if err != nil {
			fmt.Printf("[subscriptions.Create.tx] FAIL findActiveWithTx err=%T %v\n", err, err)
			return err
		}
		if active != nil {
			fmt.Printf("[subscriptions.Create.tx] active exists (race) subID=%s\n", active.ID)
			existingAfter = active
			skipped = true
			return nil
		}

		fmt.Printf("[subscriptions.Create.tx] BuildSubscriptionFromSquare\n")
		sub, err := BuildSubscriptionFromSquare(squareSub, storeID, priceID, customerID, paymentMethodID)
		if err != nil {
			fmt.Printf("[subscriptions.Create.tx] FAIL BuildSubscriptionFromSquare err=%T %v\n", err, err)
			return err
		}

		fmt.Printf("[subscriptions.Create.tx] CreateSubscription\n")
		if err := txRepo.CreateSubscription(ctx, sub); err != nil {
			fmt.Printf("[subscriptions.Create.tx] FAIL CreateSubscription err=%T %v\n", err, err)
			return err
		}

		fmt.Printf("[subscriptions.Create.tx] load store\n")
		store, err := s.storeRepo.FindByIDWithTx(tx, storeID)
		if err != nil {
			fmt.Printf("[subscriptions.Create.tx] FAIL FindByIDWithTx err=%T %v\n", err, err)
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
		}

		store.SubscriptionActive = IsActiveStatus(sub.Status)
		fmt.Printf("[subscriptions.Create.tx] Update store subscription flag => %t\n", store.SubscriptionActive)
		if err := s.storeRepo.UpdateSubscriptionActiveWithTx(tx, storeID, store.SubscriptionActive); err != nil {
			fmt.Printf("[subscriptions.Create.tx] FAIL UpdateWithTx err=%T %v\n", err, err)
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update store subscription flag")
		}

		createdSub = sub
		fmt.Printf("[subscriptions.Create.tx] OK\n")
		return nil
	})

	if err != nil {
		fmt.Printf("[subscriptions.Create] FAIL persist err=%T %v skipped=%t\n", err, err, skipped)
		if !skipped {
			fmt.Printf("[subscriptions.Create] cancelSquare due to db error squareSubID=%s\n", squareSub.ID)
			if cancelErr := s.cancelSquare(ctx, squareSub.ID); cancelErr != nil {
				fmt.Printf("[subscriptions.Create] FAIL cancelSquare err=%T %v\n", cancelErr, cancelErr)
				return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, cancelErr, "cancel square subscription after db error")
			}
		}
		return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist subscription")
	}

	if skipped {
		fmt.Printf("[subscriptions.Create] skipped due to race, cancelSquare squareSubID=%s\n", squareSub.ID)
		if cancelErr := s.cancelSquare(ctx, squareSub.ID); cancelErr != nil {
			fmt.Printf("[subscriptions.Create] FAIL cancelSquare(race) err=%T %v\n", cancelErr, cancelErr)
			return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, cancelErr, "cancel square subscription due to race")
		}
		return existingAfter, false, nil
	}

	fmt.Printf("[subscriptions.Create] DONE createdSubID=%s\n", createdSub.ID)
	return createdSub, true, nil
}

// debugSquareErr tries to print useful details when Square SDK returns structured errors.
// Keep it best-effort: it should never panic.
func debugSquareErr(err error) {
	if err == nil {
		return
	}
	fmt.Printf("[square.err] raw=%T %v\n", err, err)

	// Many Square SDK errors implement these shapes; adapt as needed once you see output.
	type hasErrors interface {
		GetErrors() []sq.Error
	}
	if he, ok := err.(hasErrors); ok {
		es := he.GetErrors()
		fmt.Printf("[square.err] count=%d\n", len(es))
		for i, e := range es {
			fmt.Printf("[square.err] #%d category=%s code=%s detail=%s field=%s\n",
				i, e.Category, e.Code, e.Detail, e.Field)
		}
	}
}

// Cancel terminates the active subscription (if any) and flips the store flag.
func (s *service) Cancel(ctx context.Context, storeID uuid.UUID) error {
	if storeID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}

	active, err := s.findActive(ctx, storeID)
	if err != nil {
		return err
	}
	if active == nil {
		return s.ensureStoreFlag(ctx, storeID, false)
	}

	squareSub, err := s.square.Cancel(ctx, active.SquareSubscriptionID, &SquareSubscriptionCancelParams{})
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "cancel square subscription")
	}
	fmt.Printf("[subscriptions.Cancel] square cancel response=%+v\n", squareSub)

	if err := s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		txRepo := s.billingRepo.WithTx(tx)
		stored, err := s.findActiveWithTx(ctx, txRepo, storeID)
		if err != nil {
			return err
		}
		if stored == nil {
			return pkgerrors.New(pkgerrors.CodeNotFound, "subscription not found")
		}

		if err := UpdateSubscriptionFromSquare(stored, squareSub, stored.PriceID); err != nil {
			return err
		}
		if err := txRepo.UpdateSubscription(ctx, stored); err != nil {
			return err
		}

		store, err := s.storeRepo.FindByIDWithTx(tx, storeID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
		}
		store.SubscriptionActive = false
		if err := s.storeRepo.UpdateWithTx(tx, store); err != nil {
			fmt.Printf("[subscriptions.persistSquareUpdate] FAIL storeRepo.UpdateWithTx err=%T %v storeID=%s\n", err, err, storeID)
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update store subscription flag")
		}
		return nil
	}); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist cancellation")
	}

	return nil
}
func (s *service) Pause(ctx context.Context, storeID uuid.UUID) error {
	if storeID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}

	sub, err := s.findSubscription(ctx, storeID)
	if err != nil {
		return err
	}
	if sub == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "subscription not found")
	}
	if strings.TrimSpace(sub.SquareSubscriptionID) == "" {
		return pkgerrors.New(pkgerrors.CodeStateConflict, "square subscription id missing")
	}

	fmt.Printf("[subscriptions.Pause] start storeID=%s squareID=%s status=%s\n", storeID, sub.SquareSubscriptionID, sub.Status)

	live, err := s.square.Get(ctx, sub.SquareSubscriptionID, &SquareSubscriptionParams{
		PriceID: resolvePriceID(sub.PriceID),
		Metadata: map[string]string{
			"store_id": storeID.String(),
		},
	})
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "get square subscription")
	}
	if live == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "square subscription not found")
	}

	fmt.Printf("[subscriptions.Pause] square live state=%+v\n", live)

	if strings.ToUpper(live.Status) != "ACTIVE" {
		return pkgerrors.New(pkgerrors.CodeStateConflict,
			fmt.Sprintf("subscription not active in Square (status=%s)", live.Status),
		)
	}

	paused, err := s.square.Pause(ctx, sub.SquareSubscriptionID, &SquareSubscriptionPauseParams{
		PriceID: resolvePriceID(sub.PriceID),
	})
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeStateConflict, err, "pause square subscription")
	}

	fmt.Printf("[subscriptions.Pause] square paused state=%+v\n", paused)

	return s.persistSquareUpdate(ctx, storeID, paused, func(stored *models.Subscription) {
		now := time.Now().UTC()
		stored.PausedAt = &now
	})
}

func (s *service) Resume(ctx context.Context, storeID uuid.UUID) error {
	if storeID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}

	sub, err := s.findSubscription(ctx, storeID)
	if err != nil {
		return err
	}
	if sub == nil {
		return pkgerrors.New(pkgerrors.CodeNotFound, "subscription not found")
	}
	if sub.Status != enums.SubscriptionStatusPaused {
		return pkgerrors.New(pkgerrors.CodeStateConflict, "subscription not paused")
	}

	fmt.Printf("[subscriptions.Resume] start storeID=%s squareID=%s status=%s\n", storeID, sub.SquareSubscriptionID, sub.Status)

	squareSub, err := s.square.Resume(ctx, sub.SquareSubscriptionID, &SquareSubscriptionResumeParams{
		PriceID: resolvePriceID(sub.PriceID),
	})
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "resume square subscription")
	}

	fmt.Printf("[subscriptions.Resume] square resumed state=%+v\n", squareSub)

	return s.persistSquareUpdate(ctx, storeID, squareSub, func(stored *models.Subscription) {
		stored.PausedAt = nil
	})
}

// GetActive returns the current active subscription if one exists.
func (s *service) GetActive(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	if storeID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}
	fmt.Printf("[subscriptions.GetActive] storeID=%s\n", storeID)
	return s.findActive(ctx, storeID)
}

func (s *service) findActive(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	sub, err := s.findSubscription(ctx, storeID)
	if err != nil {
		return nil, err
	}
	active := sub != nil && IsActiveStatus(sub.Status)
	fmt.Printf("[subscriptions.findActive] storeID=%s found=%t sub=%+v\n", storeID, active, sub)
	if !active {
		return nil, nil
	}
	return sub, nil
}

func (s *service) findActiveWithTx(ctx context.Context, repo billing.Repository, storeID uuid.UUID) (*models.Subscription, error) {
	sub, err := s.findSubscriptionWithTx(ctx, repo, storeID)
	if err != nil {
		return nil, err
	}
	if sub == nil || !IsActiveStatus(sub.Status) {
		return nil, nil
	}
	return sub, nil
}

func (s *service) persistSquareUpdate(ctx context.Context, storeID uuid.UUID, squareSub *SquareSubscription, modify func(*models.Subscription)) error {
	return s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		txRepo := s.billingRepo.WithTx(tx)
		stored, err := s.findSubscriptionWithTx(ctx, txRepo, storeID)
		if err != nil {
			return err
		}
		if stored == nil {
			return pkgerrors.New(pkgerrors.CodeNotFound, "subscription not found")
		}

		if err := UpdateSubscriptionFromSquare(stored, squareSub, stored.PriceID); err != nil {
			return err
		}
		if modify != nil {
			modify(stored)
		}
		if err := txRepo.UpdateSubscription(ctx, stored); err != nil {
			return err
		}

		store, err := s.storeRepo.FindByIDWithTx(tx, storeID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
		}
		store.SubscriptionActive = IsActiveStatus(stored.Status)
		if err := s.storeRepo.UpdateWithTx(tx, store); err != nil {
			fmt.Printf("[subscriptions.persistSquareUpdate] FAIL storeRepo.UpdateWithTx err=%T %v storeID=%s\n", err, err, storeID)
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update store subscription flag")
		}

		return nil
	})
}

func (s *service) findSubscription(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	sub, err := s.billingRepo.FindSubscription(ctx, storeID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup subscription")
	}
	return sub, nil
}

func (s *service) findSubscriptionWithTx(ctx context.Context, repo billing.Repository, storeID uuid.UUID) (*models.Subscription, error) {
	sub, err := repo.FindSubscription(ctx, storeID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup subscription")
	}
	return sub, nil
}

func resolvePriceID(price *string) string {
	if price == nil {
		return ""
	}
	return *price
}

func (s *service) ensureStoreFlag(ctx context.Context, storeID uuid.UUID, active bool) error {
	return s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		store, err := s.storeRepo.FindByIDWithTx(tx, storeID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
		}
		if store.SubscriptionActive == active {
			return nil
		}
		store.SubscriptionActive = active
		return s.storeRepo.UpdateWithTx(tx, store)
	})
}

func (s *service) cancelSquare(ctx context.Context, id string) error {
	_, err := s.square.Cancel(ctx, id, &SquareSubscriptionCancelParams{})
	return err
}
