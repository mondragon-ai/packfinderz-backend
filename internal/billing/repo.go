package billing

import (
	"context"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository handles billing persistence.
type Repository interface {
	WithTx(tx *gorm.DB) Repository
	CreateSubscription(ctx context.Context, subscription *models.Subscription) error
	UpdateSubscription(ctx context.Context, subscription *models.Subscription) error
	ListSubscriptionsForReconciliation(ctx context.Context, limit int, lookback time.Duration) ([]models.Subscription, error)
	ListSubscriptionsByStore(ctx context.Context, storeID uuid.UUID) ([]models.Subscription, error)
	FindSubscription(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error)
	FindSubscriptionBySquareID(ctx context.Context, squareSubscriptionID string) (*models.Subscription, error)
	CreateBillingPlan(ctx context.Context, plan *models.BillingPlan) error
	UpdateBillingPlan(ctx context.Context, plan *models.BillingPlan) error
	ListBillingPlans(ctx context.Context, params ListBillingPlansQuery) ([]models.BillingPlan, error)
	FindBillingPlanByID(ctx context.Context, id string) (*models.BillingPlan, error)
	FindBillingPlanBySquareID(ctx context.Context, squareBillingPlanID string) (*models.BillingPlan, error)
	FindDefaultBillingPlan(ctx context.Context) (*models.BillingPlan, error)
	CreatePaymentMethod(ctx context.Context, method *models.PaymentMethod) error
	ListPaymentMethodsByStore(ctx context.Context, storeID uuid.UUID) ([]models.PaymentMethod, error)
	ClearDefaultPaymentMethod(ctx context.Context, storeID uuid.UUID) error
	CreateCharge(ctx context.Context, charge *models.Charge) error
	ListCharges(ctx context.Context, params ListChargesQuery) ([]models.Charge, *pagination.Cursor, error)
	CreateUsageCharge(ctx context.Context, usage *models.UsageCharge) error
	ListUsageChargesByStore(ctx context.Context, storeID uuid.UUID) ([]models.UsageCharge, error)
}

type repository struct {
	db *gorm.DB
}

// ListBillingPlansQuery configures billing plan list queries.
type ListBillingPlansQuery struct {
	Status    *enums.PlanStatus
	IsDefault *bool
}

// NewRepository returns a billing repository bound to the provided database.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) WithTx(tx *gorm.DB) Repository {
	if tx == nil {
		return r
	}
	return &repository{db: tx}
}

func (r *repository) CreateSubscription(ctx context.Context, subscription *models.Subscription) error {
	return r.db.WithContext(ctx).Create(subscription).Error
}

func (r *repository) UpdateSubscription(ctx context.Context, subscription *models.Subscription) error {
	return r.db.WithContext(ctx).Save(subscription).Error
}

func (r *repository) ListSubscriptionsByStore(ctx context.Context, storeID uuid.UUID) ([]models.Subscription, error) {
	var subs []models.Subscription
	if err := r.db.WithContext(ctx).
		Where("store_id = ?", storeID).
		Order("created_at DESC").
		Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *repository) ListSubscriptionsForReconciliation(ctx context.Context, limit int, lookback time.Duration) ([]models.Subscription, error) {
	if limit <= 0 {
		limit = 250
	}
	if lookback <= 0 {
		lookback = 7 * 24 * time.Hour
	}
	cutoff := time.Now().UTC().Add(-lookback)
	statuses := []enums.SubscriptionStatus{
		enums.SubscriptionStatusActive,
		enums.SubscriptionStatusTrialing,
		enums.SubscriptionStatusPastDue,
		enums.SubscriptionStatusIncomplete,
		enums.SubscriptionStatusIncompleteExpired,
		enums.SubscriptionStatusUnpaid,
		enums.SubscriptionStatusPaused,
	}
	var subs []models.Subscription
	query := r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("square_subscription_id <> ''").
		Where("(status IN (?) OR cancel_at_period_end OR pause_effective_at IS NOT NULL OR current_period_end >= ?)", statuses, cutoff).
		Order("updated_at DESC").
		Limit(limit)
	if err := query.Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *repository) FindSubscription(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error) {
	var sub models.Subscription
	if err := r.db.WithContext(ctx).
		Where("store_id = ?", storeID).
		Order("created_at DESC").
		First(&sub).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *repository) FindSubscriptionBySquareID(ctx context.Context, squareSubscriptionID string) (*models.Subscription, error) {
	if squareSubscriptionID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var sub models.Subscription
	if err := r.db.WithContext(ctx).
		Where("square_subscription_id = ?", squareSubscriptionID).
		First(&sub).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *repository) CreateBillingPlan(ctx context.Context, plan *models.BillingPlan) error {
	return r.db.WithContext(ctx).Create(plan).Error
}

