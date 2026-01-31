package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// Store represents the canonical tenant model.
type Store struct {
	ID                   uuid.UUID            `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Type                 enums.StoreType      `gorm:"column:type;type:store_type;not null"`
	CompanyName          string               `gorm:"column:company_name;not null"`
	DBAName              *string              `gorm:"column:dba_name"`
	Description          *string              `gorm:"column:description"`
	Phone                *string              `gorm:"column:phone"`
	Email                *string              `gorm:"column:email"`
	KYCStatus            enums.KYCStatus      `gorm:"column:kyc_status;type:kyc_status;not null;default:'pending_verification'"`
	SubscriptionActive   bool                 `gorm:"column:subscription_active;not null;default:false"`
	DeliveryRadiusMeters int                  `gorm:"column:delivery_radius_meters;not null;default:0"`
	Address              types.Address        `gorm:"column:address;type:address_t;not null"`
	Geom                 types.GeographyPoint `gorm:"column:geom;type:geography(Point,4326);not null"` // REMOVE THIS
	Social               *types.Social        `gorm:"column:social;type:social_t"`
	BannerURL            *string              `gorm:"column:banner_url"`
	LogoURL              *string              `gorm:"column:logo_url"`
	Ratings              types.Ratings        `gorm:"column:ratings;type:jsonb"`
	Categories           pq.StringArray       `gorm:"column:categories;type:text[]"`
	OwnerID              uuid.UUID            `gorm:"column:owner;type:uuid;not null"`
	LastActiveAt         *time.Time           `gorm:"column:last_active_at"`
	CreatedAt            time.Time            `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt            time.Time            `gorm:"column:updated_at;autoUpdateTime"`
}

// Add These as an array to the store.
// type StoreMedia struct {
// 	GCS     string
// 	URL     string
// 	MediaId uuid.UUID
// }
