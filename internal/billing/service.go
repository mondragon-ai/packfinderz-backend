package billing

import (
	"context"
	"errors"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
)

// ServiceParams groups dependencies for the billing service.
type ServiceParams struct {
	Repo Repository
}

// Service orchestrates billing operations.
type Service struct {
	repo Repository
}

// NewService builds a billing service.
func NewService(params ServiceParams) (*Service, error) {
	if params.Repo == nil {
		return nil, errors.New("repo is required")
	}
	return &Service{repo: params.Repo}, nil
}

func (s *Service) CreateSubscription(ctx context.Context, subscription *models.Subscription) error {
	return s.repo.CreateSubscription(ctx, subscription)
}

func (s *Service) UpdateSubscription(ctx context.Context, subscription *models.Subscription) error {
	return s.repo.UpdateSubscription(ctx, subscription)
}

func (s *Service) ListSubscriptions(ctx context.Context, storeID uuid.UUID) ([]models.Subscription, error) {
	return s.repo.ListSubscriptionsByStore(ctx, storeID)
}

func (s *Service) CreatePaymentMethod(ctx context.Context, method *models.PaymentMethod) error {
	return s.repo.CreatePaymentMethod(ctx, method)
}

func (s *Service) ListPaymentMethods(ctx context.Context, storeID uuid.UUID) ([]models.PaymentMethod, error) {
	return s.repo.ListPaymentMethodsByStore(ctx, storeID)
}

func (s *Service) CreateCharge(ctx context.Context, charge *models.Charge) error {
	return s.repo.CreateCharge(ctx, charge)
}

// ListChargesParams configures the vendor billing history request.
type ListChargesParams struct {
	StoreID uuid.UUID
	Limit   int
	Cursor  string
	Type    *enums.ChargeType
	Status  *enums.ChargeStatus
}

// ListChargesResult wraps the returned charges and cursor metadata.
type ListChargesResult struct {
	Items  []models.Charge `json:"items"`
	Cursor string          `json:"cursor"`
}

func (s *Service) ListCharges(ctx context.Context, params ListChargesParams) (*ListChargesResult, error) {
	if params.StoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}

	query := ListChargesQuery{
		StoreID: params.StoreID,
		Limit:   pagination.NormalizeLimit(params.Limit),
		Type:    params.Type,
		Status:  params.Status,
	}

	if params.Cursor != "" {
		cursor, err := pagination.ParseCursor(params.Cursor)
		if err != nil {
			return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid cursor")
		}
		query.Cursor = cursor
	}

	charges, next, err := s.repo.ListCharges(ctx, query)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list charges")
	}

	result := &ListChargesResult{
		Items: charges,
	}
	if next != nil {
		result.Cursor = pagination.EncodeCursor(*next)
	}
	return result, nil
}

func (s *Service) CreateUsageCharge(ctx context.Context, usage *models.UsageCharge) error {
	return s.repo.CreateUsageCharge(ctx, usage)
}

func (s *Service) ListUsageCharges(ctx context.Context, storeID uuid.UUID) ([]models.UsageCharge, error) {
	return s.repo.ListUsageChargesByStore(ctx, storeID)
}
