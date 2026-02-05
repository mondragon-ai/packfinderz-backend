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

func (s *Service) CreateBillingPlan(ctx context.Context, plan *models.BillingPlan) error {
	if err := s.repo.CreateBillingPlan(ctx, plan); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "create billing plan")
	}
	return nil
}

func (s *Service) UpdateBillingPlan(ctx context.Context, plan *models.BillingPlan) error {
	if err := s.repo.UpdateBillingPlan(ctx, plan); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update billing plan")
	}
	return nil
}

// ListBillingPlansParams configures plan listing.
type ListBillingPlansParams struct {
	Status    *enums.PlanStatus
	IsDefault *bool
}

func (s *Service) ListBillingPlans(ctx context.Context, params ListBillingPlansParams) ([]models.BillingPlan, error) {
	query := ListBillingPlansQuery(params)
	plans, err := s.repo.ListBillingPlans(ctx, query)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list billing plans")
	}
	return plans, nil
}

func (s *Service) FindBillingPlanByID(ctx context.Context, id string) (*models.BillingPlan, error) {
	if id == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "billing plan id is required")
	}

	plan, err := s.repo.FindBillingPlanByID(ctx, id)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "find billing plan by id")
	}
	return plan, nil
}

func (s *Service) FindBillingPlanBySquareID(ctx context.Context, squareID string) (*models.BillingPlan, error) {
	if squareID == "" {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "square billing plan id is required")
	}

	plan, err := s.repo.FindBillingPlanBySquareID(ctx, squareID)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "find billing plan by square id")
	}
	return plan, nil
}

func (s *Service) FindDefaultBillingPlan(ctx context.Context) (*models.BillingPlan, error) {
	plan, err := s.repo.FindDefaultBillingPlan(ctx)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "find default billing plan")
	}
	return plan, nil
}

func (s *Service) CreateUsageCharge(ctx context.Context, usage *models.UsageCharge) error {
	return s.repo.CreateUsageCharge(ctx, usage)
}

func (s *Service) ListUsageCharges(ctx context.Context, storeID uuid.UUID) ([]models.UsageCharge, error) {
	return s.repo.ListUsageChargesByStore(ctx, storeID)
}
