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
	db *gorm.DB
}

// NewRepository constructs a cart repository bound to the provided DB.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// WithTx binds the repository to a transaction.
func (r *Repository) WithTx(tx *gorm.DB) CartRepository {
	if tx == nil {
		return r
	}
	return &Repository{db: tx}
}

// Create inserts a new CartRecord.
func (r *Repository) Create(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	if record.Status == "" {
		record.Status = enums.CartStatusActive
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// Update saves the provided cart record.
func (r *Repository) Update(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	if err := r.db.WithContext(ctx).Save(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// FindActiveByBuyerStore loads the latest active CartRecord for the buyer store.
func (r *Repository) FindActiveByBuyerStore(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	var record models.CartRecord
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("VendorGroups").
		Where("buyer_store_id = ? AND status = ?", buyerStoreID, enums.CartStatusActive).
		Order("created_at DESC").
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// FindByIDAndBuyerStore returns a CartRecord restricted to the provided buyer store.
func (r *Repository) FindByIDAndBuyerStore(ctx context.Context, id, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	var record models.CartRecord
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("VendorGroups").
		Where("id = ? AND buyer_store_id = ?", id, buyerStoreID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// UpdateStatus updates the status of a CartRecord owned by the buyer store.
func (r *Repository) UpdateStatus(ctx context.Context, id, buyerStoreID uuid.UUID, status enums.CartStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.CartRecord{}).
		Where("id = ? AND buyer_store_id = ?", id, buyerStoreID).
		Update("status", status).Error
}

// ReplaceItems atomically replaces cart items for the provided cart.
func (r *Repository) ReplaceItems(ctx context.Context, cartID uuid.UUID, items []models.CartItem) error {
	tx := r.db.WithContext(ctx)
	if err := tx.Where("cart_id = ?", cartID).Delete(&models.CartItem{}).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	for i := range items {
		items[i].CartID = cartID
	}
	return tx.Create(&items).Error
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
