package billing

import (
	"context"
	"errors"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
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

func (s *Service) ListCharges(ctx context.Context, storeID uuid.UUID) ([]models.Charge, error) {
	return s.repo.ListChargesByStore(ctx, storeID)
}

func (s *Service) CreateUsageCharge(ctx context.Context, usage *models.UsageCharge) error {
	return s.repo.CreateUsageCharge(ctx, usage)
}

func (s *Service) ListUsageCharges(ctx context.Context, storeID uuid.UUID) ([]models.UsageCharge, error) {
	return s.repo.ListUsageChargesByStore(ctx, storeID)
}
