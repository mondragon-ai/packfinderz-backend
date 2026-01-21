package memberships

import (
	"context"
	"fmt"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository exposes membership persistence operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository binds the repo to the provided GORM connection.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ListUserStores returns the stores a user belongs to along with membership metadata.
func (r *Repository) ListUserStores(ctx context.Context, userID uuid.UUID) ([]MembershipWithStore, error) {
	var rows []membershipWithStoreRow

	err := r.db.WithContext(ctx).
		Model(&models.StoreMembership{}).
		Select("store_memberships.*, stores.company_name AS store_name, stores.type AS store_type").
		Joins("JOIN stores ON stores.id = store_memberships.store_id").
		Where("store_memberships.user_id = ?", userID).
		Order("stores.company_name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return membershipRowsToDTO(rows), nil
}

// GetMembership retrieves a membership by user and store.
func (r *Repository) GetMembership(ctx context.Context, userID, storeID uuid.UUID) (*models.StoreMembership, error) {
	var membership models.StoreMembership
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND store_id = ?", userID, storeID).
		First(&membership).Error
	if err != nil {
		return nil, err
	}
	return &membership, nil
}

// CreateMembership persists a new membership record.
func (r *Repository) CreateMembership(ctx context.Context, storeID, userID uuid.UUID, role enums.MemberRole, invitedBy *uuid.UUID, status enums.MembershipStatus) (*models.StoreMembership, error) {
	if !role.IsValid() {
		return nil, fmt.Errorf("invalid member role %q", role)
	}
	if !status.IsValid() {
		return nil, fmt.Errorf("invalid membership status %q", status)
	}

	membership := &models.StoreMembership{
		StoreID:         storeID,
		UserID:          userID,
		Role:            role,
		Status:          status,
		InvitedByUserID: invitedBy,
	}

	if err := r.db.WithContext(ctx).Create(membership).Error; err != nil {
		return nil, err
	}
	return membership, nil
}

// UserHasRole reports whether the user holds one of the provided roles for the store.
func (r *Repository) UserHasRole(ctx context.Context, userID, storeID uuid.UUID, roles ...enums.MemberRole) (bool, error) {
	if len(roles) == 0 {
		return false, nil
	}

	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.StoreMembership{}).
		Where("user_id = ? AND store_id = ? AND role IN ?", userID, storeID, roles).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetMembershipWithStore returns membership details joined with store metadata.
func (r *Repository) GetMembershipWithStore(ctx context.Context, userID, storeID uuid.UUID) (*MembershipWithStore, error) {
	var row membershipWithStoreRow
	err := r.db.WithContext(ctx).
		Model(&models.StoreMembership{}).
		Select("store_memberships.*, stores.company_name AS store_name, stores.type AS store_type").
		Joins("JOIN stores ON stores.id = store_memberships.store_id").
		Where("store_memberships.user_id = ? AND store_memberships.store_id = ?", userID, storeID).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	dto := membershipWithStoreFromRow(row)
	return &dto, nil
}

// ListStoreUsers returns memberships for the store along with user metadata.
func (r *Repository) ListStoreUsers(ctx context.Context, storeID uuid.UUID) ([]StoreUserDTO, error) {
	var rows []storeUserRow
	err := r.db.WithContext(ctx).
		Model(&models.StoreMembership{}).
		Select("store_memberships.*, users.email, users.first_name, users.last_name, users.last_login_at").
		Joins("JOIN users ON users.id = store_memberships.user_id").
		Where("store_memberships.store_id = ?", storeID).
		Order("store_memberships.created_at").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return storeUsersFromRows(rows), nil
}
