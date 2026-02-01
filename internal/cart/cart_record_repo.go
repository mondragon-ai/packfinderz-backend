package cart

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

// CartRecordSnapshot captures the authoritative values to persist for a buyer's active cart.
type CartRecordSnapshot struct {
	BuyerStoreID    uuid.UUID
	ShippingAddress *types.Address
	CheckoutGroupID *uuid.UUID
	Currency        enums.Currency
	ValidUntil      *time.Time
	SubtotalCents   int
	DiscountsCents  int
	TotalCents      int
	AdTokens        []string
}

// CartRecordRepository encapsulates cart record persistence.
type CartRecordRepository struct {
	db *gorm.DB
}

// NewCartRecordRepository binds the repository to the provided GORM handle.
func NewCartRecordRepository(db *gorm.DB) *CartRecordRepository {
	return &CartRecordRepository{db: db}
}

// WithTx scopes the repository to the provided transaction.
func (r *CartRecordRepository) WithTx(tx *gorm.DB) *CartRecordRepository {
	if tx == nil {
		return r
	}
	return &CartRecordRepository{db: tx}
}

// FindActiveByBuyerStore returns the latest active cart for the buyer store.
func (r *CartRecordRepository) FindActiveByBuyerStore(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	var record models.CartRecord
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("VendorGroups").
		Where("buyer_store_id = ? AND status = ?", buyerStoreID, enums.CartStatusActive).
		Order("created_at DESC").
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// FindByIDAndBuyerStore returns the cart record belonging to the buyer store.
func (r *CartRecordRepository) FindByIDAndBuyerStore(ctx context.Context, id, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	var record models.CartRecord
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("VendorGroups").
		Where("id = ? AND buyer_store_id = ?", id, buyerStoreID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// Create inserts the provided cart record.
func (r *CartRecordRepository) Create(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	if record.Status == "" {
		record.Status = enums.CartStatusActive
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// Update saves the provided cart record.
func (r *CartRecordRepository) Update(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	if err := r.db.WithContext(ctx).Save(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// UpdateStatus updates the status for the specified cart.
func (r *CartRecordRepository) UpdateStatus(ctx context.Context, id, buyerStoreID uuid.UUID, status enums.CartStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.CartRecord{}).
		Where("id = ? AND buyer_store_id = ?", id, buyerStoreID).
		Update("status", status).Error
}

// SaveAuthoritativeSnapshot creates or updates the buyer's active cart with the provided snapshot.
func (r *CartRecordRepository) SaveAuthoritativeSnapshot(ctx context.Context, snapshot CartRecordSnapshot) (*models.CartRecord, error) {
	record, err := r.FindActiveByBuyerStore(ctx, snapshot.BuyerStoreID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = &models.CartRecord{
			BuyerStoreID:    snapshot.BuyerStoreID,
			Status:          enums.CartStatusActive,
			ShippingAddress: snapshot.ShippingAddress,
			CheckoutGroupID: snapshot.CheckoutGroupID,
			Currency:        currencyOrDefault(snapshot.Currency),
			ValidUntil:      validUntilOrDefault(snapshot.ValidUntil),
			SubtotalCents:   snapshot.SubtotalCents,
			DiscountsCents:  snapshot.DiscountsCents,
			TotalCents:      snapshot.TotalCents,
			AdTokens:        pq.StringArray(snapshot.AdTokens),
		}
		if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
			return nil, err
		}
		return record, nil
	}

	applySnapshot(record, snapshot)
	if err := r.db.WithContext(ctx).Save(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func applySnapshot(record *models.CartRecord, snapshot CartRecordSnapshot) {
	record.Status = enums.CartStatusActive
	record.ShippingAddress = snapshot.ShippingAddress
	record.CheckoutGroupID = snapshot.CheckoutGroupID
	if snapshot.Currency.IsValid() {
		record.Currency = snapshot.Currency
	} else if record.Currency == "" {
		record.Currency = enums.CurrencyUSD
	}
	record.ValidUntil = validUntilOrDefault(snapshot.ValidUntil)
	record.SubtotalCents = snapshot.SubtotalCents
	record.DiscountsCents = snapshot.DiscountsCents
	record.TotalCents = snapshot.TotalCents
	record.AdTokens = pq.StringArray(snapshot.AdTokens)
}

func currencyOrDefault(currency enums.Currency) enums.Currency {
	if currency.IsValid() {
		return currency
	}
	return enums.CurrencyUSD
}

func validUntilOrDefault(value *time.Time) time.Time {
	if value != nil && !value.IsZero() {
		return *value
	}
	return time.Now().Add(15 * time.Minute)
}
