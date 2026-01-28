package stripewebhook

import (
	"context"
	"encoding/json"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/internal/subscriptions"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
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

type ServiceParams struct {
	BillingRepo       billing.Repository
	StoreRepo         storeRepository
	StripeClient      subscriptions.StripeSubscriptionClient
	TransactionRunner txRunner
}

type Service struct {
	billingRepo billing.Repository
	storeRepo   storeRepository
	stripe      subscriptions.StripeSubscriptionClient
	txRunner    txRunner
}

func NewService(params ServiceParams) (*Service, error) {
	if params.BillingRepo == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "billing repo required")
	}
	if params.StoreRepo == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "store repo required")
	}
	if params.StripeClient == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "stripe client required")
	}
	if params.TransactionRunner == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "transaction runner required")
	}
	return &Service{
		billingRepo: params.BillingRepo,
		storeRepo:   params.StoreRepo,
		stripe:      params.StripeClient,
		txRunner:    params.TransactionRunner,
	}, nil
}

func (s *Service) HandleEvent(ctx context.Context, event *stripe.Event) error {
	if event == nil || event.Data == nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "stripe event data required")
	}

	switch event.Type {
	case stripe.EventTypeCustomerSubscriptionCreated,
		stripe.EventTypeCustomerSubscriptionUpdated,
		stripe.EventTypeCustomerSubscriptionDeleted:
		var stripeSub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "decode subscription event")
		}
		return s.syncSubscription(ctx, &stripeSub)
	case stripe.EventTypeInvoicePaid, stripe.EventTypeInvoicePaymentFailed:
		subscriptionID := event.GetObjectValue("subscription")
		if subscriptionID == "" {
			return pkgerrors.New(pkgerrors.CodeValidation, "subscription id missing")
		}
		stripeSub, err := s.stripe.Get(ctx, subscriptionID, &stripe.SubscriptionParams{})
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "fetch stripe subscription")
		}
		return s.syncSubscription(ctx, stripeSub)
	default:
		return nil
	}
}

func (s *Service) syncSubscription(ctx context.Context, stripeSub *stripe.Subscription) error {
	if stripeSub == nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "subscription is required")
	}
	return s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		repo := s.billingRepo.WithTx(tx)
		stored, err := repo.FindSubscriptionByStripeID(ctx, stripeSub.ID)
		if err != nil {
			return err
		}

		storeID, metadataErr := subscriptions.StoreIDFromMetadata(stripeSub.Metadata)
		if metadataErr != nil && stored != nil {
			storeID = stored.StoreID
			metadataErr = nil
		}
		if metadataErr != nil {
			return metadataErr
		}

		priceID := determinePriceID(stripeSub)
		var successSub *models.Subscription

		if stored == nil {
			customerID := stripeSub.Metadata["stripe_customer_id"]
			paymentMethodID := stripeSub.Metadata["stripe_payment_method_id"]
			built, buildErr := subscriptions.BuildSubscriptionFromStripe(stripeSub, storeID, priceID, customerID, paymentMethodID)
			if buildErr != nil {
				return buildErr
			}
			if err := repo.CreateSubscription(ctx, built); err != nil {
				return err
			}
			successSub = built
		} else {
			var pricePtr *string
			if priceID != "" {
				pricePtr = &priceID
			}
			if err := subscriptions.UpdateSubscriptionFromStripe(stored, stripeSub, pricePtr); err != nil {
				return err
			}
			if err := repo.UpdateSubscription(ctx, stored); err != nil {
				return err
			}
			successSub = stored
		}

		if successSub == nil {
			return pkgerrors.New(pkgerrors.CodeInternal, "subscription not persisted")
		}

		store, err := s.storeRepo.FindByIDWithTx(tx, storeID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return pkgerrors.New(pkgerrors.CodeNotFound, "store not found")
			}
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load store")
		}

		active := subscriptions.IsActiveStatus(successSub.Status)
		if store.SubscriptionActive != active {
			store.SubscriptionActive = active
			if err := s.storeRepo.UpdateWithTx(tx, store); err != nil {
				return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update store subscription flag")
			}
		}

		return nil
	})
}

func determinePriceID(sub *stripe.Subscription) string {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 {
		return ""
	}
	if sub.Items.Data[0].Price != nil {
		return sub.Items.Data[0].Price.ID
	}
	return ""
}
