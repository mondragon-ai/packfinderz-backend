package wishlist

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	products "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const promoExistsClause = "EXISTS (SELECT 1 FROM product_volume_discounts d WHERE d.product_id = p.id)"

// Repository encapsulates wishlist persistence.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a wishlist repository bound to the provided gorm DB.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// AddItem inserts a wishlist entry and ignores duplicates.
func (r *Repository) AddItem(ctx context.Context, storeID, productID uuid.UUID) error {
	if storeID == uuid.Nil || productID == uuid.Nil {
		return gorm.ErrInvalidValue
	}

	return r.db.WithContext(ctx).
		Exec(`INSERT INTO wishlist_items (store_id, product_id) VALUES (?, ?) ON CONFLICT (store_id, product_id) DO NOTHING`, storeID, productID).
		Error
}

// RemoveItem deletes the store-product like if it exists.
func (r *Repository) RemoveItem(ctx context.Context, storeID, productID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("store_id = ? AND product_id = ?", storeID, productID).
		Delete(&models.WishlistItem{}).
		Error
}

// ListItems returns a paginated list of wishlist products for a store.
func (r *Repository) ListItems(ctx context.Context, storeID uuid.UUID, cursor string, limit int) (WishlistItemsPageDTO, error) {
	normalizedLimit := pagination.NormalizeLimit(limit)
	limitWithBuffer := pagination.LimitWithBuffer(limit)
	cursorValue := strings.TrimSpace(cursor)
	decodedCursor, err := pagination.ParseCursor(cursorValue)
	if err != nil {
		return WishlistItemsPageDTO{}, err
	}

	selectColumns := []string{
		"wi.id AS wishlist_id",
		"wi.created_at AS wishlist_created_at",
		"p.id AS product_id",
		"p.sku",
		"p.title",
		"p.subtitle",
		"p.category",
		"p.classification",
		"p.unit",
		"p.moq",
		"p.price_cents",
		"p.compare_at_price_cents",
		"p.thc_percent",
		"p.cbd_percent",
		"p.created_at AS product_created_at",
		"p.updated_at AS product_updated_at",
		"p.store_id AS vendor_store_id",
		"p.max_qty",
		promoExistsClause + " AS has_promo",
		"p.coa_added AS coa_added",
		"pm_thumb.thumbnail_url AS thumbnail_url",
	}

	dataQuery := r.db.WithContext(ctx).
		Table("wishlist_items wi").
		Select(strings.Join(selectColumns, ", ")).
		Joins("JOIN products p ON p.id = wi.product_id").
		Joins(`LEFT JOIN LATERAL (
  SELECT COALESCE(pm.url, m.public_url) AS thumbnail_url
  FROM product_media pm
  LEFT JOIN media m ON pm.media_id = m.id
  WHERE pm.product_id = p.id
  ORDER BY pm.position ASC, pm.created_at ASC
  LIMIT 1
) pm_thumb ON true`).
		Where("wi.store_id = ?", storeID)

	if decodedCursor != nil {
		dataQuery = dataQuery.Where("(wi.created_at < ?) OR (wi.created_at = ? AND wi.id < ?)", decodedCursor.CreatedAt, decodedCursor.CreatedAt, decodedCursor.ID)
	}

	dataQuery = dataQuery.Order("wi.created_at DESC").Order("wi.id DESC").Limit(limitWithBuffer)

	var records []wishlistProductSummaryRecord
	if err := dataQuery.Scan(&records).Error; err != nil {
		return WishlistItemsPageDTO{}, err
	}

	resultRows := records
	nextCursor := ""
	if len(records) > normalizedLimit {
		resultRows = records[:normalizedLimit]
		last := resultRows[len(resultRows)-1]
		nextCursor = pagination.EncodeCursor(pagination.Cursor{
			CreatedAt: last.WishlistCreatedAt,
			ID:        last.WishlistID,
		})
	}

	items := make([]WishlistItemDTO, 0, len(resultRows))
	for _, record := range resultRows {
		items = append(items, record.toDTO())
	}

	totalCount, err := r.countWishlistItems(ctx, storeID)
	if err != nil {
		return WishlistItemsPageDTO{}, err
	}
	firstCursor, err := r.fetchWishlistBoundaryCursor(ctx, storeID, true)
	if err != nil {
		return WishlistItemsPageDTO{}, err
	}
	lastCursor, err := r.fetchWishlistBoundaryCursor(ctx, storeID, false)
	if err != nil {
		return WishlistItemsPageDTO{}, err
	}

	prevCursor := ""
	if cursorValue != "" {
		prevCursor = cursorValue
	}

	paginationMeta := products.ProductPagination{
		Page:    1,
		Total:   int(totalCount),
		Current: cursorValue,
		First:   firstCursor,
		Last:    lastCursor,
		Prev:    prevCursor,
		Next:    nextCursor,
	}

	return WishlistItemsPageDTO{
		Items:      items,
		Pagination: paginationMeta,
	}, nil
}

