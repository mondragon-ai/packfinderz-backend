package stores

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
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
	if err := tx.First(&store, "id = ?", id).Error; err != nil {
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
