package product

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
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
  WHERE ma.entity_type = 'store_logo' AND ma.entity_id = s.id
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

func (r *Repository) ListProductMediaIDs(ctx context.Context, productID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	if err := r.db.WithContext(ctx).
		Model(&models.ProductMedia{}).
		Where("product_id = ? AND media_id IS NOT NULL", productID).
		Pluck("media_id", &ids).
		Error; err != nil {
		return nil, err
	}
	return ids, nil
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

type productListQuery struct {
	Pagination     pagination.Params
	Filters        ProductListFilters
	RequestedState string
	VendorStoreID  *uuid.UUID
}

func (r *Repository) ListProductSummaries(ctx context.Context, query productListQuery) (*ProductListResult, error) {
	pageSize := pagination.NormalizeLimit(query.Pagination.Limit)
	limitWithBuffer := pagination.LimitWithBuffer(query.Pagination.Limit)
	if limitWithBuffer <= pageSize {
		limitWithBuffer = pageSize + 1
	}

	cursor, err := pagination.ParseCursor(query.Pagination.Cursor)
	if err != nil {
		return nil, err
	}

	promoExistsClause := "EXISTS (SELECT 1 FROM product_volume_discounts d WHERE d.product_id = p.id)"

	qb := r.db.WithContext(ctx).
		Table("products p").
		Select(strings.Join([]string{
			"p.id",
			"p.sku",
			"p.title",
			"p.subtitle",
			"p.category",
			"p.classification",
			"p.unit",
			"p.price_cents",
			"p.compare_at_price_cents",
			"p.thc_percent",
			"p.cbd_percent",
			"p.created_at",
			"p.updated_at",
			"p.store_id",
			"p.max_qty",
			promoExistsClause + " AS has_promo",
		}, ", ")).
		Joins("JOIN stores s ON s.id = p.store_id")

	filter := query.Filters
	if filter.Category != nil {
		qb = qb.Where("p.category = ?", *filter.Category)
	}
	if filter.Classification != nil {
		qb = qb.Where("p.classification = ?", *filter.Classification)
	}
	if filter.PriceMinCents != nil {
		qb = qb.Where("p.price_cents >= ?", *filter.PriceMinCents)
	}
	if filter.PriceMaxCents != nil {
		qb = qb.Where("p.price_cents <= ?", *filter.PriceMaxCents)
	}
	if filter.THCMin != nil {
		qb = qb.Where("p.thc_percent >= ?", *filter.THCMin)
	}
	if filter.THCMax != nil {
		qb = qb.Where("p.thc_percent <= ?", *filter.THCMax)
	}
	if filter.CBDMin != nil {
		qb = qb.Where("p.cbd_percent >= ?", *filter.CBDMin)
	}
	if filter.CBDMax != nil {
		qb = qb.Where("p.cbd_percent <= ?", *filter.CBDMax)
	}
	if filter.HasPromo != nil {
		if *filter.HasPromo {
			qb = qb.Where(promoExistsClause)
		} else {
			qb = qb.Where("NOT " + promoExistsClause)
		}
	}
	if search := strings.TrimSpace(filter.Query); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		qb = qb.Where("(LOWER(p.title) LIKE ? OR LOWER(p.sku) LIKE ?)", pattern, pattern)
	}

	if query.VendorStoreID != nil {
		qb = qb.Where("p.store_id = ?", *query.VendorStoreID)
	} else {
		qb = qb.Where("s.type = ?", enums.StoreTypeVendor)
		qb = qb.Where("s.kyc_status = ?", enums.KYCStatusVerified)
		qb = qb.Where("s.subscription_active = ?", true)
		qb = qb.Where("p.is_active = ?", true)
		if query.RequestedState != "" {
			qb = qb.Where("(s.address).state = ?", query.RequestedState)
		}
	}

	if cursor != nil {
		qb = qb.Where("(p.created_at < ?) OR (p.created_at = ? AND p.id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}

	qb = qb.Order("p.created_at DESC").Order("p.id DESC").Limit(limitWithBuffer)

	var records []productSummaryRecord
	if err := qb.Scan(&records).Error; err != nil {
		return nil, err
	}

	resultRows := records
	nextCursor := ""
	if len(records) > pageSize {
		resultRows = records[:pageSize]
		last := resultRows[len(resultRows)-1]
		nextCursor = pagination.EncodeCursor(pagination.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}

	summaries := make([]ProductSummary, 0, len(resultRows))
	for _, record := range resultRows {
		summaries = append(summaries, record.toSummary())
	}

	return &ProductListResult{
		Products:   summaries,
		NextCursor: nextCursor,
	}, nil
}

type productSummaryRecord struct {
	ID                  uuid.UUID
	SKU                 string
	Title               string
	Subtitle            sql.NullString
	Category            string
	Classification      sql.NullString
	Unit                string
	PriceCents          int
	CompareAtPriceCents sql.NullInt64
	THCPercent          sql.NullFloat64
	CBDPercent          sql.NullFloat64
	HasPromo            bool
	StoreID             uuid.UUID
	CreatedAt           time.Time
	UpdatedAt           time.Time
	MaxQty              int
}

func (r productSummaryRecord) toSummary() ProductSummary {
	return ProductSummary{
		ID:                  r.ID,
		SKU:                 r.SKU,
		Title:               r.Title,
		Subtitle:            nullStringPtr(r.Subtitle),
		Category:            r.Category,
		Classification:      nullStringPtr(r.Classification),
		Unit:                r.Unit,
		PriceCents:          r.PriceCents,
		CompareAtPriceCents: nullIntPtr(r.CompareAtPriceCents),
		THCPercent:          nullFloatPtr(r.THCPercent),
		CBDPercent:          nullFloatPtr(r.CBDPercent),
		HasPromo:            r.HasPromo,
		VendorStoreID:       r.StoreID,
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
		MaxQty:              r.MaxQty,
	}
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func nullIntPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func nullFloatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
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
