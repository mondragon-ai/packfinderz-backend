package squarewebhook

import (
	"context"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/internal/subscriptions"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
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

type ServiceParams struct {
	BillingRepo       billing.Repository
	StoreRepo         storeRepository
	SquareClient      subscriptions.SquareSubscriptionClient
	TransactionRunner txRunner
}

type Service struct {
	billingRepo billing.Repository
	storeRepo   storeRepository
	square      subscriptions.SquareSubscriptionClient
	txRunner    txRunner
}

func NewService(params ServiceParams) (*Service, error) {
	if params.BillingRepo == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "billing repo required")
	}
	if params.StoreRepo == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "store repo required")
	}
	if params.SquareClient == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "square client required")
	}
	if params.TransactionRunner == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "transaction runner required")
	}
	return &Service{
		billingRepo: params.BillingRepo,
		storeRepo:   params.StoreRepo,
		square:      params.SquareClient,
		txRunner:    params.TransactionRunner,
	}, nil
}

type SquareWebhookEvent struct {
	EventID string            `json:"event_id"`
	Type    string            `json:"type"`
	Data    SquareWebhookData `json:"data"`
}

type SquareWebhookData struct {
	Type   string              `json:"type"`
	ID     string              `json:"id"`
	Object SquareWebhookObject `json:"object"`
}

type SquareWebhookObject struct {
	Type         string                            `json:"type"`
	ID           string                            `json:"id"`
	Subscription *subscriptions.SquareSubscription `json:"subscription"`
}

// HandleEvent processes Square subscription / invoice events.
func (s *Service) HandleEvent(ctx context.Context, event *SquareWebhookEvent) error {
	if event == nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "square event required")
	}

	switch strings.ToLower(event.Type) {
	case "subscription.created", "subscription.updated", "subscription.canceled":
		if event.Data.Object.Subscription == nil {
			return pkgerrors.New(pkgerrors.CodeValidation, "subscription payload missing")
		}
		return s.syncSubscription(ctx, event.Data.Object.Subscription)
	case "invoice.paid", "invoice.payment_failed":
		subscriptionID := event.Data.Object.ID
		if subscriptionID == "" {
			return pkgerrors.New(pkgerrors.CodeValidation, "subscription id missing")
		}
		squareSub, err := s.square.Get(ctx, subscriptionID, nil)
		if err != nil {
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "fetch square subscription")
		}
		return s.syncSubscription(ctx, squareSub)
	default:
		return nil
	}
}

func (s *Service) syncSubscription(ctx context.Context, squareSub *subscriptions.SquareSubscription) error {
	if squareSub == nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "subscription is required")
	}
	return s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		repo := s.billingRepo.WithTx(tx)
		stored, err := repo.FindSubscriptionBySquareID(ctx, squareSub.ID)
		if err != nil {
			return err
		}

		storeID, metadataErr := subscriptions.StoreIDFromMetadata(squareSub.Metadata)
		if metadataErr != nil && stored != nil {
			storeID = stored.StoreID
			metadataErr = nil
		}
		if metadataErr != nil {
			return metadataErr
		}

		priceID := determinePriceID(squareSub)
		var successSub *models.Subscription

		if stored == nil {
			customerID := ""
			if squareSub.Metadata != nil {
				customerID = squareSub.Metadata["square_customer_id"]
			}
			paymentMethodID := ""
			if squareSub.Metadata != nil {
				paymentMethodID = squareSub.Metadata["square_payment_method_id"]
			}
			built, buildErr := subscriptions.BuildSubscriptionFromSquare(squareSub, storeID, priceID, customerID, paymentMethodID)
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
			if err := subscriptions.UpdateSubscriptionFromSquare(stored, squareSub, pricePtr); err != nil {
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

func determinePriceID(sub *subscriptions.SquareSubscription) string {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 {
		return ""
	}
	if sub.Items.Data[0].Price != nil {
		return sub.Items.Data[0].Price.ID
	}
	return ""
}
