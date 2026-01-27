package notifications

import (
	"context"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository exposes persistence helpers for notifications.
type Repository interface {
	WithTx(tx *gorm.DB) Repository
	Create(ctx context.Context, notification *models.Notification) error
	List(ctx context.Context, params listNotificationsParams) ([]models.Notification, *pagination.Cursor, error)
	MarkRead(ctx context.Context, storeID, notificationID uuid.UUID, now time.Time) (notificationMarkResult, error)
	MarkAllRead(ctx context.Context, storeID uuid.UUID, now time.Time) (int64, error)
}

type repositoryImpl struct {
	db *gorm.DB
}

// NewRepository returns a notifications repository bound to the provided database.
func NewRepository(db *gorm.DB) Repository {
	return &repositoryImpl{db: db}
}

type listNotificationsParams struct {
	StoreID    uuid.UUID
	Limit      int
	Cursor     *pagination.Cursor
	UnreadOnly bool
}

type notificationMarkResult struct {
	Updated bool
	Found   bool
}

func (r *repositoryImpl) WithTx(tx *gorm.DB) Repository {
	if tx == nil {
		return r
	}
	return &repositoryImpl{db: tx}
}

func (r *repositoryImpl) Create(ctx context.Context, notification *models.Notification) error {
	return r.db.WithContext(ctx).Create(notification).Error
}

func (r *repositoryImpl) List(ctx context.Context, params listNotificationsParams) ([]models.Notification, *pagination.Cursor, error) {
	limit := pagination.LimitWithBuffer(params.Limit)
	normalized := pagination.NormalizeLimit(params.Limit)
	query := r.db.WithContext(ctx).Model(&models.Notification{}).Where("store_id = ?", params.StoreID)
	if params.UnreadOnly {
		query = query.Where("read_at IS NULL")
	}
	if params.Cursor != nil {
		query = query.Where("(created_at, id) < (?, ?)", params.Cursor.CreatedAt, params.Cursor.ID)
	}

	var notifications []models.Notification
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Find(&notifications).Error; err != nil {
		return nil, nil, err
	}

	if len(notifications) > normalized {
		next := notifications[normalized]
		notifications = notifications[:normalized]
		return notifications, &pagination.Cursor{CreatedAt: next.CreatedAt, ID: next.ID}, nil
	}
	return notifications, nil, nil
}

func (r *repositoryImpl) MarkRead(ctx context.Context, storeID, notificationID uuid.UUID, now time.Time) (notificationMarkResult, error) {
	result := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("id = ? AND store_id = ? AND read_at IS NULL", notificationID, storeID).
		UpdateColumn("read_at", now)
	if result.Error != nil {
		return notificationMarkResult{}, result.Error
	}

	mark := notificationMarkResult{Updated: result.RowsAffected > 0}
	if result.RowsAffected > 0 {
		mark.Found = true
		return mark, nil
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("id = ? AND store_id = ?", notificationID, storeID).
		Count(&count).Error; err != nil {
		return notificationMarkResult{}, err
	}
	mark.Found = count > 0
	return mark, nil
}

func (r *repositoryImpl) MarkAllRead(ctx context.Context, storeID uuid.UUID, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("store_id = ? AND read_at IS NULL", storeID).
		UpdateColumn("read_at", now)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
