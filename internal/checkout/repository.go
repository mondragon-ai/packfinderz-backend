package checkout

import (
	"context"
	"errors"

	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository exposes helper queries for checkout metadata.
type Repository interface {
	WithTx(tx *gorm.DB) Repository
	FindByCheckoutGroupID(ctx context.Context, checkoutGroupID uuid.UUID) (*models.CheckoutGroup, error)
	FindByCartID(ctx context.Context, cartID uuid.UUID) (*models.CheckoutGroup, error)
}

type repository struct {
	db         *gorm.DB
	orders     orders.Repository
	cartLoader cartLoader
}

// NewRepository builds a checkout repository backed by the provided DB and order repo.
func NewRepository(db *gorm.DB, ordersRepo orders.Repository) Repository {
	if db == nil {
		return nil
	}
	return &repository{
		db:         db,
		orders:     ordersRepo,
		cartLoader: newCartLoader(db),
	}
}

func (r *repository) WithTx(tx *gorm.DB) Repository {
	if tx == nil {
		return r
	}
	return &repository{
		db:         tx,
		orders:     r.orders.WithTx(tx),
		cartLoader: r.cartLoader.WithTx(tx),
	}
}

func (r *repository) FindByCheckoutGroupID(ctx context.Context, checkoutGroupID uuid.UUID) (*models.CheckoutGroup, error) {
	return r.findByCheckoutGroup(ctx, checkoutGroupID, nil)
}

func (r *repository) FindByCartID(ctx context.Context, cartID uuid.UUID) (*models.CheckoutGroup, error) {
	if cartID == uuid.Nil {
		return nil, nil
	}
	record, err := r.cartLoader.LoadByID(ctx, cartID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if record == nil || record.CheckoutGroupID == nil {
		return nil, nil
	}
	return r.findByCheckoutGroup(ctx, *record.CheckoutGroupID, record)
}

func (r *repository) findByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID, record *models.CartRecord) (*models.CheckoutGroup, error) {
	if checkoutGroupID == uuid.Nil {
		return nil, nil
	}

	ordersRepo := r.orders.WithTx(r.db)
	vendorOrders, err := ordersRepo.FindVendorOrdersByCheckoutGroup(ctx, checkoutGroupID)
	if err != nil {
		return nil, err
	}
	if len(vendorOrders) == 0 {
		return nil, nil
	}

	group := &models.CheckoutGroup{
		ID:           checkoutGroupID,
		VendorOrders: vendorOrders,
	}

	var cartRecord *models.CartRecord
	if record != nil {
		cartRecord = record
	} else {
		cartRecord, err = r.cartLoader.LoadByCheckoutGroup(ctx, checkoutGroupID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	if cartRecord != nil {
		group.BuyerStoreID = cartRecord.BuyerStoreID
		group.CartID = &cartRecord.ID
		group.CartVendorGroups = append([]models.CartVendorGroup(nil), cartRecord.VendorGroups...)
	} else if len(vendorOrders) > 0 {
		group.BuyerStoreID = vendorOrders[0].BuyerStoreID
		group.CartID = &vendorOrders[0].CartID
	}

	return group, nil
}

type cartLoader interface {
	WithTx(tx *gorm.DB) cartLoader
	LoadByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) (*models.CartRecord, error)
	LoadByID(ctx context.Context, cartID uuid.UUID) (*models.CartRecord, error)
}

type gormCartLoader struct {
	db *gorm.DB
}

func newCartLoader(db *gorm.DB) cartLoader {
	return &gormCartLoader{db: db}
}

func (l *gormCartLoader) WithTx(tx *gorm.DB) cartLoader {
	if tx == nil {
		return l
	}
	return &gormCartLoader{db: tx}
}

func (l *gormCartLoader) LoadByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) (*models.CartRecord, error) {
	var record models.CartRecord
	err := l.db.WithContext(ctx).
		Preload("VendorGroups").
		Where("checkout_group_id = ?", checkoutGroupID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (l *gormCartLoader) LoadByID(ctx context.Context, cartID uuid.UUID) (*models.CartRecord, error) {
	var record models.CartRecord
	err := l.db.WithContext(ctx).
		Preload("VendorGroups").
		Where("id = ?", cartID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}
