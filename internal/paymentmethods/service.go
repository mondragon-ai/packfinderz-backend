package paymentmethods

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	sq "github.com/square/square-go-sdk"
	"gorm.io/gorm"

	"github.com/angelmondragon/packfinderz-backend/internal/billing"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/square"
)

// Service orchestrates card-on-file persistence.
type Service interface {
	StoreCard(ctx context.Context, storeID uuid.UUID, input StoreCardInput) (*models.PaymentMethod, error)
}

// StoreCardInput captures the payload required to vault a card.
type StoreCardInput struct {
	SourceID          string
	CardholderName    string
	VerificationToken string
	IsDefault         bool
	IdempotencyKey    string
}

// ServiceParams groups dependencies for the payment method service.
type ServiceParams struct {
	BillingRepo       billing.Repository
	StoreLoader       storeCustomerLoader
	SquareClient      cardCreator
	TransactionRunner txRunner
}

type cardCreator interface {
	CreateCard(ctx context.Context, params square.CardCreateParams) (*sq.Card, error)
}

type storeCustomerLoader interface {
	SquareCustomerID(ctx context.Context, storeID uuid.UUID) (*string, error)
}

type txRunner interface {
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type service struct {
	repo     billing.Repository
	store    storeCustomerLoader
	square   cardCreator
	txRunner txRunner
}

// NewService constructs a payment method service.
func NewService(params ServiceParams) (*service, error) {
	if params.BillingRepo == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "billing repo required")
	}
	if params.StoreLoader == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "store loader required")
	}
	if params.SquareClient == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "square client required")
	}
	if params.TransactionRunner == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInternal, "transaction runner required")
	}

	return &service{
		repo:     params.BillingRepo,
		store:    params.StoreLoader,
		square:   params.SquareClient,
		txRunner: params.TransactionRunner,
	}, nil
}

// StoreCard creates a Square card and persists the metadata.
func (s *service) StoreCard(ctx context.Context, storeID uuid.UUID, input StoreCardInput) (*models.PaymentMethod, error) {
	if storeID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}
	sourceID := strings.TrimSpace(input.SourceID)
	if sourceID == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "source_id is required")
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "idempotency key is required")
	}

	customerID, err := s.store.SquareCustomerID(ctx, storeID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "load square customer id")
	}
	if customerID == nil || strings.TrimSpace(*customerID) == "" {
		return nil, pkgerrors.New(pkgerrors.CodeStateConflict, "store is not linked to a square customer")
	}

	params := square.CardCreateParams{
		CustomerID:     strings.TrimSpace(*customerID),
		SourceID:       sourceID,
		IdempotencyKey: idempotencyKey,
	}
	if cardholder := strings.TrimSpace(input.CardholderName); cardholder != "" {
		params.CardholderName = cardholder
	}
	if token := strings.TrimSpace(input.VerificationToken); token != "" {
		params.VerificationToken = token
	}

	card, err := s.square.CreateCard(ctx, params)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create square card")
	}
	if card == nil {
		return nil, pkgerrors.New(pkgerrors.CodeDependency, "square card response is nil")
	}
	cardID := card.GetID()
	if cardID == nil || strings.TrimSpace(*cardID) == "" {
		return nil, pkgerrors.New(pkgerrors.CodeDependency, "square card missing id")
	}

	existingMethods, err := s.repo.ListPaymentMethodsByStore(ctx, storeID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list payment methods")
	}

	hasDefault := false
	hasExisting := len(existingMethods) > 0
	for _, method := range existingMethods {
		if method.IsDefault {
			hasDefault = true
			break
		}
	}

	shouldDefault := !hasExisting || input.IsDefault
	if !hasDefault && len(existingMethods) > 0 {
		shouldDefault = true
	}

	method, err := buildPaymentMethod(card, storeID, shouldDefault, customerID)
	if err != nil {
		return nil, err
	}

	if err := s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		txRepo := s.repo.WithTx(tx)
		if shouldDefault && hasExisting {
			if err := txRepo.ClearDefaultPaymentMethod(ctx, storeID); err != nil {
				return err
			}
		}
		return txRepo.CreatePaymentMethod(ctx, method)
	}); err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "persist payment method")
	}

	return method, nil
}

func buildPaymentMethod(card *sq.Card, storeID uuid.UUID, isDefault bool, customerID *string) (*models.PaymentMethod, error) {
	billingDetails, err := marshalBillingDetails(card)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "marshal billing details")
	}
	metadata, err := marshalMetadata(card, customerID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "marshal metadata")
	}

	brand := cardBrandString(card)

	return &models.PaymentMethod{
		StoreID:               storeID,
		SquarePaymentMethodID: strings.TrimSpace(*card.GetID()),
		Type:                  enums.PaymentMethodTypeCard,
		Fingerprint:           card.GetFingerprint(),
		CardBrand:             brand,
		CardLast4:             card.GetLast4(),
		CardExpMonth:          intPointer(card.GetExpMonth()),
		CardExpYear:           intPointer(card.GetExpYear()),
		BillingDetails:        billingDetails,
		Metadata:              metadata,
		IsDefault:             isDefault,
	}, nil
}

func marshalBillingDetails(card *sq.Card) (json.RawMessage, error) {
	details := map[string]any{}
	if name := card.GetCardholderName(); name != nil && strings.TrimSpace(*name) != "" {
		details["cardholder_name"] = *name
	}
	if customer := card.GetCustomerID(); customer != nil && strings.TrimSpace(*customer) != "" {
		details["customer_id"] = *customer
	}
	if addr := card.GetBillingAddress(); addr != nil {
		details["billing_address"] = addr
	}
	if len(details) == 0 {
		return json.RawMessage("{}"), nil
	}
	data, err := json.Marshal(details)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func marshalMetadata(card *sq.Card, customerID *string) (json.RawMessage, error) {
	meta := map[string]string{}
	if id := card.GetID(); id != nil && strings.TrimSpace(*id) != "" {
		meta["square_card_id"] = strings.TrimSpace(*id)
	}
	if customerID != nil && strings.TrimSpace(*customerID) != "" {
		meta["square_customer_id"] = strings.TrimSpace(*customerID)
	}
	if name := card.GetCardholderName(); name != nil && strings.TrimSpace(*name) != "" {
		meta["cardholder_name"] = strings.TrimSpace(*name)
	}
	if len(meta) == 0 {
		return json.RawMessage("{}"), nil
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func cardBrandString(card *sq.Card) *string {
	if card == nil {
		return nil
	}
	if brand := card.GetCardBrand(); brand != nil && strings.TrimSpace(string(*brand)) != "" {
		value := string(*brand)
		return &value
	}
	return nil
}

func intPointer(value *int64) *int {
	if value == nil {
		return nil
	}
	v := int(*value)
	return &v
}
