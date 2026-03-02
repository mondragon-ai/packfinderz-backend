package models

import (
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

// Ad captures the advertiser campaign configuration.
type Ad struct {
	ID               uuid.UUID          `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	StoreID          uuid.UUID          `gorm:"column:store_id;type:uuid;not null;index"`
	Status           enums.AdStatus     `gorm:"column:status;type:ad_status;not null;default:'draft'"`
	Placement        string             `gorm:"column:placement;not null"`
	TargetType       enums.AdTargetType `gorm:"column:target_type;type:ad_target_type;not null"`
	TargetID         uuid.UUID          `gorm:"column:target_id;type:uuid;not null"`
	BidCents         int64              `gorm:"column:bid_cents;not null;default:0"`
	DailyBudgetCents int64              `gorm:"column:daily_budget_cents;not null;default:0"`
	StartsAt         *time.Time         `gorm:"column:starts_at"`
	EndsAt           *time.Time         `gorm:"column:ends_at"`
	Creatives        []AdCreative       `gorm:"foreignKey:AdID;constraint:OnDelete:CASCADE"`
	DailyRollups     []AdDailyRollup    `gorm:"foreignKey:AdID;constraint:OnDelete:CASCADE"`
	CreatedAt        time.Time          `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time          `gorm:"column:updated_at;autoUpdateTime"`
}

// AdCreative ties media + copy to an ad.
type AdCreative struct {
	ID             uuid.UUID  `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	AdID           uuid.UUID  `gorm:"column:ad_id;type:uuid;not null;index"`
	MediaID        *uuid.UUID `gorm:"column:media_id;type:uuid"`
	DestinationURL string     `gorm:"column:destination_url;not null"`
	Headline       *string    `gorm:"column:headline"`
	Body           *string    `gorm:"column:body"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

// AdDailyRollup accumulates the daily impressions/clicks for an ad.
type AdDailyRollup struct {
	ID          uuid.UUID `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	AdID        uuid.UUID `gorm:"column:ad_id;type:uuid;not null;index"`
	Day         time.Time `gorm:"column:day;type:date;not null;index:ad_daily_rollups_ad_day_key,unique"`
	Impressions int64     `gorm:"column:impressions;not null;default:0"`
	Clicks      int64     `gorm:"column:clicks;not null;default:0"`
	SpendCents  int64     `gorm:"column:spend_cents;not null;default:0"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}
