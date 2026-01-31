package outbox

import (
	"context"
	"errors"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const maxDLQErrorLen = 1024

type DLQRepository struct {
	db *gorm.DB
}

func NewDLQRepository(db *gorm.DB) *DLQRepository {
	return &DLQRepository{db: db}
}

func (r *DLQRepository) InsertTx(tx *gorm.DB, entry models.OutboxDLQ) error {
	if tx == nil {
		return errors.New("transaction required")
	}
	if entry.ErrorMessage != nil {
		msg := truncateDLQError(*entry.ErrorMessage)
		entry.ErrorMessage = &msg
	}
	return tx.Create(&entry).Error
}

func (r *DLQRepository) FindByEventID(ctx context.Context, eventID uuid.UUID) (*models.OutboxDLQ, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var dlq models.OutboxDLQ
	err := r.db.WithContext(ctx).Where("event_id = ?", eventID).First(&dlq).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &dlq, nil
}

func (r *DLQRepository) List(ctx context.Context, limit int) ([]models.OutboxDLQ, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 50
	}
	var rows []models.OutboxDLQ
	err := r.db.WithContext(ctx).
		Order("failed_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func truncateDLQError(message string) string {
	if len(message) <= maxDLQErrorLen {
		return message
	}
	return message[:maxDLQErrorLen]
}