func (r *repository) UpdateBillingPlan(ctx context.Context, plan *models.BillingPlan) error {
	return r.db.WithContext(ctx).Save(plan).Error
}

func (r *repository) ListBillingPlans(ctx context.Context, params ListBillingPlansQuery) ([]models.BillingPlan, error) {
	query := r.db.WithContext(ctx).Model(&models.BillingPlan{})
	if params.Status != nil {
		query = query.Where("status = ?", *params.Status)
	}
	if params.IsDefault != nil {
		query = query.Where("is_default = ?", *params.IsDefault)
	}

	var plans []models.BillingPlan
	if err := query.Order("is_default DESC, test DESC, name ASC").Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

func (r *repository) FindBillingPlanByID(ctx context.Context, id string) (*models.BillingPlan, error) {
	if id == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var plan models.BillingPlan
	if err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&plan).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

func (r *repository) FindBillingPlanBySquareID(ctx context.Context, squareBillingPlanID string) (*models.BillingPlan, error) {
	if squareBillingPlanID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var plan models.BillingPlan
	if err := r.db.WithContext(ctx).
		Where("square_billing_plan_id = ?", squareBillingPlanID).
		First(&plan).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

func (r *repository) FindDefaultBillingPlan(ctx context.Context) (*models.BillingPlan, error) {
	var plan models.BillingPlan
	if err := r.db.WithContext(ctx).
		Where("is_default = true").
		Order("updated_at DESC").
		First(&plan).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

func (r *repository) CreatePaymentMethod(ctx context.Context, method *models.PaymentMethod) error {
	return r.db.WithContext(ctx).Create(method).Error
}

func (r *repository) ListPaymentMethodsByStore(ctx context.Context, storeID uuid.UUID) ([]models.PaymentMethod, error) {
	var methods []models.PaymentMethod
	if err := r.db.WithContext(ctx).
		Where("store_id = ?", storeID).
		Order("created_at DESC").
		Find(&methods).Error; err != nil {
		return nil, err
	}
	return methods, nil
}

func (r *repository) ClearDefaultPaymentMethod(ctx context.Context, storeID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.PaymentMethod{}).
		Where("store_id = ? AND is_default", storeID).
		Update("is_default", false).Error
}

func (r *repository) CreateCharge(ctx context.Context, charge *models.Charge) error {
	return r.db.WithContext(ctx).Create(charge).Error
}

type ListChargesQuery struct {
	StoreID uuid.UUID
	Limit   int
	Cursor  *pagination.Cursor
	Type    *enums.ChargeType
	Status  *enums.ChargeStatus
}

func (r *repository) ListCharges(ctx context.Context, params ListChargesQuery) ([]models.Charge, *pagination.Cursor, error) {
	limit := pagination.NormalizeLimit(params.Limit)
	query := r.db.WithContext(ctx).Model(&models.Charge{}).Where("store_id = ?", params.StoreID)
	if params.Type != nil {
		query = query.Where("type = ?", *params.Type)
	}
	if params.Status != nil {
		query = query.Where("status = ?", *params.Status)
	}
	if params.Cursor != nil {
		query = query.Where("(created_at, id) < (?, ?)", params.Cursor.CreatedAt, params.Cursor.ID)
	}

	var charges []models.Charge
	if err := query.Order("created_at DESC, id DESC").Limit(pagination.LimitWithBuffer(limit)).Find(&charges).Error; err != nil {
		return nil, nil, err
	}

	if len(charges) > limit {
		next := charges[limit]
		charges = charges[:limit]
		return charges, &pagination.Cursor{
			CreatedAt: next.CreatedAt,
			ID:        next.ID,
		}, nil
	}

	return charges, nil, nil
}

func (r *repository) CreateUsageCharge(ctx context.Context, usage *models.UsageCharge) error {
	return r.db.WithContext(ctx).Create(usage).Error
}

func (r *repository) ListUsageChargesByStore(ctx context.Context, storeID uuid.UUID) ([]models.UsageCharge, error) {
	var usages []models.UsageCharge
	if err := r.db.WithContext(ctx).
		Where("store_id = ?", storeID).
		Order("created_at DESC").
		Find(&usages).Error; err != nil {
		return nil, err
	}
	return usages, nil
}
