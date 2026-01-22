package outbox

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Insert(tx *gorm.DB, event models.OutboxEvent) error {
	if tx == nil {
		return errors.New("transaction required")
	}
	return tx.Create(&event).Error
}

func (r *Repository) FetchUnpublished(limit int) ([]models.OutboxEvent, error) {
	var rows []models.OutboxEvent
	err := r.db.Where("published_at IS NULL").
		Order("created_at ASC").
		Order("id ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *Repository) MarkPublished(id uuid.UUID) error {
	return r.db.Model(&models.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"published_at": time.Now(),
		}).Error
}

func (r *Repository) MarkFailed(id uuid.UUID, err error) error {
	return r.db.Model(&models.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_error":    err.Error(),
			"attempt_count": gorm.Expr("attempt_count + 1"),
		}).Error
}
