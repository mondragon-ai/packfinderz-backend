package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

// Product represents the canonical vendor listing.
type Product struct {
	ID                  uuid.UUID                    `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID             uuid.UUID                    `gorm:"column:store_id;type:uuid;not null"`
	SKU                 string                       `gorm:"column:sku;not null"`
	Title               string                       `gorm:"column:title;not null"`
	Subtitle            *string                      `gorm:"column:subtitle"`
	BodyHTML            *string                      `gorm:"column:body_html"`
	Category            enums.ProductCategory        `gorm:"column:category;type:category;not null"`
	Feelings            pq.StringArray               `gorm:"column:feelings;type:feelings[];not null;default:ARRAY[]::feelings[]"`
	Flavors             pq.StringArray               `gorm:"column:flavors;type:flavors[];not null;default:ARRAY[]::flavors[]"`
	Usage               pq.StringArray               `gorm:"column:usage;type:usage[];not null;default:ARRAY[]::usage[]"`
	Strain              *string                      `gorm:"column:strain"`
	Classification      *enums.ProductClassification `gorm:"column:classification;type:classification"`
	COAMediaID          *uuid.UUID                   `gorm:"column:coa_media_id;type:uuid"`
	COAAdded            bool                         `gorm:"column:coa_added;not null;default:false"`
	Unit                enums.ProductUnit            `gorm:"column:unit;type:unit;not null"`
	MOQ                 int                          `gorm:"column:moq;not null;default:1"`
	PriceCents          int                          `gorm:"column:price_cents;not null"`
	CompareAtPriceCents *int                         `gorm:"column:compare_at_price_cents"`
	IsActive            bool                         `gorm:"column:is_active;not null;default:true"`
	IsFeatured          bool                         `gorm:"column:is_featured;not null;default:false"`
	THCPercent          *float64                     `gorm:"column:thc_percent;type:numeric(5,2)"`
	CBDPercent          *float64                     `gorm:"column:cbd_percent;type:numeric(5,2)"`
	MaxQty              int                          `gorm:"column:max_qty;not null;default:0"`
	Inventory           *InventoryItem               `gorm:"foreignKey:ProductID;constraint:OnDelete:CASCADE"`
	VolumeDiscounts     []ProductVolumeDiscount      `gorm:"foreignKey:ProductID;constraint:OnDelete:CASCADE"`
	Media               []ProductMedia               `gorm:"foreignKey:ProductID;constraint:OnDelete:CASCADE"`
	CreatedAt           time.Time                    `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time                    `gorm:"column:updated_at;autoUpdateTime"`
}
