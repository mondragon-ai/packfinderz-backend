package cart

import (
	"context"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CartVendorGroupRepository manages vendor group aggregates.
type CartVendorGroupRepository struct {
	db *gorm.DB
}

// NewCartVendorGroupRepository binds the repo to the provided DB.
func NewCartVendorGroupRepository(db *gorm.DB) *CartVendorGroupRepository {
	return &CartVendorGroupRepository{db: db}
}

// WithTx scopes the repository to the provided transaction.
func (r *CartVendorGroupRepository) WithTx(tx *gorm.DB) *CartVendorGroupRepository {
	if tx == nil {
		return r
	}
	return &CartVendorGroupRepository{db: tx}
}

// ReplaceForCart deletes existing vendor groups and inserts the provided snapshot.
func (r *CartVendorGroupRepository) ReplaceForCart(ctx context.Context, cartID uuid.UUID, groups []models.CartVendorGroup) error {
	tx := r.db.WithContext(ctx).Debug()
	if err := tx.Where("cart_id = ?", cartID).Delete(&models.CartVendorGroup{}).Error; err != nil {
		return err
	}
	if len(groups) == 0 {
		return nil
	}
	for i := range groups {
		groups[i].CartID = cartID
	}
	return tx.Create(&groups).Error
}
