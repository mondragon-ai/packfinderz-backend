package users

import (
	"context"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository exposes user-related persistence operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a users repo bound to the provided GORM DB.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new user and returns the persisted model.
func (r *Repository) Create(ctx context.Context, dto CreateUserDTO) (*models.User, error) {
	user := dto.ToModel()
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// FindByEmail retrieves the user matching the provided email.
func (r *Repository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID loads a user by their UUID.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateLastLogin refreshes the user's last_login_at timestamp.
func (r *Repository) UpdateLastLogin(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		UpdateColumn("last_login_at", at).Error
}

// UpdateStoreIDs overwrites the user's store_ids array.
func (r *Repository) UpdateStoreIDs(ctx context.Context, id uuid.UUID, storeIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		UpdateColumn("store_ids", storeIDs).Error
}
