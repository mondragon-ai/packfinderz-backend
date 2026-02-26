package models

import (
	"time"

	"github.com/google/uuid"
)

// Review captures buyer feedback for a vendor store or product.
type Review struct {
	ID                 uuid.UUID  `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	ReviewType         string     `gorm:"column:review_type;type:review_type;not null"`
	BuyerStoreID       uuid.UUID  `gorm:"column:buyer_store_id;type:uuid;not null;index:idx_reviews_buyer_store_id"`
	BuyerUserID        uuid.UUID  `gorm:"column:buyer_user_id;type:uuid;not null;index:idx_reviews_buyer_user_id"`
	VendorStoreID      *uuid.UUID `gorm:"column:vendor_store_id;type:uuid;index:idx_reviews_vendor_store_id"`
	ProductID          *uuid.UUID `gorm:"column:product_id;type:uuid;index:idx_reviews_product_id"`
	OrderID            *uuid.UUID `gorm:"column:order_id;type:uuid;index:idx_reviews_order_id"`
	Rating             int16      `gorm:"column:rating;type:smallint;not null"`
	Title              *string    `gorm:"column:title;size:150"`
	Body               *string    `gorm:"column:body;type:text"`
	IsVerifiedPurchase bool       `gorm:"column:is_verified_purchase;not null;default:false"`
	IsVisible          bool       `gorm:"column:is_visible;not null;default:true;index:idx_reviews_is_visible"`
	CreatedAt          time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}
