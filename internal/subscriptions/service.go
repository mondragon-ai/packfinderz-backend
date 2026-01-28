package subscriptions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v84"
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
}

// ServiceParams groups dependencies for the subscription service.
type ServiceParams struct {
	BillingRepo       billing.Repository
	StoreRepo         storeRepository
	StripeClient      StripeSubscriptionClient
	DefaultPriceID    string
	TransactionRunner txRunner
}

// CreateSubscriptionInput captures the data required to start a subscription.
type CreateSubscriptionInput struct {
	StripeCustomerID      string
	StripePaymentMethodID string
	PriceID               string
}

type service struct {
	billingRepo billing.Repository
	storeRepo   storeRepository
	stripe      StripeSubscriptionClient
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
	if params.StripeClient == nil {
		return nil, fmt.Errorf("stripe client required")
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
		stripe:      params.StripeClient,
		priceID:     strings.TrimSpace(params.DefaultPriceID),
		txRunner:    params.TransactionRunner,
	}, nil
}

// Create either returns the existing active subscription or creates a new one.
func (s *service) Create(ctx context.Context, storeID uuid.UUID, input CreateSubscriptionInput) (*models.Subscription, bool, error) {
	if storeID == uuid.Nil {
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}
	customerID := strings.TrimSpace(input.StripeCustomerID)
	if customerID == "" {
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "stripe_customer_id is required")
	}
	paymentMethodID := strings.TrimSpace(input.StripePaymentMethodID)
	if paymentMethodID == "" {
		return nil, false, pkgerrors.New(pkgerrors.CodeValidation, "stripe_payment_method_id is required")
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

	params := &stripe.SubscriptionParams{
		Customer: stripe.String(customerID),
		Items: []*stripe.SubscriptionItemsParams{
			{Price: stripe.String(priceID)},
		},
		DefaultPaymentMethod: stripe.String(paymentMethodID),
	}
	params.Metadata = map[string]string{
		"store_id": storeID.String(),
	}

	stripeSub, err := s.stripe.Create(ctx, params)
	if err != nil {
		return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create stripe subscription")
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

		sub, err := buildSubscriptionModel(stripeSub, storeID, priceID, customerID, paymentMethodID)
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

		store.SubscriptionActive = isActiveStatus(sub.Status)
		if err := s.storeRepo.UpdateWithTx(tx, store); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update store subscription flag")
		}

		createdSub = sub
		return nil
	})

	if err != nil {
		if !skipped {
			if cancelErr := s.cancelStripe(ctx, stripeSub.ID); cancelErr != nil {
				return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, cancelErr, "cancel stripe subscription after db error")
			}
		}
		return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist subscription")
	}

	if skipped {
		if cancelErr := s.cancelStripe(ctx, stripeSub.ID); cancelErr != nil {
			return nil, false, pkgerrors.Wrap(pkgerrors.CodeDependency, cancelErr, "cancel stripe subscription due to race")
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

	stripeSub, err := s.stripe.Cancel(ctx, active.StripeSubscriptionID, &stripe.SubscriptionCancelParams{})
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "cancel stripe subscription")
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

		if err := applyStripeUpdates(stored, stripeSub, stored.PriceID); err != nil {
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

// GetActive returns the current active subscription if one exists.
func (s *service) GetActive(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	if storeID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}
	return s.findActive(ctx, storeID)
}

func (s *service) findActive(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	sub, err := s.billingRepo.FindSubscription(ctx, storeID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup subscription")
	}
	if sub == nil || !isActiveStatus(sub.Status) {
		return nil, nil
	}
	return sub, nil
}

func (s *service) findActiveWithTx(ctx context.Context, repo billing.Repository, storeID uuid.UUID) (*models.Subscription, error) {
	sub, err := repo.FindSubscription(ctx, storeID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "lookup subscription")
	}
	if sub == nil || !isActiveStatus(sub.Status) {
		return nil, nil
	}
	return sub, nil
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

func buildSubscriptionModel(stripeSub *stripe.Subscription, storeID uuid.UUID, priceID, customerID, paymentMethodID string) (*models.Subscription, error) {
	status := enums.SubscriptionStatus(stripeSub.Status)
	if !status.IsValid() {
		return nil, pkgerrors.New(pkgerrors.CodeDependency, "invalid stripe subscription status")
	}

	meta, err := mergeMetadata(stripeSub.Metadata, map[string]string{
		"stripe_customer_id":       customerID,
		"stripe_payment_method_id": paymentMethodID,
	})
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "marshal metadata")
	}

	startTS, endTS := periodFromSubscription(stripeSub)
	return &models.Subscription{
		StoreID:              storeID,
		StripeSubscriptionID: stripeSub.ID,
		Status:               status,
		PriceID:              &priceID,
		CurrentPeriodStart:   toTimePtr(startTS),
		CurrentPeriodEnd:     toTime(endTS),
		CancelAtPeriodEnd:    stripeSub.CancelAtPeriodEnd,
		CanceledAt:           toTimePtr(stripeSub.CanceledAt),
		Metadata:             meta,
	}, nil
}

func applyStripeUpdates(target *models.Subscription, stripeSub *stripe.Subscription, priceID *string) error {
	status := enums.SubscriptionStatus(stripeSub.Status)
	if !status.IsValid() {
		return pkgerrors.New(pkgerrors.CodeDependency, "invalid stripe subscription status")
	}

	meta, err := mergeMetadata(stripeSub.Metadata, nil)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "marshal metadata")
	}

	target.StripeSubscriptionID = stripeSub.ID
	target.Status = status
	if priceID != nil {
		target.PriceID = priceID
	}
	startTS, endTS := periodFromSubscription(stripeSub)
	target.CurrentPeriodStart = toTimePtr(startTS)
	target.CurrentPeriodEnd = toTime(endTS)
	target.CancelAtPeriodEnd = stripeSub.CancelAtPeriodEnd
	target.CanceledAt = toTimePtr(stripeSub.CanceledAt)
	target.Metadata = meta
	return nil
}

func mergeMetadata(base map[string]string, extras map[string]string) (json.RawMessage, error) {
	if len(base) == 0 && len(extras) == 0 {
		return json.RawMessage("{}"), nil
	}
	merged := make(map[string]string, len(base)+len(extras))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range extras {
		if v == "" {
			continue
		}
		merged[k] = v
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func toTime(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0).UTC()
}

func toTimePtr(ts int64) *time.Time {
	if ts == 0 {
		return nil
	}
	t := time.Unix(ts, 0).UTC()
	return &t
}

func isActiveStatus(status enums.SubscriptionStatus) bool {
	return status != enums.SubscriptionStatusCanceled
}

func periodFromSubscription(sub *stripe.Subscription) (int64, int64) {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 {
		return 0, 0
	}
	item := sub.Items.Data[0]
	return item.CurrentPeriodStart, item.CurrentPeriodEnd
}

func (s *service) cancelStripe(ctx context.Context, id string) error {
	_, err := s.stripe.Cancel(ctx, id, &stripe.SubscriptionCancelParams{})
	return err
}
