package ads

import (
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
)

// AdCreativeDTO mirrors the creative payload returned to clients.
type AdCreativeDTO struct {
	ID             uuid.UUID  `json:"id"`
	MediaID        *uuid.UUID `json:"media_id,omitempty"`
	DestinationURL string     `json:"destination_url"`
	Headline       *string    `json:"headline,omitempty"`
	Body           *string    `json:"body,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// AdDTO represents the persisted ad row returned by controllers or services.
type AdDTO struct {
	ID               uuid.UUID          `json:"id"`
	StoreID          uuid.UUID          `json:"store_id"`
	Status           enums.AdStatus     `json:"status"`
	Placement        enums.AdPlacement  `json:"placement"`
	TargetType       enums.AdTargetType `json:"target_type"`
	TargetID         uuid.UUID          `json:"target_id"`
	BidCents         int64              `json:"bid_cents"`
	DailyBudgetCents int64              `json:"daily_budget_cents"`
	StartsAt         *time.Time         `json:"starts_at,omitempty"`
	EndsAt           *time.Time         `json:"ends_at,omitempty"`
	Creatives        []AdCreativeDTO    `json:"creatives"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

// AdPagination mirrors the canonical cursor pagination metadata used across the API.
type AdPagination struct {
	Page    int    `json:"page"`
	Total   int    `json:"total"`
	Current string `json:"current,omitempty"`
	First   string `json:"first,omitempty"`
	Last    string `json:"last,omitempty"`
	Prev    string `json:"prev,omitempty"`
	Next    string `json:"next,omitempty"`
}

// AdListResult wraps a page of ads plus pagination metadata.
type AdListResult struct {
	Ads        []AdDTO      `json:"ads"`
	Pagination AdPagination `json:"pagination"`
}

// AdCreativeInput carries the fields required to build an ad creative.
type AdCreativeInput struct {
	MediaID        *uuid.UUID `json:"media_id,omitempty"`
	DestinationURL string     `json:"destination_url"`
	Headline       *string    `json:"headline,omitempty"`
	Body           *string    `json:"body,omitempty"`
}

// CreateAdInput contains the data needed to persist a new ad and its creatives.
type CreateAdInput struct {
	StoreID          uuid.UUID          `json:"store_id"`
	Status           enums.AdStatus     `json:"status"`
	Placement        enums.AdPlacement  `json:"placement"`
	TargetType       enums.AdTargetType `json:"target_type"`
	TargetID         uuid.UUID          `json:"target_id"`
	BidCents         int64              `json:"bid_cents"`
	DailyBudgetCents int64              `json:"daily_budget_cents"`
	StartsAt         *time.Time         `json:"starts_at,omitempty"`
	EndsAt           *time.Time         `json:"ends_at,omitempty"`
	Creatives        []AdCreativeInput  `json:"creatives"`
}

// GetAdInput encapsulates the IDs required to fetch a single ad detail row.
type GetAdInput struct {
	StoreID uuid.UUID `json:"store_id"`
	AdID    uuid.UUID `json:"ad_id"`
}

// ListAdsFilters exposes the supported filters for store-scoped ad listings.
type ListAdsFilters struct {
	Status     *enums.AdStatus     `json:"status,omitempty"`
	Placement  *enums.AdPlacement  `json:"placement,omitempty"`
	TargetType *enums.AdTargetType `json:"target_type,omitempty"`
	TargetID   *uuid.UUID          `json:"target_id,omitempty"`
}

// ListAdsInput captures the inputs required to paginate store ads.
type ListAdsInput struct {
	StoreID    uuid.UUID         `json:"store_id"`
	Filters    ListAdsFilters    `json:"filters"`
	Pagination pagination.Params `json:"pagination"`
	Page       int               `json:"page"`
}
