package billing

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository handles billing persistence.
type Repository interface {
	WithTx(tx *gorm.DB) Repository
	CreateSubscription(ctx context.Context, subscription *models.Subscription) error
	ListSubscriptionsByStore(ctx context.Context, storeID uuid.UUID) ([]models.Subscription, error)
	FindSubscription(ctx context.Context, storeID uuid.UUID) (*models.Subscription, error)
	CreatePaymentMethod(ctx context.Context, method *models.PaymentMethod) error
	ListPaymentMethodsByStore(ctx context.Context, storeID uuid.UUID) ([]models.PaymentMethod, error)
	CreateCharge(ctx context.Context, charge *models.Charge) error
	ListChargesByStore(ctx context.Context, storeID uuid.UUID) ([]models.Charge, error)
	CreateUsageCharge(ctx context.Context, usage *models.UsageCharge) error
	ListUsageChargesByStore(ctx context.Context, storeID uuid.UUID) ([]models.UsageCharge, error)
}

type repository struct {
	db *gorm.DB
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

func (r *repository) CreateCharge(ctx context.Context, charge *models.Charge) error {
	return r.db.WithContext(ctx).Create(charge).Error
}

func (r *repository) ListChargesByStore(ctx context.Context, storeID uuid.UUID) ([]models.Charge, error) {
	var charges []models.Charge
	if err := r.db.WithContext(ctx).
		Where("store_id = ?", storeID).
		Order("created_at DESC").
		Find(&charges).Error; err != nil {
		return nil, err
	}
	return charges, nil
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