// ListItemIDs returns only the product IDs a store has liked.
func (r *Repository) ListItemIDs(ctx context.Context, storeID uuid.UUID, cursor string, limit int) (WishlistIDsDTO, error) {
	normalizedLimit := pagination.NormalizeLimit(limit)
	limitWithBuffer := pagination.LimitWithBuffer(limit)
	cursorValue := strings.TrimSpace(cursor)
	decodedCursor, err := pagination.ParseCursor(cursorValue)
	if err != nil {
		return WishlistIDsDTO{}, err
	}

	query := r.db.WithContext(ctx).
		Model(&models.WishlistItem{}).
		Select("id AS wishlist_id", "created_at AS wishlist_created_at", "product_id").
		Where("store_id = ?", storeID)

	if decodedCursor != nil {
		query = query.Where("(created_at < ?) OR (created_at = ? AND id < ?)", decodedCursor.CreatedAt, decodedCursor.CreatedAt, decodedCursor.ID)
	}

	query = query.Order("created_at DESC").Order("id DESC").Limit(limitWithBuffer)

	type idRecord struct {
		WishlistID        uuid.UUID
		WishlistCreatedAt time.Time
		ProductID         uuid.UUID
	}

	var records []idRecord
	if err := query.Scan(&records).Error; err != nil {
		return WishlistIDsDTO{}, err
	}

	resultRows := records
	nextCursor := ""
	if len(records) > normalizedLimit {
		resultRows = records[:normalizedLimit]
		last := resultRows[len(resultRows)-1]
		nextCursor = pagination.EncodeCursor(pagination.Cursor{
			CreatedAt: last.WishlistCreatedAt,
			ID:        last.WishlistID,
		})
	}

	items := make([]uuid.UUID, 0, len(resultRows))
	for _, record := range resultRows {
		items = append(items, record.ProductID)
	}

	totalCount, err := r.countWishlistItems(ctx, storeID)
	if err != nil {
		return WishlistIDsDTO{}, err
	}
	firstCursor, err := r.fetchWishlistBoundaryCursor(ctx, storeID, true)
	if err != nil {
		return WishlistIDsDTO{}, err
	}
	lastCursor, err := r.fetchWishlistBoundaryCursor(ctx, storeID, false)
	if err != nil {
		return WishlistIDsDTO{}, err
	}

	prevCursor := ""
	if cursorValue != "" {
		prevCursor = cursorValue
	}

	paginationMeta := products.ProductPagination{
		Page:    1,
		Total:   int(totalCount),
		Current: cursorValue,
		First:   firstCursor,
		Last:    lastCursor,
		Prev:    prevCursor,
		Next:    nextCursor,
	}

	return WishlistIDsDTO{
		ProductIDs: items,
		Pagination: paginationMeta,
	}, nil
}

func (r *Repository) countWishlistItems(ctx context.Context, storeID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.WishlistItem{}).
		Where("store_id = ?", storeID).
		Count(&count).
		Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) fetchWishlistBoundaryCursor(ctx context.Context, storeID uuid.UUID, ascending bool) (string, error) {
	order := "created_at DESC, id DESC"
	if ascending {
		order = "created_at ASC, id ASC"
	}

	var row struct {
		CreatedAt time.Time
		ID        uuid.UUID
	}

	query := r.db.WithContext(ctx).
		Model(&models.WishlistItem{}).
		Select("created_at", "id").
		Where("store_id = ?", storeID).
		Order(order).
		Limit(1)

	if err := query.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}

	return pagination.EncodeCursor(pagination.Cursor{
		CreatedAt: row.CreatedAt,
		ID:        row.ID,
	}), nil
}

type wishlistProductSummaryRecord struct {
	WishlistID          uuid.UUID       `gorm:"column:wishlist_id"`
	WishlistCreatedAt   time.Time       `gorm:"column:wishlist_created_at"`
	ID                  uuid.UUID       `gorm:"column:product_id"`
	SKU                 string          `gorm:"column:sku"`
	Title               string          `gorm:"column:title"`
	Subtitle            sql.NullString  `gorm:"column:subtitle"`
	Category            string          `gorm:"column:category"`
	Classification      sql.NullString  `gorm:"column:classification"`
	Unit                string          `gorm:"column:unit"`
	MOQ                 int             `gorm:"column:moq"`
	PriceCents          int             `gorm:"column:price_cents"`
	CompareAtPriceCents sql.NullInt64   `gorm:"column:compare_at_price_cents"`
	THCPercent          sql.NullFloat64 `gorm:"column:thc_percent"`
	CBDPercent          sql.NullFloat64 `gorm:"column:cbd_percent"`
	HasPromo            bool            `gorm:"column:has_promo"`
	VendorStoreID       uuid.UUID       `gorm:"column:vendor_store_id"`
	COAAdded            bool            `gorm:"column:coa_added"`
	CreatedAt           time.Time       `gorm:"column:product_created_at"`
	UpdatedAt           time.Time       `gorm:"column:product_updated_at"`
	ThumbnailURL        sql.NullString  `gorm:"column:thumbnail_url"`
	MaxQty              int             `gorm:"column:max_qty"`
}

func (r wishlistProductSummaryRecord) toDTO() WishlistItemDTO {
	return WishlistItemDTO{
		Product:   r.toSummary(),
		CreatedAt: r.WishlistCreatedAt,
	}
}

func (r wishlistProductSummaryRecord) toSummary() products.ProductSummary {
	return products.ProductSummary{
		ID:                  r.ID,
		SKU:                 r.SKU,
		Title:               r.Title,
		Subtitle:            nullStringPtr(r.Subtitle),
		Category:            r.Category,
		Classification:      nullStringPtr(r.Classification),
		Unit:                r.Unit,
		MOQ:                 r.MOQ,
		PriceCents:          r.PriceCents,
		CompareAtPriceCents: nullIntPtr(r.CompareAtPriceCents),
		THCPercent:          nullFloatPtr(r.THCPercent),
		CBDPercent:          nullFloatPtr(r.CBDPercent),
		HasPromo:            r.HasPromo,
		VendorStoreID:       r.VendorStoreID,
		COAAdded:            r.COAAdded,
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
		MaxQty:              r.MaxQty,
		ThumbnailURL:        nullStringPtr(r.ThumbnailURL),
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
