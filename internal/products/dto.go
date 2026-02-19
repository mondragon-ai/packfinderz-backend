package product

import (
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/google/uuid"
)

// ProductDTO represents the vendor product payload returned to clients.
type ProductDTO struct {
	ID                  uuid.UUID           `json:"id"`
	SKU                 string              `json:"sku"`
	Title               string              `json:"title"`
	Subtitle            *string             `json:"subtitle,omitempty"`
	BodyHTML            *string             `json:"body_html,omitempty"`
	Category            string              `json:"category"`
	Feelings            []string            `json:"feelings"`
	Flavors             []string            `json:"flavors"`
	Usage               []string            `json:"usage"`
	Strain              *string             `json:"strain,omitempty"`
	Classification      *string             `json:"classification,omitempty"`
	Unit                string              `json:"unit"`
	MOQ                 int                 `json:"moq"`
	PriceCents          int                 `json:"price_cents"`
	CompareAtPriceCents *int                `json:"compare_at_price_cents,omitempty"`
	IsActive            bool                `json:"is_active"`
	IsFeatured          bool                `json:"is_featured"`
	THCPercent          *float64            `json:"thc_percent,omitempty"`
	CBDPercent          *float64            `json:"cbd_percent,omitempty"`
	Inventory           *InventoryDTO       `json:"inventory,omitempty"`
	VolumeDiscounts     []VolumeDiscountDTO `json:"volume_discounts,omitempty"`
	Media               []ProductMediaDTO   `json:"media,omitempty"`
	COAMediaID          *uuid.UUID          `json:"coa_media_id,omitempty"`
	COAReadURL          *string             `json:"coa_read_url,omitempty"`
	Vendor              VendorSummaryDTO    `json:"vendor"`
	MaxQty              int                 `json:"max_qty"`
	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
}

// ProductSummary captures the lightweight product payload returned by listing endpoints.
type ProductSummary struct {
	ID                  uuid.UUID `json:"id"`
	SKU                 string    `json:"sku"`
	Title               string    `json:"title"`
	Subtitle            *string   `json:"subtitle,omitempty"`
	Category            string    `json:"category"`
	Classification      *string   `json:"classification,omitempty"`
	Unit                string    `json:"unit"`
	PriceCents          int       `json:"price_cents"`
	CompareAtPriceCents *int      `json:"compare_at_price_cents,omitempty"`
	THCPercent          *float64  `json:"thc_percent,omitempty"`
	CBDPercent          *float64  `json:"cbd_percent,omitempty"`
	HasPromo            bool      `json:"has_promo"`
	VendorStoreID       uuid.UUID `json:"vendor_store_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	MaxQty              int       `json:"max_qty"`
	ThumbnailURL        *string   `json:"thumbnail_url,omitempty"`
}

// ProductListResult wraps a page of product summaries plus the cursor for the next page.
type ProductPagination struct {
	Page    int    `json:"page"`
	Total   int    `json:"total"`
	Current string `json:"current,omitempty"`
	First   string `json:"first,omitempty"`
	Last    string `json:"last,omitempty"`
	Prev    string `json:"prev,omitempty"`
	Next    string `json:"next,omitempty"`
}

// ProductListResult wraps a page of product summaries plus pagination metadata.
type ProductListResult struct {
	Products   []ProductSummary  `json:"products"`
	Pagination ProductPagination `json:"pagination"`
}

// InventoryDTO exposes inventory counts.
type InventoryDTO struct {
	AvailableQty      int       `json:"available_qty"`
	ReservedQty       int       `json:"reserved_qty"`
	UpdatedAt         time.Time `json:"updated_at"`
	LowStockThreshold int       `json:"low_stock_threshold"`
}

// VolumeDiscountDTO represents a tiered unit price.
type VolumeDiscountDTO struct {
	ID              uuid.UUID `json:"id"`
	MinQty          int       `json:"min_qty"`
	DiscountPercent float64   `json:"discount_percent"`
	CreatedAt       time.Time `json:"created_at"`
}

// ProductMediaDTO captures product media metadata.
type ProductMediaDTO struct {
	ID        uuid.UUID  `json:"id"`
	URL       *string    `json:"url,omitempty"`
	GCSKey    string     `json:"gcs_key"`
	MediaID   *uuid.UUID `json:"media_id,omitempty"`
	Position  int        `json:"position"`
	CreatedAt time.Time  `json:"created_at"`
}

// VendorSummaryDTO surfaces limited store data for product responses.
type VendorSummaryDTO struct {
	StoreID     uuid.UUID  `json:"store_id"`
	CompanyName string     `json:"company_name"`
	LogoMediaID *uuid.UUID `json:"logo_media_id,omitempty"`
	LogoGCSKey  *string    `json:"logo_gcs_key,omitempty"`
}

// NewProductDTO builds a DTO from the persisted model and vendor summary.
func NewProductDTO(product *models.Product, summary *VendorSummary) *ProductDTO {
	dto := &ProductDTO{
		ID:                  product.ID,
		SKU:                 product.SKU,
		Title:               product.Title,
		Subtitle:            product.Subtitle,
		BodyHTML:            product.BodyHTML,
		Category:            string(product.Category),
		Feelings:            append([]string{}, product.Feelings...),
		Flavors:             append([]string{}, product.Flavors...),
		Usage:               append([]string{}, product.Usage...),
		Strain:              product.Strain,
		Unit:                string(product.Unit),
		MOQ:                 product.MOQ,
		PriceCents:          product.PriceCents,
		CompareAtPriceCents: product.CompareAtPriceCents,
		IsActive:            product.IsActive,
		IsFeatured:          product.IsFeatured,
		THCPercent:          product.THCPercent,
		CBDPercent:          product.CBDPercent,
		CreatedAt:           product.CreatedAt,
		UpdatedAt:           product.UpdatedAt,
		MaxQty:              product.MaxQty,
	}
	if product.Classification != nil {
		classification := string(*product.Classification)
		dto.Classification = &classification
	}

	if product.Inventory != nil {
		dto.Inventory = &InventoryDTO{
			AvailableQty:      product.Inventory.AvailableQty,
			ReservedQty:       product.Inventory.ReservedQty,
			UpdatedAt:         product.Inventory.UpdatedAt,
			LowStockThreshold: product.Inventory.LowStockThreshold,
		}
	}

	if len(product.VolumeDiscounts) > 0 {
		dto.VolumeDiscounts = make([]VolumeDiscountDTO, len(product.VolumeDiscounts))
		for i, tier := range product.VolumeDiscounts {
			dto.VolumeDiscounts[i] = VolumeDiscountDTO{
				ID:              tier.ID,
				MinQty:          tier.MinQty,
				DiscountPercent: tier.DiscountPercent,
				CreatedAt:       tier.CreatedAt,
			}
		}
	}

	if len(product.Media) > 0 {
		dto.Media = make([]ProductMediaDTO, len(product.Media))
		for i, pm := range product.Media {
			dto.Media[i] = ProductMediaDTO{
				ID:        pm.ID,
				URL:       pm.URL,
				GCSKey:    pm.GCSKey,
				MediaID:   pm.MediaID,
				Position:  pm.Position,
				CreatedAt: pm.CreatedAt,
			}
		}
	}
	dto.COAMediaID = product.COAMediaID

	if summary != nil {
		dto.Vendor = VendorSummaryDTO{
			StoreID:     summary.StoreID,
			CompanyName: summary.CompanyName,
			LogoMediaID: summary.LogoMediaID,
			LogoGCSKey:  summary.LogoGCSKey,
		}
	}

	return dto
}
