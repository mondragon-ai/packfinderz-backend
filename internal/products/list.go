package product

import (
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
)

// ProductListFilters describe the supported filter knobs for the browse endpoint.
type ProductListFilters struct {
	Category       *enums.ProductCategory       `json:"category,omitempty"`
	Classification *enums.ProductClassification `json:"classification,omitempty"`
	THCMin         *float64                     `json:"thc_min,omitempty"`
	THCMax         *float64                     `json:"thc_max,omitempty"`
	CBDMin         *float64                     `json:"cbd_min,omitempty"`
	CBDMax         *float64                     `json:"cbd_max,omitempty"`
	PriceMinCents  *int                         `json:"price_min_cents,omitempty"`
	PriceMaxCents  *int                         `json:"price_max_cents,omitempty"`
	HasPromo       *bool                        `json:"has_promo,omitempty"`
	Query          string                       `json:"q,omitempty"`
}

// ListProductsInput captures the inputs needed to paginate/filter products for a store.
type ListProductsInput struct {
	StoreID        uuid.UUID
	StoreType      enums.StoreType
	RequestedState string
	Filters        ProductListFilters
	Pagination     pagination.Params
	Page           int
}
