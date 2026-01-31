package cart

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository exposes persistence operations for cart staging data.
type Repository struct {
	db              *gorm.DB
	cartRepo        *CartRecordRepository
	itemRepo        *CartItemRepository
	vendorGroupRepo *CartVendorGroupRepository
}

// NewRepository constructs a cart repository bound to the provided DB.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db:              db,
		cartRepo:        NewCartRecordRepository(db),
		itemRepo:        NewCartItemRepository(db),
		vendorGroupRepo: NewCartVendorGroupRepository(db),
	}
}

// WithTx binds the repository to a transaction.
func (r *Repository) WithTx(tx *gorm.DB) CartRepository {
	if tx == nil {
		return r
	}
	return &Repository{
		db:              tx,
		cartRepo:        NewCartRecordRepository(tx),
		itemRepo:        NewCartItemRepository(tx),
		vendorGroupRepo: NewCartVendorGroupRepository(tx),
	}
}

// Create inserts a new CartRecord.
func (r *Repository) Create(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	return r.cartRepo.Create(ctx, record)
}

// Update saves the provided cart record.
func (r *Repository) Update(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	return r.cartRepo.Update(ctx, record)
}

// FindActiveByBuyerStore loads the latest active CartRecord for the buyer store.
func (r *Repository) FindActiveByBuyerStore(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	return r.cartRepo.FindActiveByBuyerStore(ctx, buyerStoreID)
}

// FindByIDAndBuyerStore returns a CartRecord restricted to the provided buyer store.
func (r *Repository) FindByIDAndBuyerStore(ctx context.Context, id, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	return r.cartRepo.FindByIDAndBuyerStore(ctx, id, buyerStoreID)
}

// UpdateStatus updates the status of a CartRecord owned by the buyer store.
func (r *Repository) UpdateStatus(ctx context.Context, id, buyerStoreID uuid.UUID, status enums.CartStatus) error {
	return r.cartRepo.UpdateStatus(ctx, id, buyerStoreID, status)
}

// ReplaceItems atomically replaces cart items for the provided cart.
func (r *Repository) ReplaceItems(ctx context.Context, cartID uuid.UUID, items []models.CartItem) error {
	return r.itemRepo.ReplaceForCart(ctx, cartID, items)
}

// ReplaceVendorGroups replaces the vendor aggregate rows for the cart.
func (r *Repository) ReplaceVendorGroups(ctx context.Context, cartID uuid.UUID, groups []models.CartVendorGroup) error {
	return r.vendorGroupRepo.ReplaceForCart(ctx, cartID, groups)
}

// ListItems returns items belonging to a cart.
func (r *Repository) ListItems(ctx context.Context, cartID uuid.UUID) ([]models.CartItem, error) {
	var rows []models.CartItem
	if err := r.db.WithContext(ctx).
		Where("cart_id = ?", cartID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteByBuyerStore removes all CartRecords for the buyer store.
func (r *Repository) DeleteByBuyerStore(ctx context.Context, buyerStoreID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("buyer_store_id = ?", buyerStoreID).
		Delete(&models.CartRecord{}).Error
}
