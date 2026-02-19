package models

import (
	"time"

	"github.com/google/uuid"
)

// WishlistItem links a store to a liked product.
type WishlistItem struct {
	ID        uuid.UUID `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID   uuid.UUID `gorm:"column:store_id;type:uuid;not null;index:wishlist_items_store_id_idx;uniqueIndex:wishlist_items_store_product_key"`
	ProductID uuid.UUID `gorm:"column:product_id;type:uuid;not null;index:wishlist_items_product_id_idx;uniqueIndex:wishlist_items_store_product_key"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}
