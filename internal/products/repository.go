package product

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProductRepository defines CRUD operations for product listings.
type ProductRepository interface {
	CreateProduct(context.Context, *models.Product) (*models.Product, error)
	UpdateProduct(context.Context, *models.Product) (*models.Product, error)
	DeleteProduct(context.Context, uuid.UUID) error
	GetProductDetail(context.Context, uuid.UUID) (*models.Product, *VendorSummary, error)
	ListProductsByStore(context.Context, uuid.UUID) ([]models.Product, error)
}

// InventoryRepository defines persistence operations for inventory items.
type InventoryRepository interface {
	UpsertInventory(context.Context, *models.InventoryItem) (*models.InventoryItem, error)
	GetInventoryByProductID(context.Context, uuid.UUID) (*models.InventoryItem, error)
}

// DiscountRepository exposes volume discount persistence.
type DiscountRepository interface {
	CreateVolumeDiscount(context.Context, *models.ProductVolumeDiscount) (*models.ProductVolumeDiscount, error)
	ListVolumeDiscounts(context.Context, uuid.UUID) ([]models.ProductVolumeDiscount, error)
	DeleteVolumeDiscount(context.Context, uuid.UUID) error
}

// VendorSummary exposes the minimal store data used by product read paths.
type VendorSummary struct {
	StoreID     uuid.UUID
	CompanyName string
	LogoMediaID *uuid.UUID
	LogoGCSKey  *string
}

const vendorSummaryQuery = `
SELECT s.id AS store_id,
       s.company_name,
       logo.logo_media_id,
       logo.logo_gcs_key
FROM stores s
LEFT JOIN LATERAL (
  SELECT ma.media_id AS logo_media_id,
         ma.gcs_key AS logo_gcs_key
  FROM media_attachments ma
  WHERE ma.entity_type = 'store' AND ma.entity_id = s.id
  ORDER BY ma.created_at DESC
  LIMIT 1
) logo ON true
WHERE s.id = ?
`

// Repository wires together all product-related persistence helpers.
type Repository struct {
	db *gorm.DB
}

// NewRepository builds a repository tied to the provided GORM DB.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// WithTx returns a repository bound to the provided transaction.
func (r *Repository) WithTx(tx *gorm.DB) *Repository {
	return &Repository{db: tx}
}

// FindByID loads the product without associations.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	var product models.Product
	if err := r.db.WithContext(ctx).First(&product, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

// ReplaceVolumeDiscounts replaces all volume discounts for the product.
func (r *Repository) ReplaceVolumeDiscounts(ctx context.Context, productID uuid.UUID, tiers []models.ProductVolumeDiscount) error {
	tx := r.db.WithContext(ctx)
	if err := tx.Where("product_id = ?", productID).Delete(&models.ProductVolumeDiscount{}).Error; err != nil {
		return err
	}
	if len(tiers) == 0 {
		return nil
	}
	return tx.Create(&tiers).Error
}

// ReplaceProductMedia replaces media attachments for the product.
func (r *Repository) ReplaceProductMedia(ctx context.Context, productID uuid.UUID, media []models.ProductMedia) error {
	tx := r.db.WithContext(ctx)
	if err := tx.Where("product_id = ?", productID).Delete(&models.ProductMedia{}).Error; err != nil {
		return err
	}
	if len(media) == 0 {
		return nil
	}
	return tx.Create(&media).Error
}

// CreateProduct inserts a new product row.
func (r *Repository) CreateProduct(ctx context.Context, product *models.Product) (*models.Product, error) {
	if err := r.db.WithContext(ctx).Create(product).Error; err != nil {
		return nil, err
	}
	return product, nil
}

// UpdateProduct updates an existing product row.
func (r *Repository) UpdateProduct(ctx context.Context, product *models.Product) (*models.Product, error) {
	if err := r.db.WithContext(ctx).Save(product).Error; err != nil {
		return nil, err
	}
	return product, nil
}

// DeleteProduct removes a product by ID.
func (r *Repository) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Product{}).Error
}

