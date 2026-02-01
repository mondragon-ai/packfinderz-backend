package cart

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CartItemRepository manages persistent cart items.
type CartItemRepository struct {
	db *gorm.DB
}

// NewCartItemRepository binds the repository to the provided DB handle.
func NewCartItemRepository(db *gorm.DB) *CartItemRepository {
	return &CartItemRepository{db: db}
}

// WithTx scopes the repository to the provided transaction.
func (r *CartItemRepository) WithTx(tx *gorm.DB) *CartItemRepository {
	if tx == nil {
		return r
	}
	return &CartItemRepository{db: tx}
}

// ReplaceForCart deletes existing items and inserts the provided ones transactionally.
func (r *CartItemRepository) ReplaceForCart(ctx context.Context, cartID uuid.UUID, items []models.CartItem) error {
	tx := r.db.WithContext(ctx).Debug()
	if err := tx.Where("cart_id = ?", cartID).Delete(&models.CartItem{}).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	for i := range items {
		items[i].CartID = cartID

		fmt.Printf("[cart.ReplaceForCart] item[%d] product_id=%s vendor_id=%s status=%v warnings=%#v applied_volume_discount=%#v\n",
			i,
			items[i].ProductID,
			items[i].VendorStoreID,
			items[i].Status,
			items[i].Warnings,
			items[i].AppliedVolumeDiscount,
		)
	}
	return tx.Create(&items).Error
}
