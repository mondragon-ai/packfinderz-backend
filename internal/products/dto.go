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
	Vendor              VendorSummaryDTO    `json:"vendor"`
	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
}

// InventoryDTO exposes inventory counts.
type InventoryDTO struct {
	AvailableQty int       `json:"available_qty"`
	ReservedQty  int       `json:"reserved_qty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// VolumeDiscountDTO represents a tiered unit price.
type VolumeDiscountDTO struct {
	ID             uuid.UUID `json:"id"`
	MinQty         int       `json:"min_qty"`
	UnitPriceCents int       `json:"unit_price_cents"`
	CreatedAt      time.Time `json:"created_at"`
}

// ProductMediaDTO captures product media metadata.
type ProductMediaDTO struct {
	ID        uuid.UUID `json:"id"`
	URL       *string   `json:"url,omitempty"`
	GCSKey    string    `json:"gcs_key"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
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
	}
	if product.Classification != nil {
		classification := string(*product.Classification)
		dto.Classification = &classification
	}

	if product.Inventory != nil {
		dto.Inventory = &InventoryDTO{
			AvailableQty: product.Inventory.AvailableQty,
			ReservedQty:  product.Inventory.ReservedQty,
			UpdatedAt:    product.Inventory.UpdatedAt,
		}
	}

	if len(product.VolumeDiscounts) > 0 {
		dto.VolumeDiscounts = make([]VolumeDiscountDTO, len(product.VolumeDiscounts))
		for i, tier := range product.VolumeDiscounts {
			dto.VolumeDiscounts[i] = VolumeDiscountDTO{
				ID:             tier.ID,
				MinQty:         tier.MinQty,
				UnitPriceCents: tier.UnitPriceCents,
				CreatedAt:      tier.CreatedAt,
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
				Position:  pm.Position,
				CreatedAt: pm.CreatedAt,
			}
		}
	}

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
