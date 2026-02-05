package stores

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository handles store persistence.
type Repository struct {
	db *gorm.DB
}

// NewRepository binds a GORM DB to store operations.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create persists a new store row.
func (r *Repository) Create(ctx context.Context, dto CreateStoreDTO) (*models.Store, error) {
	store := dto.ToModel()
	if err := r.db.WithContext(ctx).Create(store).Error; err != nil {
		return nil, err
	}
	return store, nil
}

// FindByID loads a store by its UUID.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*models.Store, error) {
	var store models.Store
	if err := r.db.WithContext(ctx).
		Omit("geom").
		Where("id = ?", id).
		First(&store).Error; err != nil {
		return nil, err
	}
	return &store, nil
}

// FindByOwner returns all stores owned by the provided user.
func (r *Repository) FindByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.Store, error) {
	var stores []models.Store
	if err := r.db.WithContext(ctx).Where("owner = ?", ownerID).Find(&stores).Error; err != nil {
		return nil, err
	}
	return stores, nil
}

// SquareCustomerID returns the stored Square customer identifier for the given store.
func (r *Repository) SquareCustomerID(ctx context.Context, storeID uuid.UUID) (*string, error) {
	var store models.Store
	if err := r.db.WithContext(ctx).
		Select("square_customer_id").
		Where("id = ?", storeID).
		First(&store).Error; err != nil {
		return nil, err
	}
	return store.SquareCustomerID, nil
}

// UpdateSquareCustomerID sets the Square customer identifier for the provided store.
func (r *Repository) UpdateSquareCustomerID(ctx context.Context, storeID uuid.UUID, customerID *string) error {
	if err := r.db.WithContext(ctx).
		Model(&models.Store{}).
		Where("id = ?", storeID).
		Update("square_customer_id", customerID).Error; err != nil {
		return err
	}
	return nil
}

// Update saves the provided store.
func (r *Repository) Update(ctx context.Context, store *models.Store) error {
	if store == nil {
		return fmt.Errorf("store is required")
	}
	return r.db.WithContext(ctx).Save(store).Error
}

// FindByIDWithTx loads a store using the provided transaction.
func (r *Repository) FindByIDWithTx(tx *gorm.DB, id uuid.UUID) (*models.Store, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidTransaction
	}
	var store models.Store
	if err := tx.Omit("geom").First(&store, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &store, nil
}

// UpdateWithTx persists the store using the provided transaction.
func (r *Repository) UpdateWithTx(tx *gorm.DB, store *models.Store) error {
	if tx == nil {
		return gorm.ErrInvalidTransaction
	}
	if store == nil {
		return fmt.Errorf("store is required")
	}
	return tx.Save(store).Error
}

// UpdateStatusWithTx persists the store using the provided transaction & mod status.
func (r *Repository) UpdateStatusWithTx(tx *gorm.DB, storeID uuid.UUID, newStatus enums.KYCStatus) error {
	if tx == nil {
		return gorm.ErrInvalidTransaction
	}
	if err := tx.Model(&models.Store{}).
		Where("id = ?", storeID).
		Update("kyc_status", newStatus).Error; err != nil {
		return err
	}
	return nil
}
