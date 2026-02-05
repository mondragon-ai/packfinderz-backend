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
	"gorm.io/gorm"
)

type storeRepository interface {
	FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.Store, error)
	UpdateWithTx(tx *gorm.DB, store *models.Store) error
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
	if storeID == uuid.Nil {
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}
	customerID := strings.TrimSpace(input.SquareCustomerID)
	if customerID == "" {
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "square_customer_id is required")
	}
	paymentMethodID := strings.TrimSpace(input.SquarePaymentMethodID)
	if paymentMethodID == "" {
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "square_payment_method_id is required")
	}

	priceID := strings.TrimSpace(input.PriceID)
	if priceID == "" {
		priceID = s.priceID
	}
	if priceID == "" {
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "price_id is required")
	}

	if existing, err := s.findActive(ctx, storeID); err != nil {
		return nil, false, err
	} else if existing != nil {
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

	squareSub, err := s.square.Create(ctx, params)
	if err != nil {
		return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create square subscription")
	}

	var (
		createdSub    *models.Subscription
		existingAfter *models.Subscription
		skipped       bool
	)

	err = s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		txRepo := s.billingRepo.WithTx(tx)
		active, err := s.findActiveWithTx(ctx, txRepo, storeID)
		if err != nil {
			return err
		}
		if active != nil {
			existingAfter = active
			skipped = true
			return nil
		}

		sub, err := BuildSubscriptionFromSquare(squareSub, storeID, priceID, customerID, paymentMethodID)
		if err != nil {
			return err
		}

		if err := txRepo.CreateSubscription(ctx, sub); err != nil {
			return err
		}

		store, err := s.storeRepo.FindByIDWithTx(tx, storeID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
		}

		store.SubscriptionActive = IsActiveStatus(sub.Status)
		if err := s.storeRepo.UpdateWithTx(tx, store); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update store subscription flag")
		}

		createdSub = sub
		return nil
	})

	if err != nil {
		if !skipped {
			if cancelErr := s.cancelSquare(ctx, squareSub.ID); cancelErr != nil {
				return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, cancelErr, "cancel square subscription after db error")
			}
		}
		return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist subscription")
	}

	if skipped {
		if cancelErr := s.cancelSquare(ctx, squareSub.ID); cancelErr != nil {
			return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, cancelErr, "cancel square subscription due to race")
		}
		return existingAfter, false, nil
	}

	return createdSub, true, nil
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
	if !IsActiveStatus(sub.Status) {
		return pkgerrors.New(pkgerrors.CodeStateConflict, "subscription not active")
	}

	squareSub, err := s.square.Pause(ctx, sub.SquareSubscriptionID, &SquareSubscriptionPauseParams{
		PriceID: resolvePriceID(sub.PriceID),
	})
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "pause square subscription")
	}

	return s.persistSquareUpdate(ctx, storeID, squareSub, func(stored *models.Subscription) {
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

	squareSub, err := s.square.Resume(ctx, sub.SquareSubscriptionID, &SquareSubscriptionResumeParams{
		PriceID: resolvePriceID(sub.PriceID),
	})
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "resume square subscription")
	}

	return s.persistSquareUpdate(ctx, storeID, squareSub, func(stored *models.Subscription) {
		stored.PausedAt = nil
	})
}

// GetActive returns the current active subscription if one exists.
func (s *service) GetActive(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	if storeID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}
	return s.findActive(ctx, storeID)
}

func (s *service) findActive(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	sub, err := s.findSubscription(ctx, storeID)
	if err != nil {
		return nil, err
	}
	if sub == nil || !IsActiveStatus(sub.Status) {
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