// GetProductDetail fetches a product with inventory, discounts, media, and vendor summary.
func (r *Repository) GetProductDetail(ctx context.Context, id uuid.UUID) (*models.Product, *VendorSummary, error) {
	var product models.Product
	err := r.db.WithContext(ctx).
		Preload("Inventory").
		Preload("VolumeDiscounts", func(db *gorm.DB) *gorm.DB {
			return db.Order("min_qty DESC")
		}).
		Preload("Media", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		First(&product, "id = ?", id).
		Error
	if err != nil {
		return nil, nil, err
	}

	summary, err := r.fetchVendorSummary(ctx, product.StoreID)
	if err != nil {
		return &product, nil, err
	}
	return &product, summary, nil
}

// ListProductsByStore lists the products owned by a store with preloaded relations.
func (r *Repository) ListProductsByStore(ctx context.Context, storeID uuid.UUID) ([]models.Product, error) {
	var rows []models.Product
	err := r.db.WithContext(ctx).
		Preload("Inventory").
		Preload("VolumeDiscounts", func(db *gorm.DB) *gorm.DB {
			return db.Order("min_qty DESC")
		}).
		Preload("Media", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Where("store_id = ?", storeID).
		Order("created_at DESC").
		Find(&rows).
		Error
	return rows, err
}

// UpsertInventory creates or updates the inventory row for a product.
func (r *Repository) UpsertInventory(ctx context.Context, item *models.InventoryItem) (*models.InventoryItem, error) {
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

// GetInventoryByProductID returns the inventory row for the provided product.
func (r *Repository) GetInventoryByProductID(ctx context.Context, productID uuid.UUID) (*models.InventoryItem, error) {
	var item models.InventoryItem
	if err := r.db.WithContext(ctx).First(&item, "product_id = ?", productID).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateVolumeDiscount inserts a tiered pricing entry.
func (r *Repository) CreateVolumeDiscount(ctx context.Context, discount *models.ProductVolumeDiscount) (*models.ProductVolumeDiscount, error) {
	if err := r.db.WithContext(ctx).Create(discount).Error; err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err,
			fmt.Sprintf("insert volume discount (product_id=%s min_qty=%d)", discount.ProductID, discount.MinQty),
		)
	}
	return discount, nil
}

// ListVolumeDiscounts returns all tiers for a product ordered by min_qty descending.
func (r *Repository) ListVolumeDiscounts(ctx context.Context, productID uuid.UUID) ([]models.ProductVolumeDiscount, error) {
	var rows []models.ProductVolumeDiscount
	err := r.db.WithContext(ctx).
		Where("product_id = ?", productID).
		Order("min_qty DESC").
		Find(&rows).
		Error
	return rows, err
}

// DeleteVolumeDiscount removes a discount row by ID.
func (r *Repository) DeleteVolumeDiscount(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&models.ProductVolumeDiscount{}).
		Error
}

func (r *Repository) fetchVendorSummary(ctx context.Context, storeID uuid.UUID) (*VendorSummary, error) {
	type vendorRow struct {
		StoreID     uuid.UUID
		CompanyName string
		LogoMediaID sql.NullString
		LogoGCSKey  sql.NullString
	}

	var row vendorRow
	if err := r.db.WithContext(ctx).Raw(vendorSummaryQuery, storeID).Scan(&row).Error; err != nil {
		return nil, err
	}

	summary := VendorSummary{
		StoreID:     row.StoreID,
		CompanyName: row.CompanyName,
	}

	if row.LogoMediaID.Valid {
		parsed, err := uuid.Parse(row.LogoMediaID.String)
		if err != nil {
			return nil, err
		}
		summary.LogoMediaID = &parsed
	}

	if row.LogoGCSKey.Valid {
		summary.LogoGCSKey = &row.LogoGCSKey.String
	}

	return &summary, nil
}
